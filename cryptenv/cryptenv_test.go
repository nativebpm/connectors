package cryptenv

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestNewSecureEnvEmptyMasterKey(t *testing.T) {
	_, err := NewSecureEnv("")
	if err == nil {
		t.Fatal("expected error when initializing with empty master key")
	}
}

func TestGetSetAndDefault(t *testing.T) {
	se, err := NewSecureEnv("master-password-123")
	if err != nil {
		t.Fatalf("failed to create SecureEnv: %v", err)
	}

	err = se.Set("DATABASE_URL", "postgres://localhost/db")
	if err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}

	val, err := se.Get("DATABASE_URL")
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}

	if val != "postgres://localhost/db" {
		t.Errorf("expected %q, got %q", "postgres://localhost/db", val)
	}

	// Test default value
	if val := se.GetDefault("NON_EXISTENT", "default-value"); val != "default-value" {
		t.Errorf("expected default value, got %q", val)
	}

	if val := se.GetDefault("DATABASE_URL", "default-value"); val != "postgres://localhost/db" {
		t.Errorf("expected actual secret value, got %q", val)
	}
}

func TestInvalidMasterKey(t *testing.T) {
	se, err := NewSecureEnv("master-password-123")
	if err != nil {
		t.Fatalf("failed to create SecureEnv: %v", err)
	}

	if err := se.Set("KEY", "secret-value"); err != nil {
		t.Fatalf("failed to set: %v", err)
	}

	// Create another secure env with a different master key but same map structure mock
	se2, err := NewSecureEnv("different-password")
	if err != nil {
		t.Fatalf("failed to create second SecureEnv: %v", err)
	}

	// Copy encrypted value and nonce manually
	se2.secrets["KEY"] = se.secrets["KEY"]
	se2.nonces["KEY"] = se.nonces["KEY"]

	_, err = se2.Get("KEY")
	if err == nil {
		t.Fatal("expected decryption to fail with invalid master key")
	}
}

func TestLoadAgeSymmetric(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "secrets.env.age")

	passphrase := "super-master-key-passphrase"
	rawEnvContent := "DB_USER=test_db_user\nDB_PASS=secret_pass_123"

	// Encrypt programmatically using filippo.io/age symmetric (scrypt) mode
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		t.Fatalf("failed to create scrypt recipient: %v", err)
	}

	outFile, err := os.Create(envPath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	w, err := age.Encrypt(outFile, recipient)
	if err != nil {
		t.Fatalf("failed to create age encryptor: %v", err)
	}

	if _, err := io.Copy(w, bytes.NewBufferString(rawEnvContent)); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close encryptor: %v", err)
	}
	outFile.Close()

	// Decrypt using SecureEnv
	se, err := NewSecureEnv(passphrase)
	if err != nil {
		t.Fatalf("failed to create SecureEnv: %v", err)
	}

	err = se.LoadAgeSymmetric(envPath, passphrase)
	if err != nil {
		t.Fatalf("failed to load age symmetric env: %v", err)
	}

	dbUser, err := se.Get("DB_USER")
	if err != nil {
		t.Fatalf("failed to get DB_USER: %v", err)
	}
	if dbUser != "test_db_user" {
		t.Errorf("expected %q, got %q", "test_db_user", dbUser)
	}

	dbPass, err := se.Get("DB_PASS")
	if err != nil {
		t.Fatalf("failed to get DB_PASS: %v", err)
	}
	if dbPass != "secret_pass_123" {
		t.Errorf("expected %q, got %q", "secret_pass_123", dbPass)
	}
}

func TestLoadAgeAsymmetric(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "secrets.asym.age")

	rawEnvContent := "API_KEY=asym_test_key_abc\nAPI_URL=https://api.test"

	// Generate X25519 identity keypair
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("failed to generate X25519 identity: %v", err)
	}
	recipient := identity.Recipient()

	outFile, err := os.Create(envPath)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	w, err := age.Encrypt(outFile, recipient)
	if err != nil {
		t.Fatalf("failed to create age encryptor: %v", err)
	}

	if _, err := io.Copy(w, bytes.NewBufferString(rawEnvContent)); err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close encryptor: %v", err)
	}
	outFile.Close()

	// Decrypt using SecureEnv
	se, err := NewSecureEnv("some-master-key")
	if err != nil {
		t.Fatalf("failed to create SecureEnv: %v", err)
	}

	err = se.LoadAgeAsymmetric(envPath, identity)
	if err != nil {
		t.Fatalf("failed to load age asymmetric env: %v", err)
	}

	apiKey, err := se.Get("API_KEY")
	if err != nil {
		t.Fatalf("failed to get API_KEY: %v", err)
	}
	if apiKey != "asym_test_key_abc" {
		t.Errorf("expected %q, got %q", "asym_test_key_abc", apiKey)
	}
}

func TestAESLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "secrets.aes")

	masterKey := "my-master-aes-key"
	kv := map[string]string{
		"PORT": "9000",
		"HOST": "127.0.0.1",
	}

	se1, err := NewSecureEnv(masterKey)
	if err != nil {
		t.Fatalf("failed to create SecureEnv 1: %v", err)
	}

	// Save
	err = se1.SaveAES(envPath, kv)
	if err != nil {
		t.Fatalf("failed to save AES file: %v", err)
	}

	// Load
	se2, err := NewSecureEnv(masterKey)
	if err != nil {
		t.Fatalf("failed to create SecureEnv 2: %v", err)
	}

	err = se2.LoadAES(envPath)
	if err != nil {
		t.Fatalf("failed to load AES file: %v", err)
	}

	port, err := se2.Get("PORT")
	if err != nil {
		t.Fatalf("failed to get PORT: %v", err)
	}
	if port != "9000" {
		t.Errorf("expected %q, got %q", "9000", port)
	}
}

type TestConfigStruct struct {
	DBUser   string `env:"DB_USER"`
	DBPass   string `env:"DB_PASS"`
	DBPort   int    `env:"DB_PORT" envDefault:"5432"`
	SSLMode  bool   `env:"SSL_MODE"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

func TestUnmarshal(t *testing.T) {
	se, err := NewSecureEnv("master-key-123")
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	_ = se.Set("DB_USER", "postgres")
	_ = se.Set("DB_PASS", "postgres_password")
	_ = se.Set("SSL_MODE", "true")

	var cfg TestConfigStruct
	err = se.Unmarshal(&cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.DBUser != "postgres" {
		t.Errorf("DBUser mismatch: %s", cfg.DBUser)
	}
	if cfg.DBPass != "postgres_password" {
		t.Errorf("DBPass mismatch: %s", cfg.DBPass)
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort should use default value 5432, got %d", cfg.DBPort)
	}
	if !cfg.SSLMode {
		t.Errorf("SSLMode should be true")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel should use default value 'info', got %s", cfg.LogLevel)
	}
}

func TestToMap(t *testing.T) {
	se, err := NewSecureEnv("master-key-123")
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	_ = se.Set("KEY1", "val1")
	_ = se.Set("KEY2", "val2")

	m, err := se.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}

	if len(m) != 2 {
		t.Errorf("map length mismatch: %d", len(m))
	}
	if m["KEY1"] != "val1" || m["KEY2"] != "val2" {
		t.Errorf("map content mismatch")
	}
}

type YamlConfig struct {
	Database struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"database"`
}

func TestGetYAML(t *testing.T) {
	se, err := NewSecureEnv("master-key-123")
	if err != nil {
		t.Fatalf("failed to init: %v", err)
	}

	yamlStr := `
database:
  host: 127.0.0.1
  port: 5432
`
	_ = se.Set("CONFIG_YAML", yamlStr)

	var cfg YamlConfig
	err = se.GetYAML("CONFIG_YAML", &cfg)
	if err != nil {
		t.Fatalf("GetYAML failed: %v", err)
	}

	if cfg.Database.Host != "127.0.0.1" {
		t.Errorf("Host mismatch: %s", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Port mismatch: %d", cfg.Database.Port)
	}
}
