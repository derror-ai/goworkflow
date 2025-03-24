package workflow

import (
	"context"
	"fmt"
)

// NodeFunc 是工作流节点的通用函数定义
// 每个节点接收 context、原始输入数据、父节点结果，返回结果、信号和错误
// NodeFunc is the general function definition for workflow nodes
// Each node receives context, original input data, parent node results, and returns result, signal, and error
type NodeFunc func(ctx context.Context, req interface{}, parentResult interface{}) (interface{}, Signal, error)

// Node 表示工作流中的一个节点
// Node represents a node in the workflow
type Node struct {
	ID         string   // 节点唯一标识 (Node unique identifier)
	Func       NodeFunc // 节点执行的函数 (Function executed by the node)
	Parents    []string // 父节点ID列表 (List of parent node IDs)
	Children   []string // 子节点ID列表 (List of child node IDs)
	DirectCall bool     // 标记是否可以直接调用（不使用协程） (Flag to mark if it can be called directly (without goroutine))
}

// NewNode 创建一个新的工作流节点
// NewNode creates a new workflow node
func NewNode(id string, fn NodeFunc) *Node {
	return &Node{
		ID:         id,
		Func:       fn,
		Parents:    []string{},
		Children:   []string{},
		DirectCall: false, // 默认使用协程执行 (Default to execute using goroutine)
	}
}

// Execute 执行节点函数
// Execute executes the node function
func (n *Node) Execute(ctx context.Context, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
	// 尝试从上下文中获取workflowID
	// Try to get workflowID from context
	var workflowID string
	if wfID, ok := ctx.Value(workflowIDKey).(string); ok {
		workflowID = wfID
	} else {
		workflowID = "unknown"
	}

	// 直接执行节点函数并返回结果
	// Directly execute node function and return result
	result, signal, err := n.Func(ctx, req, parentResult)
	if err != nil {
		return nil, nil, NewWorkflowError(workflowID, fmt.Sprintf("execution error: %v", err), n.ID)
	}

	// 处理信号
	// Process signal
	if signal != nil {
		switch s := signal.(type) {
		case SelectNodeSignal:
			// 为条件分发信号添加节点和工作流信息
			// Add node and workflow information to condition dispatch signal
			s.NodeID = n.ID
			s.WorkflowID = workflowID
			return result, s, nil
		}
	}

	return result, nil, nil
}

// SelectNodes 辅助函数，用于节点内部选择性触发后续节点
// 节点函数可以调用 return SelectNodes(result, []string{"nodeA", "nodeB"}) 来指定下一步执行的节点
// SelectNodes helper function for nodes to selectively trigger subsequent nodes
// Node functions can call return SelectNodes(result, []string{"nodeA", "nodeB"}) to specify the next nodes to execute
func SelectNodes(result interface{}, targetNodeIDs []string) (interface{}, Signal, error) {
	// 创建条件分发信号 (Create conditional dispatch signal)
	return result, NewSelectNodeSignal(targetNodeIDs), nil
}
