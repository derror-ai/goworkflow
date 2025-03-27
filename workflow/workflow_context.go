package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// 工作流状态常量
// Workflow state constants
const (
	WorkflowStateCreated   = "created"   // 工作流初始创建状态 (Workflow initial created state)
	WorkflowStateCompleted = "completed" // 工作流已完成状态 (Workflow completed state)
	WorkflowStateCanceled  = "canceled"  // 工作流已取消状态 (Workflow canceled state)
	WorkflowStateRunning   = "running"   // 工作流运行中状态 (Workflow running state)
	WorkflowStateError     = "error"     // 工作流错误状态 (Workflow error state)
)

// WorkflowContext 封装工作流执行所需的上下文和状态
// WorkflowContext encapsulates the context and state needed for workflow execution
type WorkflowContext struct {
	Workflow     *Workflow
	Ctx          context.Context
	Cancel       context.CancelFunc
	Input        interface{}
	NodeContexts map[string]*NodeContext // 节点ID到节点上下文的映射，在初始化后不再修改 (Mapping from node ID to node context, not modified after initialization)
	// NodeContextMutex 锁移除：因为NodeContexts在初始化后内容不变，不需要锁保护
	// 每个NodeContext实例都有自己的锁来保护其内部状态
	// NodeContextMutex lock removed: because NodeContexts content does not change after initialization, no lock protection needed
	// Each NodeContext instance has its own lock to protect its internal state

	// 工作流节点总数
	// Total number of workflow nodes
	totalNodes int

	// 工作流状态
	// Workflow state
	State       string       // 工作流状态 (Workflow state)
	IsStarted   bool         // 工作流是否已启动 (Whether the workflow has started)
	IsCompleted bool         // 工作流是否已完成 (Whether the workflow has completed)
	IsCanceled  bool         // 工作流是否已取消 (Whether the workflow has been canceled)
	Error       error        // 工作流执行错误 (Workflow execution error)
	stateMutex  sync.RWMutex // 状态相关数据的互斥锁 (Mutex for state-related data)

	// 工作流时间信息
	// Workflow time information
	StartTime   time.Time     // 工作流开始时间 (Workflow start time)
	EndTime     time.Time     // 工作流结束时间 (Workflow end time)
	ElapsedTime time.Duration // 工作流执行耗时 (Workflow execution elapsed time)
	timesMutex  sync.Mutex    // 时间相关数据的互斥锁 (Mutex for time-related data)

	// 节点完成通知通道
	// Node completion notification channel
	nodeCompleteChan chan string
}

// NewWorkflowContext 创建一个新的工作流执行上下文
// NewWorkflowContext creates a new workflow execution context
func NewWorkflowContext(w *Workflow, input interface{}) *WorkflowContext {
	ctx, cancel := context.WithCancel(context.Background())

	now := time.Now()

	// 节点数量
	// Node count
	nodeCount := len(w.nodes)

	workflowCtx := &WorkflowContext{
		Workflow:         w,
		Ctx:              ctx,
		Cancel:           cancel,
		Input:            input,
		totalNodes:       nodeCount,
		State:            WorkflowStateCreated,
		IsStarted:        false,
		IsCompleted:      false,
		IsCanceled:       false,
		Error:            nil,
		StartTime:        now,
		EndTime:          time.Time{},
		nodeCompleteChan: make(chan string, nodeCount*2),
	}

	// 初始化NodeContexts
	// Initialize NodeContexts
	nodeContexts := make(map[string]*NodeContext, nodeCount)
	for nodeID, node := range w.nodes {
		nodeContexts[nodeID] = NewNodeContext(workflowCtx, nodeID, node)
	}

	workflowCtx.NodeContexts = nodeContexts

	// 初始化上下文结构
	// Initialize context structure
	return workflowCtx
}

// getNodeContext 安全地获取节点上下文，如果不存在则返回nil
// getNodeContext safely gets the node context, returns nil if it doesn't exist
func (ec *WorkflowContext) getNodeContext(nodeID string) *NodeContext {
	// NodeContexts在初始化后不再修改，无需加锁
	// NodeContexts is not modified after initialization, no lock needed
	nc, exists := ec.NodeContexts[nodeID]

	if !exists {
		return nil
	}

	return nc
}

// TryStartNode 尝试启动节点，如果所有父节点都已完成则执行该节点
// 如果节点不存在，将返回错误
// TryStartNode attempts to start a node, executes it if all parent nodes have completed
// If the node doesn't exist, an error will be returned
func (ec *WorkflowContext) tryStartNode(nodeID string) {

	// 获取节点上下文
	// Get node context
	nc := ec.getNodeContext(nodeID)
	if nc == nil {
		ec.markWorkflowAsError(nil, ec.Workflow.NewError(fmt.Sprintf("node with ID %s does not exist", nodeID)))
		return
	}

	// 检查工作流是否已完成或上下文是否已取消
	// Check if the workflow has completed or context has been canceled
	workflowState := ec.GetState()

	if workflowState != WorkflowStateCreated && workflowState != WorkflowStateRunning || ec.isContextCancelled() {
		ec.markNodeAsCanceled(nc)
		return
	}

	// 检查节点是否已启动或已取消
	// Check if the node has started or been canceled
	nodeState := nc.GetState()
	if nodeState != NodeStateCreated {
		// 如果节点不是初始状态，则不需要启动
		// If the node is not in the initial state, it doesn't need to be started
		return
	}

	// 检查是否所有父节点都是已取消
	// Check if all parent nodes have been canceled
	allParentsCanceled := len(nc.Node.Parents) > 0 // 只有至少有一个父节点时才考虑 (Only consider when there is at least one parent node)

	// 遍历父节点状态
	// Traverse parent node states
	for _, parentID := range nc.Node.Parents {
		parentNC := ec.getNodeContext(parentID)
		parentState := parentNC.GetState()

		// 如果有任何父节点未完成，则不能启动当前节点
		// If any parent node is not completed, the current node cannot be started
		if !parentNC.IsCompleted {
			return
		}

		// 检查父节点是否使用了条件分发
		// Check if the parent node used conditional dispatching
		if parentNC.HasSelectedChildren() && !parentNC.IsChildSelected(nc.NodeID) {
			// 父节点使用了条件分发但未选择此节点，则取消该节点
			// The parent node used conditional dispatching but did not select this node, so cancel this node
			ec.markNodeAsCanceled(nc)
			return
		}

		// 如果有父节点未取消，则不能直接cancel子节点
		// If any parent node is not canceled, the child node cannot be directly canceled
		if parentState != NodeStateCanceled {
			allParentsCanceled = false
		}
	}

	// 如果所有父节点都已取消，则取消当前节点
	// If all parent nodes have been canceled, cancel the current node
	if allParentsCanceled {
		ec.markNodeAsCanceled(nc)
		return
	}

	// 所有父节点已完成且该节点在条件分发中被选中，标记为已启动
	// All parent nodes have completed and this node has been selected in conditional dispatching, mark as started
	now := time.Now()
	nc.SetStarted(now)

	// 收集所有父节点的结果作为当前节点的输入
	// Collect all parent node results as input for the current node
	nodeInput := ec.collectParentResults(nc)
	nc.Input = nodeInput

	// 检查节点是否可以直接调用
	// Check if the node can be directly called
	if nc.Node.DirectCall {
		// 直接执行节点，传入节点和输入数据
		// Directly execute the node, passing node and input data
		ec.executeNode(nc)
	} else {
		// 启动协程执行节点
		// Start a goroutine to execute the node
		go func() {
			ec.executeNode(nc)
		}()
	}
}

// Start 启动工作流执行，仅启动入口节点
// 这是推荐的工作流启动方法，避免了外部调用者需要知道入口节点ID
// Start starts workflow execution, only starting the entry node
// This is the recommended workflow start method, avoiding the need for external callers to know the entry node ID
func (ec *WorkflowContext) Start() error {
	// 检查工作流是否已编译
	// Check if the workflow has been compiled
	if !ec.Workflow.IsCompiled() {
		return ec.Workflow.NewError("workflow must be compiled before starting")
	}

	// 获取入口节点ID
	// Get entry node ID
	entryNodeID := ec.Workflow.entryNodeID
	if entryNodeID == "" {
		return ec.Workflow.NewError("workflow has no entry point")
	}

	// 更新工作流状态为已启动
	// Update workflow state to started
	ec.setStarted()

	// 启动入口节点
	// Start the entry node
	go func() {
		ec.tryStartNode(entryNodeID)
	}()

	return nil
}

// executeNode 执行节点的核心逻辑，无论是直接调用还是在协程中调用
// executeNode executes the core logic of the node, whether called directly or in a goroutine
func (ec *WorkflowContext) executeNode(nc *NodeContext) {
	// 执行节点函数
	// Execute node function
	result, signal, err := nc.Node.Execute(nc, ec.Input, nc.Input)
	nodeEndTime := time.Now()

	// 标记节点为已完成并存储相关信息
	// Mark the node as completed and store related information
	nc.SetResult(result)
	nc.SetCompleted(nodeEndTime)

	// 先处理错误
	// Handle errors first
	if err != nil {
		ec.markWorkflowAsError(nc, err)
		return
	}

	// 然后处理信号
	// Then process signals
	if signal != nil {
		switch s := signal.(type) {
		case SelectNodeSignal:
			// 条件分发处理 - 直接在节点上下文中保存选择的子节点
			// 即使是空数组，也明确设置，表示使用了条件分发但不选择任何节点
			// Conditional dispatch processing - directly save selected child nodes in node context
			// Even if it's an empty array, set it explicitly, indicating that conditional dispatching is used but no nodes are selected
			nc.SetSelectedChildren(s.TargetIDs)
		}
	}

	// 通知节点完成 - 使用非阻塞发送以防止死锁
	// Notify node completion - use non-blocking send to prevent deadlock
	ec.notifyNodeCompleted(nc.NodeID)
}

// notifyNodeComplete 通知节点完成
// notifyNodeComplete notifies node completion
func (ec *WorkflowContext) notifyNodeCompleted(nodeId string) {
	// 通知节点完成 - 使用非阻塞发送以防止死锁
	// Notify node completion - use non-blocking send to prevent deadlock
	select {
	case ec.nodeCompleteChan <- nodeId:
		// 消息成功发送
		// Message sent successfully
	default:
		// 通道已满或没有接收方，这里不阻塞
		// Channel is full or has no receiver, don't block here
	}
}

// isContextCancelled 检查上下文是否已取消
// isContextCancelled checks if the context has been canceled
func (ec *WorkflowContext) isContextCancelled() bool {
	select {
	case <-ec.Ctx.Done():
		return true
	default:
		return false
	}
}

// setStarted 标记工作流为已启动
// setStarted marks the workflow as started
func (ec *WorkflowContext) setStarted() {
	ec.stateMutex.Lock()
	defer ec.stateMutex.Unlock()
	ec.IsStarted = true
	ec.State = WorkflowStateRunning
}

// setCompleted 标记工作流为已完成
// setCompleted marks the workflow as completed
func (ec *WorkflowContext) setCompleted() {
	ec.stateMutex.Lock()
	defer ec.stateMutex.Unlock()
	ec.IsCompleted = true
	ec.State = WorkflowStateCompleted

	ec.timesMutex.Lock()
	ec.EndTime = time.Now()
	ec.ElapsedTime = ec.EndTime.Sub(ec.StartTime)
	ec.timesMutex.Unlock()
}

// setCanceled 标记工作流为已取消
// setCanceled marks the workflow as canceled
func (ec *WorkflowContext) setCanceled() {
	ec.stateMutex.Lock()
	defer ec.stateMutex.Unlock()
	ec.IsCanceled = true
	ec.IsCompleted = true
	ec.State = WorkflowStateCanceled

	ec.timesMutex.Lock()
	ec.EndTime = time.Now()
	ec.ElapsedTime = ec.EndTime.Sub(ec.StartTime)
	ec.timesMutex.Unlock()
}

// setError 设置工作流错误并更新状态
// setError sets workflow error and updates state
func (ec *WorkflowContext) setError(err error) {
	ec.stateMutex.Lock()
	defer ec.stateMutex.Unlock()
	ec.Error = err
	ec.IsCompleted = true
	ec.State = WorkflowStateError

	ec.timesMutex.Lock()
	ec.EndTime = time.Now()
	ec.ElapsedTime = ec.EndTime.Sub(ec.StartTime)
	ec.timesMutex.Unlock()
}

// GetState 获取工作流状态
// GetState gets the workflow state
func (ec *WorkflowContext) GetState() string {
	ec.stateMutex.RLock()
	defer ec.stateMutex.RUnlock()
	return ec.State
}

// markWorkflowAsError 处理节点执行错误
// markWorkflowAsError handles node execution errors
func (ec *WorkflowContext) markWorkflowAsError(nc *NodeContext, err error) {
	// 如果节点上下文不为nil，设置错误
	// If node context is not nil, set error
	if nc != nil {
		nc.SetError(err)
	}

	// 设置工作流错误状态
	// Set workflow error state
	ec.setError(err)

	// 标记所有创建状态的节点为已取消
	// NodeContexts在初始化后不再修改，无需加锁
	// Mark all nodes in created state as canceled
	// NodeContexts is not modified after initialization, no lock needed
	for _, nodeCtx := range ec.NodeContexts {
		if nodeCtx != nil {
			if nodeCtx.GetState() == NodeStateCreated {
				// 逐个标记节点为已取消
				// Mark each node as canceled
				ec.markNodeAsCanceled(nodeCtx)
			}
		}
	}

	// 取消工作流上下文
	// Cancel workflow context
	ec.Cancel()
}

// markNodeAsCanceled 标记节点为已取消状态
// markNodeAsCanceled marks the node as canceled
func (ec *WorkflowContext) markNodeAsCanceled(nc *NodeContext) {

	// 如果节点不是创建状态，则跳过
	// If the node is not in created state, skip
	nodeState := nc.GetState()

	// 只有创建状态的节点才能被取消
	// Only nodes in created state can be canceled
	if nodeState != NodeStateCreated {
		return
	}

	// 标记节点为已取消
	// Mark the node as canceled
	nc.SetCanceled()

	// 通知下一个节点
	// Notify the next node
	ec.notifyNodeCompleted(nc.NodeID)
}

// GetNodeTimingInfo 获取所有节点的时间信息（纳秒）
// GetNodeTimingInfo gets time information for all nodes (nanoseconds)
func (ec *WorkflowContext) GetNodeTimingInfo() map[string]int64 {
	result := make(map[string]int64)

	// NodeContexts在初始化后不再修改，无需加锁
	// NodeContexts is not modified after initialization, no lock needed
	for nodeID, nc := range ec.NodeContexts {
		if nc.ElapsedTime > 0 {
			result[nodeID] = nc.ElapsedTime.Nanoseconds()
		}
	}

	return result
}

// GetStartTimeNano 获取工作流开始时间（纳秒时间戳）
// GetStartTimeNano gets workflow start time (nanosecond timestamp)
func (ec *WorkflowContext) GetStartTimeNano() int64 {
	ec.timesMutex.Lock()
	defer ec.timesMutex.Unlock()
	return ec.StartTime.UnixNano()
}

// GetEndTimeNano 获取工作流结束时间（纳秒时间戳）
// GetEndTimeNano gets workflow end time (nanosecond timestamp)
func (ec *WorkflowContext) GetEndTimeNano() int64 {
	ec.timesMutex.Lock()
	defer ec.timesMutex.Unlock()
	return ec.EndTime.UnixNano()
}

// GetNodeStartTimeNano 获取指定节点的开始时间（纳秒时间戳）
// GetNodeStartTimeNano gets the start time of the specified node (nanosecond timestamp)
func (ec *WorkflowContext) GetNodeStartTimeNano(nodeID string) int64 {
	nc := ec.getNodeContext(nodeID)

	if nc.StartTime.IsZero() {
		return 0
	}
	return nc.StartTime.UnixNano()
}

// GetNodeEndTimeNano 获取指定节点的结束时间（纳秒时间戳）
// GetNodeEndTimeNano gets the end time of the specified node (nanosecond timestamp)
func (ec *WorkflowContext) GetNodeEndTimeNano(nodeID string) int64 {
	nc := ec.getNodeContext(nodeID)

	if nc.EndTime.IsZero() {
		return 0
	}
	return nc.EndTime.UnixNano()
}

// GetWorkflowTimingInfo 获取工作流的时间信息（纳秒）
// GetWorkflowTimingInfo gets the workflow timing information (nanoseconds)
func (ec *WorkflowContext) GetWorkflowTimingInfo() int64 {
	ec.timesMutex.Lock()
	defer ec.timesMutex.Unlock()

	// 如果EndTime不为零，返回计算好的ElapsedTime
	// If EndTime is not zero, return the calculated ElapsedTime
	if !ec.EndTime.IsZero() {
		return ec.ElapsedTime.Nanoseconds()
	}
	// 否则计算到当前时间的时间
	// Otherwise calculate time to current time
	return time.Since(ec.StartTime).Nanoseconds()
}

// Wait 等待所有节点完成或工作流被取消
// Wait waits for all nodes to complete or for the workflow to be canceled
func (ec *WorkflowContext) Wait() error {
	defer ec.Cancel() // 确保上下文被取消 (Ensure context is canceled)

	// 首先检查全局错误，如果有则直接返回
	// First check for global errors, if any return directly
	ec.stateMutex.RLock()
	globalErr := ec.Error
	ec.stateMutex.RUnlock()

	if globalErr != nil {
		return globalErr
	}

	// 等待所有节点完成或工作流被取消
	// Wait for all nodes to complete or for the workflow to be canceled
	for ec.getRemainingNodes() > 0 {
		select {
		case completedNodeID := <-ec.nodeCompleteChan:

			// 获取已完成节点的信息
			// Get information about the completed node
			nc := ec.getNodeContext(completedNodeID)
			if nc == nil {
				continue // 节点可能不存在，跳过 (Node might not exist, skip)
			}

			// 尝试启动所有子节点
			// Try to start all child nodes
			for _, childID := range nc.Node.Children {
				ec.tryStartNode(childID)
			}

		case <-ec.Ctx.Done():

			// 检查是否是由于错误, 提前退出引起的取消
			// Check if the cancellation is due to an error, causing early exit
			if ec.Error != nil {
				return ec.Error
			}

			// 如果是正常处理完成，记录结束时间并返回
			// If it's normal completion, record end time and return
			if ec.IsCompleted {
				ec.setCompleted() // 确保完成状态被正确设置 (Ensure completed state is correctly set)
				return nil
			}

			// 如果是其他意外
			// 处理上下文取消错误
			// If it's another unexpected event
			// Handle context cancellation error
			ctxErr := ec.Ctx.Err()
			if ctxErr != nil {
				// 设置取消状态和错误
				// Set cancellation state and error
				ec.setCanceled()
				ec.setError(ec.Workflow.NewError(fmt.Sprintf("workflow execution cancelled: %v", ctxErr)))
				return ec.Error
			}

			// 设置未知原因的取消
			// Set cancellation for unknown reason
			ec.setCanceled()
			ec.setError(ec.Workflow.NewError("workflow execution cancelled for unknown reason"))
			return ec.Error
		}
	}

	// 所有节点已处理完，再次检查全局错误
	// All nodes have been processed, check global error again
	if ec.Error != nil {
		return ec.Error
	}

	// 标记工作流为已完成状态
	// Mark workflow as completed
	ec.setCompleted()

	return nil
}

// 获取剩余未完成的节点数
// Get the number of remaining incomplete nodes
func (ec *WorkflowContext) getRemainingNodes() int {
	completedNodes := 0

	// NodeContexts在初始化后不再修改，无需加锁
	// NodeContexts is not modified after initialization, no lock needed
	for _, nc := range ec.NodeContexts {
		if nc.IsCompleted {
			completedNodes++
		}
	}

	return ec.totalNodes - completedNodes
}

// IsActive 检查工作流是否已启动但未完成
// IsActive checks if the workflow has started but not completed
func (ec *WorkflowContext) IsActive() bool {
	ec.stateMutex.RLock()
	defer ec.stateMutex.RUnlock()
	return ec.IsStarted && !ec.IsCompleted
}

// collectParentResults 收集节点的父节点结果作为输入
// collectParentResults collects parent node results as input for the node
func (ec *WorkflowContext) collectParentResults(nc *NodeContext) interface{} {
	parentIDs := nc.Node.Parents

	// 如果没有父节点，直接使用初始输入
	// If there are no parent nodes, use the initial input directly
	if len(parentIDs) == 0 {
		return ec.Input
	}

	// 如果有父节点，构建输入映射
	// If there are parent nodes, build an input map
	inputMap := make(map[string]interface{})

	// 收集所有父节点的结果
	// Collect results from all parent nodes
	for _, parentID := range parentIDs {
		parentNC := ec.getNodeContext(parentID)

		// 检查父节点状态
		// Check parent node state
		state := parentNC.GetState()
		result := parentNC.GetResult()

		// 跳过已取消的节点
		// Skip canceled nodes
		if state == NodeStateCanceled {
			continue
		}

		// 添加父节点结果
		// Add parent node result
		if result != nil {
			inputMap[parentID] = result
		}
	}

	return inputMap
}

// GetNodeState 获取指定节点的状态
// GetNodeState gets the state of the specified node
func (ec *WorkflowContext) GetNodeState(nodeID string) string {
	nc := ec.getNodeContext(nodeID)
	return nc.GetState()
}

// GetResults 获取所有节点的结果
// GetResults gets results for all nodes
func (ec *WorkflowContext) GetResults() map[string]interface{} {
	results := make(map[string]interface{})

	// NodeContexts在初始化后不再修改，无需加锁
	// NodeContexts is not modified after initialization, no lock needed
	for nodeID, nc := range ec.NodeContexts {
		if nc.IsCompleted && !nc.IsCanceled && nc.Result != nil {
			results[nodeID] = nc.Result
		}
	}

	return results
}
