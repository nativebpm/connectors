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

	log.Printf("Starting ironpress server...")
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
	log.Println("Server stopped cleanly.")
}
