package ironpress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WasmConverter handles in-memory PDF conversion using wazero and a compiled ironpress WASM module.
type WasmConverter struct {
	wasmBytes []byte
	runtime   wazero.Runtime
}

// NewWasmConverter creates a new WasmConverter using raw WASM module bytes.
func NewWasmConverter(ctx context.Context, wasmBytes []byte) (*WasmConverter, error) {
	r := wazero.NewRuntime(ctx)

	// Instantiate WASI
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	return &WasmConverter{
		wasmBytes: wasmBytes,
		runtime:   r,
	}, nil
}

// Close closes the wazero runtime.
func (w *WasmConverter) Close(ctx context.Context) error {
	return w.runtime.Close(ctx)
}

// Convert initiates a new fluent builder for WASM-based conversions.
func (w *WasmConverter) Convert() *WasmRequest {
	return &WasmRequest{
		converter: w,
	}
}

// WasmRequest is a fluent builder for WASM conversions.
type WasmRequest struct {
	converter *WasmConverter
	err       error

	content    []byte
	isMarkdown bool
	pageSize   string
	landscape  *bool
	margin     *float64
	header     string
	footer     string
}

// HTML sets the HTML string content to convert.
func (r *WasmRequest) HTML(content string) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.content = []byte(content)
	r.isMarkdown = false
	return r
}

// Markdown sets the Markdown string content to convert.
func (r *WasmRequest) Markdown(content string) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.content = []byte(content)
	r.isMarkdown = true
	return r
}

// PageSize sets the page size (e.g. "a4", "letter").
func (r *WasmRequest) PageSize(size string) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.pageSize = size
	return r
}

// Landscape sets the orientation to landscape if true.
func (r *WasmRequest) Landscape(landscape bool) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.landscape = &landscape
	return r
}

// Margin sets the document margins.
func (r *WasmRequest) Margin(margin float64) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.margin = &margin
	return r
}

// Header sets the running header text.
func (r *WasmRequest) Header(text string) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.header = text
	return r
}

// Footer sets the running footer text.
func (r *WasmRequest) Footer(text string) *WasmRequest {
	if r.err != nil {
		return r
	}
	r.footer = text
	return r
}

// Do compiles and runs the WASM module inside the wazero sandboxed runtime.
func (r *WasmRequest) Do(ctx context.Context) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if len(r.content) == 0 {
		return nil, fmt.Errorf("no input content provided")
	}

	// 1. Create a temp directory on the host to exchange files with WASM
	tempDir, err := os.MkdirTemp("", "ironpress-wasm-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp exchange dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Keep original extension (HTML or Markdown) so ironpress detects type
	ext := ".html"
	if r.isMarkdown {
		ext = ".md"
	}
	inputFilename := "input" + ext
	outputPath := filepath.Join(tempDir, "output.pdf")

	err = os.WriteFile(filepath.Join(tempDir, inputFilename), r.content, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}

	// Build arguments (ironpress expects input path and output path at the end)
	args := []string{"ironpress"}

	if r.pageSize != "" {
		args = append(args, "--page-size", r.pageSize)
	}
	if r.landscape != nil && *r.landscape {
		args = append(args, "--landscape")
	}
	if r.margin != nil {
		args = append(args, "--margin", fmt.Sprintf("%f", *r.margin))
	}
	if r.header != "" {
		args = append(args, "--header", r.header)
	}
	if r.footer != "" {
		args = append(args, "--footer", r.footer)
	}

	// Virtual paths inside the sandboxed WASM filesystem
	args = append(args, "/work/"+inputFilename, "/work/output.pdf")

	// Prepare wazero module configuration
	config := wazero.NewModuleConfig().
		WithArgs(args...).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithFSConfig(wazero.NewFSConfig().WithDirMount(tempDir, "/work"))

	// Compile & run the module.
	compiled, err := r.converter.runtime.CompileModule(ctx, r.converter.wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	mod, err := r.converter.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate and run WASM module: %w", err)
	}
	_ = mod.Close(ctx) // close module instance after execution to free resources

	// Read generated PDF from the exchange directory
	pdfBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output PDF: %w", err)
	}

	return pdfBytes, nil
}
