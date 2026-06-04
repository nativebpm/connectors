package nativebpm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nativebpm/httpstream"
)

// Client represents the NativeBPM SDK client.
type Client struct {
	httpClient *httpstream.Client
}

// NewClient creates a new NativeBPM SDK Client.
func NewClient(baseURL string) (*Client, error) {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	httpClient, err := httpstream.NewClient(&http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create httpstream client: %w", err)
	}

	return &Client{
		httpClient: httpClient,
	}, nil
}

// Use registers a middleware transport layer for the underlying HTTP client.
func (c *Client) Use(middleware func(http.RoundTripper) http.RoundTripper) *Client {
	c.httpClient = c.httpClient.Use(middleware)
	return c
}

// --- Fluent API Builders ---

// DeployBuilder provides a fluent builder for deploying a process.
type DeployBuilder struct {
	client  *Client
	id      string
	name    string
	xmlData []byte
}

// Deploy creates a new DeployBuilder.
func (c *Client) Deploy(id, name string) *DeployBuilder {
	return &DeployBuilder{
		client: c,
		id:     id,
		name:   name,
	}
}

// XML sets the XML bpmn/dmn data for the deployment.
func (b *DeployBuilder) XML(xmlData []byte) *DeployBuilder {
	b.xmlData = xmlData
	return b
}

// Send executes the deployment request.
func (b *DeployBuilder) Send(ctx context.Context) (*DeploymentResponse, error) {
	resp, err := b.client.httpClient.Multipart(ctx, "/api/deploy").
		Param("id", b.id).
		Param("name", b.name).
		File("file", b.name+".bpmn", bytes.NewReader(b.xmlData)).
		Send()
	if err != nil {
		return nil, fmt.Errorf("deploy request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deploy failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result DeploymentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// StartInstanceBuilder provides a fluent builder for launching a process instance.
type StartInstanceBuilder struct {
	client      *Client
	processID   string
	instanceID  string
	businessKey string
	variables   map[string]interface{}
}

// StartProcessInstance creates a new StartInstanceBuilder.
func (c *Client) StartProcessInstance(processID string) *StartInstanceBuilder {
	return &StartInstanceBuilder{
		client:    c,
		processID: processID,
		variables: make(map[string]interface{}),
	}
}

// InstanceID configures a specific ID for the process instance.
func (b *StartInstanceBuilder) InstanceID(instanceID string) *StartInstanceBuilder {
	b.instanceID = instanceID
	return b
}

// BusinessKey configures the business key for the process instance.
func (b *StartInstanceBuilder) BusinessKey(businessKey string) *StartInstanceBuilder {
	b.businessKey = businessKey
	return b
}

// Variable sets a single variable value.
func (b *StartInstanceBuilder) Variable(name string, value interface{}) *StartInstanceBuilder {
	b.variables[name] = value
	return b
}

// Variables adds multiple variables to the instance request.
func (b *StartInstanceBuilder) Variables(vars map[string]interface{}) *StartInstanceBuilder {
	for k, v := range vars {
		b.variables[k] = v
	}
	return b
}

type startInstanceRequest struct {
	InstanceID  string                 `json:"instance_id"`
	BusinessKey string                 `json:"business_key"`
	Variables   map[string]interface{} `json:"variables"`
}

// Send executes the start process instance request.
func (b *StartInstanceBuilder) Send(ctx context.Context) (*ProcessInstance, error) {
	payload := startInstanceRequest{
		InstanceID:  b.instanceID,
		BusinessKey: b.businessKey,
		Variables:   b.variables,
	}
	resp, err := b.client.httpClient.POST(ctx, "/api/definitions/{id}/start").
		PathParam("id", b.processID).
		JSON(payload).
		Send()
	if err != nil {
		return nil, fmt.Errorf("start process instance request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("start process instance failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ProcessInstance
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// CompleteTaskBuilder provides a fluent builder for completing a user task or wait state.
type CompleteTaskBuilder struct {
	client     *Client
	instanceID string
	nodeID     string
	variables  map[string]interface{}
}

// CompleteTask creates a new CompleteTaskBuilder.
func (c *Client) CompleteTask(instanceID, nodeID string) *CompleteTaskBuilder {
	return &CompleteTaskBuilder{
		client:     c,
		instanceID: instanceID,
		nodeID:     nodeID,
		variables:  make(map[string]interface{}),
	}
}

// Variable sets a single variable value.
func (b *CompleteTaskBuilder) Variable(name string, value interface{}) *CompleteTaskBuilder {
	b.variables[name] = value
	return b
}

// Variables adds multiple variables to the task completion request.
func (b *CompleteTaskBuilder) Variables(vars map[string]interface{}) *CompleteTaskBuilder {
	for k, v := range vars {
		b.variables[k] = v
	}
	return b
}

type completeTaskRequest struct {
	NodeID    string                 `json:"node_id"`
	Variables map[string]interface{} `json:"variables"`
}

// Send executes the task completion request.
func (b *CompleteTaskBuilder) Send(ctx context.Context) (*ProcessInstance, error) {
	payload := completeTaskRequest{
		NodeID:    b.nodeID,
		Variables: b.variables,
	}
	resp, err := b.client.httpClient.POST(ctx, "/api/instances/{id}/complete").
		PathParam("id", b.instanceID).
		JSON(payload).
		Send()
	if err != nil {
		return nil, fmt.Errorf("complete task request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("complete task failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ProcessInstance
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// ResumeInstanceBuilder provides a fluent builder for resuming a process instance.
type ResumeInstanceBuilder struct {
	client     *Client
	instanceID string
	variables  map[string]interface{}
}

// ResumeProcessInstance creates a new ResumeInstanceBuilder.
func (c *Client) ResumeProcessInstance(instanceID string) *ResumeInstanceBuilder {
	return &ResumeInstanceBuilder{
		client:     c,
		instanceID: instanceID,
		variables:  make(map[string]interface{}),
	}
}

// Variable sets a single variable value.
func (b *ResumeInstanceBuilder) Variable(name string, value interface{}) *ResumeInstanceBuilder {
	b.variables[name] = value
	return b
}

// Variables adds multiple variables to the resumption request.
func (b *ResumeInstanceBuilder) Variables(vars map[string]interface{}) *ResumeInstanceBuilder {
	for k, v := range vars {
		b.variables[k] = v
	}
	return b
}

type resumeInstanceRequest struct {
	Variables map[string]interface{} `json:"variables"`
}

// Send executes the resume process instance request.
func (b *ResumeInstanceBuilder) Send(ctx context.Context) (*ProcessInstance, error) {
	payload := resumeInstanceRequest{
		Variables: b.variables,
	}
	resp, err := b.client.httpClient.POST(ctx, "/api/instances/{id}/resume").
		PathParam("id", b.instanceID).
		JSON(payload).
		Send()
	if err != nil {
		return nil, fmt.Errorf("resume process instance request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resume process instance failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ProcessInstance
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// --- Query methods ---

// ListDefinitions retrieves all deployed process definitions.
func (c *Client) ListDefinitions(ctx context.Context) ([]*ProcessDefinition, error) {
	resp, err := c.httpClient.GET(ctx, "/api/definitions").Send()
	if err != nil {
		return nil, fmt.Errorf("list definitions request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list definitions failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result []*ProcessDefinition
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// ListInstances lists all process instance snapshots in the storage.
func (c *Client) ListInstances(ctx context.Context) ([]*ProcessInstanceRecord, error) {
	resp, err := c.httpClient.GET(ctx, "/api/instances").Send()
	if err != nil {
		return nil, fmt.Errorf("list instances request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list instances failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result []*ProcessInstanceRecord
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// GetInstance fetches the current active state of a process instance.
func (c *Client) GetInstance(ctx context.Context, id string) (*ProcessInstance, error) {
	resp, err := c.httpClient.GET(ctx, "/api/instances/{id}").
		PathParam("id", id).
		Send()
	if err != nil {
		return nil, fmt.Errorf("get instance request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get instance failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ProcessInstance
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// GetInstanceLogs retrieves audit log records for visual state tracking.
func (c *Client) GetInstanceLogs(ctx context.Context, instanceID string) ([]*LogRecord, error) {
	resp, err := c.httpClient.GET(ctx, "/api/instances/{id}/logs").
		PathParam("id", instanceID).
		Send()
	if err != nil {
		return nil, fmt.Errorf("get instance logs request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get instance logs failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result []*LogRecord
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}
