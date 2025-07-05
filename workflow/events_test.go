package workflow

import (
	"context"
	"testing"
	"time"
)

func TestEventSystem(t *testing.T) {
	// 创建一个简单的工作流
	w := NewWorkflow("test-workflow")

	// 添加两个节点
	err := w.AddNodeFunc("node1", func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		return "result1", nil, nil
	})
	if err != nil {
		t.Fatalf("添加节点1失败: %v", err)
	}

	err = w.AddNodeFunc("node2", func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		return "result2", nil, nil
	})
	if err != nil {
		t.Fatalf("添加节点2失败: %v", err)
	}

	// 添加边
	err = w.AddEdge("node1", "node2")
	if err != nil {
		t.Fatalf("添加边失败: %v", err)
	}

	// 编译工作流
	err = w.Compile()
	if err != nil {
		t.Fatalf("编译工作流失败: %v", err)
	}

	// 创建工作流上下文
	wc := NewWorkflowContext(w, "test-input")

	// 收集事件
	events := make([]Event, 0)
	go func() {
		ctx := context.Background()
		for {
			event, ok := wc.EventStore.Next(ctx)
			if !ok {
				break
			}
			events = append(events, event)
		}
	}()

	// 启动工作流
	err = wc.Start()
	if err != nil {
		t.Fatalf("启动工作流失败: %v", err)
	}

	// 等待工作流完成
	err = wc.Wait()
	if err != nil {
		t.Fatalf("等待工作流完成失败: %v", err)
	}

	// 给事件收集一些时间
	time.Sleep(100 * time.Millisecond)

	// 验证事件
	if len(events) == 0 {
		t.Fatal("没有收到任何事件")
	}

	// 至少应该有工作流开始、一个节点开始、一个节点完成、工作流完成
	if len(events) < 4 {
		t.Fatalf("事件数量太少，期望至少4个，实际: %d", len(events))
	}

	// 验证WorkflowStarted事件
	workflowStartedEvent := events[0]
	if workflowStartedEvent.Type != WorkflowStarted {
		t.Error("第一个事件应该是WorkflowStarted")
	}
	if workflowStartedEvent.WorkflowContext == nil {
		t.Error("WorkflowStarted事件应该包含WorkflowContext")
	}
	if workflowStartedEvent.NodeContext != nil {
		t.Error("WorkflowStarted事件不应该包含NodeContext")
	}

	// 验证最后一个事件是WorkflowCompleted
	lastEvent := events[len(events)-1]
	if lastEvent.Type != WorkflowCompleted {
		t.Error("最后一个事件应该是WorkflowCompleted")
	}
	if lastEvent.State != StateSuccess {
		t.Error("WorkflowCompleted事件状态应该是StateSuccess")
	}

	t.Log("事件系统测试通过")
}

func TestEventStoreReset(t *testing.T) {
	// 创建事件存储器
	eventStore := NewDefaultEventStore()

	// 创建测试工作流上下文
	w := NewWorkflow("test-workflow")
	w.AddNodeFunc("node1", func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		return "result", nil, nil
	})
	w.AddNodeFunc("node2", func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		return "result", nil, nil
	})
	w.AddEdge("node1", "node2")
	w.Compile()
	wc := NewWorkflowContext(w, "test-input")

	// 添加测试事件
	eventStore.Emit(WorkflowStarted, "", wc, nil, nil)
	eventStore.Emit(WorkflowCompleted, StateSuccess, wc, nil, nil)

	// 第一次读取
	ctx := context.Background()
	event1, ok1 := eventStore.Next(ctx)
	if !ok1 {
		t.Error("应该能够读取第一个事件")
	}
	if event1.Type != WorkflowStarted {
		t.Error("第一个事件应该是WorkflowStarted")
	}

	event2, ok2 := eventStore.Next(ctx)
	if !ok2 {
		t.Error("应该能够读取第二个事件")
	}
	if event2.Type != WorkflowCompleted {
		t.Error("第二个事件应该是WorkflowCompleted")
	}

	// 重置索引
	eventStore.Reset()

	// 再次读取
	event3, ok3 := eventStore.Next(ctx)
	if !ok3 {
		t.Error("重置后应该能够再次读取第一个事件")
	}
	if event3.Type != WorkflowStarted {
		t.Error("重置后第一个事件应该是WorkflowStarted")
	}

	t.Log("事件存储重置测试通过")
}
