package workflow

import (
	"context"
	"sync"
	"time"
)

// 节点状态常量
// Node state constants
const (
	NodeStateCreated   = "created"   // 节点初始创建状态 (Node initial created state)
	NodeStateCompleted = "completed" // 节点已完成状态 (Node completed state)
	NodeStateCanceled  = "canceled"  // 节点已取消状态 (Node canceled state)
)

// NodeContext 包含节点的所有相关信息和状态
// NodeContext contains all relevant information and state of the node
type NodeContext struct {
	Ctx              context.Context
	WorkflowCtx      *WorkflowContext // 工作流上下文 (Workflow context)
	NodeID           string           // 节点的唯一标识 (Node unique identifier)
	Node             *Node            // 节点的引用 (Reference to the node)
	state            string           // 节点状态："created"、"completed"或"canceled"，私有字段 (Node state: "created", "completed" or "canceled", private field)
	Result           interface{}      // 节点执行结果 (Node execution result)
	Input            interface{}      // 节点输入数据 (Node input data)
	Error            error            // 节点执行错误 (Node execution error)
	StartTime        time.Time        // 节点开始执行时间 (Node execution start time)
	EndTime          time.Time        // 节点结束执行时间 (Node execution end time)
	ElapsedTime      time.Duration    // 节点执行耗时 (Node execution elapsed time)
	IsStarted        bool             // 节点是否已启动 (Whether the node has started)
	StartSessionID   string           // 节点启动会话ID (Node start session ID)
	IsCompleted      bool             // 节点是否已完成 (Whether the node has completed)
	IsCanceled       bool             // 节点是否已取消 (Whether the node has been canceled)
	SelectedChildren []string         // 存储节点选择的子节点ID列表，用于条件分发 (Stores list of child node IDs selected by the node, used for conditional dispatching)
	mutex            sync.RWMutex     // 保护此节点上下文的锁 (Lock to protect this node context)
}

// NewNodeContext 创建一个新的节点上下文实例
// NewNodeContext creates a new node context instance
func NewNodeContext(workflowCtx *WorkflowContext, nodeID string, node *Node) *NodeContext {
	return &NodeContext{
		Ctx:         workflowCtx.Ctx,
		WorkflowCtx: workflowCtx,
		NodeID:      nodeID,
		Node:        node,
		state:       NodeStateCreated,
		// 其他字段保持默认零值
		// Other fields keep default zero values
	}
}

// GetState 获取节点状态
// GetState gets the node state
func (nc *NodeContext) GetState() string {
	nc.mutex.RLock()
	defer nc.mutex.RUnlock()
	return nc.state
}

// SetState 设置节点状态
// SetState sets the node state
func (nc *NodeContext) SetState(state string) {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()
	nc.state = state
}

// SetResult 设置节点结果
// SetResult sets the node result
func (nc *NodeContext) SetResult(result interface{}) {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()
	nc.Result = result
}

// GetResult 获取节点结果
// GetResult gets the node result
func (nc *NodeContext) GetResult() interface{} {
	nc.mutex.RLock()
	defer nc.mutex.RUnlock()
	return nc.Result
}

// SetStarted 标记节点为已启动
// SetStarted marks the node as started
func (nc *NodeContext) SetStarted(startTime time.Time, sessionID string) bool {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()

	if nc.IsStarted || sessionID != "" {
		return false
	}

	nc.IsStarted = true
	nc.StartTime = startTime
	nc.StartSessionID = sessionID
	// 节点启动时，不再是创建状态，但也不是完成状态
	// 因此只需要修改IsStarted标志，保持State不变
	// When the node starts, it is no longer in the created state, but not yet in the completed state
	// Therefore, only the IsStarted flag needs to be modified, keeping the State unchanged

	return true
}

// SetCompleted 标记节点为已完成
// SetCompleted marks the node as completed
func (nc *NodeContext) SetCompleted(endTime time.Time) {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()
	nc.IsCompleted = true
	nc.EndTime = endTime
	nc.ElapsedTime = endTime.Sub(nc.StartTime)
	nc.state = NodeStateCompleted
}

// SetCanceled 标记节点为已取消
// SetCanceled marks the node as canceled
func (nc *NodeContext) SetCanceled() {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()
	nc.IsCanceled = true
	nc.IsCompleted = true
	nc.state = NodeStateCanceled
}

// SetError 设置节点错误
// SetError sets the node error
func (nc *NodeContext) SetError(err error) {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()
	nc.Error = err
}

// IsActive 检查节点是否已启动但未完成
// IsActive checks if the node has started but not completed
func (nc *NodeContext) IsActive() bool {
	nc.mutex.RLock()
	defer nc.mutex.RUnlock()
	return nc.IsStarted && !nc.IsCompleted
}

// SetSelectedChildren 设置选中的子节点列表
// SetSelectedChildren sets the list of selected child nodes
func (nc *NodeContext) SetSelectedChildren(childrenIDs []string) {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()
	nc.SelectedChildren = childrenIDs
}

// HasSelectedChildren 检查是否使用了条件分发
// HasSelectedChildren checks if conditional dispatching has been used
func (nc *NodeContext) HasSelectedChildren() bool {
	nc.mutex.RLock()
	defer nc.mutex.RUnlock()
	// SelectedChildren被设置过（即使是空数组）也表示使用了条件分发
	// SelectedChildren being set (even if it's an empty array) indicates that conditional dispatching has been used
	return nc.SelectedChildren != nil
}

// IsChildSelected 检查特定子节点是否被选中
// IsChildSelected checks if a specific child node is selected
func (nc *NodeContext) IsChildSelected(childID string) bool {
	nc.mutex.RLock()
	defer nc.mutex.RUnlock()

	// 如果没有设置条件分发，则所有子节点都被选中
	// If conditional dispatching is not set, all child nodes are selected
	if nc.SelectedChildren == nil {
		return true
	}

	// 如果是空数组，表示不选择任何子节点
	// If it's an empty array, it means no child nodes are selected
	if len(nc.SelectedChildren) == 0 {
		return false
	}

	// 检查子节点是否在选中列表中
	// Check if the child node is in the selected list
	for _, selectedID := range nc.SelectedChildren {
		if selectedID == childID {
			return true
		}
	}
	return false
}
