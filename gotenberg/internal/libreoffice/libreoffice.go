// Package libreoffice provides a client for the Gotenberg LibreOffice service.
// It offers a convenient API for converting Office documents to PDF documents.
package libreoffice

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/nativebpm/connectors/gotenberg/v8/internal/gotenberg"
	"github.com/nativebpm/connectors/httpstream"
)

// LibreOffice represents a Gotenberg conversion request builder.
type LibreOffice struct {
	*gotenberg.Gotenberg
}

func NewLibreOffice(client *httpstream.Client) *LibreOffice {
	return &LibreOffice{
		Gotenberg: gotenberg.NewGotenberg(client),
	}
}

// Convert creates a request to convert Office documents to PDF.
// The files parameter should contain the Office documents to be converted.
func (r *LibreOffice) Convert(ctx context.Context) *LibreOffice {
	r.Req = r.HttpStream.Multipart(ctx, "/forms/libreoffice/convert")
	return r
}

// Send executes the conversion request and returns the response.
// Returns an error if the request fails or the conversion cannot be completed.
func (r *LibreOffice) Send() (*gotenberg.Response, error) {
	return r.Gotenberg.Send()
}

// Header adds a header to the conversion request.
func (r *LibreOffice) Header(key, value string) *LibreOffice {
	r.Gotenberg.Header(key, value)
	return r
}

// Param adds a form parameter to the conversion request.
func (r *LibreOffice) Param(key, value string) *LibreOffice {
	r.Gotenberg.Param(key, value)
	return r
}

// Bool adds a boolean form parameter to the conversion request.
func (r *LibreOffice) Bool(fieldName string, value bool) *LibreOffice {
	r.Gotenberg.Bool(fieldName, value)
	return r
}

// Float adds a float64 form parameter to the conversion request.
func (r *LibreOffice) Float(fieldName string, value float64) *LibreOffice {
	r.Gotenberg.Float(fieldName, value)
	return r
}

// File adds a file to the conversion request.
func (r *LibreOffice) File(filename string, content io.Reader) *LibreOffice {
	r.Gotenberg.File("files", filename, content)
	return r
}

// WebhookURL sets the webhook URL and HTTP method for successful conversions.
func (r *LibreOffice) WebhookURL(url, method string) *LibreOffice {
	r.Gotenberg.WebhookURL(url, method)
	return r
}

// WebhookErrorURL sets the webhook URL and HTTP method for failed conversions.
func (r *LibreOffice) WebhookErrorURL(url, method string) *LibreOffice {
	r.Gotenberg.WebhookErrorURL(url, method)
	return r
}

// WebhookEventsURL sets the webhook events URL for structured JSON event callbacks.
// Added in Gotenberg v8.29.0. Structured JSON events (webhook.success, webhook.error)
// are POSTed after each webhook operation. Gotenberg-Webhook-Error-Url becomes optional.
func (r *LibreOffice) WebhookEventsURL(url string) *LibreOffice {
	r.Gotenberg.WebhookEventsURL(url)
	return r
}

// WebhookHeader adds a custom header to be sent with webhook requests.
// Multiple headers can be added by calling this method multiple times.
func (r *LibreOffice) WebhookHeader(key, value string) *LibreOffice {
	r.Gotenberg.WebhookHeader(key, value)
	return r
}

// DownloadFrom sets the downloadFrom parameter for downloading files from URLs.
// The data should be a slice of DownloadItem representing the download configuration.
func (r *LibreOffice) DownloadFrom(url string, headers map[string]string) *LibreOffice {
	r.Gotenberg.DownloadFrom(url, headers)
	return r
}

// OutputFilename sets the output filename for the generated PDF.
func (r *LibreOffice) OutputFilename(filename string) *LibreOffice {
	r.Gotenberg.OutputFilename(filename)
	return r
}

// Trace sets the request trace identifier for debugging and monitoring.
// If not set, Gotenberg will assign a unique UUID trace.
func (r *LibreOffice) Trace(trace string) *LibreOffice {
	r.Gotenberg.Trace(trace)
	return r
}

// Timeout sets a timeout for the request.
func (r *LibreOffice) Timeout(duration time.Duration) *LibreOffice {
	r.Gotenberg.Timeout(duration)
	return r
}

// Password sets the password for opening the source file.
func (r *LibreOffice) Password(password string) *LibreOffice {
	r.Gotenberg.Param("password", password)
	return r
}

// Landscape sets the paper orientation to landscape.
func (r *LibreOffice) Landscape(landscape bool) *LibreOffice {
	r.Gotenberg.Bool("landscape", landscape)
	return r
}

// NativePageRanges sets the page ranges to print.
func (r *LibreOffice) NativePageRanges(ranges string) *LibreOffice {
	r.Gotenberg.Param("nativePageRanges", ranges)
	return r
}

// UpdateIndexes specifies whether to update the indexes before conversion.
func (r *LibreOffice) UpdateIndexes(update bool) *LibreOffice {
	r.Gotenberg.Bool("updateIndexes", update)
	return r
}

// ExportFormFields specifies whether form fields are exported as widgets.
func (r *LibreOffice) ExportFormFields(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportFormFields", export)
	return r
}

// AllowDuplicateFieldNames specifies whether multiple form fields can have the same name.
func (r *LibreOffice) AllowDuplicateFieldNames(allow bool) *LibreOffice {
	r.Gotenberg.Bool("allowDuplicateFieldNames", allow)
	return r
}

// ExportBookmarks specifies if bookmarks are exported to PDF.
func (r *LibreOffice) ExportBookmarks(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportBookmarks", export)
	return r
}

// ExportBookmarksToPdfDestination specifies bookmarks export to PDF destination.
func (r *LibreOffice) ExportBookmarksToPdfDestination(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportBookmarksToPdfDestination", export)
	return r
}

// ExportPlaceholders exports placeholders fields visual markings only.
func (r *LibreOffice) ExportPlaceholders(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportPlaceholders", export)
	return r
}

// ExportNotes specifies if notes are exported to PDF.
func (r *LibreOffice) ExportNotes(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportNotes", export)
	return r
}

// ExportNotesPages specifies if notes pages are exported to PDF.
func (r *LibreOffice) ExportNotesPages(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportNotesPages", export)
	return r
}

// ExportOnlyNotesPages specifies if only notes pages are exported.
func (r *LibreOffice) ExportOnlyNotesPages(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportOnlyNotesPages", export)
	return r
}

// ExportNotesInMargin specifies if notes in margin are exported.
func (r *LibreOffice) ExportNotesInMargin(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportNotesInMargin", export)
	return r
}

// ConvertOooTargetToPdfTarget converts OOo target to PDF target.
func (r *LibreOffice) ConvertOooTargetToPdfTarget(convert bool) *LibreOffice {
	r.Gotenberg.Bool("convertOooTargetToPdfTarget", convert)
	return r
}

// ExportLinksRelativeFsys exports relative filesystem links.
func (r *LibreOffice) ExportLinksRelativeFsys(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportLinksRelativeFsys", export)
	return r
}

// ExportHiddenSlides exports hidden slides for Impress.
func (r *LibreOffice) ExportHiddenSlides(export bool) *LibreOffice {
	r.Gotenberg.Bool("exportHiddenSlides", export)
	return r
}

// SkipEmptyPages suppresses automatically inserted empty pages.
func (r *LibreOffice) SkipEmptyPages(skip bool) *LibreOffice {
	r.Gotenberg.Bool("skipEmptyPages", skip)
	return r
}

// AddOriginalDocumentAsStream adds original document as stream.
func (r *LibreOffice) AddOriginalDocumentAsStream(add bool) *LibreOffice {
	r.Gotenberg.Bool("addOriginalDocumentAsStream", add)
	return r
}

// SinglePageSheets puts every sheet on exactly one page.
func (r *LibreOffice) SinglePageSheets(single bool) *LibreOffice {
	r.Gotenberg.Bool("singlePageSheets", single)
	return r
}

// LosslessImageCompression specifies lossless compression for images.
func (r *LibreOffice) LosslessImageCompression(lossless bool) *LibreOffice {
	r.Gotenberg.Bool("losslessImageCompression", lossless)
	return r
}

// Quality sets the JPG export quality.
func (r *LibreOffice) Quality(quality int) *LibreOffice {
	r.Gotenberg.Param("quality", strconv.Itoa(quality))
	return r
}

// ReduceImageResolution reduces image resolution.
func (r *LibreOffice) ReduceImageResolution(reduce bool) *LibreOffice {
	r.Gotenberg.Bool("reduceImageResolution", reduce)
	return r
}

// MaxImageResolution sets the max image resolution in DPI.
func (r *LibreOffice) MaxImageResolution(resolution int) *LibreOffice {
	r.Gotenberg.Param("maxImageResolution", strconv.Itoa(resolution))
	return r
}

// Merge merges the resulting PDFs alphanumerically.
func (r *LibreOffice) Merge(merge bool) *LibreOffice {
	r.Gotenberg.Bool("merge", merge)
	return r
}

// SplitMode sets the split mode.
func (r *LibreOffice) SplitMode(mode string) *LibreOffice {
	r.Gotenberg.Param("splitMode", mode)
	return r
}

// SplitSpan sets the split span.
func (r *LibreOffice) SplitSpan(span string) *LibreOffice {
	r.Gotenberg.Param("splitSpan", span)
	return r
}

// SplitUnify specifies whether to unify split pages.
func (r *LibreOffice) SplitUnify(unify bool) *LibreOffice {
	r.Gotenberg.Bool("splitUnify", unify)
	return r
}

// PDFA converts to PDF/A format.
func (r *LibreOffice) PDFA(pdfa string) *LibreOffice {
	r.Gotenberg.Param("pdfa", pdfa)
	return r
}

// PDFUA enables PDF for Universal Access.
func (r *LibreOffice) PDFUA(pdfua bool) *LibreOffice {
	r.Gotenberg.Bool("pdfua", pdfua)
	return r
}

// Metadata sets the metadata for the PDF.
func (r *LibreOffice) Metadata(key, value string) *LibreOffice {
	r.Gotenberg.Metadata(key, value)
	return r
}

// Flatten flattens the resulting PDF.
func (r *LibreOffice) Flatten(flatten bool) *LibreOffice {
	r.Gotenberg.Bool("flatten", flatten)
	return r
}

// NativeWatermarkText sets the text for LibreOffice's built-in watermark during PDF export.
// Added in Gotenberg v8.28.0.
func (r *LibreOffice) NativeWatermarkText(text string) *LibreOffice {
	return r.Param("nativeWatermarkText", text)
}

// NativeWatermarkColor sets the color of the native watermark (e.g. "#FF0000").
// Added in Gotenberg v8.28.0.
func (r *LibreOffice) NativeWatermarkColor(color string) *LibreOffice {
	return r.Param("nativeWatermarkColor", color)
}

// NativeWatermarkFontHeight sets the font height of the native watermark in points.
// Added in Gotenberg v8.28.0.
func (r *LibreOffice) NativeWatermarkFontHeight(height int) *LibreOffice {
	return r.Param("nativeWatermarkFontHeight", strconv.Itoa(height))
}

// NativeWatermarkRotateAngle sets the rotation angle of the native watermark in degrees.
// Added in Gotenberg v8.28.0.
func (r *LibreOffice) NativeWatermarkRotateAngle(angle int) *LibreOffice {
	return r.Param("nativeWatermarkRotateAngle", strconv.Itoa(angle))
}

// NativeWatermarkFontName sets the font name for the native watermark.
// Added in Gotenberg v8.28.0.
func (r *LibreOffice) NativeWatermarkFontName(name string) *LibreOffice {
	return r.Param("nativeWatermarkFontName", name)
}

// NativeTiledWatermarkText sets a tiled watermark text using LibreOffice's built-in rendering.
// Added in Gotenberg v8.28.0.
func (r *LibreOffice) NativeTiledWatermarkText(text string) *LibreOffice {
	return r.Param("nativeTiledWatermarkText", text)
}

// InitialView sets the initial view when the PDF is opened (e.g. "UseOutlines", "UseThumbs").
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) InitialView(view string) *LibreOffice {
	return r.Param("initialView", view)
}

// InitialPage sets the initial page number displayed when the PDF is opened.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) InitialPage(page int) *LibreOffice {
	return r.Param("initialPage", strconv.Itoa(page))
}

// Magnification sets the magnification mode when the PDF is opened
// (e.g. "Default", "Fit", "FitWidth", "FitHeight").
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) Magnification(mag string) *LibreOffice {
	return r.Param("magnification", mag)
}

// Zoom sets the zoom percentage when the PDF is opened (used when Magnification is not set).
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) Zoom(zoom int) *LibreOffice {
	return r.Param("zoom", strconv.Itoa(zoom))
}

// PageLayout sets the page layout when the PDF is opened
// (e.g. "SinglePage", "Continuous", "ContinuousFacing").
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) PageLayout(layout string) *LibreOffice {
	return r.Param("pageLayout", layout)
}

// FirstPageOnLeft sets whether the first page is shown on the left in facing-page view.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) FirstPageOnLeft(v bool) *LibreOffice {
	return r.Bool("firstPageOnLeft", v)
}

// ResizeWindowToInitialPage sets whether to resize the viewer window to the initial page.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) ResizeWindowToInitialPage(v bool) *LibreOffice {
	return r.Bool("resizeWindowToInitialPage", v)
}

// CenterWindow sets whether to center the viewer window on screen when opened.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) CenterWindow(v bool) *LibreOffice {
	return r.Bool("centerWindow", v)
}

// OpenInFullScreenMode sets whether to open the PDF in full-screen mode.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) OpenInFullScreenMode(v bool) *LibreOffice {
	return r.Bool("openInFullScreenMode", v)
}

// DisplayPDFDocumentTitle sets whether to display the document title in the viewer title bar.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) DisplayPDFDocumentTitle(v bool) *LibreOffice {
	return r.Bool("displayPDFDocumentTitle", v)
}

// HideViewerMenubar sets whether to hide the viewer's menu bar.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) HideViewerMenubar(v bool) *LibreOffice {
	return r.Bool("hideViewerMenubar", v)
}

// HideViewerToolbar sets whether to hide the viewer's toolbar.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) HideViewerToolbar(v bool) *LibreOffice {
	return r.Bool("hideViewerToolbar", v)
}

// HideViewerWindowControls sets whether to hide the viewer's window controls.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) HideViewerWindowControls(v bool) *LibreOffice {
	return r.Bool("hideViewerWindowControls", v)
}

// UseTransitionEffects sets whether to use slide transition effects in Impress presentations.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) UseTransitionEffects(v bool) *LibreOffice {
	return r.Bool("useTransitionEffects", v)
}

// OpenBookmarkLevels sets the number of bookmark levels to expand when the PDF is opened.
// Added in Gotenberg v8.29.0.
func (r *LibreOffice) OpenBookmarkLevels(levels int) *LibreOffice {
	return r.Param("openBookmarkLevels", strconv.Itoa(levels))
}

// EmbedsMetadata sets per-file metadata for embedded files as a JSON object
// keyed by filename with fields like mimeType, relationship, etc.
// Added in Gotenberg v8.31.0.
func (r *LibreOffice) EmbedsMetadata(metadataJSON string) *LibreOffice {
	return r.Param("embedsMetadata", metadataJSON)
}
