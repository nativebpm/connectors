package temporal

import (
	"context"
	"crypto/tls"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
)

// Client является оберткой над официальным клиентом Temporal, 
// предоставляя методы с упрощенным интерфейсом.
type Client struct {
	rawClient client.Client
	config    *Config
}

// NewClient создает новое подключение к Temporal Server с возможностью настройки TLS.
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

	c, err := client.Dial(options)
	if err != nil {
		return nil, err
	}

	return &Client{
		rawClient: c,
		config:    cfg,
	}, nil
}

// ExecuteWorkflow запускает выполнение Workflow асинхронно и возвращает информацию о его запуске.
func (c *Client) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflowFunc any, args ...any) (client.WorkflowRun, error) {
	if options.TaskQueue == "" {
		options.TaskQueue = c.config.TaskQueue
	}
	return c.rawClient.ExecuteWorkflow(ctx, options, workflowFunc, args...)
}

// SignalWorkflow отправляет сигнал (Signal) в активный Workflow.
func (c *Client) SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg any) error {
	return c.rawClient.SignalWorkflow(ctx, workflowID, runID, signalName, arg)
}

// QueryWorkflow отправляет запрос (Query) состояния в активный или завершенный Workflow.
func (c *Client) QueryWorkflow(ctx context.Context, workflowID string, runID string, queryType string, args ...any) (converter.EncodedValue, error) {
	return c.rawClient.QueryWorkflow(ctx, workflowID, runID, queryType, args...)
}

// RawClient возвращает базовый клиент SDK для выполнения сложных низкоуровневых операций.
func (c *Client) RawClient() client.Client {
	return c.rawClient
}

// Close закрывает сетевое подключение к Temporal Server.
func (c *Client) Close() {
	if c.rawClient != nil {
		c.rawClient.Close()
	}
}
