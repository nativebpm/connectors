package jsonschema

import (
	"encoding/json"
	"testing"
)

func TestParseSchema(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"title": "Test User Form",
		"required": ["username", "email"],
		"ui:order": ["email", "username"],
		"properties": {
			"username": {
				"type": "string",
				"title": "Operator Name",
				"minLength": 3
			},
			"email": {
				"type": "string",
				"format": "email"
			},
			"approved": {
				"type": "boolean",
				"default": true
			},
			"role": {
				"type": "string",
				"enum": ["admin", "viewer"]
			}
		}
	}`

	// Test 1: Basic parsing with default values and ordering
	widgets, err := ParseSchema(schemaJSON, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(widgets) != 4 {
		t.Fatalf("expected 4 widgets, got %d", len(widgets))
	}

	// Verify order matching ui:order (email should be first, then username, then others alphabetically)
	if widgets[0].Name != "email" {
		t.Errorf("expected widgets[0] to be email, got %s", widgets[0].Name)
	}
	if widgets[1].Name != "username" {
		t.Errorf("expected widgets[1] to be username, got %s", widgets[1].Name)
	}

	// Verify required field constraints mapping
	if !widgets[0].Required {
		t.Errorf("expected email to be required")
	}
	if !widgets[1].Required {
		t.Errorf("expected username to be required")
	}
	if widgets[2].Required {
		t.Errorf("expected approved to not be required")
	}

	// Verify type & widget resolution
	if widgets[2].Widget != "switch" {
		t.Errorf("expected approved widget to be switch, got %s", widgets[2].Widget)
	}
	if widgets[2].Value != true {
		t.Errorf("expected approved default value to be true, got %v", widgets[2].Value)
	}

	// Verify enum select widget options
	roleWidget := widgets[3]
	if roleWidget.Widget != "select" {
		t.Errorf("expected role widget to be select, got %s", roleWidget.Widget)
	}
	if len(roleWidget.Options) != 2 || roleWidget.Options[0] != "admin" {
		t.Errorf("expected options to be ['admin', 'viewer'], got %v", roleWidget.Options)
	}

	// Test 2: Injected variables override defaults
	vars := map[string]interface{}{
		"username": "Autotest Operator",
		"approved": false,
	}

	widgets, err = ParseSchema(schemaJSON, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that value mapping worked correctly
	for _, w := range widgets {
		if w.Name == "username" && w.Value != "Autotest Operator" {
			t.Errorf("expected username value 'Autotest Operator', got %v", w.Value)
		}
		if w.Name == "approved" && w.Value != false {
			t.Errorf("expected approved value false, got %v", w.Value)
		}
	}
}

func TestValidateJSON(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"required": ["username"],
		"properties": {
			"username": {
				"type": "string",
				"minLength": 3
			},
			"age": {
				"type": "integer",
				"minimum": 18
			}
		}
	}`

	// Test case 1: Valid payload
	validPayload := `{"username": "gopher", "age": 20}`
	ok, errs, err := ValidateJSON(schemaJSON, validPayload)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if !ok {
		t.Errorf("expected payload to be valid, got validation errors: %v", errs)
	}

	// Test case 2: Invalid payload (username too short)
	invalidPayload := `{"username": "go", "age": 10}`
	ok, errs, err = ValidateJSON(schemaJSON, invalidPayload)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if ok {
		t.Errorf("expected validation to fail for invalid payload")
	}
	if len(errs) == 0 {
		t.Errorf("expected validation errors list to not be empty")
	}
}

func TestJSONSchemaBuilder(t *testing.T) {
	// Test builder with successful dynamic schema construction
	schemaJSON, err := NewBuilder().
		Title("Dynamic User Tasks").
		UIOrder("comments", "approved").
		AddString("comments").
			Label("Operator Comments").
			Required().
			MinLength(5).
			MaxLength(100).
			UIWidget("textarea").
		AddBoolean("approved").
			Label("Approve Workflow").
			Default(true).
		AddInteger("attempts").
			Label("Task Retries").
			Minimum(1).
			Maximum(5).
		BuildJSON()

	if err != nil {
		t.Fatalf("unexpected error building schema: %v", err)
	}

	// Unmarshal and verify properties
	var schema JSONSchema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		t.Fatalf("failed to unmarshal generated schema: %v", err)
	}

	if schema.Title != "Dynamic User Tasks" {
		t.Errorf("expected title 'Dynamic User Tasks', got %s", schema.Title)
	}

	if len(schema.UIOrder) != 2 || schema.UIOrder[0] != "comments" {
		t.Errorf("expected UIOrder ['comments', 'approved'], got %v", schema.UIOrder)
	}

	commentsProp := schema.Properties["comments"]
	if commentsProp == nil {
		t.Fatalf("comments property should exist")
	}
	if commentsProp.Type != "string" || commentsProp.Title != "Operator Comments" {
		t.Errorf("invalid comments property spec: %v", commentsProp)
	}
	if commentsProp.MinLength == nil || *commentsProp.MinLength != 5 {
		t.Errorf("expected minLength 5, got %v", commentsProp.MinLength)
	}
	if commentsProp.UIWidget != "textarea" {
		t.Errorf("expected UIWidget 'textarea', got %s", commentsProp.UIWidget)
	}

	approvedProp := schema.Properties["approved"]
	if approvedProp == nil {
		t.Fatalf("approved property should exist")
	}
	if approvedProp.Type != "boolean" || approvedProp.Default != true {
		t.Errorf("invalid approved property spec: %v", approvedProp)
	}

	attemptsProp := schema.Properties["attempts"]
	if attemptsProp == nil {
		t.Fatalf("attempts property should exist")
	}
	if attemptsProp.Type != "integer" || attemptsProp.Minimum == nil || *attemptsProp.Minimum != 1 {
		t.Errorf("invalid attempts property spec: %v", attemptsProp)
	}

	// Verify required array
	if len(schema.Required) != 1 || schema.Required[0] != "comments" {
		t.Errorf("expected required fields ['comments'], got %v", schema.Required)
	}
}

func TestJSONSchemaBuilderStickyErrors(t *testing.T) {
	// Test case: Invalid property name empty key should trigger sticky error
	_, err := NewBuilder().
		AddString("").
		Label("Empty Key").
		BuildJSON()

	if err == nil {
		t.Errorf("expected error for empty property name")
	}

	// Test case: Negative minLength should trigger sticky error
	_, err = NewBuilder().
		AddString("comments").
		MinLength(-10).
		BuildJSON()

	if err == nil {
		t.Errorf("expected error for negative minLength")
	}

	// Test case: Negative maxLength should trigger sticky error
	_, err = NewBuilder().
		AddString("comments").
		MaxLength(-5).
		BuildJSON()

	if err == nil {
		t.Errorf("expected error for negative maxLength")
	}
}
