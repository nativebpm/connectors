package ironpress

import (
	"context"
	"fmt"
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

func TestWasmWarmup(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Pre-warm client once
	client := NewClient(WithWasm(wasmBytes))
	if err := client.Warmup(ctx); err != nil {
		t.Fatalf("client warmup failed: %v", err)
	}
	defer client.Close(ctx)

	// Execute pre-warmed conversion
	start := time.Now()
	pdfBytes, err := client.Convert(Pure_WASM_Mode).
		HTML("<html><body><h1>Hello from Prewarmed Wazero</h1></body></html>").
		PageSize("a4").
		Do(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WASM conversion failed: %v", err)
	}
	if len(pdfBytes) < 100 || string(pdfBytes[:4]) != "%PDF" {
		t.Fatalf("invalid PDF generated: len=%d", len(pdfBytes))
	}

	t.Logf("Pre-warmed WASM PDF conversion succeeded in %v. Size: %d bytes", elapsed, len(pdfBytes))
}

func TestWasmConcurrent(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := NewClient(WithWasm(wasmBytes))
	if err := client.Warmup(ctx); err != nil {
		t.Fatalf("client warmup failed: %v", err)
	}
	defer client.Close(ctx)

	const concurrency = 10
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			pdf, err := client.Convert(Pure_WASM_Mode).
				HTML(fmt.Sprintf("<html><body><h1>Invoice #%d</h1><p>Concurrent execution test</p></body></html>", id)).
				PageSize("a4").
				Do(ctx)
			if err != nil {
				errCh <- fmt.Errorf("worker %d failed: %w", id, err)
				return
			}
			if len(pdf) < 100 || string(pdf[:4]) != "%PDF" {
				errCh <- fmt.Errorf("worker %d got invalid pdf: %d bytes", id, len(pdf))
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("Concurrent WASM error: %v", err)
		}
	}
}

