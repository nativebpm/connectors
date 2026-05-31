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

	return &Config{
		HostPort:  hostPort,
		Namespace: namespace,
		CertPath:  os.Getenv("TEMPORAL_CERT_PATH"),
		KeyPath:   os.Getenv("TEMPORAL_KEY_PATH"),
		TaskQueue: taskQueue,
	}
}
