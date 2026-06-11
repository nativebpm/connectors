package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/andybalholm/brotli"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	compilerDir := wd
	if filepath.Base(wd) != "mrml-compiler" {
		compilerDir = filepath.Join(wd, "mrml-compiler")
	}

	wasmPath := filepath.Join(compilerDir, "target", "wasm32-wasip1", "release", "mrml-compiler.wasm")
	destPath := filepath.Join(compilerDir, "..", "wasm", "mjml.wasm.br")

	fmt.Printf("==> Compressing %s with Brotli...\n", wasmPath)
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		fmt.Printf("failed to read compiled WASM binary: %v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	bw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
	if _, err := bw.Write(wasmBytes); err != nil {
		fmt.Printf("brotli write failed: %v\n", err)
		os.Exit(1)
	}
	if err := bw.Close(); err != nil {
		fmt.Printf("brotli close failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		fmt.Printf("failed to create destination directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
		fmt.Printf("failed to write compressed wasm file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("==> Compression completed successfully!\n==> Output: %s\n", destPath)
}
