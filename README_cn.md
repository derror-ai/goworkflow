# GoWorkflow

Golang 工作流编排系统，支持有向无环图(DAG)调度。

## 功能特点

1. 支持 DAG (有向无环图) 工作流模型
2. 节点函数统一接口，接收 `ctx, req, parentResult` 参数
3. 节点使用协程并发执行
4. 支持工作流编译 (compile) 功能，验证 DAG 结构
5. 编译后的工作流被冻结，不允许修改
6. 支持节点超时控制
7. 支持工作流取消控制
8. 并发执行多个工作流实例是安全的
9. 节点执行保证所有父节点执行完成后才会启动
10. 提供纳秒级时间精度的执行指标，包括节点和工作流的开始、结束和执行时间
11. 支持条件分发，允许节点选择性地触发特定的下游节点
12. 优化的节点上下文结构，提供更好的封装和状态管理

## 代码结构

```
workflow/
├── workflow.go        # 核心工作流实现
├── node.go            # 节点定义和执行逻辑
├── signal.go          # 条件分发信号系统
├── workflow_context.go # 执行上下文和状态跟踪
├── node_context.go    # 节点状态和结果管理
├── errors.go          # 错误处理工具
├── workflow_test.go   # 工作流功能的主要测试
└── benchmark_test.go  # 性能基准测试
```

## 实现细节

### 工作流结构

工作流 (Workflow) 是整个系统的核心，它包含以下组件：

- **图结构**: 使用 gograph 库实现 DAG 拓扑结构
- **节点映射**: 存储节点ID到节点对象的映射
- **编译状态**: 标识工作流是否已编译且被冻结
- **同步锁**: 保证并发安全性

### 节点结构

节点 (Node) 代表工作流中的一个处理单元：

- **ID**: 节点唯一标识符
- **执行函数**: 定义节点的处理逻辑
- **父子关系**: 记录节点之间的依赖关系

### 节点上下文

节点上下文 (NodeContext) 封装节点的运行时信息：

- **节点状态**: 记录节点是否已完成、已取消等状态
- **结果数据**: 存储节点执行的输出结果
- **时间信息**: 记录节点的开始和结束时间
- **条件分发**: 记录节点选择的下游节点，用于条件路由

### 工作流上下文

工作流上下文 (WorkflowContext) 管理整体执行状态：

- **上下文管理**: 包装用户提供的上下文，并添加取消功能
- **结果收集**: 聚合所有已执行节点的结果
- **执行跟踪**: 监控哪些节点已完成、正在等待或已取消
- **时间指标**: 记录工作流开始时间、结束时间和总持续时间
- **信号处理**: 处理来自节点的条件分发信号
- **错误传播**: 管理错误处理和工作流取消

### 工作流执行过程

1. **编译检查**: 在执行前必须先编译工作流
2. **拓扑排序**: 使用拓扑排序确定节点执行顺序
3. **并发执行**: 对每个节点启动协程并发执行
4. **依赖等待**: A->B 关系中，B 会等待 A 完成后才执行
5. **条件分发**: 节点可以返回 SelectNodeSignal 选择性激活特定的子节点
6. **结果聚合**: 每个节点的输出会被收集并返回

## 使用方法

### 定义节点函数

```go
// 定义节点函数
nodeFunc := func(ctx *workflow.NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
    // req是工作流的原始输入
    input := req.(string)
    
    // parentResult是父节点的结果，为map[string]interface{}类型
    parentData := parentResult.(map[string]interface{})
    parentResult := parentData["parentNodeID"] // 获取特定父节点的结果
    
    // 业务处理逻辑
    output := fmt.Sprintf("处理后的%s", input)
    
    // 返回结果
    return output, nil, nil
}
```

### 创建工作流

```go
// 创建工作流
flow := workflow.NewWorkflow("示例工作流")

// 添加节点到工作流
flow.AddNodeFunc("A", nodeAFunc)
flow.AddNodeFunc("B", nodeBFunc)

// 添加边（定义依赖关系）
flow.AddEdge("A", "B")  // B 依赖 A

// 编译工作流
if err := flow.Compile(); err != nil {
    // 处理编译错误
}
```

### 执行工作流

```go
// 创建上下文
ctx := context.Background()

// 执行工作流
result, err := flow.Execute(ctx, "初始输入")
if err != nil {
    // 处理执行错误
}

// 处理结果
results := result.GetResults()
for nodeID, output := range results {
    fmt.Printf("节点 %s 的结果: %v\n", nodeID, output)
}
```

### 使用条件分发

```go
// 定义具有条件分发的节点函数
routeNodeFunc := func(ctx *workflow.NodeContext, req interface{}, parentResult interface{}) (interface{}, Signal, error) {
    // 处理逻辑
    result := "处理结果"
    
    // 根据条件选择下游节点
    if someCondition {
        // 只激活 "nodeA" 和 "nodeC"
        return workflow.SelectNodes(result, []string{"nodeA", "nodeC"})
    }
    
    // 不指定时，默认激活所有子节点
    return result, nil, nil
}
```

## 示例工作流

包含在 `main.go` 中的示例演示了一个复杂的工作流，其结构如下：

```
A
├──> B ──> D
│        ├──> E ──> H ──> J
│        ├──> F ──> I ───┤
│        └──> G ─────────┘
├──> C ──┘
└──> K <──────────────────┘
```

节点 D 使用 SelectNodeSignal 特别选择节点 F，导致通过 E 和 G 的路径被取消。

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
pkg: github.com/derror-ai/goworkflow/workflow
cpu: Apple M4
=== RUN   BenchmarkWorkflowExecution
BenchmarkWorkflowExecution
BenchmarkWorkflowExecution-10                      79782             14845 ns/op            5031 B/op         64 allocs/op
=== RUN   BenchmarkWorkflowCompileAndExecute
BenchmarkWorkflowCompileAndExecute
BenchmarkWorkflowCompileAndExecute-10              64549             18860 ns/op           10667 B/op        217 allocs/op
=== RUN   BenchmarkParallelWorkflowExecution
BenchmarkParallelWorkflowExecution
BenchmarkParallelWorkflowExecution-10             503532              2343 ns/op            5030 B/op         64 allocs/op
=== RUN   BenchmarkSelectiveExecution
BenchmarkSelectiveExecution
BenchmarkSelectiveExecution-10                    282290              4198 ns/op            2625 B/op         29 allocs/op
PASS
ok      github.com/derror-ai/goworkflow/workflow        5.326s
```

## 许可证

关于许可权利和限制，请参阅 [LICENSE](LICENSE) 文件。
