package manticore

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// InitSchema creates the necessary RT tables across all peers.
func (c *Client) InitSchema(ctx context.Context) error {
	ddlInstances := GetProcessInstancesTableDDL("process_instances", c.cfg.UseColumnar)
	if err := c.Broadcast(ctx, ddlInstances); err != nil {
		return fmt.Errorf("failed to init process_instances schema: %w", err)
	}

	ddlLogs := GetAuditLogsTableDDL("process_audit_logs", c.cfg.UseColumnar)
	if err := c.Broadcast(ctx, ddlLogs); err != nil {
		return fmt.Errorf("failed to init process_audit_logs schema: %w", err)
	}

	return nil
}

// UpsertInstance performs an idempotent REPLACE INTO across all peer nodes.
func (c *Client) UpsertInstance(ctx context.Context, inst *ProcessInstance) error {
	if inst.ID == 0 {
		return fmt.Errorf("manticore: instance ID cannot be 0")
	}

	query := `REPLACE INTO process_instances (
		id, process_key, version, status, tenant_id, assignee,
		start_time, end_time, duration_ms, amount, error_message, variables_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	startTimeUnix := inst.StartTime.Unix()
	var endTimeUnix int64
	if !inst.EndTime.IsZero() {
		endTimeUnix = inst.EndTime.Unix()
	}

	return c.Broadcast(ctx, query,
		inst.ID,
		inst.ProcessKey,
		inst.Version,
		inst.Status,
		inst.TenantID,
		inst.Assignee,
		startTimeUnix,
		endTimeUnix,
		inst.DurationMs,
		inst.Amount,
		inst.ErrorMessage,
		inst.VariablesJSON,
	)
}

// UpdateStatus performs a fast in-place update of scalar status and duration attributes.
func (c *Client) UpdateStatus(ctx context.Context, id uint64, status string, durationMs int64) error {
	query := `UPDATE process_instances SET status = ?, duration_ms = ? WHERE id = ?`
	return c.Broadcast(ctx, query, status, durationMs, id)
}

// InsertLog appends an audit event to the append-only log table across all peers.
func (c *Client) InsertLog(ctx context.Context, log *AuditLog) error {
	query := `INSERT INTO process_audit_logs (
		id, instance_id, process_key, log_level, activity_id, timestamp, payload
	) VALUES (?, ?, ?, ?, ?, ?, ?)`

	ts := log.Timestamp.Unix()
	if ts <= 0 {
		ts = time.Now().Unix()
	}

	return c.Broadcast(ctx, query,
		log.ID,
		log.InstanceID,
		log.ProcessKey,
		log.LogLevel,
		log.ActivityID,
		ts,
		log.Payload,
	)
}

// SearchInstances executes a multi-attribute and full-text search against a balanced peer node.
func (c *Client) SearchInstances(ctx context.Context, filter SearchFilter) (*SearchResult, error) {
	var whereClauses []string
	var args []any

	if filter.ProcessKey != "" {
		whereClauses = append(whereClauses, "process_key = ?")
		args = append(args, filter.ProcessKey)
	}
	if filter.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.TenantID != nil {
		whereClauses = append(whereClauses, "tenant_id = ?")
		args = append(args, *filter.TenantID)
	}
	if filter.Assignee != "" {
		whereClauses = append(whereClauses, "assignee = ?")
		args = append(args, filter.Assignee)
	}
	if filter.MinAmount != nil {
		whereClauses = append(whereClauses, "amount >= ?")
		args = append(args, *filter.MinAmount)
	}
	if filter.MaxAmount != nil {
		whereClauses = append(whereClauses, "amount <= ?")
		args = append(args, *filter.MaxAmount)
	}
	if filter.QueryText != "" {
		whereClauses = append(whereClauses, "MATCH(?)")
		args = append(args, filter.QueryText)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	querySQL := fmt.Sprintf(`SELECT 
		id, process_key, version, status, tenant_id, assignee,
		start_time, end_time, duration_ms, amount, error_message, variables_json
	FROM process_instances%s ORDER BY id DESC LIMIT %d, %d`, whereSQL, offset, limit)

	rows, err := c.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("manticore: search query failed: %w", err)
	}
	defer rows.Close()

	var instances []ProcessInstance
	for rows.Next() {
		var inst ProcessInstance
		var startUnix, endUnix int64
		err := rows.Scan(
			&inst.ID,
			&inst.ProcessKey,
			&inst.Version,
			&inst.Status,
			&inst.TenantID,
			&inst.Assignee,
			&startUnix,
			&endUnix,
			&inst.DurationMs,
			&inst.Amount,
			&inst.ErrorMessage,
			&inst.VariablesJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("manticore: scan row failed: %w", err)
		}
		inst.StartTime = time.Unix(startUnix, 0)
		if endUnix > 0 {
			inst.EndTime = time.Unix(endUnix, 0)
		}
		instances = append(instances, inst)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("manticore: row iteration error: %w", err)
	}

	return &SearchResult{
		Total:     int64(len(instances)),
		Instances: instances,
	}, nil
}
