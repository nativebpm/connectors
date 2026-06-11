package totp

import (
	"testing"
	"time"
)

func TestTOTPGenerationAndValidation(t *testing.T) {
	// Secret key "JBSWY3DPEHPK3PXP" (base32 for "Hello!")
	secret := "JBSWY3DPEHPK3PXP"

	// Let's generate a token for a fixed timestamp
	timestamp := int64(1700000000) // 2023-11-14 22:13:20 UTC
	token, err := Generate(secret, timestamp)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify the token format (must be 6 digits)
	if len(token) != 6 {
		t.Errorf("Expected token length of 6, got %d (value: %s)", len(token), token)
	}

	// Verify validation succeeds for the generated token at the same timestamp
	if !ValidateAt(token, secret, timestamp) {
		t.Errorf("Validation failed for token: %s", token)
	}

	// Verify validation fails for an incorrect token
	if ValidateAt("123456", secret, timestamp) {
		t.Error("Validation succeeded for incorrect token '123456'")
	}

	// Verify validation fails for incorrect secret
	if ValidateAt(token, "JBSWY3DPEHPK3PXQ", timestamp) {
		t.Error("Validation succeeded for correct token but incorrect secret")
	}
}

func TestTOTPDriftTolerance(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	baseTime := int64(1700000000)

	// Token generated at baseTime
	token, err := Generate(secret, baseTime)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 1. Same time step validation (0 drift)
	if !ValidateAt(token, secret, baseTime) {
		t.Error("Failed to validate token at exact time of generation")
	}

	// 2. Validate at 15 seconds after (same time step: 1700000000/30 == 1700000015/30)
	if !ValidateAt(token, secret, baseTime+15) {
		t.Error("Failed to validate token at time + 15 seconds (same step)")
	}

	// 3. Validate at 30 seconds after (drift of +1 step: 1700000030/30 == step + 1)
	if !ValidateAt(token, secret, baseTime+30) {
		t.Error("Failed to validate token at time + 30 seconds (+1 step drift)")
	}

	// 4. Validate at 30 seconds before (drift of -1 step: 1699999970/30 == step - 1)
	if !ValidateAt(token, secret, baseTime-30) {
		t.Error("Failed to validate token at time - 30 seconds (-1 step drift)")
	}

	// 5. Verify a token 60 seconds in the future (+2 steps) fails
	if ValidateAt(token, secret, baseTime+60) {
		t.Error("Validation succeeded for token + 60 seconds (out of drift bounds)")
	}

	// 6. Verify a token 60 seconds in the past (-2 steps) fails
	if ValidateAt(token, secret, baseTime-60) {
		t.Error("Validation succeeded for token - 60 seconds (out of drift bounds)")
	}
}

func TestTOTPFluentAPI(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	baseTime := time.Unix(1700000000, 0)

	token, err := NewGenerator().
		Secret(secret).
		Time(baseTime).
		Generate()
	if err != nil {
		t.Fatalf("Fluent Generator failed: %v", err)
	}

	if len(token) != 6 {
		t.Fatalf("Fluent Generator token has invalid length: %d", len(token))
	}

	isValid := NewValidator().
		Secret(secret).
		Token(token).
		Time(baseTime).
		Drift(1).
		Validate()
	if !isValid {
		t.Error("Fluent Validator failed with exact time")
	}

	// Test with +30 seconds time and drift=1
	isValidDrift := NewValidator().
		Secret(secret).
		Token(token).
		Time(baseTime.Add(30 * time.Second)).
		Drift(1).
		Validate()
	if !isValidDrift {
		t.Error("Fluent Validator failed with +30s drift and drift tolerance = 1")
	}
}

func BenchmarkTOTPGenerate(b *testing.B) {
	secret := "JBSWY3DPEHPK3PXP"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = Generate(secret, 1700000000)
	}
}

func BenchmarkTOTPValidate(b *testing.B) {
	secret := "JBSWY3DPEHPK3PXP"
	token := "067645"
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ValidateAt(token, secret, 1700000000)
	}
}

