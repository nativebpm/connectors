package streamhttp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/nativebpm/connectors/streamhttp/internal/httprequest"
	"github.com/nativebpm/connectors/streamhttp/internal/httptransport"
)

type method string

const (
	GET     method = http.MethodGet
	POST    method = http.MethodPost
	PUT     method = http.MethodPut
	PATCH   method = http.MethodPatch
	DELETE  method = http.MethodDelete
	HEAD    method = http.MethodHead
	CONNECT method = http.MethodConnect
	OPTIONS method = http.MethodOptions
	TRACE   method = http.MethodTrace
)

type Multipart = httprequest.Multipart
type Request = httprequest.Request

type streamhttp struct {
	client  http.Client
	baseURL url.URL
	// mu protects access to middlewares for concurrent Use()/Request() calls.
	mu          sync.RWMutex
	middlewares []func(http.RoundTripper) http.RoundTripper
}

// clone creates a shallow copy of the underlying http.Client and applies the
// configured middlewares to the copy's Transport. This avoids mutating the
// shared client or its Transport when building per-request middleware chains,
// preventing data races when the streamhttp is used concurrently.
func (c *streamhttp) clone() http.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()

	client := c.client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	for _, mw := range c.middlewares {
		transport = mw(transport)
	}
	client.Transport = transport
	return client
}

func NewClient(client http.Client, baseURL string) (*streamhttp, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	return &streamhttp{
		client:      client,
		baseURL:     *u,
		middlewares: []func(http.RoundTripper) http.RoundTripper{},
	}, nil
}

func (c *streamhttp) url(path string) string {
	return c.baseURL.JoinPath(path).String()
}

func (c *streamhttp) Use(middleware func(http.RoundTripper) http.RoundTripper) *streamhttp {
	c.mu.Lock()
	c.middlewares = append(c.middlewares, middleware)
	c.mu.Unlock()
	return c
}

func (c *streamhttp) Request(ctx context.Context, method method, path string) *httprequest.Request {
	return httprequest.NewRequest(ctx, c.clone(), string(method), c.url(path))
}

func (c *streamhttp) MultipartRequest(ctx context.Context, method method, path string) *httprequest.Multipart {
	return httprequest.NewMultipart(ctx, c.clone(), string(method), c.url(path))
}

func (c *streamhttp) GET(ctx context.Context, path string) *httprequest.Request {
	return c.Request(ctx, GET, path)
}

func (c *streamhttp) POST(ctx context.Context, path string) *httprequest.Request {
	return c.Request(ctx, POST, path)
}

func (c *streamhttp) PUT(ctx context.Context, path string) *httprequest.Request {
	return c.Request(ctx, PUT, path)
}

func (c *streamhttp) PATCH(ctx context.Context, path string) *httprequest.Request {
	return c.Request(ctx, PATCH, path)
}

func (c *streamhttp) DELETE(ctx context.Context, path string) *httprequest.Request {
	return c.Request(ctx, DELETE, path)
}

func (c *streamhttp) Multipart(ctx context.Context, path string) *httprequest.Multipart {
	return c.MultipartRequest(ctx, POST, path)
}

func (c *streamhttp) WithLogger(logger *slog.Logger) *streamhttp {
	return c.Use(httptransport.LoggingMiddleware(logger))
}

// ConcurrencyMiddleware is a convenience wrapper that exposes the internal
// concurrency limiter middleware for external packages. It returns a
// Middleware that limits the number of concurrent in-flight HTTP requests.
func ConcurrencyMiddleware(limit int) func(http.RoundTripper) http.RoundTripper {
	return httptransport.ConcurrencyMiddleware(limit)
}
