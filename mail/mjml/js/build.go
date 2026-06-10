package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/andybalholm/brotli"
)

const javyVersion = "v8.1.1"

func main() {
	ctx := context.Background()

	// Determine working directories
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	// Ensure we are running from connectors/mail/mjml/js/ or root
	jsDir := wd
	if filepath.Base(wd) != "js" {
		jsDir = filepath.Join(wd, "connectors", "mail", "mjml", "js")
	}

	fmt.Printf("==> Running Go-based MJML WASM builder in: %s\n", jsDir)

	javyPath := filepath.Join(jsDir, "javy")
	if runtime.GOOS == "windows" {
		javyPath += ".exe"
	}

	// Always do a clean build: clean up previous Javy binary if exists
	_ = os.Remove(javyPath)

	// Check if local dependencies are present
	if !cmdExists("node") || !cmdExists("npm") {
		fmt.Println("Error: node and npm must be installed for compilation.")
		os.Exit(1)
	}

	// Download Javy CLI for host OS/arch
	fmt.Printf("==> Downloading official Bytecode Alliance Javy %s for host %s/%s...\n", javyVersion, runtime.GOOS, runtime.GOARCH)
	if err := downloadJavyForHost(javyPath); err != nil {
		fmt.Printf("failed to download Javy: %v\n", err)
		os.Exit(1)
	}

	// npm install
	fmt.Println("==> Installing npm packages...")
	cmdInstall := exec.CommandContext(ctx, "npm", "install")
	cmdInstall.Dir = jsDir
	cmdInstall.Stdout = os.Stdout
	cmdInstall.Stderr = os.Stderr
	if err := cmdInstall.Run(); err != nil {
		fmt.Printf("npm install failed: %v\n", err)
		_ = os.Remove(javyPath)
		os.Exit(1)
	}

	// webpack
	fmt.Println("==> Bundling JS wrapper via Webpack...")
	cmdWebpack := exec.CommandContext(ctx, "npx", "webpack")
	cmdWebpack.Dir = jsDir
	cmdWebpack.Stdout = os.Stdout
	cmdWebpack.Stderr = os.Stderr
	if err := cmdWebpack.Run(); err != nil {
		fmt.Printf("webpack bundling failed: %v\n", err)
		_ = os.Remove(javyPath)
		os.Exit(1)
	}

	// Javy compile
	fmt.Println("==> Compiling JS bundle to WebAssembly using Javy build...")
	cmdJavy := exec.CommandContext(ctx, javyPath, "build", filepath.Join(jsDir, "dist", "mjml.js"), "-o", filepath.Join(jsDir, "dist", "mjml.wasm"), "-J", "javy-stream-io=y", "-J", "text-encoding=y")
	cmdJavy.Stdout = os.Stdout
	cmdJavy.Stderr = os.Stderr
	if err := cmdJavy.Run(); err != nil {
		fmt.Printf("javy compilation failed: %v\n", err)
		_ = os.Remove(javyPath)
		os.Exit(1)
	}

	// Compress with Brotli
	fmt.Println("==> Compressing WebAssembly module with Brotli...")
	wasmPath := filepath.Join(jsDir, "dist", "mjml.wasm")
	wasmBrPath := filepath.Join(jsDir, "..", "wasm", "mjml.wasm.br")

	if err := compressBrotli(wasmPath, wasmBrPath); err != nil {
		fmt.Printf("brotli compression failed: %v\n", err)
		_ = os.Remove(javyPath)
		os.Exit(1)
	}

	// Clean up Javy binary
	_ = os.Remove(javyPath)
	fmt.Println("==> Compilation completed successfully! Output: ../wasm/mjml.wasm.br")
}

func cmdExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func downloadJavyForHost(dest string) error {
	var osName string
	var archName string

	switch runtime.GOOS {
	case "darwin":
		osName = "macos"
	case "linux":
		osName = "linux"
	case "windows":
		osName = "windows"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm"
	default:
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	url := fmt.Sprintf("https://github.com/bytecodealliance/javy/releases/download/%s/javy-%s-%s-%s.gz", javyVersion, archName, osName, javyVersion)
	fmt.Printf("Downloading Javy from: %s\n", url)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d %s", resp.StatusCode, resp.Status)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gr.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, gr)
	return err
}

func compressBrotli(src, dest string) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	bw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
	if _, err := bw.Write(srcData); err != nil {
		return err
	}
	if err := bw.Close(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	return os.WriteFile(dest, buf.Bytes(), 0644)
}
