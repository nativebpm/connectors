package ironpress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// Server wraps the HTTP service exposing ironpress conversion endpoints.
type Server struct {
	mu           sync.RWMutex
	addr         string
	ironpressBin string
	maxWorkers   int
	sem          chan struct{}
	httpServer   *http.Server
	wg           sync.WaitGroup
}

// NewServer creates a new Server instance.
// If ironpressBin is empty, it attempts to look it up in PATH.
// If maxWorkers <= 0, it defaults to runtime.NumCPU().
func NewServer(addr string, ironpressBin string, maxWorkers int) *Server {
	if ironpressBin == "" {
		var err error
		ironpressBin, err = exec.LookPath("ironpress")
		if err != nil {
			ironpressBin = "ironpress" // fallback to raw string, will fail at runtime if not found
		}
	}

	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}

	return &Server{
		addr:         addr,
		ironpressBin: ironpressBin,
		maxWorkers:   maxWorkers,
		sem:          make(chan struct{}, maxWorkers),
	}
}

// Start runs the HTTP server and blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/convert", s.handleConvert)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	// Update addr if dynamic port was used (e.g. :0)
	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.mu.Unlock()

	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting ironpress wrapper server on %s (max workers: %d, binary: %s)", s.Addr(), s.maxWorkers, s.ironpressBin)
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case <-ctx.Done():
		log.Println("Shutting down ironpress wrapper server gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
		// Wait for active CLI executions to finish
		s.wg.Wait()
		return nil
	case err := <-errChan:
		return err
	}
}

// Addr returns the server's listening address.
func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

// handleHealth checks if the ironpress binary is available.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Test if binary runs --help or version
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.ironpressBin, "--help")
	if err := cmd.Run(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf("ironpress binary check failed: %v", err)))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleConvert processes document conversion requests.
func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Restrict size of multipart form upload to 20MB
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse multipart form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing required 'file' parameter", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Acquire semaphore token before launching process
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-r.Context().Done():
		http.Error(w, "request timed out waiting for available worker slot", http.StatusServiceUnavailable)
		return
	}

	s.wg.Add(1)
	defer s.wg.Done()

	// Create temp directory for conversion run
	tempDir, err := os.MkdirTemp("", "ironpress-*")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create temp workspace: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Keep original extension (HTML or Markdown) so ironpress detects type
	inputFilename := header.Filename
	if inputFilename == "" {
		inputFilename = "index.html"
	}
	inputPath := filepath.Join(tempDir, inputFilename)
	outputPath := filepath.Join(tempDir, "output.pdf")

	inputFile, err := os.Create(inputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create temp input file: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(inputFile, file); err != nil {
		inputFile.Close()
		http.Error(w, fmt.Sprintf("failed to save input content: %v", err), http.StatusInternalServerError)
		return
	}
	inputFile.Close()

	// Build CLI arguments
	args := []string{}

	// Page Size
	if ps := r.FormValue("page-size"); ps != "" {
		args = append(args, "--page-size", ps)
	}

	// Landscape
	if ls := r.FormValue("landscape"); ls != "" {
		if val, err := strconv.ParseBool(ls); err == nil && val {
			args = append(args, "--landscape")
		}
	}

	// Margin
	if mg := r.FormValue("margin"); mg != "" {
		if _, err := strconv.ParseFloat(mg, 64); err == nil {
			args = append(args, "--margin", mg)
		}
	}

	// Header
	if hd := r.FormValue("header"); hd != "" {
		args = append(args, "--header", hd)
	}

	// Footer
	if ft := r.FormValue("footer"); ft != "" {
		args = append(args, "--footer", ft)
	}

	// Input and output file targets
	args = append(args, inputPath, outputPath)

	// Run command
	cmd := exec.CommandContext(r.Context(), s.ironpressBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		http.Error(w, fmt.Sprintf("conversion engine failed: %v\nstderr: %s", err, stderr.String()), http.StatusInternalServerError)
		return
	}

	// Stream generated PDF back to response
	outputFile, err := os.Open(outputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to open output PDF: %v", err), http.StatusInternalServerError)
		return
	}
	defer outputFile.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, outputFile)
}
