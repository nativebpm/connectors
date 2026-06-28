package ironpress

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWasmConversion(t *testing.T) {
	// Find compiled wasm file
	wasmPath := "/tmp/ironpress_wasm/bin/ironpress.wasm"
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		wasmPath = filepath.Join("testdata", "ironpress.wasm")
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			t.Skipf("Skipping WASM test because %s does not exist", wasmPath)
			return
		}
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatalf("failed to read WASM file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize unified client with WASM option
	client := NewClient(WithWasm(wasmBytes))

	pdfBytes, err := client.Convert(Pure_WASM_Mode).
		HTML("<html><body><h1>Hello from Wazero</h1></body></html>").
		PageSize("a4").
		Landscape(false).
		Margin(10).
		Do(ctx)

	if err != nil {
		t.Fatalf("WASM conversion failed: %v", err)
	}

	if len(pdfBytes) < 100 {
		t.Errorf("PDF too small: %d bytes", len(pdfBytes))
	}

	if string(pdfBytes[:4]) != "%PDF" {
		t.Errorf("expected PDF header magic, got: %s", string(pdfBytes[:4]))
	}

	t.Logf("WASM PDF conversion succeeded. Size: %d bytes", len(pdfBytes))
}
