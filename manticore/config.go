package manticore

import (
	"errors"
	"fmt"
	"time"
)

// Config defines the configuration for connecting to Manticore Search cluster/peers
// and governing data retention policies (Zero Defaults & Strict Validation).
type Config struct {
	// Peers holds a list of Manticore host:port endpoints (e.g. ["127.0.0.1:9306"]).
	Peers []string `json:"peers" yaml:"peers"`

	// Timeout specifies the maximum duration for network dial and query execution.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxOpenConns sets the maximum number of open connections per peer.
	MaxOpenConns int `json:"max_open_conns" yaml:"max_open_conns"`

	// MaxIdleConns sets the maximum number of idle connections per peer.
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns"`

	// RetentionPeriod defines the TTL after which completed instances are evicted from Manticore.
	// Primary persistent state is securely archived in Universal S3.
	RetentionPeriod time.Duration `json:"retention_period" yaml:"retention_period"`

	// MaxRowsCapacity sets the maximum row count limit in Manticore before FIFO eviction triggers.
	MaxRowsCapacity int64 `json:"max_rows_capacity" yaml:"max_rows_capacity"`

	// UseColumnar instructs table DDL creation to enable Manticore Columnar Library.
	UseColumnar bool `json:"use_columnar" yaml:"use_columnar"`
}

// Validate verifies that all mandatory configuration parameters are explicitly supplied.
func (c Config) Validate() error {
	if len(c.Peers) == 0 {
		return errors.New("manticore: at least one peer address is required")
	}
	for i, peer := range c.Peers {
		if peer == "" {
			return fmt.Errorf("manticore: peer[%d] address cannot be empty", i)
		}
	}
	if c.Timeout <= 0 {
		return errors.New("manticore: timeout must be positive")
	}
	if c.RetentionPeriod <= 0 {
		return errors.New("manticore: retention_period must be positive")
	}
	if c.MaxRowsCapacity <= 0 {
		return errors.New("manticore: max_rows_capacity must be positive")
	}
	return nil
}
