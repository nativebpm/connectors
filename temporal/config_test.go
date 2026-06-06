package temporal_test

import (
	"os"
	"testing"

	"github.com/nativebpm/connectors/cryptenv"
	"github.com/nativebpm/connectors/temporal"
)

func TestConfigBuilderSetters(t *testing.T) {
	cfg, err := temporal.NewConfigBuilder().
		WithHostPort("localhost:9000").
		WithNamespace("custom-ns").
		WithTaskQueue("custom-queue").
		WithCertPath("/path/to/cert").
		WithKeyPath("/path/to/key").
		WithEncryptionKey([]byte("secret")).
		WithDBHost("localhost").
		WithDBPort("9999").
		WithDBUser("my-user").
		WithDBPassword("my-pass").
		WithDBName("my-db").
		Build()

	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	if cfg.HostPort != "localhost:9000" {
		t.Errorf("HostPort mismatch: got %s", cfg.HostPort)
	}
	if cfg.Namespace != "custom-ns" {
		t.Errorf("Namespace mismatch: got %s", cfg.Namespace)
	}
	if cfg.TaskQueue != "custom-queue" {
		t.Errorf("TaskQueue mismatch: got %s", cfg.TaskQueue)
	}
	if cfg.CertPath != "/path/to/cert" {
		t.Errorf("CertPath mismatch")
	}
	if cfg.KeyPath != "/path/to/key" {
		t.Errorf("KeyPath mismatch")
	}
	if string(cfg.EncryptionKey) != "secret" {
		t.Errorf("EncryptionKey mismatch")
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost mismatch")
	}
	if cfg.DBPort != "9999" {
		t.Errorf("DBPort mismatch")
	}
	if cfg.DBUser != "my-user" {
		t.Errorf("DBUser mismatch")
	}
	if cfg.DBPassword != "my-pass" {
		t.Errorf("DBPassword mismatch")
	}
	if cfg.DBName != "my-db" {
		t.Errorf("DBName mismatch")
	}
}

func TestConfigBuilderFromEnv(t *testing.T) {
	// Set mock environment variables
	os.Setenv("TEMPORAL_HOST_PORT", "127.0.0.1:8233")
	os.Setenv("TEMPORAL_NAMESPACE", "env-ns")
	os.Setenv("TEMPORAL_DB_USER", "env-db-user")

	defer func() {
		os.Unsetenv("TEMPORAL_HOST_PORT")
		os.Unsetenv("TEMPORAL_NAMESPACE")
		os.Unsetenv("TEMPORAL_DB_USER")
	}()

	cfg, err := temporal.NewConfigBuilder().
		FromEnv().
		Build()

	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	if cfg.HostPort != "127.0.0.1:8233" {
		t.Errorf("HostPort should load from env, got %s", cfg.HostPort)
	}
	if cfg.Namespace != "env-ns" {
		t.Errorf("Namespace should load from env, got %s", cfg.Namespace)
	}
	if cfg.DBUser != "env-db-user" {
		t.Errorf("DBUser should load from env, got %s", cfg.DBUser)
	}
}

func TestConfigBuilderFromSecureEnv(t *testing.T) {
	se, err := cryptenv.NewSecureEnv("master-pwd")
	if err != nil {
		t.Fatalf("failed to init secure env: %v", err)
	}

	err = se.Set("TEMPORAL_HOST_PORT", "10.0.0.1:7233")
	if err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}
	err = se.Set("TEMPORAL_NAMESPACE", "secure-ns")
	if err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}
	err = se.Set("TEMPORAL_DB_USER", "secure-db-user")
	if err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}

	cfg, err := temporal.NewConfigBuilder().
		FromSecureEnv(se).
		Build()

	if err != nil {
		t.Fatalf("failed to build config: %v", err)
	}

	if cfg.HostPort != "10.0.0.1:7233" {
		t.Errorf("HostPort should load from secure env, got %s", cfg.HostPort)
	}
	if cfg.Namespace != "secure-ns" {
		t.Errorf("Namespace should load from secure env, got %s", cfg.Namespace)
	}
	if cfg.DBUser != "secure-db-user" {
		t.Errorf("DBUser should load from secure env, got %s", cfg.DBUser)
	}
}
