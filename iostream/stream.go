package iostream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// WriterFunc defines a producer function that writes data to an io.Writer.
type WriterFunc func(w io.Writer) error

// PipeReader constructs a zero-allocation io.Reader by executing writerFn in a managed background goroutine.
// It handles io.Pipe lifecycle, CloseWithError propagation, and cleanup automatically.
func PipeReader(writerFn WriterFunc) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		if err := writerFn(pw); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	return pr
}

// StreamBuilder provides a clean Fluent API for zero-allocation streaming requests.
type StreamBuilder struct {
	writerFn   WriterFunc
	err        error
	headers    http.Header
	method     string
	targetURL  string
	httpClient *http.Client
}

// NewStream creates a new StreamBuilder instance.
func NewStream() *StreamBuilder {
	return &StreamBuilder{
		headers:    make(http.Header),
		method:     http.MethodPost,
		httpClient: http.DefaultClient,
	}
}

// WithWriter sets a custom WriterFunc for streaming data.
func (b *StreamBuilder) WithWriter(fn WriterFunc) *StreamBuilder {
	if b.err != nil {
		return b
	}
	if fn == nil {
		b.err = errors.New("iostream: writer function cannot be nil")
		return b
	}
	b.writerFn = fn
	return b
}

// WithJSONPayload sets a JSON encoder as the streaming writer.
func (b *StreamBuilder) WithJSONPayload(payload any) *StreamBuilder {
	if b.err != nil {
		return b
	}
	b.headers.Set("Content-Type", "application/json")
	b.writerFn = func(w io.Writer) error {
		return json.NewEncoder(w).Encode(payload)
	}
	return b
}

// ToURL configures the target URL and HTTP method.
func (b *StreamBuilder) ToURL(method, targetURL string) *StreamBuilder {
	if b.err != nil {
		return b
	}
	if method != "" {
		b.method = method
	}
	b.targetURL = targetURL
	return b
}

// WithHeader adds an HTTP header to the streaming request.
func (b *StreamBuilder) WithHeader(key, value string) *StreamBuilder {
	if b.err != nil {
		return b
	}
	b.headers.Add(key, value)
	return b
}

// WithHTTPClient sets a custom HTTP client.
func (b *StreamBuilder) WithHTTPClient(client *http.Client) *StreamBuilder {
	if b.err != nil {
		return b
	}
	if client != nil {
		b.httpClient = client
	}
	return b
}

// Reader returns an io.ReadCloser streaming the configured payload.
func (b *StreamBuilder) Reader() (io.ReadCloser, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.writerFn == nil {
		return nil, errors.New("iostream: no payload writer configured")
	}
	return PipeReader(b.writerFn), nil
}

// ExecuteHTTPRequest executes the HTTP request, streaming bytes directly without RAM buffering.
func (b *StreamBuilder) ExecuteHTTPRequest(ctx context.Context) (*http.Response, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.targetURL == "" {
		return nil, errors.New("iostream: target URL is required")
	}

	reader, err := b.Reader()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, b.method, b.targetURL, reader)
	if err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("iostream: failed to create request: %w", err)
	}

	for k, v := range b.headers {
		req.Header[k] = v
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("iostream: request failed: %w", err)
	}

	return resp, nil
}
