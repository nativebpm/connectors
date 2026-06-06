package restate

import (
	"context"

	restateingress "github.com/restatedev/sdk-go/ingress"
)

// Client is a wrapper around the Restate Ingress client.
type Client struct {
	rawClient *restateingress.Client
	config    *Config
}

// NewClient creates a new Client pointing to the Restate Server coordinator.
func NewClient(cfg *Config) *Client {
	c := restateingress.NewClient(cfg.ServerURL)
	return &Client{
		rawClient: c,
		config:    cfg,
	}
}

// ServiceCall represents a typed service invocation client.
type ServiceCall[Req any, Resp any] struct {
	client *Client
	name   string
	method string
}

// Service creates a caller for a Restate basic service handler.
func Service[Req any, Resp any](c *Client, name string, method string) *ServiceCall[Req, Resp] {
	return &ServiceCall[Req, Resp]{
		client: c,
		name:   name,
		method: method,
	}
}

// Request sends a request to the service and waits for the response.
func (sc *ServiceCall[Req, Resp]) Request(ctx context.Context, req Req) (Resp, error) {
	return restateingress.Service[Req, Resp](sc.client.rawClient, sc.name, sc.method).Request(ctx, req)
}

// Send sends a request as a one-way fire-and-forget message.
func (sc *ServiceCall[Req, Resp]) Send(ctx context.Context, req Req) error {
	_, err := restateingress.ServiceSend[Req](sc.client.rawClient, sc.name, sc.method).Send(ctx, req)
	return err
}

// ObjectCall represents a typed virtual object invocation client.
type ObjectCall[Req any, Resp any] struct {
	client *Client
	name   string
	key    string
	method string
}

// Object creates a caller for a Restate virtual object handler.
func Object[Req any, Resp any](c *Client, name string, key string, method string) *ObjectCall[Req, Resp] {
	return &ObjectCall[Req, Resp]{
		client: c,
		name:   name,
		key:    key,
		method: method,
	}
}

// Request sends a request to the virtual object and waits for the response.
func (oc *ObjectCall[Req, Resp]) Request(ctx context.Context, req Req) (Resp, error) {
	return restateingress.Object[Req, Resp](oc.client.rawClient, oc.name, oc.key, oc.method).Request(ctx, req)
}

// Send sends a request to the virtual object as a one-way fire-and-forget message.
func (oc *ObjectCall[Req, Resp]) Send(ctx context.Context, req Req) error {
	_, err := restateingress.ObjectSend[Req](oc.client.rawClient, oc.name, oc.key, oc.method).Send(ctx, req)
	return err
}

// RawClient returns the underlying Restate Ingress Client.
func (c *Client) RawClient() *restateingress.Client {
	return c.rawClient
}
