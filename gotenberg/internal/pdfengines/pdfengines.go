// Package pdfengines provides a client for the Gotenberg PDF Engines service.
// It offers a convenient API for converting PDFs to PDF/A & PDF/UA, reading/writing metadata, merging, splitting, and flattening PDFs.
package pdfengines

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/nativebpm/connectors/gotenberg/v8/internal/gotenberg"
	"github.com/nativebpm/connectors/httpstream"
)

// PDFEngines represents a Gotenberg conversion request builder.
// It wraps the underlying multipart request and provides PDF Engines-specific methods.
type PDFEngines struct {
	*gotenberg.Gotenberg
}

func NewPDFEngines(client *httpstream.Client) *PDFEngines {
	return &PDFEngines{
		Gotenberg: gotenberg.NewGotenberg(client),
	}
}

// Convert creates a request to convert PDFs to PDF/A & PDF/UA.
func (r *PDFEngines) Convert(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/convert")
	return r
}

// MetadataRead creates a request to read metadata from PDFs.
func (r *PDFEngines) MetadataRead(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/metadata/read")
	return r
}

// MetadataWrite creates a request to write metadata to PDFs.
func (r *PDFEngines) MetadataWrite(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/metadata/write")
	return r
}

// Merge creates a request to merge PDFs.
func (r *PDFEngines) Merge(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/merge")
	return r
}

// Split creates a request to split PDFs.
func (r *PDFEngines) Split(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/split")
	return r
}

// Flatten creates a request to flatten PDFs.
func (r *PDFEngines) Flatten(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/flatten")
	return r
}

// Watermark creates a request to apply a watermark behind page content.
// Supports text, image, or PDF sources via Param/File methods.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) Watermark(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/watermark")
	return r
}

// Stamp creates a request to apply a stamp on top of page content.
// Supports text, image, or PDF sources via Param/File methods.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) Stamp(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/stamp")
	return r
}

// Rotate creates a request to rotate PDF pages by 90°, 180°, or 270°.
// Use RotateAngle and RotatePages to configure the rotation.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) Rotate(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/rotate")
	return r
}

// BookmarksRead creates a request to read the bookmark outline from PDF files as JSON.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) BookmarksRead(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/bookmarks/read")
	return r
}

// BookmarksWrite creates a request to write bookmarks to PDF files.
// Accepts a flat list (applied to all) or a filename-keyed map via Bookmarks().
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) BookmarksWrite(ctx context.Context) *PDFEngines {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/pdfengines/bookmarks/write")
	return r
}

// Send executes the request and returns the response.
// Returns an error if the request fails.
func (r *PDFEngines) Send() (*gotenberg.Response, error) {
	return r.Gotenberg.Send()
}

// Header adds a header to the request.
func (r *PDFEngines) Header(key, value string) *PDFEngines {
	r.Gotenberg.Header(key, value)
	return r
}

// Param adds a form parameter to the request.
func (r *PDFEngines) Param(key, value string) *PDFEngines {
	r.Gotenberg.Param(key, value)
	return r
}

// Bool adds a boolean form parameter to the request.
func (r *PDFEngines) Bool(fieldName string, value bool) *PDFEngines {
	r.Gotenberg.Bool(fieldName, value)
	return r
}

// File adds a file to the request.
func (r *PDFEngines) File(filename string, content io.Reader) *PDFEngines {
	r.Gotenberg.File("files", filename, content)
	return r
}

// WebhookURL sets the webhook URL and HTTP method for successful operations.
func (r *PDFEngines) WebhookURL(url, method string) *PDFEngines {
	r.Gotenberg.WebhookURL(url, method)
	return r
}

// WebhookErrorURL sets the webhook URL and HTTP method for failed operations.
func (r *PDFEngines) WebhookErrorURL(url, method string) *PDFEngines {
	r.Gotenberg.WebhookErrorURL(url, method)
	return r
}

// WebhookEventsURL sets the webhook events URL for structured JSON event callbacks.
// Added in Gotenberg v8.29.0. Structured JSON events (webhook.success, webhook.error)
// are POSTed after each webhook operation. Gotenberg-Webhook-Error-Url becomes optional.
func (r *PDFEngines) WebhookEventsURL(url string) *PDFEngines {
	r.Gotenberg.WebhookEventsURL(url)
	return r
}

// WebhookHeader adds a custom header to be sent with webhook requests.
// Multiple headers can be added by calling this method multiple times.
func (r *PDFEngines) WebhookHeader(key, value string) *PDFEngines {
	r.Gotenberg.WebhookHeader(key, value)
	return r
}

// DownloadFrom sets the downloadFrom parameter for downloading files from URLs.
// The data should be a slice of DownloadItem representing the download configuration.
func (r *PDFEngines) DownloadFrom(url string, headers map[string]string) *PDFEngines {
	r.Gotenberg.DownloadFrom(url, headers)
	return r
}

// OutputFilename sets the output filename.
func (r *PDFEngines) OutputFilename(filename string) *PDFEngines {
	r.Gotenberg.OutputFilename(filename)
	return r
}

// Trace sets the request trace identifier for debugging and monitoring.
// If not set, Gotenberg will assign a unique UUID trace.
func (r *PDFEngines) Trace(trace string) *PDFEngines {
	r.Gotenberg.Trace(trace)
	return r
}

// Timeout sets a timeout for the request.
func (r *PDFEngines) Timeout(duration time.Duration) *PDFEngines {
	r.Gotenberg.Timeout(duration)
	return r
}

// PDFA converts to PDF/A format.
func (r *PDFEngines) PDFA(pdfa string) *PDFEngines {
	r.Gotenberg.Param("pdfa", pdfa)
	return r
}

// PDFUA enables PDF for Universal Access.
func (r *PDFEngines) PDFUA(pdfua bool) *PDFEngines {
	r.Gotenberg.Bool("pdfua", pdfua)
	return r
}

// Metadata sets the metadata for the PDF.
func (r *PDFEngines) Metadata(key, value string) *PDFEngines {
	r.Gotenberg.Metadata(key, value)
	return r
}

// SplitMode sets the split mode.
func (r *PDFEngines) SplitMode(mode string) *PDFEngines {
	r.Gotenberg.Param("splitMode", mode)
	return r
}

// SplitSpan sets the split span.
func (r *PDFEngines) SplitSpan(span string) *PDFEngines {
	r.Gotenberg.Param("splitSpan", span)
	return r
}

// SplitUnify specifies whether to unify split pages.
func (r *PDFEngines) SplitUnify(unify bool) *PDFEngines {
	r.Gotenberg.Bool("splitUnify", unify)
	return r
}

// FlattenPDF sets the flatten flag.
func (r *PDFEngines) FlattenPDF(flatten bool) *PDFEngines {
	r.Gotenberg.Bool("flatten", flatten)
	return r
}

// RotateAngle sets the rotation angle for pages: 90, 180, or 270 degrees.
// Used with Rotate route and as optional field on composite routes.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) RotateAngle(angle int) *PDFEngines {
	return r.Param("rotateAngle", strconv.Itoa(angle))
}

// RotatePages sets the page selection for rotation (e.g. "1-3", "2,4", "all").
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) RotatePages(pages string) *PDFEngines {
	return r.Param("rotatePages", pages)
}

// Bookmarks sets the bookmarks JSON for BookmarksWrite or Merge routes.
// Accepts a flat list or a filename-keyed map as a JSON string.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) Bookmarks(json string) *PDFEngines {
	return r.Param("bookmarks", json)
}

// AutoIndexBookmarks extracts and reindexes existing bookmarks from input files during merge.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) AutoIndexBookmarks(v bool) *PDFEngines {
	return r.Bool("autoIndexBookmarks", v)
}

// WatermarkFile adds a watermark source file (image or PDF) to the request.
// The fieldName should be "watermark" for the watermark route or composite routes.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) WatermarkFile(filename string, content io.Reader) *PDFEngines {
	r.Gotenberg.File("watermark", filename, content)
	return r
}

// StampFile adds a stamp source file (image or PDF) to the request.
// The fieldName should be "stamp" for the stamp route or composite routes.
// Added in Gotenberg v8.28.0.
func (r *PDFEngines) StampFile(filename string, content io.Reader) *PDFEngines {
	r.Gotenberg.File("stamp", filename, content)
	return r
}

// EmbedsMetadata sets per-file metadata for embedded files as a JSON object
// keyed by filename with fields like mimeType, relationship, etc.
// Added in Gotenberg v8.31.0.
func (r *PDFEngines) EmbedsMetadata(metadataJSON string) *PDFEngines {
	return r.Param("embedsMetadata", metadataJSON)
}
