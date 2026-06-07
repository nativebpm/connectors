package main

import (
	"encoding/json"
	"unsafe"
)

// Declare shared static buffer (1MB) to communicate with host.
const maxBufferSize = 1024 * 1024
var exchangeBuffer [maxBufferSize]byte

// Host imports
//go:wasmimport env checkpoint
func hostCheckpoint()

//go:wasmimport env host_call_api
func hostCallAPI(apiNamePtr uint32, apiNameLen uint32, reqPtr uint32, reqLen uint32, respPtr uint32, respMaxLen uint32) int32

func main() {}

//export get_exchange_buffer_pointer
func getExchangeBufferPointer() uint32 {
	return uint32(uintptr(unsafe.Pointer(&exchangeBuffer[0])))
}

// Graph Definition structures
type GraphDefinition struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Nodes       map[string]GraphNode `json:"nodes"`
	Connections []Connection         `json:"connections"`
	StartNodeID string               `json:"start_node_id"`
}

type GraphNode struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "StartEvent", "EndEvent", "ServiceTask", "UserTask", "ExclusiveGateway"
	Name string `json:"name"`
}

type Connection struct {
	ID        string `json:"id"`
	SourceRef string `json:"source_ref"`
	TargetRef string `json:"target_ref"`
	Condition string `json:"condition,omitempty"`
}

// Process Instance State structure
type ProcessInstance struct {
	ID                       string                 `json:"id"`
	ProcessID                string                 `json:"process_id"`
	ActiveActivityInstances  []string               `json:"active_activity_instances"`
	WaitingActivityInstances []string               `json:"waiting_activity_instances"`
	CompletedNodes           []string               `json:"completed_nodes"`
	Variables                map[string]interface{} `json:"variables"`
	Completed                bool                   `json:"completed"`
}

// runVM runs the traversal loop for the process instance.
func runVM(graph *GraphDefinition, instance *ProcessInstance) {
	for len(instance.ActiveActivityInstances) > 0 {
		currentNodeID := instance.ActiveActivityInstances[0]
		instance.ActiveActivityInstances = instance.ActiveActivityInstances[1:]

		node, exists := graph.Nodes[currentNodeID]
		if !exists {
			println("[WASM VM] Error: node not found: " + currentNodeID)
			return
		}

		instance.CompletedNodes = append(instance.CompletedNodes, currentNodeID)

		switch node.Type {
		case "StartEvent":
			// Transition to outbound connections
			transitionOutbound(graph, instance, currentNodeID)
		case "ExclusiveGateway":
			// Evaluate condition and transition
			transitionOutbound(graph, instance, currentNodeID)
		case "ServiceTask":
			// Trigger host API call or pause
			// For Phase 1, we execute service task via host API call or pause
			payload, _ := json.Marshal(map[string]string{
				"instance_id": instance.ID,
				"task_id":     currentNodeID,
			})
			apiName := "execute_service_task"
			respBuf := make([]byte, 1024)

			// Checkpoint before executing service task logic to guarantee crash recovery
			hostCheckpoint()

			res := hostCallAPI(
				uint32(uintptr(unsafe.Pointer(&[]byte(apiName)[0]))), uint32(len(apiName)),
				uint32(uintptr(unsafe.Pointer(&payload[0]))), uint32(len(payload)),
				uint32(uintptr(unsafe.Pointer(&respBuf[0]))), uint32(len(respBuf)),
			)

			if res >= 0 {
				// Success, transition out
				transitionOutbound(graph, instance, currentNodeID)
			} else {
				// Failed or paused, put on waiting list and save checkpoint
				instance.WaitingActivityInstances = append(instance.WaitingActivityInstances, currentNodeID)
				hostCheckpoint()
				return
			}
		case "UserTask":
			// User tasks are always wait states
			instance.WaitingActivityInstances = append(instance.WaitingActivityInstances, currentNodeID)
			hostCheckpoint()
			return
		case "EndEvent":
			if len(instance.ActiveActivityInstances) == 0 && len(instance.WaitingActivityInstances) == 0 {
				instance.Completed = true
			}
		}
	}
}

func transitionOutbound(graph *GraphDefinition, instance *ProcessInstance, sourceRef string) {
	for _, conn := range graph.Connections {
		if conn.SourceRef == sourceRef {
			// In Phase 1, we assume sequence flows trigger immediately.
			instance.ActiveActivityInstances = append(instance.ActiveActivityInstances, conn.TargetRef)
		}
	}
}

// Execute starts a new process instance.
//export execute
func execute(graphLen uint32, variablesLen uint32) int32 {
	if graphLen+variablesLen > maxBufferSize {
		return -1
	}

	graphBytes := exchangeBuffer[:graphLen]
	variablesBytes := exchangeBuffer[graphLen : graphLen+variablesLen]

	var graph GraphDefinition
	if err := json.Unmarshal(graphBytes, &graph); err != nil {
		println("[WASM VM] Failed to unmarshal graph definition:", err.Error())
		return -2
	}

	var vars map[string]interface{}
	if err := json.Unmarshal(variablesBytes, &vars); err != nil {
		println("[WASM VM] Failed to unmarshal variables:", err.Error())
		return -3
	}

	instance := &ProcessInstance{
		ID:                      "inst_" + graph.ID,
		ProcessID:               graph.ID,
		ActiveActivityInstances: []string{graph.StartNodeID},
		CompletedNodes:          []string{},
		Variables:               vars,
		Completed:               false,
	}

	runVM(&graph, instance)

	resBytes, err := json.Marshal(instance)
	if err != nil {
		return -4
	}

	copy(exchangeBuffer[:], resBytes)
	return int32(len(resBytes))
}

// Resume continues execution after completing a task.
//export resume
func resume(graphLen uint32, instanceLen uint32, completedTaskIDPtr uint32, completedTaskIDLen uint32) int32 {
	graphBytes := exchangeBuffer[:graphLen]
	instanceBytes := exchangeBuffer[graphLen : graphLen+instanceLen]

	var graph GraphDefinition
	if err := json.Unmarshal(graphBytes, &graph); err != nil {
		return -2
	}

	var instance ProcessInstance
	if err := json.Unmarshal(instanceBytes, &instance); err != nil {
		return -3
	}

	// Read completed task ID
	memBytes := exchangeBuffer[:]
	taskIDBytes := memBytes[completedTaskIDPtr : completedTaskIDPtr+completedTaskIDLen]
	completedTaskID := string(taskIDBytes)

	// Remove from waiting list
	found := false
	var newWaiting []string
	for _, id := range instance.WaitingActivityInstances {
		if id == completedTaskID {
			found = true
		} else {
			newWaiting = append(newWaiting, id)
		}
	}
	instance.WaitingActivityInstances = newWaiting

	if found {
		// Complete the wait node and transition outbound
		instance.CompletedNodes = append(instance.CompletedNodes, completedTaskID)
		transitionOutbound(&graph, &instance, completedTaskID)
		runVM(&graph, &instance)
	}

	resBytes, err := json.Marshal(instance)
	if err != nil {
		return -4
	}

	copy(exchangeBuffer[:], resBytes)
	return int32(len(resBytes))
}
