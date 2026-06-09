package wasman

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestAgeEncryptedGzipCompression(t *testing.T) {
	// Original test data
	originalData := []byte("The quick brown fox jumps over the lazy dog. WebAssembly sandbox persistence security verification.")

	// Clean up any existing master key file for clean test environment
	os.Setenv("NATIVEBPM_SNAPSHOT_KEY_FILE", "secrets/test_snapshot_master.key")
	_ = os.Remove("secrets/test_snapshot_master.key")
	defer func() {
		_ = os.Remove("secrets/test_snapshot_master.key")
		_ = os.Remove("secrets")
	}()

	t.Run("Default Encryption Out-of-the-Box", func(t *testing.T) {
		_ = os.Unsetenv("NATIVEBPM_SNAPSHOT_PASSPHRASE")
		keyInitialized = false // Reset internal state for test isolation

		compressed, err := compressData(originalData)
		if err != nil {
			t.Fatalf("compressData failed: %v", err)
		}

		// Encrypted snapshots are always encrypted by default
		decompressed, err := decompressData(compressed)
		if err != nil {
			t.Fatalf("decompressData failed: %v", err)
		}

		if !bytes.Equal(originalData, decompressed) {
			t.Errorf("decompressed data mismatch")
		}

		// Verify key file was created automatically in the secrets directory
		if _, err := os.Stat("secrets/test_snapshot_master.key"); os.IsNotExist(err) {
			t.Errorf("expected secrets/test_snapshot_master.key to be created automatically")
		}
	})

	t.Run("Encrypted Gzip Compression (With Custom Passphrase)", func(t *testing.T) {
		passphrase := "my-custom-strong-snapshot-passphrase"
		_ = os.Setenv("NATIVEBPM_SNAPSHOT_PASSPHRASE", passphrase)
		defer os.Unsetenv("NATIVEBPM_SNAPSHOT_PASSPHRASE")
		keyInitialized = false

		compressed, err := compressData(originalData)
		if err != nil {
			t.Fatalf("compressData failed: %v", err)
		}

		decompressed, err := decompressData(compressed)
		if err != nil {
			t.Fatalf("decompressData failed: %v", err)
		}

		if !bytes.Equal(originalData, decompressed) {
			t.Errorf("decompressed data mismatch")
		}
	})

	t.Run("Decryption Failure with Invalid Passphrase", func(t *testing.T) {
		// Compress with passphrase 1
		_ = os.Setenv("NATIVEBPM_SNAPSHOT_PASSPHRASE", "passphrase-1")
		keyInitialized = false
		encrypted, err := compressData(originalData)
		if err != nil {
			t.Fatalf("compressData failed: %v", err)
		}

		// Try to decrypt with passphrase 2
		_ = os.Setenv("NATIVEBPM_SNAPSHOT_PASSPHRASE", "passphrase-2")
		defer os.Unsetenv("NATIVEBPM_SNAPSHOT_PASSPHRASE")
		keyInitialized = false

		_, err = decompressData(encrypted)
		if err == nil {
			t.Fatalf("expected decryption error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to decrypt snapshot") {
			t.Errorf("expected 'failed to decrypt snapshot' error, got: %v", err)
		}
	})
}
