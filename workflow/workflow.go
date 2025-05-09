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
	ID             string // 工作流ID (Workflow ID)
	graph          gograph.Graph[string]
	nodes          map[string]*Node // 节点ID到节点的映射 (Mapping from node ID to node)
	edges          []Edge           // 工作流中的所有边 (All edges in the workflow)
	mutex          sync.RWMutex
	isCompiled     bool     // 标记工作流是否已编译 (Flag to indicate if the workflow is compiled)
	executionOrder []string // 节点执行顺序（拓扑排序结果） (Node execution order (result of topological sort))
	entryNodeID    string   // 入口节点ID（没有父节点的节点） (Entry node ID (node without parents))
}

// Edge 表示工作流中的一条边
// Edge represents an edge in the workflow
type Edge struct {
	From string
	To   string
}

// NewWorkflow 创建一个新的工作流
// NewWorkflow creates a new workflow
func NewWorkflow(id string) *Workflow {
	return &Workflow{
		ID:          id,
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
		return w.NewError("cannot modify workflow after compilation")
	}

	// 检查节点ID是否已存在
	// Check if the node ID already exists
	if _, exists := w.nodes[node.ID]; exists {
		return w.NewError(fmt.Sprintf("node with ID %s already exists", node.ID))
	}

	// 将节点添加到图中
	w.nodes[node.ID] = node

	return nil
}

// AddNodeFunc 添加一个节点到工作流中（简化版，直接接收ID和函数）
// AddNodeFunc adds a node to the workflow (simplified version, directly accepts ID and function)
func (w *Workflow) AddNodeFunc(id string, fn NodeFunc) error {
	// 内部创建Node对象 (Internally create Node object)
	node := NewNode(w, id, fn)
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
		return w.NewError("cannot modify workflow after compilation")
	}

	// 检查节点是否存在
	// Check if the nodes exist
	fromNode, fromExists := w.nodes[fromNodeID]
	if !fromExists {
		return w.NewError(fmt.Sprintf("source node %s does not exist", fromNodeID))
	}

	toNode, toExists := w.nodes[toNodeID]
	if !toExists {
		return w.NewError(fmt.Sprintf("target node %s does not exist", toNodeID))
	}

	// 创建从源到目标的边
	// Create edge from source to target
	fromVertex := gograph.NewVertex(fromNode.ID)
	toVertex := gograph.NewVertex(toNode.ID)

	_, err := w.graph.AddEdge(fromVertex, toVertex)
	if err != nil {
		return w.NewError(fmt.Sprintf("failed to add edge from %s to %s: %v", fromNodeID, toNodeID, err))
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
		return w.NewError("workflow is already compiled")
	}

	// 如果没有节点，返回错误
	// If there are no nodes, return an error
	if len(w.nodes) == 0 {
		return w.NewError("workflow has no nodes")
	}

	// 检查DAG结构并获取拓扑排序
	// Check DAG structure and get topological sort
	iterator, err := traverse.NewTopologicalIterator(w.graph)
	if err != nil {
		return w.NewError(fmt.Sprintf("invalid DAG structure: %v", err))
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
		return w.NewError("failed to determine execution order")
	}

	// 找出入口节点（没有父节点的节点）
	// Find the entry node (node without parents)
	var entryNodeID string
	for _, nodeID := range executionOrder {
		node := w.nodes[nodeID]
		if len(node.Parents) == 0 {
			if entryNodeID != "" {
				return w.NewError("workflow has multiple entry points, expected only one")
			}
			entryNodeID = nodeID
		}
	}

	// 如果没有入口节点，表示有问题
	// If there is no entry node, there is a problem
	if entryNodeID == "" {
		return w.NewError("workflow has no entry point")
	}

	// 收集所有边信息
	// Collect all edge information
	var edges []Edge
	for nodeID, node := range w.nodes {
		for _, childID := range node.Children {
			edges = append(edges, Edge{
				From: nodeID,
				To:   childID,
			})
		}
	}
	w.edges = edges

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
func (w *Workflow) Execute(ctx context.Context, input interface{}) (*WorkflowContext, error) {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	if !w.isCompiled {
		return nil, w.NewError("workflow must be compiled before execution")
	}

	// 创建执行上下文
	// Create execution context
	execCtx := NewWorkflowContext(w, input)

	// 将传入的ctx设置为上下文的父级
	// Set the passed ctx as the parent of the context
	execCtx.Ctx, execCtx.Cancel = context.WithCancel(ctx)

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

// NewError 创建一个工作流错误，可选包含节点ID
// NewError creates a workflow error, optionally containing a node ID
func (w *Workflow) NewError(message string, nodeID ...string) error {
	err := &WorkflowError{
		WorkflowID: w.ID,
		Message:    message,
	}

	if len(nodeID) > 0 && nodeID[0] != "" {
		err.NodeID = nodeID[0]
	}

	return err
}

// GetAllNodes 返回工作流中的所有节点
// GetAllNodes returns all nodes in the workflow
func (w *Workflow) GetAllNodes() map[string]*Node {
	return w.nodes
}

// GetAllEdges 返回工作流中的所有边
// GetAllEdges returns all edges in the workflow
func (w *Workflow) GetAllEdges() []Edge {
	return w.edges
}

// GetEntryNodeID 返回工作流的入口节点ID
// GetEntryNodeID returns the entry node ID of the workflow
func (w *Workflow) GetEntryNodeID() string {
	return w.entryNodeID
}
