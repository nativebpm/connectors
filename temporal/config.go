package temporal

import (
	"os"
)

// Config содержит параметры подключения к Temporal Server или Temporal Cloud.
type Config struct {
	HostPort  string
	Namespace string
	CertPath  string
	KeyPath   string
	TaskQueue string

	// Настройки базы данных для CDC активности
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
}

// LoadFromEnv загружает настройки из переменных окружения с разумными дефолтами.
func LoadFromEnv() *Config {
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

	return &Config{
		HostPort:   hostPort,
		Namespace:  namespace,
		CertPath:   os.Getenv("TEMPORAL_CERT_PATH"),
		KeyPath:    os.Getenv("TEMPORAL_KEY_PATH"),
		TaskQueue:  taskQueue,
		DBHost:     dbHost,
		DBPort:     dbPort,
		DBUser:     dbUser,
		DBPassword: dbPassword,
		DBName:     dbName,
	}
}
