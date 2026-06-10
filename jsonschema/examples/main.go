package main

import (
	"fmt"
	"log"

	"github.com/nativebpm/connectors/jsonschema"
)

func main() {
	fmt.Println("--- NativeBPM JSON Schema Fluent Builder Example ---")

	// 1. Build a dynamic form schema using the Fluent Builder API
	schemaJSON, err := jsonschema.NewBuilder().
		Title("Operator Task Review Form").
		UIOrder("operator_name", "approved", "rejection_reason").
		AddString("operator_name").
			Label("Operator Full Name").
			Required().
			MinLength(3).
		AddBoolean("approved").
			Label("Approve Instance?").
			Default(true).
		AddString("rejection_reason").
			Label("Rejection Reason").
			UIWidget("textarea").
		BuildJSON()

	if err != nil {
		log.Fatalf("Failed to build schema: %v", err)
	}

	fmt.Println("\nGenerated JSON Schema:")
	fmt.Println(schemaJSON)

	// 2. Parse the schema into UI Widgets layout list
	// Injecting pre-existing variables (simulate Edit/Review Mode)
	currentVariables := map[string]interface{}{
		"operator_name": "Ivan Gopherov",
		"approved":      false,
	}

	widgets, err := jsonschema.ParseSchema(schemaJSON, currentVariables)
	if err != nil {
		log.Fatalf("Failed to parse schema to widgets: %v", err)
	}

	fmt.Println("\nCompiled UI Widgets Layout Blueprint:")
	for i, w := range widgets {
		fmt.Printf("[%d] Name: %-16s | Label: %-20s | Widget: %-8s | Required: %-5t | Value: %v\n",
			i, w.Name, w.Label, w.Widget, w.Required, w.Value)
	}

	// 3. Strict validation of incoming submission payloads
	fmt.Println("\n--- Validating Submission Payloads ---")

	// Case A: Valid payload matching all constraints
	validPayload := `{"operator_name": "Ivan Gopherov", "approved": false, "rejection_reason": "Missing documents"}`
	ok, errs, err := jsonschema.ValidateJSON(schemaJSON, validPayload)
	if err != nil {
		log.Fatalf("Execution error during validation: %v", err)
	}
	fmt.Printf("Payload: %s\nValid? %t (Errors: %v)\n\n", validPayload, ok, errs)

	// Case B: Invalid payload (missing required operator_name)
	invalidPayload := `{"approved": true}`
	ok, errs, err = jsonschema.ValidateJSON(schemaJSON, invalidPayload)
	if err != nil {
		log.Fatalf("Execution error during validation: %v", err)
	}
	fmt.Printf("Payload: %s\nValid? %t (Errors: %v)\n\n", invalidPayload, ok, errs)

	// Case C: Invalid payload (operator_name is too short: minLength 3)
	tooShortPayload := `{"operator_name": "Io", "approved": true}`
	ok, errs, err = jsonschema.ValidateJSON(schemaJSON, tooShortPayload)
	if err != nil {
		log.Fatalf("Execution error during validation: %v", err)
	}
	fmt.Printf("Payload: %s\nValid? %t (Errors: %v)\n", tooShortPayload, ok, errs)
}
