package camunda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/nativebpm/connectors/httpclient"
)

// ExternalTask represents a Camunda external task
type ExternalTask struct {
	ID                  string              `json:"id"`
	TopicName           string              `json:"topicName"`
	WorkerID            string              `json:"workerId"`
	LockExpirationTime  *time.Time          `json:"lockExpirationTime,omitempty"`
	Retries             *int                `json:"retries,omitempty"`
	ErrorMessage        string              `json:"errorMessage,omitempty"`
	ErrorDetails        string              `json:"errorDetails,omitempty"`
	Variables           map[string]Variable `json:"variables,omitempty"`
	BusinessKey         string              `json:"businessKey,omitempty"`
	TenantID            string              `json:"tenantId,omitempty"`
	Priority            int                 `json:"priority,omitempty"`
	ActivityID          string              `json:"activityId,omitempty"`
	ActivityInstanceID  string              `json:"activityInstanceId,omitempty"`
	ExecutionID         string              `json:"executionId,omitempty"`
	ProcessInstanceID   string              `json:"processInstanceId,omitempty"`
	ProcessDefinitionID string              `json:"processDefinitionId,omitempty"`
}

// Variable represents a Camunda variable with type safety
type Variable struct {
	Value     any    `json:"value"`
	Type      string `json:"type"`
	ValueInfo any    `json:"valueInfo,omitempty"`
}

// StringVariable creates a string variable
func StringVariable(value string) Variable {
	return Variable{
		Value: value,
		Type:  "String",
	}
}

// IntVariable creates an integer variable
func IntVariable(value int64) Variable {
	return Variable{
		Value: value,
		Type:  "Integer",
	}
}

// LongVariable creates a long variable
func LongVariable(value int64) Variable {
	return Variable{
		Value: value,
		Type:  "Long",
	}
}

// DoubleVariable creates a double variable
func DoubleVariable(value float64) Variable {
	return Variable{
		Value: value,
		Type:  "Double",
	}
}

// BooleanVariable creates a boolean variable
func BooleanVariable(value bool) Variable {
	return Variable{
		Value: value,
		Type:  "Boolean",
	}
}

// DateVariable creates a date variable
func DateVariable(value time.Time) Variable {
	return Variable{
		Value: value.Format(time.RFC3339),
		Type:  "Date",
	}
}

// JSONVariable creates a JSON variable from any value
func JSONVariable(value any) Variable {
	return Variable{
		Value: value,
		Type:  "json",
	}
}

// NullVariable creates a null variable
func NullVariable() Variable {
	return Variable{
		Value: nil,
		Type:  "Null",
	}
}

// Client represents a Camunda external task client
type Client struct {
	httpClient *httpclient.HTTPClient
	workerID   string
}

// NewClient creates a new Camunda external task client
func NewClient(hostURL, workerID string) (*Client, error) {
	baseURL := hostURL + "/engine-rest"
	httpClient, err := httpclient.NewClient(http.Client{Timeout: 30 * time.Second}, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &Client{
		httpClient: httpClient,
		workerID:   workerID,
	}, nil
}

// Use adds middleware to the HTTP client
func (c *Client) Use(middleware httpclient.Middleware) *Client {
	c.httpClient.Use(middleware)
	return c
}

// WithLogger adds logging middleware to the HTTP client
func (c *Client) WithLogger(logger *slog.Logger) *Client {
	c.httpClient.WithLogger(logger)
	return c
}

// FetchAndLock fetches and locks external tasks for the given topics
func (c *Client) FetchAndLock(ctx context.Context, topics []TopicRequest, maxTasks int, asyncResponseTimeout *int) ([]ExternalTask, error) {
	req := struct {
		WorkerID             string         `json:"workerId"`
		MaxTasks             int            `json:"maxTasks"`
		UsePriority          bool           `json:"usePriority"`
		Topics               []TopicRequest `json:"topics"`
		AsyncResponseTimeout *int           `json:"asyncResponseTimeout,omitempty"`
	}{
		WorkerID:             c.workerID,
		MaxTasks:             maxTasks,
		UsePriority:          true,
		Topics:               topics,
		AsyncResponseTimeout: asyncResponseTimeout,
	}

	resp, err := c.httpClient.POST(ctx, "/external-task/fetchAndLock").
		JSON(req).
		Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send fetchAndLock request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchAndLock request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tasks []ExternalTask
	if err := json.Unmarshal(body, &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tasks: %w", err)
	}

	return tasks, nil
}

// TopicRequest represents a topic request for fetching tasks
type TopicRequest struct {
	TopicName            string   `json:"topicName"`
	LockDuration         int      `json:"lockDuration"`
	Variables            []string `json:"variables,omitempty"`
	LocalVariables       bool     `json:"localVariables,omitempty"`
	BusinessKey          string   `json:"businessKey,omitempty"`
	ProcessDefinitionID  string   `json:"processDefinitionId,omitempty"`
	ProcessDefinitionKey string   `json:"processDefinitionKey,omitempty"`
	TenantIDs            []string `json:"tenantIds,omitempty"`
}

// TaskCompletion provides a fluent API for completing external tasks
type TaskCompletion struct {
	client         *Client
	ctx            context.Context
	taskID         string
	variables      map[string]Variable
	localVariables map[string]Variable
}

// Complete creates a new TaskCompletion builder
func (c *Client) Complete(taskID string) *TaskCompletion {
	return &TaskCompletion{
		client:         c,
		ctx:            context.Background(),
		taskID:         taskID,
		variables:      make(map[string]Variable),
		localVariables: make(map[string]Variable),
	}
}

// Context sets the context for the completion request
func (tc *TaskCompletion) Context(ctx context.Context) *TaskCompletion {
	tc.ctx = ctx
	return tc
}

// Variable adds a process variable
func (tc *TaskCompletion) Variable(name string, value Variable) *TaskCompletion {
	tc.variables[name] = value
	return tc
}

// Variables adds multiple process variables
func (tc *TaskCompletion) Variables(vars map[string]Variable) *TaskCompletion {
	for k, v := range vars {
		tc.variables[k] = v
	}
	return tc
}

// LocalVariable adds a local variable
func (tc *TaskCompletion) LocalVariable(name string, value Variable) *TaskCompletion {
	tc.localVariables[name] = value
	return tc
}

// LocalVariables adds multiple local variables
func (tc *TaskCompletion) LocalVariables(vars map[string]Variable) *TaskCompletion {
	for k, v := range vars {
		tc.localVariables[k] = v
	}
	return tc
}

// Execute sends the completion request
func (tc *TaskCompletion) Execute() error {
	req := struct {
		WorkerID       string              `json:"workerId"`
		Variables      map[string]Variable `json:"variables,omitempty"`
		LocalVariables map[string]Variable `json:"localVariables,omitempty"`
	}{
		WorkerID:       tc.client.workerID,
		Variables:      tc.variables,
		LocalVariables: tc.localVariables,
	}

	resp, err := tc.client.httpClient.POST(tc.ctx, "/external-task/{taskID}/complete").
		PathParam("taskID", tc.taskID).
		JSON(req).
		Send()
	if err != nil {
		return fmt.Errorf("failed to send complete request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("complete request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// TaskFailure provides a fluent API for reporting task failures
type TaskFailure struct {
	client       *Client
	ctx          context.Context
	taskID       string
	errorMessage string
	errorDetails string
	retries      int
	retryTimeout int
}

// Failure creates a new TaskFailure builder
func (c *Client) Failure(taskID string) *TaskFailure {
	return &TaskFailure{
		client:       c,
		ctx:          context.Background(),
		taskID:       taskID,
		retries:      0,
		retryTimeout: 0,
	}
}

// Context sets the context for the failure request
func (tf *TaskFailure) Context(ctx context.Context) *TaskFailure {
	tf.ctx = ctx
	return tf
}

// ErrorMessage sets the error message
func (tf *TaskFailure) ErrorMessage(msg string) *TaskFailure {
	tf.errorMessage = msg
	return tf
}

// ErrorDetails sets the error details
func (tf *TaskFailure) ErrorDetails(details string) *TaskFailure {
	tf.errorDetails = details
	return tf
}

// Retries sets the number of retries
func (tf *TaskFailure) Retries(count int) *TaskFailure {
	tf.retries = count
	return tf
}

// RetryTimeout sets the retry timeout in milliseconds
func (tf *TaskFailure) RetryTimeout(timeout int) *TaskFailure {
	tf.retryTimeout = timeout
	return tf
}

// Execute sends the failure request
func (tf *TaskFailure) Execute() error {
	req := struct {
		WorkerID     string `json:"workerId"`
		ErrorMessage string `json:"errorMessage,omitempty"`
		ErrorDetails string `json:"errorDetails,omitempty"`
		Retries      int    `json:"retries,omitempty"`
		RetryTimeout int    `json:"retryTimeout,omitempty"`
	}{
		WorkerID:     tf.client.workerID,
		ErrorMessage: tf.errorMessage,
		ErrorDetails: tf.errorDetails,
		Retries:      tf.retries,
		RetryTimeout: tf.retryTimeout,
	}

	resp, err := tf.client.httpClient.POST(tf.ctx, "/external-task/{taskID}/failure").
		PathParam("taskID", tf.taskID).
		JSON(req).
		Send()
	if err != nil {
		return fmt.Errorf("failed to send failure request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failure request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// LockExtension provides a fluent API for extending task locks
type LockExtension struct {
	client      *Client
	ctx         context.Context
	taskID      string
	newDuration int
}

// ExtendLock creates a new LockExtension builder
func (c *Client) ExtendLock(taskID string, newDuration int) *LockExtension {
	return &LockExtension{
		client:      c,
		ctx:         context.Background(),
		taskID:      taskID,
		newDuration: newDuration,
	}
}

// Context sets the context for the lock extension request
func (le *LockExtension) Context(ctx context.Context) *LockExtension {
	le.ctx = ctx
	return le
}

// Execute sends the lock extension request
func (le *LockExtension) Execute() error {
	req := struct {
		WorkerID    string `json:"workerId"`
		NewDuration int    `json:"newDuration"`
	}{
		WorkerID:    le.client.workerID,
		NewDuration: le.newDuration,
	}

	resp, err := le.client.httpClient.POST(le.ctx, "/external-task/{taskID}/extendLock").
		PathParam("taskID", le.taskID).
		JSON(req).
		Send()
	if err != nil {
		return fmt.Errorf("failed to send extendLock request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("extendLock request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// TaskUnlock provides a fluent API for unlocking tasks
type TaskUnlock struct {
	client *Client
	ctx    context.Context
	taskID string
}

// Unlock creates a new TaskUnlock builder
func (c *Client) Unlock(taskID string) *TaskUnlock {
	return &TaskUnlock{
		client: c,
		ctx:    context.Background(),
		taskID: taskID,
	}
}

// Context sets the context for the unlock request
func (tu *TaskUnlock) Context(ctx context.Context) *TaskUnlock {
	tu.ctx = ctx
	return tu
}

// Execute sends the unlock request
func (tu *TaskUnlock) Execute() error {
	req := struct {
		WorkerID string `json:"workerId"`
	}{
		WorkerID: tu.client.workerID,
	}

	resp, err := tu.client.httpClient.POST(tu.ctx, "/external-task/{taskID}/unlock").
		PathParam("taskID", tu.taskID).
		JSON(req).
		Send()
	if err != nil {
		return fmt.Errorf("failed to send unlock request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unlock request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// StartProcessInstance starts a new process instance by process definition key
func (c *Client) StartProcessInstance(ctx context.Context, processDefinitionKey string, variables map[string]interface{}) (string, error) {
	// Prepare the request payload
	payload := map[string]interface{}{
		"variables": make(map[string]map[string]interface{}),
	}

	for key, value := range variables {
		payload["variables"].(map[string]map[string]interface{})[key] = map[string]interface{}{
			"value": value,
		}
	}

	resp, err := c.httpClient.POST(ctx, "/process-definition/key/{processDefinitionKey}/start").
		PathParam("processDefinitionKey", processDefinitionKey).
		JSON(payload).
		Send()
	if err != nil {
		return "", fmt.Errorf("failed to send start process request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("start process request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal process instance: %w", err)
	}

	return result.ID, nil
}

// DeployProcess deploys a BPMN process definition to Camunda
func (c *Client) DeployProcess(ctx context.Context, deploymentName string, bpmnReader io.Reader, filename string) (string, error) {
	resp, err := c.httpClient.Multipart(ctx, "/deployment/create").
		Param("deployment-name", deploymentName).
		Param("enable-duplicate-filtering", "true").
		File("data", filename, bpmnReader).
		Send()
	if err != nil {
		return "", fmt.Errorf("failed to send deploy request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deploy request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	return result.ID, nil
}

// PollTasks polls for external tasks in a loop and processes them using the provided handler
func (c *Client) PollTasks(ctx context.Context, topics []TopicRequest, maxTasks int, handler func(*Client, ExternalTask)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tasks, err := c.FetchAndLock(ctx, topics, maxTasks, nil)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if len(tasks) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		// Log fetched tasks
		slog.Default().Info("Fetched tasks", "count", len(tasks))

		// Process each task
		for _, task := range tasks {
			go handler(c, task)
		}

		// Wait a bit before next poll
		time.Sleep(1 * time.Second)
	}
}
