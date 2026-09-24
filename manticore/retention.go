package manticore

import (
	"context"
	"fmt"
	"time"
)

// PruneExpired evicts completed process instances older than cutoffTime across all peers.
// Historical data is durably preserved in Universal S3.
func (c *Client) PruneExpired(ctx context.Context, cutoffTime time.Time) error {
	cutoffUnix := cutoffTime.Unix()
	query := `DELETE FROM process_instances WHERE status = 'COMPLETED' AND end_time > 0 AND end_time < ?`
	return c.Broadcast(ctx, query, cutoffUnix)
}

// PruneLogs evicts audit logs older than cutoffTime across all peers.
func (c *Client) PruneLogs(ctx context.Context, cutoffTime time.Time) error {
	cutoffUnix := cutoffTime.Unix()
	query := `DELETE FROM process_audit_logs WHERE timestamp < ?`
	return c.Broadcast(ctx, query, cutoffUnix)
}

// EnforceMaxCapacity ensures that the number of records in process_instances does not exceed
// the configured max capacity. If exceeded, oldest completed records are evicted first.
func (c *Client) EnforceMaxCapacity(ctx context.Context, maxRows int64) error {
	if maxRows <= 0 {
		return fmt.Errorf("manticore: maxRows must be positive, got %d", maxRows)
	}

	row, err := c.QueryRow(ctx, "SELECT COUNT(*) FROM process_instances")
	if err != nil {
		return fmt.Errorf("manticore: failed to query count: %w", err)
	}

	var total int64
	if err := row.Scan(&total); err != nil {
		return fmt.Errorf("manticore: failed to scan count: %w", err)
	}

	if total <= maxRows {
		return nil
	}

	excess := total - maxRows
	// Evict excess completed instances by earliest start_time
	query := fmt.Sprintf(`DELETE FROM process_instances WHERE status = 'COMPLETED' ORDER BY start_time ASC LIMIT %d`, excess)
	return c.Broadcast(ctx, query)
}

// Optimize triggers background chunk compaction and tombstone cleanup on the given table.
func (c *Client) Optimize(ctx context.Context, tableName string) error {
	query := fmt.Sprintf("OPTIMIZE TABLE %s", tableName)
	return c.Broadcast(ctx, query)
}
