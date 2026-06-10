package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

var gzipReaderPool = sync.Pool{}

// GzipCompress compresses data using pooled gzip.Writers to minimize heap allocations.
func GzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, ok := gzipWriterPool.Get().(*gzip.Writer)
	if !ok || zw == nil {
		zw = gzip.NewWriter(&buf)
	} else {
		zw.Reset(&buf)
	}

	if _, err := zw.Write(data); err != nil {
		zw.Reset(io.Discard)
		gzipWriterPool.Put(zw)
		return nil, err
	}

	if err := zw.Close(); err != nil {
		zw.Reset(io.Discard)
		gzipWriterPool.Put(zw)
		return nil, err
	}

	zw.Reset(io.Discard)
	gzipWriterPool.Put(zw)
	return buf.Bytes(), nil
}

// GzipDecompress decompresses data using pooled gzip.Readers to minimize heap allocations.
func GzipDecompress(data []byte) ([]byte, error) {
	zr, ok := gzipReaderPool.Get().(*gzip.Reader)
	if !ok || zr == nil {
		var err error
		zr, err = gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
	} else {
		if err := zr.Reset(bytes.NewReader(data)); err != nil {
			gzipReaderPool.Put(zr)
			return nil, err
		}
	}

	defer func() {
		_ = zr.Close()
		_ = zr.Reset(bytes.NewReader(nil))
		gzipReaderPool.Put(zr)
	}()

	return io.ReadAll(zr)
}
