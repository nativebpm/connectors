package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type DurableWasmTemporalTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestDurableWasmTemporalTestSuite(t *testing.T) {
	suite.Run(t, new(DurableWasmTemporalTestSuite))
}

func (s *DurableWasmTemporalTestSuite) Test_DurableWasmWorkflow_Success_With_Retry() {
	// 1. Clean up database files
	_ = os.Remove(dbFile)
	_ = os.Remove(sqliteDBFile)
	defer func() {
		_ = os.Remove(dbFile)
		_ = os.Remove(sqliteDBFile)
	}()

	// 2. Start mock HTTP server
	mockServer := startMockServer(serverAddr)
	defer mockServer.Shutdown(context.Background())

	// Give mock server time to start
	time.Sleep(100 * time.Millisecond)

	// 3. Create test environment
	env := s.NewTestWorkflowEnvironment()

	// Register real Workflow and Activity (no mocks!)
	env.RegisterWorkflow(DurableWasmWorkflow)
	env.RegisterActivity(ExecuteDurableWasmActivity)

	// 4. Run Workflow
	instanceID := "test-temporal-tx"
	env.ExecuteWorkflow(DurableWasmWorkflow, instanceID, serverAddr)

	// 5. Assert completion and lack of errors
	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result string
	err := env.GetWorkflowResult(&result)
	s.NoError(err)

	// 6. Verify final result format and persistence
	s.Contains(result, `"completed":true`)
	s.Contains(result, `"result_value":1800`)
	s.Contains(result, `"activity_id":"ACT-TEMP-4455"`)
}
