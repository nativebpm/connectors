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

	"github.com/nativebpm/httpstream"
)

// Client represents an ironpress client.
type Client struct {
	httpStream *httpstream.Client
}

// NewClient creates a new ironpress client with the given HTTP client and base URL.
func NewClient(httpClient *http.Client, baseURL string) (*Client, error) {
	client, err := httpstream.NewClient(httpClient, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create httpstream client: %w", err)
	}

	return &Client{
		httpStream: client,
	}, nil
}

// Use applies HTTP round-tripper middlewares to the client.
func (c *Client) Use(middleware func(http.RoundTripper) http.RoundTripper) *Client {
	c.httpStream = c.httpStream.Use(middleware)
	return c
}

// Convert initiates a new PDF conversion request builder.
func (c *Client) Convert() *Request {
	return &Request{
		client: c,
	}
}

// Request is a fluent builder for ironpress conversions with a sticky error pattern.
type Request struct {
	client *Client
	err    error

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

// Do executes the conversion request and returns the PDF document bytes.
func (r *Request) Do(ctx context.Context) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.fileContent == nil {
		return nil, fmt.Errorf("no HTML or Markdown content provided")
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
