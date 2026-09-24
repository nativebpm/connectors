package manticore_test

import (
	"strings"
	"testing"

	"github.com/nativebpm/connectors/manticore"
	"github.com/stretchr/testify/assert"
)

func TestSchema_DDL(t *testing.T) {
	t.Run("generate process_instances table DDL with columnar attributes", func(t *testing.T) {
		ddl := manticore.GetProcessInstancesTableDDL("process_instances", true)
		assert.Contains(t, ddl, "CREATE TABLE IF NOT EXISTS process_instances")
		assert.Contains(t, ddl, "id bigint")
		assert.Contains(t, ddl, "process_key string")
		assert.Contains(t, ddl, "status string")
		assert.Contains(t, ddl, "tenant_id int")
		assert.Contains(t, ddl, "error_message text")
		assert.Contains(t, ddl, "engine='columnar'")
	})

	t.Run("generate process_instances table DDL rowwise fallback", func(t *testing.T) {
		ddl := manticore.GetProcessInstancesTableDDL("process_instances", false)
		assert.Contains(t, ddl, "CREATE TABLE IF NOT EXISTS process_instances")
		assert.False(t, strings.Contains(ddl, "engine='columnar'"))
	})

	t.Run("generate process_audit_logs table DDL", func(t *testing.T) {
		ddl := manticore.GetAuditLogsTableDDL("process_audit_logs", true)
		assert.Contains(t, ddl, "CREATE TABLE IF NOT EXISTS process_audit_logs")
		assert.Contains(t, ddl, "instance_id bigint")
		assert.Contains(t, ddl, "log_level string")
		assert.Contains(t, ddl, "activity_id string")
		assert.Contains(t, ddl, "payload text")
	})
}
