package temporal

import (
	"context"
	"crypto/tls"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
)

// Client is a wrapper around the official Temporal client,
// providing methods with a simplified interface.
type Client struct {
	rawClient client.Client
	config    *Config
}

// NewClient creates a new connection to Temporal Server with optional TLS configuration.
func NewClient(cfg *Config) (*Client, error) {
	options := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	}

	if cfg.CertPath != "" && cfg.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
		if err != nil {
			return nil, err
		}
		options.ConnectionOptions = client.ConnectionOptions{
			TLS: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}
	}

	if len(cfg.EncryptionKey) > 0 {
		dc, err := GetEncryptingDataConverter(cfg.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create encrypting data converter: %w", err)
		}
		options.DataConverter = dc
	}

	c, err := client.Dial(options)
	if err != nil {
		return nil, err
	}

	return &Client{
		rawClient: c,
		config:    cfg,
	}, nil
}

// ExecuteWorkflow starts Workflow execution asynchronously and returns run info.
func (c *Client) ExecuteWorkflow(
	ctx context.Context,
	options client.StartWorkflowOptions,
	workflowFunc any,
	args ...any,
) (client.WorkflowRun, error) {
	if options.TaskQueue == "" {
		options.TaskQueue = c.config.TaskQueue
	}
	return c.rawClient.ExecuteWorkflow(ctx, options, workflowFunc, args...)
}

// SignalWorkflow sends a Signal to an active Workflow.
func (c *Client) SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg any) error {
	return c.rawClient.SignalWorkflow(ctx, workflowID, runID, signalName, arg)
}

// QueryWorkflow sends a state query to an active or completed Workflow.
func (c *Client) QueryWorkflow(ctx context.Context, workflowID string, runID string, queryType string, args ...any) (converter.EncodedValue, error) {
	return c.rawClient.QueryWorkflow(ctx, workflowID, runID, queryType, args...)
}

// RawClient returns the underlying SDK client for executing complex low-level operations.
func (c *Client) RawClient() client.Client {
	return c.rawClient
}

// Close closes the connection to Temporal Server.
func (c *Client) Close() {
	if c.rawClient != nil {
		c.rawClient.Close()
	}
}
