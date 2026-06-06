package restate

import (
	"os"
)

// Config contains connection parameters for the Restate Server and Services.
type Config struct {
	HostPort  string // Address the service server will listen on, e.g. "0.0.0.0:9080"
	ServerURL string // Endpoint of the Restate Server (coordinator), e.g. "http://localhost:8080"
}

// ConfigBuilder is a Fluent API builder for loading and constructing Restate Config.
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

// FromEnv loads configurations from environment variables.
func (b *ConfigBuilder) FromEnv() *ConfigBuilder {
	if b.err != nil {
		return b
	}

	b.cfg.HostPort = getEnvWithDefault("RESTATE_HOST_PORT", b.cfg.HostPort)
	b.cfg.ServerURL = getEnvWithDefault("RESTATE_SERVER_URL", b.cfg.ServerURL)

	return b
}

// WithHostPort sets the HostPort on the configuration.
func (b *ConfigBuilder) WithHostPort(hostPort string) *ConfigBuilder {
	if b.err != nil {
		return b
	}
	b.cfg.HostPort = hostPort
	return b
}

// WithServerURL sets the ServerURL on the configuration.
func (b *ConfigBuilder) WithServerURL(serverURL string) *ConfigBuilder {
	if b.err != nil {
		return b
	}
	b.cfg.ServerURL = serverURL
	return b
}

// Error returns any accumulated error during building.
func (b *ConfigBuilder) Error() error {
	return b.err
}

// Build validates and constructs the final Config struct with default fallbacks.
func (b *ConfigBuilder) Build() (*Config, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.cfg.HostPort == "" {
		b.cfg.HostPort = "0.0.0.0:9080"
	}
	if b.cfg.ServerURL == "" {
		b.cfg.ServerURL = "http://localhost:8080"
	}

	return b.cfg, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
