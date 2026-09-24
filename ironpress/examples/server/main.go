package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/nativebpm/connectors/ironpress"
)

func main() {
	addr := flag.String("addr", ":8080", "TCP address to listen on")
	binPath := flag.String("bin", "", "Path to ironpress binary (defaults to look in PATH or ~/.cargo/bin)")
	wasmPath := flag.String("wasm", "", "Path to ironpress.wasm module (enables in-process WASM rendering with Warmup)")
	workers := flag.Int("workers", 0, "Number of concurrent conversion workers (defaults to CPU count)")
	flag.Parse()

	// Try resolving default location if binPath is empty
	bin := *binPath
	if bin == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cargoBin := filepath.Join(home, ".cargo", "bin", "ironpress")
			if _, err := os.Stat(cargoBin); err == nil {
				bin = cargoBin
			}
		}
	}

	server := ironpress.NewServer(*addr, bin, *workers)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Check if WASM module is configured or present by default
	wasmFile := *wasmPath
	if wasmFile == "" {
		defaultWasm := "/tmp/ironpress_wasm/bin/ironpress.wasm"
		if _, err := os.Stat(defaultWasm); err == nil {
			wasmFile = defaultWasm
		}
	}

	if wasmFile != "" {
		wasmBytes, err := os.ReadFile(wasmFile)
		if err == nil {
			log.Printf("Initializing and warming up in-process WebAssembly engine from %s...", wasmFile)
			wasmClient := ironpress.NewClient(ironpress.WithWasm(wasmBytes))
			if err := wasmClient.Warmup(ctx); err == nil {
				server.WithWasmEngine(wasmClient)
				defer wasmClient.Close(context.Background())
				log.Printf("WebAssembly JIT Warmup completed successfully. In-process conversion enabled.")
			} else {
				log.Printf("WASM Warmup failed (%v), falling back to CLI", err)
			}
		}
	}

	log.Printf("Starting ironpress server...")
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
	log.Println("Server stopped cleanly.")
}
