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

// NewWorkflowError 创建一个工作流错误，可选包含节点ID
// NewWorkflowError creates a workflow error, optionally containing a node ID
func NewWorkflowError(workflowID string, message string, nodeID ...string) error {
	err := &WorkflowError{
		WorkflowID: workflowID,
		Message:    message,
	}

	if len(nodeID) > 0 && nodeID[0] != "" {
		err.NodeID = nodeID[0]
	}

	return err
}
