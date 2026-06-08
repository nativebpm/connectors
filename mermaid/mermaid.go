package mermaid

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/nativebpm/connectors/bpmn"
)

// Generate converts a parsed BPMN process to a Mermaid TD flowchart string.
func Generate(pp *bpmn.ParsedProcess) (string, error) {
	if pp == nil {
		return "", fmt.Errorf("parsed process cannot be nil")
	}

	var buf bytes.Buffer
	buf.WriteString("```mermaid\ngraph TD\n")

	// Collect and sort node IDs for deterministic output order
	nodeIDs := make([]string, 0, len(pp.Nodes))
	for id := range pp.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Helper function to return name or fallback to ID
	nameOrID := func(name, id string) string {
		if name != "" {
			return name
		}
		return id
	}

	// 1. Render all nodes with appropriate shapes
	for _, id := range nodeIDs {
		node := pp.Nodes[id]
		switch n := node.(type) {
		case bpmn.StartEvent:
			buf.WriteString(fmt.Sprintf("    %s([Start: %s])\n", id, nameOrID(n.Name, id)))
		case bpmn.EndEvent:
			buf.WriteString(fmt.Sprintf("    %s([End: %s])\n", id, nameOrID(n.Name, id)))
		case bpmn.ServiceTask:
			buf.WriteString(fmt.Sprintf("    %s[Service Task: %s]\n", id, nameOrID(n.Name, id)))
		case bpmn.UserTask:
			buf.WriteString(fmt.Sprintf("    %s[User Task: %s]\n", id, nameOrID(n.Name, id)))
		case bpmn.ExclusiveGateway:
			buf.WriteString(fmt.Sprintf("    %s{XOR Gateway: %s}\n", id, nameOrID(n.Name, id)))
		case bpmn.ParallelGateway:
			buf.WriteString(fmt.Sprintf("    %s{AND Gateway: %s}\n", id, nameOrID(n.Name, id)))
		case bpmn.InclusiveGateway:
			buf.WriteString(fmt.Sprintf("    %s{OR Gateway: %s}\n", id, nameOrID(n.Name, id)))
		case bpmn.CallActivity:
			buf.WriteString(fmt.Sprintf("    %s[[Call Activity: %s]]\n", id, nameOrID(n.Name, id)))
		default:
			buf.WriteString(fmt.Sprintf("    %s[%s]\n", id, id))
		}
	}

	// 2. Render sequence flows
	for _, id := range nodeIDs {
		flows := pp.Outflows[id]
		for _, flow := range flows {
			if flow.ConditionExpression != nil && flow.ConditionExpression.Text != "" {
				buf.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", flow.SourceRef, flow.ConditionExpression.Text, flow.TargetRef))
			} else {
				buf.WriteString(fmt.Sprintf("    %s --> %s\n", flow.SourceRef, flow.TargetRef))
			}
		}
	}

	buf.WriteString("```")
	return buf.String(), nil
}
