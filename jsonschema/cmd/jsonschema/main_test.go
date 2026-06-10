package main

import (
	"strings"
	"testing"
)

func TestValidator(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"required": ["score"],
		"properties": {
			"score": {
				"type": "integer",
				"minimum": 0,
				"maximum": 100
			},
			"notes": {
				"type": "string",
				"minLength": 5,
				"pattern": "^[A-Za-z]+$"
			}
		}
	}`

	schema, err := ParseSchema(schemaJSON)
	if err != nil {
		t.Fatalf("ParseSchema failed: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got: '%s'", schema.Type)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "score" {
		t.Errorf("Expected required field 'score', got: %v", schema.Required)
	}

	scoreProp := schema.Properties["score"]
	if scoreProp == nil || scoreProp.Type != "integer" {
		t.Fatalf("Expected integer score property")
	}
	if scoreProp.Minimum == nil || *scoreProp.Minimum != 0 {
		t.Errorf("Expected Minimum 0")
	}

	// 1. Valid validation
	dataOk := `{"score": 95, "notes": "Approved"}`
	valid, errs := Validate(schema, dataOk)
	if !valid || len(errs) > 0 {
		t.Errorf("Expected valid payload, got valid=%t, errs=%v", valid, errs)
	}

	// 2. Missing required field
	dataMissing := `{"notes": "Approved"}`
	valid, errs = Validate(schema, dataMissing)
	if valid || len(errs) != 1 {
		t.Errorf("Expected invalid payload, got valid=%t, errs=%v", valid, errs)
	}
	if !strings.Contains(errs[0], "required") {
		t.Errorf("Expected required error message, got: %s", errs[0])
	}

	// 3. String length and pattern violation
	dataBadNotes := `{"score": 50, "notes": "No1"}`
	valid, errs = Validate(schema, dataBadNotes)
	if valid || len(errs) != 2 {
		t.Errorf("Expected 2 errors, got: %v", errs)
	}
}

func TestCompileWidgets(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"required": ["score"],
		"properties": {
			"score": {
				"type": "integer",
				"minimum": 0,
				"maximum": 100,
				"x-step": 2
			},
			"notes": {
				"type": "string",
				"minLength": 5,
				"ui:widget": "textarea"
			},
			"approved": {
				"type": "boolean",
				"default": true
			}
		}
	}`

	schema, err := ParseSchema(schemaJSON)
	if err != nil {
		t.Fatalf("ParseSchema failed: %v", err)
	}

	variables := map[string]any{
		"score":    float64(85),
		"approved": false,
	}

	widgets := CompileWidgets(schema, variables)
	if len(widgets) != 3 {
		t.Fatalf("Expected 3 widgets, got %d", len(widgets))
	}

	// Order is alphabetical: approved, notes, score
	if widgets[0].Name != "approved" || widgets[0].Widget != "switch" || widgets[0].Value != false {
		t.Errorf("Unexpected widget at index 0: %+v", widgets[0])
	}
	if widgets[1].Name != "notes" || widgets[1].Widget != "textarea" || widgets[1].Value != nil {
		t.Errorf("Unexpected widget at index 1: %+v", widgets[1])
	}
	if widgets[2].Name != "score" || widgets[2].Widget != "number" || widgets[2].Value != float64(85) || widgets[2].XStep != 2 {
		t.Errorf("Unexpected widget at index 2: %+v", widgets[2])
	}
}

