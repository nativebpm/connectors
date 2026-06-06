package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// Validate checks if the provided 6-digit TOTP token is valid for the current time
// using the base32-encoded secret. It allows for a drift of 1 step (30 seconds)
// before and after to accommodate clock desynchronization.
func Validate(token string, secret string) bool {
	return ValidateAt(token, secret, time.Now().Unix())
}

// ValidateAt checks if the provided 6-digit TOTP token is valid for the given timestamp
// using the base32-encoded secret, with a 1-step drift tolerance.
func ValidateAt(token string, secret string, timestamp int64) bool {
	return validateAt(token, secret, timestamp, 1)
}

// Generate creates a 6-digit TOTP token for a specific Unix timestamp and base32 secret.
func Generate(secret string, timestamp int64) (string, error) {
	// Normalize secret: check case to avoid allocating ToUpper if already uppercase
	secret = strings.TrimSpace(secret)
	hasLower := false
	for i := 0; i < len(secret); i++ {
		if secret[i] >= 'a' && secret[i] <= 'z' {
			hasLower = true
			break
		}
	}
	if hasLower {
		secret = strings.ToUpper(secret)
	}

	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		key, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return "", fmt.Errorf("failed to decode base32 secret: %w", err)
		}
	}

	step := timestamp / 30
	return generateToken(key, step)
}

// validateAt performs the actual TOTP validation without allocating a Validator builder.
func validateAt(token string, secret string, timestamp int64, drift int) bool {
	secret = strings.TrimSpace(secret)
	hasLower := false
	for i := 0; i < len(secret); i++ {
		if secret[i] >= 'a' && secret[i] <= 'z' {
			hasLower = true
			break
		}
	}
	if hasLower {
		secret = strings.ToUpper(secret)
	}

	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		key, err = base32.StdEncoding.DecodeString(secret)
		if err != nil {
			return false
		}
	}

	token = strings.TrimSpace(token)
	if len(token) != 6 {
		return false
	}

	step := timestamp / 30

	// Check steps within drift tolerance
	for d := int64(-drift); d <= int64(drift); d++ {
		candidate, err := generateToken(key, step+d)
		if err == nil && candidate == token {
			return true
		}
	}

	return false
}

// Validator is a fluent builder for TOTP validation.
type Validator struct {
	secret    string
	token     string
	timestamp int64
	drift     int
}

// NewValidator creates a new fluent Validator builder.
func NewValidator() *Validator {
	return &Validator{
		timestamp: time.Now().Unix(),
		drift:     1,
	}
}

// Secret sets the base32-encoded TOTP secret.
func (v *Validator) Secret(s string) *Validator {
	v.secret = s
	return v
}

// Token sets the 6-digit verification token.
func (v *Validator) Token(t string) *Validator {
	v.token = t
	return v
}

// Time sets the timestamp for validation from a time.Time object.
func (v *Validator) Time(t time.Time) *Validator {
	v.timestamp = t.Unix()
	return v
}

// Timestamp sets the timestamp for validation in Unix seconds.
func (v *Validator) Timestamp(ts int64) *Validator {
	v.timestamp = ts
	return v
}

// Drift sets the drift step tolerance (e.g. 1 means +/- 30 seconds).
func (v *Validator) Drift(d int) *Validator {
	v.drift = d
	return v
}

// Validate executes the validation and returns true if the token is valid.
func (v *Validator) Validate() bool {
	return validateAt(v.token, v.secret, v.timestamp, v.drift)
}

// Generator is a fluent builder for TOTP generation.
type Generator struct {
	secret    string
	timestamp int64
}

// NewGenerator creates a new fluent Generator builder.
func NewGenerator() *Generator {
	return &Generator{
		timestamp: time.Now().Unix(),
	}
}

// Secret sets the base32-encoded TOTP secret.
func (g *Generator) Secret(s string) *Generator {
	g.secret = s
	return g
}

// Time sets the timestamp for generation from a time.Time object.
func (g *Generator) Time(t time.Time) *Generator {
	g.timestamp = t.Unix()
	return g
}

// Timestamp sets the timestamp for generation in Unix seconds.
func (g *Generator) Timestamp(ts int64) *Generator {
	g.timestamp = ts
	return g
}

// Generate creates the 6-digit TOTP token.
func (g *Generator) Generate() (string, error) {
	return Generate(g.secret, g.timestamp)
}

// generateToken creates a 6-digit token using the decoded key and the step counter.
func generateToken(key []byte, step int64) (string, error) {
	// Prepare the 8-byte big-endian step counter payload on stack
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(step))

	// Generate HMAC-SHA1 hash
	mac := hmac.New(sha1.New, key)
	_, err := mac.Write(buf[:])
	if err != nil {
		return "", err
	}
	hash := mac.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0xf
	binaryVal := binary.BigEndian.Uint32(hash[offset : offset+4])
	binaryVal &= 0x7fffffff // clear signed bit

	// Format as a 6-digit zero-padded number without fmt.Sprintf allocations
	otpVal := binaryVal % 1000000
	return format6Digits(otpVal), nil
}

// format6Digits formats a 6-digit number into a string with exactly 1 allocation.
func format6Digits(val uint32) string {
	var buf [6]byte
	buf[5] = byte('0' + val%10)
	val /= 10
	buf[4] = byte('0' + val%10)
	val /= 10
	buf[3] = byte('0' + val%10)
	val /= 10
	buf[2] = byte('0' + val%10)
	val /= 10
	buf[1] = byte('0' + val%10)
	val /= 10
	buf[0] = byte('0' + val%10)
	return string(buf[:])
}
