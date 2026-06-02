package sequinoutbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
)

// UnitTestSuite handles workflow testing
type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

func (s *UnitTestSuite) Test_DeleteUserWorkflow_Success() {
	env := s.NewTestWorkflowEnvironment()

	// Mock Activities
	env.OnActivity(CleanUpExternalSystems, mock.Anything, "user-123").Return(nil)
	env.OnActivity(SendDeletionConfirmation, mock.Anything, "user@example.com").Return(nil)

	env.ExecuteWorkflow(DeleteUserWorkflow, "user-123", "user@example.com")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}

func (s *UnitTestSuite) Test_DeleteUserWorkflow_CleanupFailure() {
	env := s.NewTestWorkflowEnvironment()

	// Mock Activities: Cleanup fails
	env.OnActivity(CleanUpExternalSystems, mock.Anything, "user-123").Return(errors.New("cleanup failed"))

	env.ExecuteWorkflow(DeleteUserWorkflow, "user-123", "user@example.com")

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
}

// Mock workflow starter for testing the HTTP handler
type MockWorkflowStarter struct {
	mock.Mock
}

func (m *MockWorkflowStarter) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow any, args ...any) (client.WorkflowRun, error) {
	mockArgs := m.Called(ctx, options, workflow, args)
	run, _ := mockArgs.Get(0).(client.WorkflowRun)
	return run, mockArgs.Error(1)
}

// Mock run info
type MockWorkflowRun struct {
	mock.Mock
}

func (m *MockWorkflowRun) GetID() string {
	return m.Called().String(0)
}

func (m *MockWorkflowRun) GetRunID() string {
	return m.Called().String(0)
}

func (m *MockWorkflowRun) Get(ctx context.Context, valuePtr any) error {
	return m.Called(ctx, valuePtr).Error(0)
}

func (m *MockWorkflowRun) GetWithOptions(ctx context.Context, valuePtr any, options client.WorkflowRunGetOptions) error {
	return m.Called(ctx, valuePtr, options).Error(0)
}

// HTTP handler tests
func TestSequinWebhookHandler(t *testing.T) {
	t.Run("Success Delete Action", func(t *testing.T) {
		starter := new(MockWorkflowStarter)
		mockRun := new(MockWorkflowRun)

		payload := SequinWebhookPayload{
			Action: "delete",
			OldRecord: &UserRecord{
				ID:    "user-123",
				Email: "john.doe@example.com",
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}

		mockRun.On("GetID").Return("delete-user-user-123")
		mockRun.On("GetRunID").Return("run-123")
		starter.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, []any{"user-123", "john.doe@example.com"}).
			Return(mockRun, nil)

		req, err := http.NewRequest("POST", "/delete-user", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()

		handler := NewWebhookHandler(starter, "test-task-queue")
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		expected := "Workflow started successfully"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
		}

		starter.AssertExpectations(t)
	})

	t.Run("Ignore Non-Delete Action", func(t *testing.T) {
		starter := new(MockWorkflowStarter)

		payload := SequinWebhookPayload{
			Action: "insert",
			Record: &UserRecord{
				ID:    "user-123",
				Email: "john.doe@example.com",
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("POST", "/delete-user", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()

		handler := NewWebhookHandler(starter, "test-task-queue")
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		expected := "Action ignored"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
		}

		starter.AssertNotCalled(t, "ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Invalid JSON Payload", func(t *testing.T) {
		starter := new(MockWorkflowStarter)

		req, err := http.NewRequest("POST", "/delete-user", bytes.NewBufferString("{invalid json}"))
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()

		handler := NewWebhookHandler(starter, "test-task-queue")
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})

	t.Run("Missing Old Record on Delete", func(t *testing.T) {
		starter := new(MockWorkflowStarter)

		payload := SequinWebhookPayload{
			Action: "delete",
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("POST", "/delete-user", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()

		handler := NewWebhookHandler(starter, "test-task-queue")
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
	})
}
