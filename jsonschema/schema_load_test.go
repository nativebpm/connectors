package jsonschema

import (
	"sync"
	"testing"
)

func TestLoadValidateJSONParallel(t *testing.T) {
	schemaJSON := `{
		"type": "object",
		"required": ["username", "email"],
		"properties": {
			"username": {
				"type": "string",
				"minLength": 3
			},
			"email": {
				"type": "string",
				"format": "email"
			},
			"age": {
				"type": "integer",
				"minimum": 18
			}
		}
	}`
	payloadJSON := `{"username": "gopher", "email": "gopher@golang.org", "age": 20}`

	const concurrency = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ok, errs, err := ValidateJSON(schemaJSON, payloadJSON)
				if err != nil {
					t.Errorf("execution error: %v", err)
					return
				}
				if !ok {
					t.Errorf("validation failed unexpectedly: %v", errs)
					return
				}
			}
		}()
	}

	wg.Wait()
}
