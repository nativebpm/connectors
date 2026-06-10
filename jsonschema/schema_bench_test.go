package jsonschema

import (
	"testing"
)

func BenchmarkParseSchema(b *testing.B) {
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseSchema(schemaJSON, nil)
		if err != nil {
			b.Fatalf("failed to parse schema: %v", err)
		}
	}
}

func BenchmarkValidateJSON(b *testing.B) {
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
	payloadJSON := `{"username": "gopher", "age": 20}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok, _, err := ValidateJSON(schemaJSON, payloadJSON)
		if err != nil {
			b.Fatalf("failed to validate JSON: %v", err)
		}
		if !ok {
			b.Fatalf("expected validation to pass")
		}
	}
}
