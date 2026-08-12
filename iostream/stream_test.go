package iostream

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestPayload struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestPipeReader(t *testing.T) {
	expected := TestPayload{ID: 100, Name: "ZeroAllocStream"}

	reader := PipeReader(func(w io.Writer) error {
		return json.NewEncoder(w).Encode(expected)
	})
	defer reader.Close()

	var actual TestPayload
	if err := json.NewDecoder(reader).Decode(&actual); err != nil {
		t.Fatalf("failed to decode stream: %v", err)
	}

	if actual.ID != expected.ID || actual.Name != expected.Name {
		t.Fatalf("expected %+v, got %+v", expected, actual)
	}
}

func TestStreamBuilder_ExecuteHTTPRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload TestPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.ID != 42 {
			http.Error(w, "invalid payload ID", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	payload := TestPayload{ID: 42, Name: "NativeBPM Stream"}

	resp, err := NewStream().
		WithJSONPayload(payload).
		ToURL(http.MethodPost, ts.URL).
		WithHeader("X-Custom-Header", "StreamValue").
		ExecuteHTTPRequest(context.Background())

	if err != nil {
		t.Fatalf("unexpected error executing stream request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", resp.StatusCode)
	}
}

func BenchmarkStreamBuilder(b *testing.B) {
	payload := TestPayload{ID: 999, Name: "BenchmarkStreamPayload"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader, _ := NewStream().
			WithJSONPayload(payload).
			Reader()

		_, _ = io.Copy(io.Discard, reader)
		_ = reader.Close()
	}
}
