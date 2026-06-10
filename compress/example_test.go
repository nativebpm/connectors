package compress_test

import (
	"fmt"
	"log"

	"github.com/nativebpm/connectors/compress"
)

// Example illustrates basic usage of GzipCompress and GzipDecompress using pooled gzip readers/writers
// to minimize allocations under high concurrency.
func Example() {
	originalText := []byte("NativeBPM high-performance gzip compression demo payload text.")

	// 1. Compress the byte slice
	compressedBytes, err := compress.GzipCompress(originalText)
	if err != nil {
		log.Fatalf("Compression failed: %v", err)
	}
	fmt.Printf("Compression output is non-empty: %t\n", len(compressedBytes) > 0)

	// 2. Decompress the byte slice back to original text
	decompressedBytes, err := compress.GzipDecompress(compressedBytes)
	if err != nil {
		log.Fatalf("Decompression failed: %v", err)
	}
	fmt.Println("Decompressed data matches original:", string(decompressedBytes))

	// Output:
	// Compression output is non-empty: true
	// Decompressed data matches original: NativeBPM high-performance gzip compression demo payload text.
}
