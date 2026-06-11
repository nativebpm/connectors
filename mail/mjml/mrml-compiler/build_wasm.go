package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/andybalholm/brotli"
)

func main() {
	// 1. Get working directories
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	// Determine paths
	compilerDir := wd
	if filepath.Base(wd) != "mrml-compiler" {
		compilerDir = filepath.Join(wd, "connectors", "mail", "mjml", "mrml-compiler")
	}

	// Go up to reach the monorepo root for Docker volume mounting
	repoRoot, err := filepath.Abs(filepath.Join(compilerDir, "..", "..", "..", ".."))
	if err != nil {
		fmt.Printf("failed to determine repository root: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("==> Monorepo root: %s\n", repoRoot)
	fmt.Printf("==> Compiler directory: %s\n", compilerDir)

	// Docker command to build the Rust binary target wasm32-wasi
	// We mount the entire monorepo root to /workspace and run inside compiler directory
	dockerCmd := exec.Command(
		"docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", repoRoot),
		"-w", "/workspace/connectors/mail/mjml/mrml-compiler",
		"rust:latest",
		"sh", "-c", "rustup target add wasm32-wasip1 && cargo build --target wasm32-wasip1 --release",
	)

	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr

	fmt.Println("==> Running Rust WASM compilation via Docker...")
	if err := dockerCmd.Run(); err != nil {
		fmt.Printf("docker cargo build failed: %v\n", err)
		os.Exit(1)
	}

	// 2. Compress resulting WASM binary with Brotli
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

	fmt.Printf("==> Compilation and compression completed successfully!\n==> Output: %s\n", destPath)
}
