package nativebpm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_AllEndpoints(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Deploy
	mux.HandleFunc("/api/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := r.FormValue("id")
		name := r.FormValue("name")
		if id != "proc-1" || name != "Test Process" {
			http.Error(w, "invalid form params", http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil || file == nil || header.Filename != "Test Process.bpmn" {
			http.Error(w, "missing/invalid file parameter", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"deployed","process_id":"proc-1","name":"Test Process"}`))
	})

	// 2. List Definitions
	mux.HandleFunc("/api/definitions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"proc-1","name":"Test Process","deployed_at":"2026-06-03T12:00:00Z"}]`))
	})

	// 3. Start Process Instance
	mux.HandleFunc("/api/definitions/proc-1/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		var body startInstanceRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.InstanceID != "inst-1" || body.Variables["amount"] != float64(500) {
			http.Error(w, "invalid payload values", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inst-1","process_id":"proc-1","variables":{"amount":500},"completed":false}`))
	})

	// 4. List Instances
	mux.HandleFunc("/api/instances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"inst-1","process_id":"proc-1","state":null,"version":1,"completed":false}]`))
	})

	// 5. Get Instance
	mux.HandleFunc("/api/instances/inst-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inst-1","process_id":"proc-1","variables":{"amount":500},"completed":false}`))
	})

	// 6. Complete Task
	mux.HandleFunc("/api/instances/inst-1/complete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		var body completeTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.NodeID != "task-1" || body.Variables["approved"] != true {
			http.Error(w, "invalid payload values", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"inst-1","process_id":"proc-1","variables":{"amount":500,"approved":true},"completed":true}`))
	})

	// 7. Get Instance Logs
	mux.HandleFunc("/api/instances/inst-1/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"log-1","instance_id":"inst-1","node_id":"task-1","node_name":"Approve Loan","node_type":"userTask","action":"complete","variables":"eyJhcHByb3ZlZCI6dHJ1ZX0=","timestamp":"2026-06-03T12:05:00Z"}]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to construct client: %v", err)
	}

	ctx := context.Background()

	// 1. Test Deploy (Fluent Builder)
	deployResp, err := client.Deploy("proc-1", "Test Process").XML([]byte("<bpmn/>")).Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if deployResp.Status != "deployed" || deployResp.ProcessID != "proc-1" {
		t.Errorf("Unexpected deploy response: %+v", deployResp)
	}

	// 2. Test List Definitions
	defs, err := client.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("ListDefinitions failed: %v", err)
	}
	if len(defs) != 1 || defs[0].ID != "proc-1" || defs[0].Name != "Test Process" {
		t.Errorf("Unexpected definitions: %+v", defs)
	}

	// 3. Test Start Process Instance (Fluent Builder)
	pi, err := client.StartProcessInstance("proc-1").
		InstanceID("inst-1").
		Variable("amount", 500).
		Send(ctx)
	if err != nil {
		t.Fatalf("StartProcessInstance failed: %v", err)
	}
	if pi.ID != "inst-1" || pi.Variables["amount"] != float64(500) || pi.Completed {
		t.Errorf("Unexpected process instance state: %+v", pi)
	}

	// 4. Test List Instances
	records, err := client.ListInstances(ctx)
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}
	if len(records) != 1 || records[0].ID != "inst-1" || records[0].Completed {
		t.Errorf("Unexpected process instance records: %+v", records)
	}

	// 5. Test Get Instance
	piGet, err := client.GetInstance(ctx, "inst-1")
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}
	if piGet.ID != "inst-1" || piGet.Variables["amount"] != float64(500) {
		t.Errorf("Unexpected get instance: %+v", piGet)
	}

	// 6. Test Complete Task (Fluent Builder)
	piComp, err := client.CompleteTask("inst-1", "task-1").
		Variable("approved", true).
		Send(ctx)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}
	if piComp.ID != "inst-1" || piComp.Variables["approved"] != true || !piComp.Completed {
		t.Errorf("Unexpected completed process instance: %+v", piComp)
	}

	// 7. Test Get Instance Logs
	logs, err := client.GetInstanceLogs(ctx, "inst-1")
	if err != nil {
		t.Fatalf("GetInstanceLogs failed: %v", err)
	}
	if len(logs) != 1 || logs[0].NodeID != "task-1" || logs[0].Action != "complete" {
		t.Errorf("Unexpected logs response: %+v", logs)
	}
	logVars, err := logs[0].ParseVariables()
	if err != nil {
		t.Fatalf("ParseVariables failed: %v", err)
	}
	if logVars["approved"] != true {
		t.Errorf("Unexpected log variables: %+v", logVars)
	}
}

func TestClient_CustomRoundTripper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Echo", r.Header.Get("X-Custom-Header"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to construct client: %v", err)
	}

	// Register middleware
	client.Use(func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r.Header.Set("X-Custom-Header", "hello-world")
			return next.RoundTrip(r)
		})
	})

	resp, err := client.httpClient.GET(context.Background(), "/").Send()
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("X-Custom-Echo") != "hello-world" {
		t.Errorf("Custom middleware was not called, got: %s", resp.Header.Get("X-Custom-Echo"))
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
