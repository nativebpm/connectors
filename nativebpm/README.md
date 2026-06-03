# NativeBPM Go SDK Client

The official Go SDK client for interacting with the closed-source `nativebpm` engine.

---

## Installation

Add the SDK dependency to your `go.mod`:

```bash
go get github.com/nativebpm/connectors/nativebpm
```

---

## Quick Start (Fluent API)

Below is a complete example demonstrating client initialization, process deployment, starting a new process instance, and completing user tasks using the Fluent API builders.

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/nativebpm/connectors/nativebpm"
)

func main() {
	logger := slog.Default()
	ctx := context.Background()

	// 1. Initialize SDK Client
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		logger.Error("Failed to initialize NativeBPM client", "error", err)
		return
	}

	// 2. Upload/Deploy BPMN Process Definition (Fluent API)
	xmlData, err := os.ReadFile("process.bpmn")
	if err != nil {
		logger.Error("Failed to read BPMN file", "error", err)
		return
	}

	deployResp, err := client.Deploy("loanApproval", "Loan Approval").
		XML(xmlData).
		Send(ctx)
	if err != nil {
		logger.Error("Failed to deploy process schema", "error", err)
		return
	}
	logger.Info("Process deployed successfully", "processID", deployResp.ProcessID)

	// 3. Start a new Process Instance (Fluent API)
	pi, err := client.StartProcessInstance("loanApproval").
		InstanceID("instance-1234").
		Variable("amount", 25000).
		Variable("user", "john_doe").
		Send(ctx)
	if err != nil {
		logger.Error("Failed to start process instance", "error", err)
		return
	}
	logger.Info("Process instance started", "instanceID", pi.ID, "waitingTasks", pi.WaitingTokens)

	// 4. Complete User Task / Wait State (Fluent API)
	if len(pi.WaitingTokens) > 0 {
		userTaskID := pi.WaitingTokens[0] // e.g. "taskApprove"
		logger.Info("Completing task", "taskID", userTaskID)

		pi, err = client.CompleteTask(pi.ID, userTaskID).
			Variable("approved", true).
			Send(ctx)
		if err != nil {
			logger.Error("Failed to complete task", "error", err)
			return
		}
		logger.Info("Task completed", "isCompleted", pi.Completed)
	}

	// 5. Query Audit Logs
	logs, err := client.GetInstanceLogs(ctx, "instance-1234")
	if err != nil {
		logger.Error("Failed to get logs", "error", err)
		return
	}
	for _, l := range logs {
		logger.Info("Audit Log", "nodeID", l.NodeID, "nodeName", l.NodeName, "action", l.Action)
	}
}
```

---

## API Capabilities

The SDK client provides builder pattern methods (Fluent API) for action requests, and direct methods for simple queries:

| SDK Method | HTTP REST Target | Description |
|:---|:---|:---|
| `client.Deploy(id, name).XML(xml).Send(ctx)` | `POST /api/deploy` | Uploads and deploys process schema. |
| `client.StartProcessInstance(procID).InstanceID(id).Variable(k,v).Send(ctx)` | `POST /api/definitions/{id}/start` | Starts a new process instance. |
| `client.CompleteTask(instID, nodeID).Variable(k,v).Send(ctx)` | `POST /api/instances/{id}/complete` | Advances wait state by completing a task. |
| `client.ListDefinitions(ctx)` | `GET /api/definitions` | Lists all active process definitions. |
| `client.ListInstances(ctx)` | `GET /api/instances` | Lists all execution instance records. |
| `client.GetInstance(ctx, id)` | `GET /api/instances/{id}` | Fetches active state metadata. |
| `client.GetInstanceLogs(ctx, id)` | `GET /api/instances/{id}/logs` | Retrieves audit trail log events. |

---

## Runnable Examples

For complete, ready-to-run examples and benchmarks, check out the **[examples](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples)** directory:

*   **[Simple Example](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/simple)**: Demonstrates process deployment, instance startup, task completion, and audit log query using a simple User Task process.
*   **[Complex Example](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/complex)**: Explains exclusive gateway branching (`XOR`) with conditions evaluating active workflow routing.
*   **[Load Testing](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/loadtest)**: High-concurrency performance benchmark demonstrating execution throughputs up to **750+ RPS** against the optimized PostgreSQL backend.
*   **[Hybrid Camunda Bridge](file:///Users/user/github.com/nativebpm/connectors/nativebpm/examples/hybrid-camunda)**: Adapts an external Camunda engine with the NativeBPM platform to delegate task execution.
