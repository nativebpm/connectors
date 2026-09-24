// Package ironpress provides a client and server wrapper for the Rust-based ironpress PDF converter.
package ironpress

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nativebpm/connectors/wasmee"
	"github.com/nativebpm/connectors/wasmee/olme"
	"github.com/nativebpm/httpstream"
)

// Mode represents the execution engine type.
type Mode int

const (
	// HTTP_CLI_Mode executes conversions via HTTP requests to an ironpress server wrapper.
	HTTP_CLI_Mode Mode = iota
	// Pure_WASM_Mode executes conversions in-process using WebAssembly via wazero.
	Pure_WASM_Mode
	// WASMEE_Mode executes conversions on the distributed durable wasmee runtime.
	WASMEE_Mode
)

// Client handles document conversions using HTTP, WebAssembly, or WASMEE.
type Client struct {
	httpStream  *httpstream.Client
	serverURL   string
	wasmBytes   []byte
	wasmeeAddr  string
	wasmeeStore olme.SnapshotStore
}

// Option configures the Client.
type Option func(*Client)

// WithHTTP configures the client to connect to an ironpress HTTP server.
func WithHTTP(httpClient *http.Client, serverURL string) Option {
	return func(c *Client) {
		c.serverURL = serverURL
		stream, err := httpstream.NewClient(httpClient, serverURL)
		if err == nil {
			c.httpStream = stream
		}
	}
}

// WithWasm configures the client with compiled ironpress WASM module bytes for in-memory execution.
func WithWasm(wasmBytes []byte) Option {
	return func(c *Client) {
		c.wasmBytes = wasmBytes
	}
}

// WithWasmee configures the client to execute conversions via the wasmee durable engine.
func WithWasmee(serverAddr string, store olme.SnapshotStore, wasmBytes []byte) Option {
	return func(c *Client) {
		c.wasmeeAddr = serverAddr
		c.wasmeeStore = store
		c.wasmBytes = wasmBytes
	}
}

// NewClient creates a new ironpress client using the provided configuration options.
func NewClient(opts ...Option) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Convert initiates a new PDF conversion request builder with the specified execution mode.
func (c *Client) Convert(mode Mode) *Request {
	return &Request{
		client: c,
		mode:   mode,
	}
}

// Request is a unified fluent builder for ironpress conversions with a sticky error pattern.
type Request struct {
	client    *Client
	mode      Mode
	err       error
	sessionID string // used for WASMEE instance tracking

	fileContent io.Reader
	fileName    string
	pageSize    string
	landscape   *bool
	margin      *float64
	header      string
	footer      string
	timeout     time.Duration
}

// Error returns any accumulated error during method chaining.
func (r *Request) Error() error {
	return r.err
}

// setErr records the first error that occurs.
func (r *Request) setErr(err error) {
	if r.err == nil {
		r.err = err
	}
}

// SessionID configures the durable execution session identifier (required for WASMEE_Mode).
func (r *Request) SessionID(id string) *Request {
	if r.err != nil {
		return r
	}
	r.sessionID = id
	return r
}

// HTML sets the HTML string content to convert.
func (r *Request) HTML(content string) *Request {
	if r.err != nil {
		return r
	}
	r.fileContent = strings.NewReader(content)
	r.fileName = "index.html"
	return r
}

// HTMLReader sets the HTML content reader to convert.
func (r *Request) HTMLReader(reader io.Reader) *Request {
	if r.err != nil {
		return r
	}
	r.fileContent = reader
	r.fileName = "index.html"
	return r
}

// Markdown sets the Markdown string content to convert.
func (r *Request) Markdown(content string) *Request {
	if r.err != nil {
		return r
	}
	r.fileContent = strings.NewReader(content)
	r.fileName = "document.md"
	return r
}

// MarkdownReader sets the Markdown content reader to convert.
func (r *Request) MarkdownReader(reader io.Reader) *Request {
	if r.err != nil {
		return r
	}
	r.fileContent = reader
	r.fileName = "document.md"
	return r
}

// PageSize sets the page size (e.g. "a4", "letter").
func (r *Request) PageSize(size string) *Request {
	if r.err != nil {
		return r
	}
	r.pageSize = size
	return r
}

// Landscape sets the orientation to landscape if true.
func (r *Request) Landscape(landscape bool) *Request {
	if r.err != nil {
		return r
	}
	r.landscape = &landscape
	return r
}

// Margin sets the document margins.
func (r *Request) Margin(margin float64) *Request {
	if r.err != nil {
		return r
	}
	r.margin = &margin
	return r
}

// Header sets the running header text.
func (r *Request) Header(text string) *Request {
	if r.err != nil {
		return r
	}
	r.header = text
	return r
}

// Footer sets the running footer text (supports placeholders like {page} and {pages}).
func (r *Request) Footer(text string) *Request {
	if r.err != nil {
		return r
	}
	r.footer = text
	return r
}

// Timeout sets a timeout for this request.
func (r *Request) Timeout(d time.Duration) *Request {
	if r.err != nil {
		return r
	}
	r.timeout = d
	return r
}

// Do executes the conversion request using the selected mode and returns the PDF document bytes.
func (r *Request) Do(ctx context.Context) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.fileContent == nil {
		return nil, fmt.Errorf("no HTML or Markdown content provided")
	}

	switch r.mode {
	case HTTP_CLI_Mode:
		return r.doHTTP(ctx)
	case Pure_WASM_Mode:
		return r.doWasm(ctx)
	case WASMEE_Mode:
		return r.doWasmee(ctx)
	default:
		return nil, fmt.Errorf("unsupported execution mode")
	}
}

func (r *Request) doHTTP(ctx context.Context) ([]byte, error) {
	if r.client.httpStream == nil {
		return nil, fmt.Errorf("http client is not configured (use WithHTTP option)")
	}

	req := r.client.httpStream.Multipart(ctx, "/convert")
	if r.timeout > 0 {
		req.Timeout(r.timeout)
	}

	req.File("file", r.fileName, r.fileContent)

	if r.pageSize != "" {
		req.Param("page-size", r.pageSize)
	}
	if r.landscape != nil {
		req.Bool("landscape", *r.landscape)
	}
	if r.margin != nil {
		req.Float("margin", *r.margin)
	}
	if r.header != "" {
		req.Param("header", r.header)
	}
	if r.footer != "" {
		req.Param("footer", r.footer)
	}

	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("request execution failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, resp.Body)
		return nil, fmt.Errorf("conversion failed with status %d: %s", resp.StatusCode, buf.String())
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func (r *Request) doWasmee(ctx context.Context) ([]byte, error) {
	if r.client.wasmeeAddr == "" {
		return nil, fmt.Errorf("wasmee address is not configured (use WithWasmee option)")
	}
	if r.client.wasmeeStore == nil {
		return nil, fmt.Errorf("wasmee store is not configured (use WithWasmee option)")
	}
	if r.sessionID == "" {
		return nil, fmt.Errorf("session ID is required for WASMEE mode")
	}

	// Read input content to pass through exchange buffer
	contentBytes, err := io.ReadAll(r.fileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to read input content: %w", err)
	}

	runner := wasmee.NewFluentRunner().
		WithContext(ctx).
		WithServerAddress(r.client.wasmeeAddr).
		WithWasmBytes(r.client.wasmBytes).
		WithStore(r.client.wasmeeStore).
		WithSessionID(r.sessionID).
		WithEntrypoint("execute").
		WithExchangeBuffer(contentBytes)

	crashed, err := runner.Run()
	if err != nil {
		return nil, fmt.Errorf("wasmee execution failed (crashed=%t): %w", crashed, err)
	}

	return runner.Response(), nil
}
