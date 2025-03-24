package workflow

import (
	"context"
	"fmt"
	"sync"

	"github.com/hmdsefi/gograph"
	"github.com/hmdsefi/gograph/traverse"
)

// Workflow 表示一个工作流，内部使用 DAG 结构存储节点和边
// Workflow represents a workflow, internally using DAG structure to store nodes and edges
type Workflow struct {
	name           string
	graph          gograph.Graph[string]
	nodes          map[string]*Node // 节点ID到节点的映射 (Mapping from node ID to node)
	mutex          sync.RWMutex
	isCompiled     bool     // 标记工作流是否已编译 (Flag to indicate if the workflow is compiled)
	executionOrder []string // 节点执行顺序（拓扑排序结果） (Node execution order (result of topological sort))
	entryNodeID    string   // 入口节点ID（没有父节点的节点） (Entry node ID (node without parents))
}

// NewWorkflow 创建一个新的工作流
// NewWorkflow creates a new workflow
func NewWorkflow(name string) *Workflow {
	return &Workflow{
		name:        name,
		graph:       gograph.New[string](gograph.Directed(), gograph.Acyclic()),
		nodes:       make(map[string]*Node),
		isCompiled:  false,
		entryNodeID: "",
	}
}

// AddNode 添加一个节点到工作流中
// AddNode adds a node to the workflow
func (w *Workflow) AddNode(node *Node) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 检查工作流是否已编译
	// Check if the workflow is already compiled
	if w.isCompiled {
		return NewWorkflowError(w.name, "cannot modify workflow after compilation")
	}

	// 检查节点ID是否已存在
	// Check if the node ID already exists
	if _, exists := w.nodes[node.ID]; exists {
		return NewWorkflowError(w.name, fmt.Sprintf("node with ID %s already exists", node.ID))
	}

	// 将节点添加到图中
	// Add the node to the graph
	vertex := gograph.NewVertex(node.ID)
	w.graph.AddVertex(vertex)
	w.nodes[node.ID] = node

	return nil
}

// AddNodeFunc 添加一个节点到工作流中（简化版，直接接收ID和函数）
// AddNodeFunc adds a node to the workflow (simplified version, directly accepts ID and function)
func (w *Workflow) AddNodeFunc(id string, fn NodeFunc) error {
	// 内部创建Node对象 (Internally create Node object)
	node := NewNode(id, fn)
	return w.AddNode(node)
}

// AddEdge 添加一条从源节点到目标节点的边
// AddEdge adds an edge from source node to target node
func (w *Workflow) AddEdge(fromNodeID, toNodeID string) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 检查工作流是否已编译
	// Check if the workflow is already compiled
	if w.isCompiled {
		return NewWorkflowError(w.name, "cannot modify workflow after compilation")
	}

	// 检查节点是否存在
	// Check if the nodes exist
	fromNode, fromExists := w.nodes[fromNodeID]
	if !fromExists {
		return NewWorkflowError(w.name, fmt.Sprintf("source node %s does not exist", fromNodeID))
	}

	toNode, toExists := w.nodes[toNodeID]
	if !toExists {
		return NewWorkflowError(w.name, fmt.Sprintf("target node %s does not exist", toNodeID))
	}

	// 创建从源到目标的边
	// Create edge from source to target
	fromVertex := gograph.NewVertex(fromNode.ID)
	toVertex := gograph.NewVertex(toNode.ID)

	_, err := w.graph.AddEdge(fromVertex, toVertex)
	if err != nil {
		return NewWorkflowError(w.name, fmt.Sprintf("failed to add edge from %s to %s: %v", fromNodeID, toNodeID, err))
	}

	// 更新节点关系
	// Update node relationships
	fromNode.Children = append(fromNode.Children, toNodeID)
	toNode.Parents = append(toNode.Parents, fromNodeID)

	return nil
}

// Compile 编译工作流，验证DAG结构并冻结工作流
// Compile compiles the workflow, validates the DAG structure and freezes the workflow
func (w *Workflow) Compile() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 检查是否已编译
	// Check if already compiled
	if w.isCompiled {
		return NewWorkflowError(w.name, "workflow is already compiled")
	}

	// 如果没有节点，返回错误
	// If there are no nodes, return an error
	if len(w.nodes) == 0 {
		return NewWorkflowError(w.name, "workflow has no nodes")
	}

	// 检查DAG结构并获取拓扑排序
	// Check DAG structure and get topological sort
	iterator, err := traverse.NewTopologicalIterator(w.graph)
	if err != nil {
		return NewWorkflowError(w.name, fmt.Sprintf("invalid DAG structure: %v", err))
	}

	// 计算拓扑排序结果
	// Calculate topological sort results
	executionOrder := make([]string, 0, len(w.nodes))
	for iterator.HasNext() {
		vertex := iterator.Next()
		executionOrder = append(executionOrder, vertex.Label())
	}

	// 如果获取的执行顺序为空但有节点，表示有问题
	// If the execution order is empty but there are nodes, there is a problem
	if len(executionOrder) == 0 {
		return NewWorkflowError(w.name, "failed to determine execution order")
	}

	// 找出入口节点（没有父节点的节点）
	// Find the entry node (node without parents)
	var entryNodeID string
	for _, nodeID := range executionOrder {
		node := w.nodes[nodeID]
		if len(node.Parents) == 0 {
			if entryNodeID != "" {
				return NewWorkflowError(w.name, "workflow has multiple entry points, expected only one")
			}
			entryNodeID = nodeID
		}
	}

	// 如果没有入口节点，表示有问题
	// If there is no entry node, there is a problem
	if entryNodeID == "" {
		return NewWorkflowError(w.name, "workflow has no entry point")
	}

	// 分析节点之间的关系，设置直接调用标记
	// Analyze relationships between nodes, set direct call flag
	for _, node := range w.nodes {
		// 只有一个子节点的节点可以使用直接调用
		// Nodes with only one child can use direct call
		if len(node.Children) == 1 {
			node.DirectCall = true
		}
	}

	// 保存编译结果
	// Save compilation results
	w.executionOrder = executionOrder
	w.entryNodeID = entryNodeID
	w.isCompiled = true
	return nil
}

// IsCompiled 检查工作流是否已编译
// IsCompiled checks if the workflow is compiled
func (w *Workflow) IsCompiled() bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.isCompiled
}

// Execute 执行完整工作流
// 这是执行工作流的推荐方式，会自动编译工作流（如果尚未编译），
// 创建执行上下文，启动入口节点，并等待所有节点完成
// Execute executes the complete workflow
// This is the recommended way to execute a workflow, it will automatically compile the workflow (if not yet compiled),
// create execution context, start the entry node, and wait for all nodes to complete
func (w *Workflow) Execute(ctx context.Context, input interface{}) (*WorkflowExecutionContext, error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if !w.isCompiled {
		return nil, NewWorkflowError(w.name, "workflow must be compiled before execution")
	}

	// 创建执行上下文
	// Create execution context
	execCtx := NewWorkflowExecutionContext(w, input)

	// 将传入的ctx设置为上下文的父级
	// Set the passed ctx as the parent of the context
	execCtx.Ctx, execCtx.Cancel = context.WithCancel(context.WithValue(ctx, workflowIDKey, w.name))

	// 使用Start方法启动工作流
	// Use Start method to start the workflow
	err := execCtx.Start()
	if err != nil {
		// 如果启动失败，直接返回错误
		// If start fails, return error directly
		return nil, err
	}

	// 等待所有节点完成
	// Wait for all nodes to complete
	err = execCtx.Wait()
	return execCtx, err
}
