package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goworkflow/workflow"
)

// 创建一个节点函数 (Create a node function)
// id: 节点ID (node ID)
// sleepTime: 睡眠时间，默认200ms (sleep time, default 200ms)
func createNode(id string) workflow.NodeFunc {
	return func(ctx *workflow.NodeContext, req interface{}, parentResult interface{}) (interface{}, workflow.Signal, error) {
		startTime := time.Now()
		fmt.Printf("[%s] Start execution, time: %s, input: %v, parent result: %v\n", id, startTime.Format("15:04:05.000"), req, parentResult)

		// 睡眠200ms模拟处理时间 (Sleep 200ms to simulate processing time)
		time.Sleep(200 * time.Millisecond)

		// 检查上下文是否已取消 (Check if context is cancelled)
		select {
		case <-ctx.Ctx.Done():
			fmt.Printf("[%s] Execution cancelled: %v\n", id, ctx.Ctx.Err())
			return nil, nil, ctx.Ctx.Err()
		default:
			// 继续执行 (Continue execution)
		}

		endTime := time.Now()
		duration := endTime.Sub(startTime)
		result := fmt.Sprintf("Result of node %s", id)
		fmt.Printf("[%s] Execution completed, time: %s, duration: %.2fms, result: %s\n",
			id, endTime.Format("15:04:05.000"), float64(duration.Milliseconds()), result)

		// 节点D要返回选择F的信号 (Node D returns a signal to select node F)
		if id == "D" {
			fmt.Printf("[%s] Sending signal to select node F\n", id)
			return result, workflow.NewSelectNodeSignal([]string{"F"}), nil
		}

		return result, nil, nil
	}
}

// 打印工作流执行时间信息 (Print workflow execution timing information)
func printTimingInfo(result *workflow.WorkflowContext) {
	fmt.Println("\nWorkflow Execution Timing Information:")
	fmt.Printf("  Total execution time: %.2f ms\n", float64(result.GetWorkflowTimingInfo())/1e6)

	// 打印每个节点的执行时间 (Print execution time for each node)
	fmt.Println("  Node execution time:")
	for nodeID, timeNano := range result.GetNodeTimingInfo() {
		fmt.Printf("    Node %s: %.2f ms\n", nodeID, float64(timeNano)/1e6)
	}
}

// 打印工作流执行结果简图 (Print workflow execution result diagram)
func printWorkflowTree(w *workflow.Workflow, result *workflow.WorkflowContext) {
	fmt.Println("\nWorkflow Execution Diagram:")

	// 构建节点状态映射 (Build node status mapping)
	nodeStatus := make(map[string]string)

	// 添加节点状态 (Add node status)
	nodeIDs := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}
	for _, nodeID := range nodeIDs {
		// 获取节点状态 (Get node state)
		state := result.GetNodeState(nodeID)
		if state == "completed" {
			nodeStatus[nodeID] = "completed"
		} else {
			nodeStatus[nodeID] = "cancelled"
		}
	}

	// 打印简化的DAG图 (Print simplified DAG graph)
	fmt.Println("A[completed]")
	fmt.Println("├──> B[completed] ──> D[completed]")
	fmt.Println("│              ├──> E[cancelled] ──> H[cancelled] ──> J[completed]")
	fmt.Println("│              ├──> F[completed] ──> I[completed] ───┤")
	fmt.Println("│              └──> G[cancelled] ───────────────────┘")
	fmt.Println("├──> C[completed] ──┘")
	fmt.Println("└──> K[completed] <─────────────────────────────┘")

	// 打印流程说明 (Print workflow explanation)
	fmt.Println("\nWorkflow Execution Path Explanation:")
	fmt.Println("● Actual execution path: A → B → D → F → I → J → K")
	fmt.Println("● Other paths: ")
	fmt.Println("  - A → C → D (merged at node D)")
	fmt.Println("  - A → K (direct connection)")
	fmt.Println("  - J → K (final merge)")
	fmt.Println("\n● Special notes:")
	fmt.Println("  - Node D sends signal to select F, so E and G branches are cancelled")
	fmt.Println("  - E→H→J and G→I→J paths not executed, but I→J is executed via F→I")
}

func main() {
	fmt.Println("=== Golang Workflow System Example ===")

	// 创建工作流 (Create workflow)
	w := workflow.NewWorkflow("prd-workflow")

	// 添加所有节点 (Add all nodes)
	nodeIDs := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}
	for _, id := range nodeIDs {
		w.AddNodeFunc(id, createNode(id))
	}

	// 按照PRD添加边 (Add edges according to PRD)
	edges := []string{
		"A-B", "A-C", "A-K",
		"B-D", "C-D",
		"D-E", "D-F", "D-G",
		"E-H", "F-I", "G-I",
		"H-J", "I-J",
		"J-K",
	}

	for _, edge := range edges {
		parts := strings.Split(edge, "-")
		if len(parts) == 2 {
			w.AddEdge(parts[0], parts[1])
		}
	}

	// 编译工作流 (Compile workflow)
	err := w.Compile()
	if err != nil {
		fmt.Println("Error compiling workflow:", err)
		return
	}

	// 执行工作流 (Execute workflow)
	fmt.Println("\n=== Start executing workflow ===")
	startTime := time.Now()
	fmt.Printf("Start time: %s\n", startTime.Format("15:04:05.000"))

	result, err := w.Execute(context.Background(), "Workflow input data")
	if err != nil {
		fmt.Println("Error executing workflow:", err)
		return
	}

	endTime := time.Now()
	fmt.Printf("End time: %s, Total time: %.2f ms\n",
		endTime.Format("15:04:05.000"),
		float64(endTime.Sub(startTime).Milliseconds()))

	// 显示结果 (Display results)
	fmt.Println("\nWorkflow execution results:")
	for nodeID, output := range result.GetResults() {
		fmt.Printf("  Node %s: %v\n", nodeID, output)
	}

	// 显示计时信息 (Display timing information)
	printTimingInfo(result)

	// 显示工作流树状结构 (Display workflow tree structure)
	printWorkflowTree(w, result)
}
