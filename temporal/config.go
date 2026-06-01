package temporal

import (
	"crypto/sha256"
	"os"

	"github.com/joho/godotenv"
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

// LoadFromEnv loads configurations from environment variables (and .env/temporal.env files) with sensible defaults.
func LoadFromEnv() *Config {
	// Attempt to load configuration files (.env or temporal.env)
	_ = godotenv.Load()
	_ = godotenv.Load("temporal.env")

	hostPort := os.Getenv("TEMPORAL_HOST_PORT")
	if hostPort == "" {
		hostPort = "127.0.0.1:7233"
	}

	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	if taskQueue == "" {
		taskQueue = "default-task-queue"
	}

	dbHost := os.Getenv("TEMPORAL_DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}

	dbPort := os.Getenv("TEMPORAL_DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("TEMPORAL_DB_USER")
	if dbUser == "" {
		dbUser = "temporal"
	}

	dbPassword := os.Getenv("TEMPORAL_DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "temporal_password"
	}

	dbName := os.Getenv("TEMPORAL_DB_NAME")
	if dbName == "" {
		dbName = "temporal"
	}

	var encryptionKey []byte
	if keyStr := os.Getenv("TEMPORAL_ENCRYPTION_KEY"); keyStr != "" {
		hash := sha256.Sum256([]byte(keyStr))
		encryptionKey = hash[:]
	}

	return &Config{
		HostPort:      hostPort,
		Namespace:     namespace,
		CertPath:      os.Getenv("TEMPORAL_CERT_PATH"),
		KeyPath:       os.Getenv("TEMPORAL_KEY_PATH"),
		TaskQueue:     taskQueue,
		EncryptionKey: encryptionKey,
		DBHost:        dbHost,
		DBPort:        dbPort,
		DBUser:        dbUser,
		DBPassword:    dbPassword,
		DBName:        dbName,
	}
}
