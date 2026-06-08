package mermaid

import (
	"testing"

	"github.com/nativebpm/connectors/bpmn"
)

func TestGenerate(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL" id="Definitions_1">
  <bpmn:process id="order_process" isExecutable="true">
    <bpmn:startEvent id="start" name="Start" />
    <bpmn:serviceTask id="task1" name="Service Task" />
    <bpmn:endEvent id="end" name="End" />
    <bpmn:sequenceFlow id="flow1" sourceRef="start" targetRef="task1" />
    <bpmn:sequenceFlow id="flow2" sourceRef="task1" targetRef="end" />
  </bpmn:process>
</bpmn:definitions>`

	pp, err := bpmn.ParseBPMN([]byte(xmlData))
	if err != nil {
		t.Fatalf("failed to parse BPMN: %v", err)
	}

	result, err := Generate(pp)
	if err != nil {
		t.Fatalf("failed to generate Mermaid: %v", err)
	}

	expected := "```mermaid\ngraph TD\n" +
		"    end([End: End])\n" +
		"    start([Start: Start])\n" +
		"    task1[Service Task: Service Task]\n" +
		"    start --> task1\n" +
		"    task1 --> end\n" +
		"```"

	if result != expected {
		t.Errorf("expected:\n%s\n\ngot:\n%s", expected, result)
	}
}
