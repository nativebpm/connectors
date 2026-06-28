package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nativebpm/connectors/ironpress"
)

func main() {
	wasmPath := flag.String("wasm", "/tmp/ironpress_wasm/bin/ironpress.wasm", "Path to ironpress.wasm file")
	outFile := flag.String("out", "output_wasm.pdf", "Path to save the generated PDF")
	flag.Parse()

	// 1. Check if WASM file exists
	if _, err := os.Stat(*wasmPath); os.IsNotExist(err) {
		log.Fatalf("WASM module not found at %s. Please run: cargo install --git https://github.com/gastongouron/ironpress.git --target wasm32-wasip1 --root /tmp/ironpress_wasm", *wasmPath)
	}

	// 2. Read WASM bytes into memory
	log.Printf("Reading WASM module from %s...", *wasmPath)
	wasmBytes, err := os.ReadFile(*wasmPath)
	if err != nil {
		log.Fatalf("Failed to read WASM file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 3. Create unified Client with WASM option
	log.Println("Initializing wazero in-memory sandboxed runtime...")
	client := ironpress.NewClient(ironpress.WithWasm(wasmBytes))

	htmlContent := `
<!DOCTYPE html>
<html>
<head>
<style>
  body {
    font-family: sans-serif;
    color: #111;
    padding: 30px;
    background-color: #fafafa;
  }
  .card {
    background: white;
    border-radius: 8px;
    padding: 20px;
    box-shadow: 0 4px 6px rgba(0,0,0,0.1);
  }
  h1 {
    color: #2e7d32;
    border-bottom: 2px solid #a5d6a7;
    padding-bottom: 10px;
  }
  .highlight {
    font-weight: bold;
    color: #1565c0;
  }
</style>
</head>
<body>
  <div class="card">
    <h1>Wazero In-Memory PDF Generation</h1>
    <p>This PDF was generated <span class="highlight">completely in-memory</span> inside a WebAssembly sandbox!</p>
    <p>We compiled the Rust-based <strong>ironpress</strong> converter to <code>wasm32-wasip1</code>, and ran it inside Go using the <strong>wazero</strong> pure Go compiler/runtime.</p>
    <ul>
      <li>No subprocess execution</li>
      <li>No network requests / HTTP overhead</li>
      <li>Zero host-level system dependencies (no Node, Chrome, or Rust needed at runtime)</li>
      <li>100% secure, sandboxed execution</li>
    </ul>
  </div>
</body>
</html>
`

	log.Println("Running in-memory compilation and PDF generation...")
	start := time.Now()

	// 4. Run conversion using Fluent API with Pure_WASM_Mode
	pdfBytes, err := client.Convert(ironpress.Pure_WASM_Mode).
		HTML(htmlContent).
		PageSize("a4").
		Landscape(false).
		Margin(10).
		Do(ctx)

	if err != nil {
		log.Fatalf("WASM conversion failed: %v", err)
	}

	log.Printf("Successfully generated PDF in %v (%d bytes)", time.Since(start), len(pdfBytes))

	// 5. Save file to disk
	if err := os.WriteFile(*outFile, pdfBytes, 0644); err != nil {
		log.Fatalf("Failed to save output PDF: %v", err)
	}

	fmt.Printf("PDF successfully written to %s\n", *outFile)
}
