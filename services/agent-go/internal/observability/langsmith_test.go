package observability_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/observability"
)

func TestLangSmithClient_Disabled(t *testing.T) {
	cfg := config.Config{
		LangSmithTracing: false,
		LangSmithAPIKey:  "",
	}
	client := observability.InitLangSmith(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Calling trace methods should be no-op and not panic
	client.StartChainRun("run-1", "Test Agent", nil, nil, nil)
	client.StartChildRun("child-1", "run-1", "gemini", observability.RunTypeLLM, nil, nil)
	client.EndRun("child-1", nil, nil)
	client.EndRun("run-1", nil, nil)
	client.Close()
}

func TestLangSmithClient_Enabled(t *testing.T) {
	var postReceived, patchReceived bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("expected x-api-key 'test-api-key', got %s", r.Header.Get("x-api-key"))
		}

		if r.Method == http.MethodPost && r.URL.Path == "/runs" {
			postReceived = true
			var run observability.CreateRunPayload
			if err := json.NewDecoder(r.Body).Decode(&run); err != nil {
				t.Errorf("failed to decode run: %v", err)
			}
			if run.ID != "test-run-1" {
				t.Errorf("expected run ID 'test-run-1', got %s", run.ID)
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodPatch && r.URL.Path == "/runs/test-run-1" {
			patchReceived = true
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{
		LangSmithTracing:  true,
		LangSmithAPIKey:   "test-api-key",
		LangSmithProject:  "test-proj",
		LangSmithEndpoint: srv.URL,
	}

	client := observability.InitLangSmith(cfg)
	client.StartChainRun("test-run-1", "JARVIS Agent", map[string]any{"msg": "hi"}, []string{"test"}, nil)
	client.EndRun("test-run-1", map[string]any{"res": "hello"}, nil)

	// Wait for queue flush
	client.Close()

	// Give a few ms for worker to finish HTTP requests
	time.Sleep(50 * time.Millisecond)

	if !postReceived {
		t.Error("expected POST /runs to be received")
	}
	if !patchReceived {
		t.Error("expected PATCH /runs/test-run-1 to be received")
	}
}
