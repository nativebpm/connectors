package ironpress

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func BenchmarkConversions(b *testing.B) {
	ctx := context.Background()
	htmlContent := "<html><body><h1>Benchmark Document</h1><p>Testing performance difference</p></body></html>"

	// 1. Prepare HTTP Server & Client
	bin := findIronpressBin()
	server := NewServer("127.0.0.1:0", bin, 4)
	go func() {
		_ = server.Start(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	client := NewClient(WithHTTP(nil, fmt.Sprintf("http://%s", server.Addr())))

	// 2. Prepare WASM Converter
	wasmPath := "/tmp/ironpress_wasm/bin/ironpress.wasm"
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		b.Skip("WASM file not found at /tmp/ironpress_wasm/bin/ironpress.wasm")
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		b.Fatalf("failed to read WASM file: %v", err)
	}

	wasmClient := NewClient(WithWasm(wasmBytes))

	// Run HTTP Benchmark
	b.Run("HTTP_CLI_Mode", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pdf, err := client.Convert(HTTP_CLI_Mode).
				HTML(htmlContent).
				PageSize("a4").
				Do(ctx)
			if err != nil {
				b.Fatalf("HTTP conversion failed: %v", err)
			}
			if len(pdf) == 0 {
				b.Fatal("empty pdf output")
			}
		}
	})

	// Run WASM Benchmark
	b.Run("Pure_WASM_Mode", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pdf, err := wasmClient.Convert(Pure_WASM_Mode).
				HTML(htmlContent).
				PageSize("a4").
				Do(ctx)
			if err != nil {
				b.Fatalf("WASM conversion failed: %v", err)
			}
			if len(pdf) == 0 {
				b.Fatal("empty pdf output")
			}
		}
	})
}
