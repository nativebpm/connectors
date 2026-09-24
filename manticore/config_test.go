package manticore_test

import (
	"testing"
	"time"

	"github.com/nativebpm/connectors/manticore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validation(t *testing.T) {
	t.Run("valid configuration passes", func(t *testing.T) {
		cfg := manticore.Config{
			Peers:           []string{"127.0.0.1:9306", "127.0.0.1:9307"},
			Timeout:         5 * time.Second,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			RetentionPeriod: 30 * 24 * time.Hour,
			MaxRowsCapacity: 500000,
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("empty peers fails validation", func(t *testing.T) {
		cfg := manticore.Config{
			Peers:           []string{},
			Timeout:         5 * time.Second,
			RetentionPeriod: 24 * time.Hour,
			MaxRowsCapacity: 1000,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one peer address is required")
	})

	t.Run("zero timeout fails validation (Zero Defaults principle)", func(t *testing.T) {
		cfg := manticore.Config{
			Peers:           []string{"127.0.0.1:9306"},
			Timeout:         0,
			RetentionPeriod: 24 * time.Hour,
			MaxRowsCapacity: 1000,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout must be positive")
	})

	t.Run("zero or negative capacity fails validation", func(t *testing.T) {
		cfg := manticore.Config{
			Peers:           []string{"127.0.0.1:9306"},
			Timeout:         2 * time.Second,
			RetentionPeriod: 24 * time.Hour,
			MaxRowsCapacity: 0,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_rows_capacity must be positive")
	})

	t.Run("zero retention period fails validation", func(t *testing.T) {
		cfg := manticore.Config{
			Peers:           []string{"127.0.0.1:9306"},
			Timeout:         2 * time.Second,
			RetentionPeriod: 0,
			MaxRowsCapacity: 1000,
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retention_period must be positive")
	})
}
