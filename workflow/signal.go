package workflow

// Signal 表示工作流节点发出的各种信号
// Signal represents various signals emitted by workflow nodes
type Signal interface {
	SignalType() string // 返回信号类型标识 (Returns signal type identifier)
}

// SelectNodeSignal 表示条件分发信号，用于指定后续要执行的节点
// SelectNodeSignal represents conditional dispatch signal, used to specify subsequent nodes to be executed
type SelectNodeSignal struct {
	WorkflowID string   // 工作流ID (Workflow ID)
	NodeID     string   // 触发条件分发的节点ID (Node ID that triggered conditional dispatch)
	TargetIDs  []string // 需要执行的目标节点ID列表 (List of target node IDs to be executed)
}

// SignalType 实现Signal接口
// SignalType implements Signal interface
func (s SelectNodeSignal) SignalType() string {
	return "select"
}

// NewSelectNodeSignal 创建新的条件分发信号
// NewSelectNodeSignal creates a new conditional dispatch signal
func NewSelectNodeSignal(targetIDs []string) SelectNodeSignal {
	return SelectNodeSignal{
		TargetIDs: targetIDs,
	}
}

// IsSelectNodeSignal 检查是否为条件分发信号类型
// IsSelectNodeSignal checks if it's a conditional dispatch signal type
func IsSelectNodeSignal(signal Signal) bool {
	_, ok := signal.(SelectNodeSignal)
	return ok
}
