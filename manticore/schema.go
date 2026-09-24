package manticore

import (
	"fmt"
)

// GetProcessInstancesTableDDL returns the DDL statement to create the Real-Time (RT)
// table for BPMN process instances.
func GetProcessInstancesTableDDL(tableName string, columnar bool) string {
	engineOption := ""
	if columnar {
		engineOption = " engine='columnar'"
	}

	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    id bigint,
    process_key string,
    version int,
    status string,
    tenant_id int,
    assignee string,
    start_time timestamp,
    end_time timestamp,
    duration_ms bigint,
    amount float,
    error_message text,
    variables_json string
)%s`, tableName, engineOption)
}

// GetAuditLogsTableDDL returns the DDL statement to create the Real-Time (RT)
// table for audit logs and execution traces.
func GetAuditLogsTableDDL(tableName string, columnar bool) string {
	engineOption := ""
	if columnar {
		engineOption = " engine='columnar'"
	}

	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    id bigint,
    instance_id bigint,
    process_key string,
    log_level string,
    activity_id string,
    timestamp timestamp,
    payload text
)%s`, tableName, engineOption)
}
