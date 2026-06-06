package cryptenv

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"filippo.io/age"
	"github.com/joho/godotenv"
)

// SecureEnv manages environment secrets in-memory by keeping them encrypted
// with AES-256-GCM. Secrets are only decrypted in-memory on the fly when requested.
type SecureEnv struct {
	mu      sync.RWMutex
	aesKey  []byte
	secrets map[string][]byte // Map of encrypted secrets
	nonces  map[string][]byte // Map of AES-GCM nonces for each secret
}

// NewSecureEnv initializes a new SecureEnv container using a masterKey.
// If the masterKey is empty, an error is returned.
func NewSecureEnv(masterKey string) (*SecureEnv, error) {
	if masterKey == "" {
		return nil, errors.New("master key cannot be empty")
	}

	// Derive a 32-byte AES key from the masterKey using SHA-256
	hash := sha256.Sum256([]byte(masterKey))

	return &SecureEnv{
		aesKey:  hash[:],
		secrets: make(map[string][]byte),
		nonces:  make(map[string][]byte),
	}, nil
}

// Get decrypts and returns a secret value for the given key.
// Returns an error if the key does not exist or decryption fails.
func (se *SecureEnv) Get(key string) (string, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	ciphertext, ok := se.secrets[key]
	if !ok {
		return "", fmt.Errorf("secret key %q not found", key)
	}
	nonce := se.nonces[key]

	block, err := aes.NewCipher(se.aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret (invalid master key?): %w", err)
	}

	return string(plaintext), nil
}

// GetDefault retrieves the secret or returns a default value if not found.
func (se *SecureEnv) GetDefault(key string, defaultValue string) string {
	val, err := se.Get(key)
	if err != nil {
		return defaultValue
	}
	return val
}

// Set encrypts and stores a secret key-value pair.
func (se *SecureEnv) Set(key, value string) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	block, err := aes.NewCipher(se.aesKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, []byte(value), nil)

	se.secrets[key] = ciphertext
	se.nonces[key] = nonce
	return nil
}

// LoadFromMap encrypts and loads all secrets from a raw key-value map.
func (se *SecureEnv) LoadFromMap(kv map[string]string) error {
	for k, v := range kv {
		if err := se.Set(k, v); err != nil {
			return err
		}
	}
	return nil
}

// LoadEnv loads all current environment variables (os.Environ()) into the secure store.
func (se *SecureEnv) LoadEnv() error {
	for _, env := range os.Environ() {
		parts := stringsSplitN(env, "=", 2)
		if len(parts) == 2 {
			if err := se.Set(parts[0], parts[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadAgeSymmetric decrypts an age-encrypted env file using scrypt passphrase-based decryption.
// The provided passphrase is used as the age identity.
func (se *SecureEnv) LoadAgeSymmetric(filepath string, passphrase string) error {
	if passphrase == "" {
		return errors.New("passphrase cannot be empty for symmetric age decryption")
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open age file: %w", err)
	}
	defer file.Close()

	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return fmt.Errorf("failed to create scrypt identity: %w", err)
	}

	decryptedReader, err := age.Decrypt(file, identity)
	if err != nil {
		return fmt.Errorf("failed to decrypt age file: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, decryptedReader); err != nil {
		return fmt.Errorf("failed to read decrypted age stream: %w", err)
	}

	envMap, err := godotenv.Unmarshal(buf.String())
	if err != nil {
		return fmt.Errorf("failed to parse decrypted env content: %w", err)
	}

	return se.LoadFromMap(envMap)
}

// LoadAgeAsymmetric decrypts an age-encrypted env file using asymmetric key-based identities.
func (se *SecureEnv) LoadAgeAsymmetric(filepath string, identities ...age.Identity) error {
	if len(identities) == 0 {
		return errors.New("at least one age identity must be provided for asymmetric decryption")
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open age file: %w", err)
	}
	defer file.Close()

	decryptedReader, err := age.Decrypt(file, identities...)
	if err != nil {
		return fmt.Errorf("failed to decrypt age file: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, decryptedReader); err != nil {
		return fmt.Errorf("failed to read decrypted age stream: %w", err)
	}

	envMap, err := godotenv.Unmarshal(buf.String())
	if err != nil {
		return fmt.Errorf("failed to parse decrypted env content: %w", err)
	}

	return se.LoadFromMap(envMap)
}

// LoadAES decrypts a raw AES-256-GCM encrypted env file using the derived AES key.
func (se *SecureEnv) LoadAES(filepath string) error {
	ciphertext, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read AES file: %w", err)
	}

	block, err := aes.NewCipher(se.aesKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return errors.New("ciphertext too short")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt AES file (invalid master key?): %w", err)
	}

	envMap, err := godotenv.Unmarshal(string(plaintext))
	if err != nil {
		return fmt.Errorf("failed to parse decrypted env content: %w", err)
	}

	return se.LoadFromMap(envMap)
}

// SaveAES encrypts and saves a key-value map as a raw AES-256-GCM file using the derived AES key.
func (se *SecureEnv) SaveAES(filepath string, kv map[string]string) error {
	envString, err := godotenv.Marshal(kv)
	if err != nil {
		return fmt.Errorf("failed to marshal env map: %w", err)
	}

	block, err := aes.NewCipher(se.aesKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, []byte(envString), nil)
	finalData := append(nonce, ciphertext...)

	if err := os.WriteFile(filepath, finalData, 0600); err != nil {
		return fmt.Errorf("failed to write AES file: %w", err)
	}

	return nil
}

// Simple helper to avoid external strings import where possible
func stringsSplitN(s, sep string, n int) []string {
	if n == 0 {
		return nil
	}
	if sep == "" {
		return []string{s}
	}
	var res []string
	start := 0
	for {
		if n > 0 && len(res) == n-1 {
			res = append(res, s[start:])
			break
		}
		idx := stringsIndex(s[start:], sep)
		if idx == -1 {
			res = append(res, s[start:])
			break
		}
		res = append(res, s[start:start+idx])
		start += idx + len(sep)
	}
	return res
}

func stringsIndex(s, substr string) int {
	n := len(s)
	m := len(substr)
	if m == 0 {
		return 0
	}
	if n < m {
		return -1
	}
	for i := 0; i <= n-m; i++ {
		if s[i:i+m] == substr {
			return i
		}
	}
	return -1
}
