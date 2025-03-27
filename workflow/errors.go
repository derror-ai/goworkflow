package workflow

import (
	"fmt"
)

// WorkflowError 表示工作流系统中的错误，包含工作流ID和可能的节点ID
// WorkflowError represents an error in the workflow system, containing workflow ID and possibly node ID
type WorkflowError struct {
	WorkflowID string
	NodeID     string
	Message    string
}

// Error 实现error接口
// Error implements the error interface
func (e *WorkflowError) Error() string {
	if e.NodeID != "" {
		return fmt.Sprintf("workflow[%s] node[%s]: %s", e.WorkflowID, e.NodeID, e.Message)
	}
	return fmt.Sprintf("workflow[%s]: %s", e.WorkflowID, e.Message)
}
