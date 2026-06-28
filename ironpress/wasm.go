package ironpress

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func (r *Request) doWasm(ctx context.Context) ([]byte, error) {
	if len(r.client.wasmBytes) == 0 {
		return nil, fmt.Errorf("wasm module bytes are not configured (use WithWasm option)")
	}

	// 1. Create wazero runtime
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// Instantiate WASI
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	// 2. Create a temp directory on the host to exchange files with WASM
	tempDir, err := os.MkdirTemp("", "ironpress-wasm-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp exchange dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Ensure file name has correct extension (.html or .md)
	inputFilename := r.fileName
	if inputFilename == "" {
		inputFilename = "index.html"
	}
	outputPath := filepath.Join(tempDir, "output.pdf")

	inputFile, err := os.Create(filepath.Join(tempDir, inputFilename))
	if err != nil {
		return nil, fmt.Errorf("failed to create input file: %w", err)
	}
	if _, err := io.Copy(inputFile, r.fileContent); err != nil {
		inputFile.Close()
		return nil, fmt.Errorf("failed to write input file content: %w", err)
	}
	inputFile.Close()

	// Build arguments
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

	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}

	// Compile & run the module
	compiled, err := rt.CompileModule(ctx, r.client.wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate and run WASM module: %w", err)
	}
	_ = mod.Close(ctx)

	// Read generated PDF from the exchange directory
	pdfBytes, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output PDF: %w", err)
	}

	return pdfBytes, nil
}
