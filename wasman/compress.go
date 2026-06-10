//go:build !wasm

package wasman

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

var gzipReaderPool = sync.Pool{}

var (
	snapshotKey    [32]byte
	keyInitialized bool
	keyInitMu      sync.Mutex
)

// ensureKeyInitialized loads or generates a unique Snapshot Master Key (Golden Key).
func ensureKeyInitialized() error {
	if keyInitialized {
		return nil
	}
	keyInitMu.Lock()
	defer keyInitMu.Unlock()
	if keyInitialized {
		return nil
	}

	// 1. Check if user configured a master key or manual passphrase in environment
	passphrase := os.Getenv("CRYPTENV_KEY")
	if passphrase == "" {
		passphrase = os.Getenv("NATIVEBPM_SNAPSHOT_PASSPHRASE")
	}
	if passphrase != "" {
		snapshotKey = sha256.Sum256([]byte(passphrase))
		keyInitialized = true
		return nil
	}

	// 2. Check if a custom key file path is specified, otherwise default to secrets/snapshot_master.key
	keyPath := os.Getenv("NATIVEBPM_SNAPSHOT_KEY_FILE")
	if keyPath == "" {
		keyPath = "secrets/snapshot_master.key"
	}

	// Try reading the existing key
	keyBytes, err := os.ReadFile(keyPath)
	if err == nil {
		trimmed := bytes.TrimSpace(keyBytes)
		snapshotKey = sha256.Sum256(trimmed)
		keyInitialized = true
		return nil
	}

	// 3. Key does not exist, dynamically generate a unique 256-bit secure key
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return fmt.Errorf("failed to generate random key bytes: %w", err)
	}

	hexKey := make([]byte, hex.EncodedLen(len(newKey)))
	hex.Encode(hexKey, newKey)

	// Ensure target directory exists
	dir := filepath.Dir(keyPath)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create key directory: %w", err)
		}
	}

	// Save the unique Golden Key with read-only file permissions
	if err := os.WriteFile(keyPath, hexKey, 0600); err != nil {
		return fmt.Errorf("failed to save snapshot master key to %s: %w", keyPath, err)
	}

	slog.Info("Successfully generated and saved new WASM Snapshot Master Key (Golden Key)", "path", keyPath)
	snapshotKey = sha256.Sum256(hexKey)
	keyInitialized = true
	return nil
}

func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, _ := gzipWriterPool.Get().(*gzip.Writer)
	if zw == nil {
		zw = gzip.NewWriter(&buf)
	} else {
		zw.Reset(&buf)
	}
	_, err := zw.Write(data)
	if err != nil {
		return nil, err
	}
	err = zw.Close()
	if err != nil {
		return nil, err
	}
	zw.Reset(io.Discard) // Detach from buf to prevent memory retention
	gzipWriterPool.Put(zw)

	compressed := buf.Bytes()

	// Symmetrically encrypt compressed gzip archive using AES-256-GCM out-of-the-box
	if err := ensureKeyInitialized(); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(snapshotKey[:])
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nil, nonce, compressed, nil)

	// Combine: NONCE [12 bytes] + CIPHERTEXT (always encrypted, no magic header required)
	finalBuf := make([]byte, len(nonce)+len(ciphertext))
	copy(finalBuf, nonce)
	copy(finalBuf[len(nonce):], ciphertext)
	return finalBuf, nil
}

func decompressData(data []byte) ([]byte, error) {
	if err := ensureKeyInitialized(); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(snapshotKey[:])
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("invalid encrypted snapshot payload size")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	decrypted, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt snapshot (invalid Snapshot Master Key?): %w", err)
	}

	zr, _ := gzipReaderPool.Get().(*gzip.Reader)


	if zr == nil {
		var err error
		zr, err = gzip.NewReader(bytes.NewReader(decrypted))
		if err != nil {
			return nil, err
		}
	} else {
		err := zr.Reset(bytes.NewReader(decrypted))
		if err != nil {
			return nil, err
		}
	}
	defer func() {
		_ = zr.Close()
		_ = zr.Reset(bytes.NewReader(nil)) // Detach from input reader to prevent memory retention
		gzipReaderPool.Put(zr)
	}()
	return io.ReadAll(zr)
}

func isGzipped(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}


