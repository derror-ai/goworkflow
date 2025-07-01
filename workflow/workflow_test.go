package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWorkflowCompile(t *testing.T) {
	// Create a new workflow
	w := NewWorkflow("test-compile")

	// Create processing node
	nodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		return fmt.Sprintf("processed-%v", req), nil, nil
	}

	// Add nodes to the workflow
	w.AddNodeFunc("A", nodeFunc)
	w.AddNodeFunc("B", nodeFunc)
	w.AddNodeFunc("C", nodeFunc)

	// Add edges
	w.AddEdge("A", "B")
	w.AddEdge("A", "C")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Check node connections
	if len(w.nodes["A"].Children) != 2 {
		t.Errorf("Expected node A to have 2 children, got %d", len(w.nodes["A"].Children))
	}

	if len(w.nodes["B"].Parents) != 1 {
		t.Errorf("Expected node B to have 1 parent, got %d", len(w.nodes["B"].Parents))
	}

	if len(w.nodes["C"].Parents) != 1 {
		t.Errorf("Expected node C to have 1 parent, got %d", len(w.nodes["C"].Parents))
	}
}

func TestWorkflowExecution(t *testing.T) {
	// Create a new workflow
	w := NewWorkflow("test-execution")

	// Create node function
	createNodeFunc := func(nodeID string) NodeFunc {
		return func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
			t.Logf("Node %s processing input: %v, parent result: %v", nodeID, req, parentResult)
			// Simulate processing time
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("%s-processed-%v", nodeID, req), nil, nil
		}
	}

	// Add nodes to the workflow
	w.AddNodeFunc("A", createNodeFunc("A"))
	w.AddNodeFunc("B", createNodeFunc("B"))
	w.AddNodeFunc("C", createNodeFunc("C"))
	w.AddNodeFunc("D", createNodeFunc("D"))
	w.AddNodeFunc("E", createNodeFunc("E"))
	w.AddNodeFunc("F", createNodeFunc("F"))

	// Add edges
	w.AddEdge("A", "B")
	w.AddEdge("A", "C")
	w.AddEdge("B", "D")
	w.AddEdge("C", "E")
	w.AddEdge("D", "F")
	w.AddEdge("E", "F")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow
	result, err := w.Execute(context.Background(), "test-input")
	if err != nil {
		t.Fatalf("Failed to execute workflow: %v", err)
	}

	// Check results
	results := result.GetResults()
	for nodeID, output := range results {
		t.Logf("Node %s result: %v", nodeID, output)
	}

	// Verify final node F result
	if _, ok := results["F"]; !ok {
		t.Errorf("Expected result for node F, but not found")
	}

	// Check timing information
	t.Logf("Workflow start time: %s", time.Unix(0, result.GetStartTimeNano()).Format(time.RFC3339Nano))
	t.Logf("Workflow end time: %s", time.Unix(0, result.GetEndTimeNano()).Format(time.RFC3339Nano))
	t.Logf("Workflow total time: %d ns", result.GetWorkflowTimingInfo())

	for nodeID, timeNano := range result.GetNodeTimingInfo() {
		t.Logf("Node %s:", nodeID)
		t.Logf("  Start time: %s", time.Unix(0, result.GetNodeStartTimeNano(nodeID)).Format(time.RFC3339Nano))
		t.Logf("  End time: %s", time.Unix(0, result.GetNodeEndTimeNano(nodeID)).Format(time.RFC3339Nano))
		t.Logf("  Execution time: %d ns", timeNano)

		// Verify node execution time is at least 50ms (our sleep time)
		if timeNano < 50000000 { // 50ms = 50,000,000ns
			t.Errorf("Node %s time too short: %d ns, expected at least 50000000 ns", nodeID, timeNano)
		}
	}
}

func TestConcurrentWorkflowExecution(t *testing.T) {
	// Create a new workflow
	w := NewWorkflow("test-concurrent")

	// Create processing node
	createNodeFunc := func(nodeID string) NodeFunc {
		return func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
			t.Logf("Node %s processing input: %v, parent result: %v", nodeID, req, parentResult)
			// Simulate processing time
			time.Sleep(50 * time.Millisecond)
			return fmt.Sprintf("%s-processed-%v", nodeID, req), nil, nil
		}
	}

	// Add nodes to the workflow
	w.AddNodeFunc("A", createNodeFunc("A"))
	w.AddNodeFunc("B", createNodeFunc("B"))
	w.AddNodeFunc("C", createNodeFunc("C"))

	// Add edges
	w.AddEdge("A", "B")
	w.AddEdge("A", "C")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow concurrently
	numConcurrent := 3
	var resultsChan = make(chan *WorkflowContext, numConcurrent)
	var errorsChan = make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(instance int) {
			input := fmt.Sprintf("input-%d", instance)
			result, err := w.Execute(context.Background(), input)
			if err != nil {
				errorsChan <- err
				return
			}
			resultsChan <- result
		}(i)
	}

	// Collect results
	for i := 0; i < numConcurrent; i++ {
		select {
		case err := <-errorsChan:
			t.Errorf("Instance %d failed: %v", i, err)
		case result := <-resultsChan:
			t.Logf("Instance completed with results: %v", result.GetResults())
			t.Logf("Instance start time: %s", time.Unix(0, result.GetStartTimeNano()).Format(time.RFC3339Nano))
			t.Logf("Instance end time: %s", time.Unix(0, result.GetEndTimeNano()).Format(time.RFC3339Nano))
			t.Logf("Instance total time: %d ns", result.GetWorkflowTimingInfo())

			// Check execution time for each node
			for nodeID, timeNano := range result.GetNodeTimingInfo() {
				t.Logf("Node %s:", nodeID)
				t.Logf("  Start time: %s", time.Unix(0, result.GetNodeStartTimeNano(nodeID)).Format(time.RFC3339Nano))
				t.Logf("  End time: %s", time.Unix(0, result.GetNodeEndTimeNano(nodeID)).Format(time.RFC3339Nano))
				t.Logf("  Execution time: %d ns", timeNano)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for workflow executions")
		}
	}
}

func TestWorkflowMultiChildrenExecution(t *testing.T) {
	// Create a new workflow
	w := NewWorkflow("test-execution-multi-children")

	// Create node function
	createNodeFunc := func(nodeID string) NodeFunc {
		return func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
			t.Logf("Node %s processing input: %v, parent result: %v", nodeID, req, parentResult)
			// Simulate processing time
			time.Sleep(1000 * time.Millisecond)
			return fmt.Sprintf("%s-processed-%v", nodeID, req), nil, nil
		}
	}

	// Add nodes to the workflow
	w.AddNodeFunc("A", createNodeFunc("A"))
	w.AddNodeFunc("B", createNodeFunc("B"))
	w.AddNodeFunc("C", createNodeFunc("C"))
	w.AddNodeFunc("D", createNodeFunc("D"))
	w.AddNodeFunc("E", createNodeFunc("E"))
	w.AddNodeFunc("F", createNodeFunc("F"))
	w.AddNodeFunc("G", createNodeFunc("G"))
	w.AddNodeFunc("H", createNodeFunc("H"))

	// Add edges
	w.AddEdge("A", "B")
	w.AddEdge("A", "C")

	w.AddEdge("B", "D")
	w.AddEdge("B", "E")

	w.AddEdge("C", "F")
	w.AddEdge("C", "G")

	w.AddEdge("D", "H")
	w.AddEdge("E", "H")
	w.AddEdge("F", "H")
	w.AddEdge("G", "H")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow
	result, err := w.Execute(context.Background(), "test-input")
	if err != nil {
		t.Fatalf("Failed to execute workflow: %v", err)
	}

	// Check results
	results := result.GetResults()
	for nodeID, output := range results {
		t.Logf("Node %s result: %v", nodeID, output)
	}

	// Verify final node H result
	if _, ok := results["H"]; !ok {
		t.Errorf("Expected result for node H, but not found")
	}

	// Check timing information
	t.Logf("Workflow start time: %s", time.Unix(0, result.GetStartTimeNano()).Format(time.RFC3339Nano))
	t.Logf("Workflow end time: %s", time.Unix(0, result.GetEndTimeNano()).Format(time.RFC3339Nano))
	t.Logf("Workflow total time: %d ns", result.GetWorkflowTimingInfo())

	// 获取D、E、F、G节点的开始时间
	dStartTime := result.GetNodeStartTimeNano("D")
	eStartTime := result.GetNodeStartTimeNano("E")
	fStartTime := result.GetNodeStartTimeNano("F")
	gStartTime := result.GetNodeStartTimeNano("G")

	// 检查这些节点的开始时间差异不应超过100ms
	maxDiff := int64(100 * 1000 * 1000) // 100ms转换为纳秒

	// 找出最早和最晚的开始时间
	startTimes := []int64{dStartTime, eStartTime, fStartTime, gStartTime}
	var earliestStart, latestStart int64

	// 初始化最早和最晚时间
	earliestStart = startTimes[0]
	latestStart = startTimes[0]

	for _, startTime := range startTimes {
		if startTime < earliestStart {
			earliestStart = startTime
		}
		if startTime > latestStart {
			latestStart = startTime
		}
	}

	timeDiff := latestStart - earliestStart
	t.Logf("Time difference between earliest and latest start of D,E,F,G: %d ns (%f ms)",
		timeDiff, float64(timeDiff)/1000000.0)

	if timeDiff > maxDiff {
		t.Errorf("Nodes D,E,F,G should start within 100ms of each other, but actual difference was %f ms",
			float64(timeDiff)/1000000.0)
	}

	for nodeID, timeNano := range result.GetNodeTimingInfo() {
		t.Logf("Node %s:", nodeID)
		t.Logf("  Start time: %s", time.Unix(0, result.GetNodeStartTimeNano(nodeID)).Format(time.RFC3339Nano))
		t.Logf("  End time: %s", time.Unix(0, result.GetNodeEndTimeNano(nodeID)).Format(time.RFC3339Nano))
		t.Logf("  Execution time: %d ns", timeNano)

		// Verify node execution time is at least 50ms (our sleep time)
		if timeNano < 50000000 { // 50ms = 50,000,000ns
			t.Errorf("Node %s time too short: %d ns, expected at least 50000000 ns", nodeID, timeNano)
		}
	}
}

// TestWorkflowTimeoutAndCancel tests workflow timeout and cancellation functionality
func TestWorkflowTimeoutAndCancel(t *testing.T) {
	// Create workflow
	w := NewWorkflow("test-timeout-cancel")

	// Create long-running node
	longRunningNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {

		// Check context state before timeout
		select {
		case <-ctx.Ctx.Done():
			return nil, nil, ctx.Ctx.Err()
		default:
			// Continue execution
		}

		// Longer sleep time to ensure timeout is triggered
		time.Sleep(500 * time.Millisecond)

		// Check for timeout again
		select {
		case <-ctx.Ctx.Done():
			return nil, nil, ctx.Ctx.Err() // Explicitly return context error
		default:
			return "result", nil, nil
		}
	}

	// Add multiple long-running nodes
	w.AddNodeFunc("nodeA", longRunningNodeFunc)
	w.AddNodeFunc("nodeB", longRunningNodeFunc)
	w.AddNodeFunc("nodeC", longRunningNodeFunc)

	// Add edges
	w.AddEdge("nodeA", "nodeB")
	w.AddEdge("nodeB", "nodeC")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Create context with 100ms timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Execute workflow
	_, err = w.Execute(ctx, "test-input") // Ignore result, only check for error
	// Expect timeout error
	if err == nil {
		t.Fatalf("Expected timeout error, got nil")
	}

	// Verify error contains timeout information
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("Expected context deadline exceeded error, got: %v", err)
	}

	t.Logf("Got expected error: %v", err)
}

// TestNodeError tests node execution error scenario
func TestNodeError(t *testing.T) {
	// Create a new workflow
	w := NewWorkflow("test-node-error")

	// Create normal node
	normalNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Normal node processing input: %v, parent result: %v", req, parentResult)
		time.Sleep(50 * time.Millisecond)
		return "normal-result", nil, nil
	}

	// Create node that will produce an error
	errorNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Error node processing input: %v, parent result: %v", req, parentResult)
		time.Sleep(50 * time.Millisecond)
		return nil, nil, fmt.Errorf("simulated error in node")
	}

	// Add nodes to the workflow
	w.AddNodeFunc("start", normalNodeFunc)
	w.AddNodeFunc("normal", normalNodeFunc)
	w.AddNodeFunc("error", errorNodeFunc)
	w.AddNodeFunc("unreachable", normalNodeFunc) // This node should not be executed

	// Add edges
	w.AddEdge("start", "normal")
	w.AddEdge("start", "error")
	w.AddEdge("normal", "unreachable")
	w.AddEdge("error", "unreachable")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow
	result, err := w.Execute(context.Background(), "test-input")

	// Expect error
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	t.Logf("Got expected error: %v", err)

	// Even though an error was returned, some nodes should have completed execution
	t.Logf("Partial results before error: %v", result.GetResults())

	// Verify that start node and normal node executed
	results := result.GetResults()
	if _, ok := results["start"]; !ok {
		t.Errorf("Start node should have executed")
	}

	// Verify unreachable node didn't execute
	if _, ok := results["unreachable"]; ok {
		t.Errorf("Unreachable node should not have executed")
	}
}

// TestConditionalDispatch tests conditional dispatch functionality
func TestConditionalDispatch(t *testing.T) {
	// Create workflow
	w := NewWorkflow("test-conditional-dispatch")

	// Create processing nodes
	startNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Start node processing input: %v, parent result: %v", req, parentResult)
		// Return conditional selection signal, only selecting route1 node for execution
		return SelectNodes("start-result", []string{"route1"})
	}

	route1NodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Route1 node processing input: %v, parent result: %v", req, parentResult)
		// Check if route2 was executed
		// In the new design, cancelled node results don't appear in the result set
		inputMap := parentResult.(map[string]interface{})
		if _, ok := inputMap["route2"]; ok {
			t.Errorf("Route1 node received result from route2 which should not be included: %v", inputMap["route2"])
		}
		return "route1-result", nil, nil
	}

	route2NodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Errorf("Route2 node should NOT be executed")
		return "route2-result", nil, nil
	}

	endNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("End node processing input: %v, parent result: %v", req, parentResult)
		inputMap := parentResult.(map[string]interface{})
		// Ensure only route1's result was received, route2 should not appear in the input
		if result, ok := inputMap["route1"]; !ok || result != "route1-result" {
			t.Errorf("End node did not receive correct result from route1: %v", inputMap)
		}
		if _, ok := inputMap["route2"]; ok {
			t.Errorf("End node received result from route2 which should not be included: %v", inputMap["route2"])
		}
		return "end-result", nil, nil
	}

	// Add nodes to the workflow
	w.AddNodeFunc("start", startNodeFunc)
	w.AddNodeFunc("route1", route1NodeFunc)
	w.AddNodeFunc("route2", route2NodeFunc)
	w.AddNodeFunc("end", endNodeFunc)

	// Build routing branches: start -> (route1, route2) -> end
	w.AddEdge("start", "route1")
	w.AddEdge("start", "route2")
	w.AddEdge("route1", "end")
	w.AddEdge("route2", "end")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow
	result, err := w.Execute(context.Background(), "test-data")
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Confirm route2 node is marked as cancelled
	state := result.GetNodeState("route2")
	if state != NodeStateCanceled {
		t.Errorf("Node route2 should be marked as canceled, got state: %v", state)
	}

	// Verify conditional dispatch worked as expected
	results := result.GetResults()
	if results["start"] != "start-result" {
		t.Errorf("Unexpected start node result: %v", results["start"])
	}
	if results["route1"] != "route1-result" {
		t.Errorf("Unexpected route1 node result: %v", results["route1"])
	}
	if results["end"] != "end-result" {
		t.Errorf("Unexpected end node result: %v", results["end"])
	}
}

// TestConditionalDispatch tests conditional dispatch functionality
func TestConditionalDispatch2(t *testing.T) {
	// Create workflow
	w := NewWorkflow("test-conditional-dispatch2")

	// Create processing nodes
	startNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Start node processing input: %v, parent result: %v", req, parentResult)
		// Return conditional selection signal, only selecting route1 node for execution
		return SelectNodes("start-result", []string{"route1"})
	}

	route1NodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Route1 node processing input: %v, parent result: %v", req, parentResult)
		return SelectNodes("route1-result", []string{"route2"})
	}

	route2NodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Route2 node processing input: %v, parent result: %v", req, parentResult)
		return "route2-result", nil, nil
	}

	endNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("End node processing input: %v, parent result: %v", req, parentResult)
		return "end-result", nil, nil
	}

	// Add nodes to the workflow
	w.AddNodeFunc("start", startNodeFunc)
	w.AddNodeFunc("route1", route1NodeFunc)
	w.AddNodeFunc("route2", route2NodeFunc)
	w.AddNodeFunc("end", endNodeFunc)

	// Build routing branches: start -> (route1, route2) -> end
	w.AddEdge("start", "route1")
	w.AddEdge("start", "route2")
	w.AddEdge("route1", "end")
	w.AddEdge("route1", "route2")
	w.AddEdge("route2", "end")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow
	result, err := w.Execute(context.Background(), "test-data")
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify conditional dispatch worked as expected
	results := result.GetResults()
	if results["start"] != "start-result" {
		t.Errorf("Unexpected start node result: %v", results["start"])
	}
	if results["route1"] != "route1-result" {
		t.Errorf("Unexpected route1 node result: %v", results["route1"])
	}
	if results["route2"] != "route2-result" {
		t.Errorf("Unexpected route2 node result: %v", results["route2"])
	}
	if results["end"] != "end-result" {
		t.Errorf("Unexpected end node result: %v", results["end"])
	}
}

// TestConditionalDispatch tests conditional dispatch functionality
func TestConditionalDispatch3(t *testing.T) {
	// Create workflow
	w := NewWorkflow("test-conditional-dispatch3")

	// Create processing node
	createNodeFunc := func(nodeID string) NodeFunc {
		return func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
			t.Logf("Node %s processing input: %v, parent result: %v", nodeID, req, parentResult)
			// Simulate processing time
			time.Sleep(100 * time.Millisecond)
			return fmt.Sprintf("%s-result", nodeID), nil, nil
		}
	}

	// Create processing nodes
	dispatchNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("node dispatch processing input: %v, parent result: %v", req, parentResult)

		inputMap := parentResult.(map[string]interface{})
		// Ensure only route1's result was received, route2 should not appear in the input
		if _, ok := inputMap["A"]; !ok {
			t.Errorf("Dispatch node did not receive correct result from A: %v", inputMap)
		}
		if _, ok := inputMap["C"]; !ok {
			t.Errorf("Dispatch node did not receive correct result from C: %v", inputMap)
		}

		// Return conditional selection signal, only selecting route1 node for execution
		return SelectNodes("dispatch-result", []string{"route1"})
	}

	route1NodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Route1 node processing input: %v, parent result: %v", req, parentResult)
		return SelectNodes("route1-result", []string{"route2"})
	}

	route2NodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("Route2 node processing input: %v, parent result: %v", req, parentResult)
		return "route2-result", nil, nil
	}

	endNodeFunc := func(ctx *NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		t.Logf("End node processing input: %v, parent result: %v", req, parentResult)
		return "end-result", nil, nil
	}

	// Add nodes to the workflow
	w.AddNodeFunc("start", createNodeFunc("start"))
	w.AddNodeFunc("A", createNodeFunc("A"))
	w.AddNodeFunc("B", createNodeFunc("B"))
	w.AddNodeFunc("C", createNodeFunc("C"))
	w.AddNodeFunc("dispatch", dispatchNodeFunc)
	w.AddNodeFunc("route1", route1NodeFunc)
	w.AddNodeFunc("route2", route2NodeFunc)
	w.AddNodeFunc("end", endNodeFunc)

	// Build routing branches: start -> (route1, route2) -> end
	w.AddEdge("start", "A")
	w.AddEdge("start", "B")
	w.AddEdge("A", "dispatch")
	w.AddEdge("B", "C")
	w.AddEdge("C", "dispatch")
	w.AddEdge("dispatch", "route1")
	w.AddEdge("dispatch", "route2")
	w.AddEdge("route1", "end")
	w.AddEdge("route1", "route2")
	w.AddEdge("route2", "end")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		t.Fatalf("Failed to compile workflow: %v", err)
	}

	// Execute workflow
	result, err := w.Execute(context.Background(), "test-data")
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify conditional dispatch worked as expected
	results := result.GetResults()
	if results["start"] != "start-result" {
		t.Errorf("Unexpected start node result: %v", results["start"])
	}
	if results["route1"] != "route1-result" {
		t.Errorf("Unexpected route1 node result: %v", results["route1"])
	}
	if results["route2"] != "route2-result" {
		t.Errorf("Unexpected route2 node result: %v", results["route2"])
	}
	if results["end"] != "end-result" {
		t.Errorf("Unexpected end node result: %v", results["end"])
	}
}
