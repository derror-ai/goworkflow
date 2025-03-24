package workflow

import (
	"context"
	"fmt"
	"testing"
)

// High-performance node function, returns directly without sleep
func benchNodeFunc(id string) NodeFunc {
	return func(ctx context.Context, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		// No sleep, return result directly
		if inputMap, ok := parentResult.(map[string]interface{}); ok {
			// Simple processing of parent node input
			result := fmt.Sprintf("%s-processed", id)
			for _, v := range inputMap {
				result = fmt.Sprintf("%s-%v", result, v)
				break // Only process the first value
			}
			return result, nil, nil
		} else {
			// Process initial input
			return fmt.Sprintf("%s-processed-%v", id, req), nil, nil
		}
	}
}

// setupBenchmarkWorkflow creates a workflow for benchmark testing
// Structure is similar to the A-F node structure in main.go
func setupBenchmarkWorkflow() *Workflow {
	flow := NewWorkflow("benchmark-workflow")

	// Add nodes
	flow.AddNodeFunc("A", benchNodeFunc("A"))
	flow.AddNodeFunc("B", benchNodeFunc("B"))
	flow.AddNodeFunc("C", benchNodeFunc("C"))
	flow.AddNodeFunc("D", benchNodeFunc("D"))
	flow.AddNodeFunc("E", benchNodeFunc("E"))
	flow.AddNodeFunc("F", benchNodeFunc("F"))

	// Add edges (A->B, A->D, B->E, D->E, E->F, F->C)
	flow.AddEdge("A", "B")
	flow.AddEdge("A", "D")
	flow.AddEdge("B", "E")
	flow.AddEdge("D", "E")
	flow.AddEdge("E", "F")
	flow.AddEdge("F", "C")

	// Compile workflow
	_ = flow.Compile()

	return flow
}

// BenchmarkWorkflowExecution benchmark tests workflow execution performance
func BenchmarkWorkflowExecution(b *testing.B) {
	w := setupBenchmarkWorkflow()
	ctx := context.Background()

	// Reset timer
	b.ResetTimer()

	// Run workflow execution b.N times
	for i := 0; i < b.N; i++ {
		result, err := w.Execute(ctx, fmt.Sprintf("bench-input-%d", i))
		if err != nil {
			b.Fatalf("Workflow execution failed: %v", err)
		}

		// Count non-cancelled nodes
		execCount := 0
		for nodeID := range result.NodeContexts {
			nc := result.getNodeContext(nodeID)
			if nc.GetState() != NodeStateCanceled && nc.IsCompleted {
				execCount++
			}
		}

		// Based on graph structure and dependencies, we should have at least 5 nodes executed (A, B, D, E, F)
		// In some cases, node C might also be executed, depending on scheduling and concurrency
		if execCount < 5 || execCount > 6 {
			b.Fatalf("Expected 5-6 executed node results, got %d", execCount)
		}
	}
}

// BenchmarkWorkflowCompileAndExecute benchmark tests combined performance of workflow compilation and execution
func BenchmarkWorkflowCompileAndExecute(b *testing.B) {
	ctx := context.Background()

	// Run workflow creation, compilation, and execution b.N times
	for i := 0; i < b.N; i++ {
		flow := NewWorkflow(fmt.Sprintf("bench-workflow-%d", i))

		// Add nodes
		flow.AddNodeFunc("A", benchNodeFunc("A"))
		flow.AddNodeFunc("B", benchNodeFunc("B"))
		flow.AddNodeFunc("C", benchNodeFunc("C"))
		flow.AddNodeFunc("D", benchNodeFunc("D"))
		flow.AddNodeFunc("E", benchNodeFunc("E"))
		flow.AddNodeFunc("F", benchNodeFunc("F"))

		// Add edges
		flow.AddEdge("A", "B")
		flow.AddEdge("A", "D")
		flow.AddEdge("B", "E")
		flow.AddEdge("D", "E")
		flow.AddEdge("E", "F")
		flow.AddEdge("F", "C")

		// Compile workflow
		err := flow.Compile()
		if err != nil {
			b.Fatalf("Failed to compile workflow: %v", err)
		}

		// Execute workflow
		result, err := flow.Execute(ctx, fmt.Sprintf("bench-input-%d", i))
		if err != nil {
			b.Fatalf("Workflow execution failed: %v", err)
		}

		// Count non-cancelled nodes
		execCount := 0
		for nodeID := range result.NodeContexts {
			nc := result.getNodeContext(nodeID)
			if nc.GetState() != NodeStateCanceled && nc.IsCompleted {
				execCount++
			}
		}

		// Based on graph structure and dependencies, we should have at least 5 nodes executed (A, B, D, E, F)
		// In some cases, node C might also be executed, depending on scheduling and concurrency
		if execCount < 5 || execCount > 6 {
			b.Fatalf("Expected 5-6 executed node results, got %d", execCount)
		}
	}
}

// BenchmarkParallelWorkflowExecution benchmark tests parallel workflow execution performance
func BenchmarkParallelWorkflowExecution(b *testing.B) {
	w := setupBenchmarkWorkflow()
	ctx := context.Background()

	// Reset timer
	b.ResetTimer()

	// Run workflow executions in parallel b.N times
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			input := fmt.Sprintf("parallel-input-%d", counter)
			result, err := w.Execute(ctx, input)
			if err != nil {
				b.Fatalf("Parallel workflow execution failed: %v", err)
			}

			// Count non-cancelled nodes
			execCount := 0
			for nodeID := range result.NodeContexts {
				nc := result.getNodeContext(nodeID)
				if nc.GetState() != NodeStateCanceled && nc.IsCompleted {
					execCount++
				}
			}

			// Based on graph structure and dependencies, we should have at least 5 nodes executed (A, B, D, E, F)
			// In some cases, node C might also be executed, depending on scheduling and concurrency
			if execCount < 5 || execCount > 6 {
				b.Fatalf("Expected 5-6 executed node results, got %d", execCount)
			}
			counter++
		}
	})
}

// BenchmarkSelectiveExecution benchmark tests performance of selective node execution
func BenchmarkSelectiveExecution(b *testing.B) {
	// Create workflow
	w := NewWorkflow("bench-selective-execution")

	// Create a node that uses conditional dispatch, not selecting any subsequent nodes
	selectiveFunc := func(ctx context.Context, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
		// Return empty target node list, indicating no child nodes are selected
		return "selective-node-result", NewSelectNodeSignal([]string{}), nil
	}

	// Add nodes
	w.AddNodeFunc("A", benchNodeFunc("A"))
	w.AddNodeFunc("B", selectiveFunc) // Node B uses selective execution, not selecting any nodes
	w.AddNodeFunc("C", benchNodeFunc("C"))
	w.AddNodeFunc("D", benchNodeFunc("D"))

	// Add edges
	w.AddEdge("A", "B")
	w.AddEdge("B", "C")
	w.AddEdge("C", "D")

	// Compile workflow
	err := w.Compile()
	if err != nil {
		b.Fatalf("Failed to compile workflow: %v", err)
	}

	ctx := context.Background()

	// Reset timer
	b.ResetTimer()

	// Run workflow execution b.N times
	for i := 0; i < b.N; i++ {
		input := fmt.Sprintf("selective-input-%d", i)
		result, err := w.Execute(ctx, input)
		if err != nil {
			b.Fatalf("Workflow execution failed: %v", err)
		}

		executedCount := 0

		for nodeID := range result.NodeContexts {
			nc := result.getNodeContext(nodeID)
			state := nc.GetState()

			if state == NodeStateCompleted {
				executedCount++
			}
		}

		// Check result count
		results := result.GetResults()
		resultCount := len(results)
		var nodesWithResults []string
		for nodeID := range results {
			nodesWithResults = append(nodesWithResults, nodeID)
		}

		if resultCount != 2 {
			b.Fatalf("Expected 2 executed node results (A and B), got %d: %v", resultCount, nodesWithResults)
		}
	}
}
