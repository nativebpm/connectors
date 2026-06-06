package temporal

import (
	"crypto/sha256"
	"os"

	"github.com/joho/godotenv"
	"github.com/nativebpm/connectors/cryptenv"
)

// Config contains connection parameters for Temporal Server or Temporal Cloud.
type Config struct {
	HostPort      string
	Namespace     string
	CertPath      string
	KeyPath       string
	TaskQueue     string
	EncryptionKey []byte // Payload encryption key

	// Database configurations for CDC activity
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

// ConfigBuilder is a Fluent API builder for loading and constructing Config.
type ConfigBuilder struct {
	cfg *Config
	err error
}

// NewConfigBuilder creates a new instance of ConfigBuilder.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		cfg: &Config{},
	}
}

// FromEnv loads configurations from environment variables and default .env files.
func (b *ConfigBuilder) FromEnv() *ConfigBuilder {
	if b.err != nil {
		return b
	}

	_ = godotenv.Load()
	_ = godotenv.Load("temporal.env")

	b.cfg.HostPort = getEnvWithDefault("TEMPORAL_HOST_PORT", b.cfg.HostPort)
	b.cfg.Namespace = getEnvWithDefault("TEMPORAL_NAMESPACE", b.cfg.Namespace)
	b.cfg.CertPath = getEnvWithDefault("TEMPORAL_CERT_PATH", b.cfg.CertPath)
	b.cfg.KeyPath = getEnvWithDefault("TEMPORAL_KEY_PATH", b.cfg.KeyPath)
	b.cfg.TaskQueue = getEnvWithDefault("TEMPORAL_TASK_QUEUE", b.cfg.TaskQueue)

	if keyStr := os.Getenv("TEMPORAL_ENCRYPTION_KEY"); keyStr != "" {
		hash := sha256.Sum256([]byte(keyStr))
		b.cfg.EncryptionKey = hash[:]
	}

	b.cfg.DBHost = getEnvWithDefault("TEMPORAL_DB_HOST", b.cfg.DBHost)
	b.cfg.DBPort = getEnvWithDefault("TEMPORAL_DB_PORT", b.cfg.DBPort)
	b.cfg.DBUser = getEnvWithDefault("TEMPORAL_DB_USER", b.cfg.DBUser)
	b.cfg.DBPassword = getEnvWithDefault("TEMPORAL_DB_PASSWORD", b.cfg.DBPassword)
	b.cfg.DBName = getEnvWithDefault("TEMPORAL_DB_NAME", b.cfg.DBName)

	return b
}

// FromSecureEnv loads configurations from a secure cryptenv container.
func (b *ConfigBuilder) FromSecureEnv(se *cryptenv.SecureEnv) *ConfigBuilder {
	if b.err != nil {
		return b
	}

	b.cfg.HostPort = getSecureEnvWithDefault(se, "TEMPORAL_HOST_PORT", b.cfg.HostPort)
	b.cfg.Namespace = getSecureEnvWithDefault(se, "TEMPORAL_NAMESPACE", b.cfg.Namespace)
	b.cfg.CertPath = getSecureEnvWithDefault(se, "TEMPORAL_CERT_PATH", b.cfg.CertPath)
	b.cfg.KeyPath = getSecureEnvWithDefault(se, "TEMPORAL_KEY_PATH", b.cfg.KeyPath)
	b.cfg.TaskQueue = getSecureEnvWithDefault(se, "TEMPORAL_TASK_QUEUE", b.cfg.TaskQueue)

	if keyStr, err := se.Get("TEMPORAL_ENCRYPTION_KEY"); err == nil && keyStr != "" {
		hash := sha256.Sum256([]byte(keyStr))
		b.cfg.EncryptionKey = hash[:]
	}

	b.cfg.DBHost = getSecureEnvWithDefault(se, "TEMPORAL_DB_HOST", b.cfg.DBHost)
	b.cfg.DBPort = getSecureEnvWithDefault(se, "TEMPORAL_DB_PORT", b.cfg.DBPort)
	b.cfg.DBUser = getSecureEnvWithDefault(se, "TEMPORAL_DB_USER", b.cfg.DBUser)
	b.cfg.DBPassword = getSecureEnvWithDefault(se, "TEMPORAL_DB_PASSWORD", b.cfg.DBPassword)
	b.cfg.DBName = getSecureEnvWithDefault(se, "TEMPORAL_DB_NAME", b.cfg.DBName)

	return b
}

// Setters for Fluent API
func (b *ConfigBuilder) WithHostPort(val string) *ConfigBuilder {
	b.cfg.HostPort = val
	return b
}

func (b *ConfigBuilder) WithNamespace(val string) *ConfigBuilder {
	b.cfg.Namespace = val
	return b
}

func (b *ConfigBuilder) WithCertPath(val string) *ConfigBuilder {
	b.cfg.CertPath = val
	return b
}

func (b *ConfigBuilder) WithKeyPath(val string) *ConfigBuilder {
	b.cfg.KeyPath = val
	return b
}

func (b *ConfigBuilder) WithTaskQueue(val string) *ConfigBuilder {
	b.cfg.TaskQueue = val
	return b
}

func (b *ConfigBuilder) WithEncryptionKey(val []byte) *ConfigBuilder {
	b.cfg.EncryptionKey = val
	return b
}

func (b *ConfigBuilder) WithDBHost(val string) *ConfigBuilder {
	b.cfg.DBHost = val
	return b
}

func (b *ConfigBuilder) WithDBPort(val string) *ConfigBuilder {
	b.cfg.DBPort = val
	return b
}

func (b *ConfigBuilder) WithDBUser(val string) *ConfigBuilder {
	b.cfg.DBUser = val
	return b
}

func (b *ConfigBuilder) WithDBPassword(val string) *ConfigBuilder {
	b.cfg.DBPassword = val
	return b
}

func (b *ConfigBuilder) WithDBName(val string) *ConfigBuilder {
	b.cfg.DBName = val
	return b
}

// Build validates and constructs the final Config struct with defaults.
func (b *ConfigBuilder) Build() (*Config, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.cfg.HostPort == "" {
		b.cfg.HostPort = "127.0.0.1:7233"
	}
	if b.cfg.Namespace == "" {
		b.cfg.Namespace = "default"
	}
	if b.cfg.TaskQueue == "" {
		b.cfg.TaskQueue = "default-task-queue"
	}
	if b.cfg.DBHost == "" {
		b.cfg.DBHost = "127.0.0.1"
	}
	if b.cfg.DBPort == "" {
		b.cfg.DBPort = "5432"
	}
	if b.cfg.DBUser == "" {
		b.cfg.DBUser = "temporal"
	}
	if b.cfg.DBPassword == "" {
		b.cfg.DBPassword = "temporal_password"
	}
	if b.cfg.DBName == "" {
		b.cfg.DBName = "temporal"
	}

	return b.cfg, nil
}

// LoadFromEnv is kept for backward compatibility.
func LoadFromEnv() *Config {
	cfg, _ := NewConfigBuilder().FromEnv().Build()
	return cfg
}

func getEnvWithDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getSecureEnvWithDefault(se *cryptenv.SecureEnv, key, defaultValue string) string {
	if val, err := se.Get(key); err == nil && val != "" {
		return val
	}
	return defaultValue
}
