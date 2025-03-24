# GoWorkflow

A Golang-based workflow orchestration system, supporting Directed Acyclic Graph (DAG) scheduling.

## Features

1. DAG (Directed Acyclic Graph) workflow model
2. Unified node function interface taking `ctx, req, parentResult` parameters
3. Concurrent node execution using goroutines
4. Workflow compilation functionality to validate DAG structure
5. Compiled workflows are frozen and cannot be modified
6. Node timeout control
7. Workflow cancellation support
8. Safe concurrent execution of multiple workflow instances
9. Node execution guarantees that all parent nodes complete before starting
10. Nanosecond precision timing metrics for node and workflow start, end, and execution times
11. Conditional dispatching that allows nodes to selectively trigger specific downstream nodes
12. Optimized node context structure for better encapsulation and state management

## Code Structure

```
workflow/
├── workflow.go        # Core workflow implementation
├── node.go            # Node definition and execution logic
├── signal.go          # Signal system for conditional dispatching
├── workflow_context.go # Execution context and state tracking
├── node_context.go    # Per-node state and result management
├── errors.go          # Error handling utilities
├── workflow_test.go   # Primary tests for workflow functionality
└── benchmark_test.go  # Performance benchmarks
```

## Implementation Details

### Workflow Structure

The Workflow is the core of the system, containing:

- **Graph Structure**: Uses the gograph library to implement the DAG topology
- **Node Mapping**: Stores a map of node IDs to node objects
- **Compilation State**: Indicates if the workflow is compiled and frozen
- **Synchronization Lock**: Ensures thread safety

### Node Structure

A Node represents a processing unit in the workflow:

- **ID**: Unique identifier for the node
- **Execution Function**: Defines the node's processing logic
- **Parent-Child Relationships**: Tracks dependencies between nodes

### Node Context

The NodeContext encapsulates runtime information for a node:

- **Node State**: Records if the node is completed, cancelled, etc.
- **Result Data**: Stores the output result of the node execution
- **Timing Information**: Tracks node start and end times
- **Conditional Dispatch**: Records nodes selected by the node for conditional routing

### Workflow Context

The WorkflowContext manages the overall execution state:

- **Context Management**: Wraps the user-provided context with cancellation capabilities
- **Result Collection**: Aggregates results from all executed nodes
- **Execution Tracking**: Monitors which nodes are completed, pending, or cancelled
- **Timing Metrics**: Records workflow start time, end time, and total duration
- **Signal Handling**: Processes conditional dispatch signals from nodes
- **Error Propagation**: Manages error handling and workflow cancellation

### Workflow Execution Process

1. **Compilation Check**: The workflow must be compiled before execution
2. **Topological Sort**: Uses topological sorting to determine the node execution order
3. **Concurrent Execution**: Launches goroutines for each node to execute concurrently
4. **Dependency Waiting**: In an A->B relationship, B waits for A to complete before executing
5. **Conditional Dispatch**: Nodes can return a SelectNodeSignal to selectively activate specific child nodes
6. **Result Aggregation**: Each node's output is collected and returned

## Usage Examples

### Defining Node Functions

```go
// Define a node function
nodeFunc := func(ctx context.Context, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
    // req is the original workflow input
    input := req.(string)
    
    // parentResult contains results from parent nodes
    parentData := parentResult.(map[string]interface{})
    parentResult := parentData["parentNodeID"] // Get result from a specific parent node
    
    // Business logic processing
    output := fmt.Sprintf("Processed %s", input)
    
    // Return the result
    return output, nil, nil
}
```

### Creating a Workflow

```go
// Create a workflow
flow := workflow.NewWorkflow("example-workflow")

// Add nodes to the workflow
flow.AddNodeFunc("A", nodeAFunc)
flow.AddNodeFunc("B", nodeBFunc)

// Add edges (define dependencies)
flow.AddEdge("A", "B")  // B depends on A

// Compile the workflow
if err := flow.Compile(); err != nil {
    // Handle compilation error
}
```

### Executing a Workflow

```go
// Create a context
ctx := context.Background()

// Execute the workflow
result, err := flow.Execute(ctx, "initial input")
if err != nil {
    // Handle execution error
}

// Process results
results := result.GetResults()
for nodeID, output := range results {
    fmt.Printf("Node %s result: %v\n", nodeID, output)
}
```

### Using Conditional Dispatch

```go
// Define a node function with conditional dispatch
routeNodeFunc := func(ctx context.Context, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
    // Processing logic
    result := "Processing result"
    
    // Choose downstream nodes based on conditions
    if someCondition {
        // Only activate "nodeA" and "nodeC"
        return workflow.SelectNodes(result, []string{"nodeA", "nodeC"})
    }
    
    // When not specified, all child nodes are activated by default
    return result, nil, nil
}
```

## Example Workflow

The included `main.go` demonstrates a complex workflow with the following structure:

```
A
├──> B ──> D
│        ├──> E ──> H ──> J
│        ├──> F ──> I ───┤
│        └──> G ─────────┘
├──> C ──┘
└──> K <──────────────────┘
```

Node D uses a SelectNodeSignal to specifically select node F, causing paths through E and G to be cancelled.

Run `go run main.go`
```
=== Golang Workflow System Example ===

=== Start executing workflow ===
Start time: 22:07:53.743
[A] Start execution, time: 22:07:53.743, input: Workflow input data, parent result: Workflow input data
[A] Execution completed, time: 22:07:53.944, duration: 201.00ms, result: Result of node A
[B] Start execution, time: 22:07:53.945, input: Workflow input data, parent result: map[A:Result of node A]
[B] Execution completed, time: 22:07:54.146, duration: 201.00ms, result: Result of node B
[C] Start execution, time: 22:07:54.146, input: Workflow input data, parent result: map[A:Result of node A]
[C] Execution completed, time: 22:07:54.347, duration: 201.00ms, result: Result of node C
[D] Start execution, time: 22:07:54.347, input: Workflow input data, parent result: map[B:Result of node B C:Result of node C]
[D] Start execution, time: 22:07:54.347, input: Workflow input data, parent result: map[B:Result of node B C:Result of node C]
[D] Execution completed, time: 22:07:54.548, duration: 201.00ms, result: Result of node D
[D] Sending signal to select node F
[D] Execution completed, time: 22:07:54.548, duration: 201.00ms, result: Result of node D
[D] Sending signal to select node F
[F] Start execution, time: 22:07:54.548, input: Workflow input data, parent result: map[D:Result of node D]
[F] Execution completed, time: 22:07:54.749, duration: 201.00ms, result: Result of node F
[I] Start execution, time: 22:07:54.749, input: Workflow input data, parent result: map[F:Result of node F]
[I] Execution completed, time: 22:07:54.951, duration: 201.00ms, result: Result of node I
[J] Start execution, time: 22:07:54.951, input: Workflow input data, parent result: map[I:Result of node I]
[J] Execution completed, time: 22:07:55.152, duration: 201.00ms, result: Result of node J
[K] Start execution, time: 22:07:55.152, input: Workflow input data, parent result: map[A:Result of node A J:Result of node J]
[K] Execution completed, time: 22:07:55.353, duration: 201.00ms, result: Result of node K
End time: 22:07:55.353, Total time: 1609.00 ms

Workflow execution results:
  Node I: Result of node I
  Node K: Result of node K
  Node A: Result of node A
  Node C: Result of node C
  Node F: Result of node F
  Node B: Result of node B
  Node D: Result of node D
  Node J: Result of node J

Workflow Execution Timing Information:
  Total execution time: 1609.50 ms
  Node execution time:
    Node K: 201.06 ms
    Node B: 201.26 ms
    Node D: 201.13 ms
    Node J: 201.14 ms
    Node A: 201.20 ms
    Node C: 201.20 ms
    Node F: 201.14 ms
    Node I: 201.21 ms

Workflow Execution Diagram:
A[completed]
├──> B[completed] ──> D[completed]
│              ├──> E[cancelled] ──> H[cancelled] ──> J[completed]
│              ├──> F[completed] ──> I[completed] ───┤
│              └──> G[cancelled] ───────────────────┘
├──> C[completed] ──┘
└──> K[completed] <─────────────────────────────┘

Workflow Execution Path Explanation:
● Actual execution path: A → B → D → F → I → J → K
● Other paths: 
  - A → C → D (merged at node D)
  - A → K (direct connection)
  - J → K (final merge)

● Special notes:
  - Node D sends signal to select F, so E and G branches are cancelled
  - E→H→J and G→I→J paths not executed, but I→J is executed via F→I
```

## Benchmark
```
goos: darwin
goarch: arm64
pkg: goworkflow/workflow
cpu: Apple M4
=== RUN   BenchmarkWorkflowExecution
BenchmarkWorkflowExecution
BenchmarkWorkflowExecution-10                     158251              6938 ns/op            4961 B/op          64 allocs/op
=== RUN   BenchmarkWorkflowCompileAndExecute
BenchmarkWorkflowCompileAndExecute
BenchmarkWorkflowCompileAndExecute-10              86570             14158 ns/op           11850 B/op         237 allocs/op
=== RUN   BenchmarkParallelWorkflowExecution
BenchmarkParallelWorkflowExecution
BenchmarkParallelWorkflowExecution-10             705471              1649 ns/op            4965 B/op          64 allocs/op
=== RUN   BenchmarkSelectiveExecution
BenchmarkSelectiveExecution
BenchmarkSelectiveExecution-10                    344964              3388 ns/op            2641 B/op          31 allocs/op
PASS
ok      goworkflow/workflow     5.079s
```

## License

See the [LICENSE](LICENSE) file for license rights and limitations.
