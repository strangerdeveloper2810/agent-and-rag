// Command server là entrypoint của JARVIS agent runtime.
// Nạp config, wire Provider + Tool Registry + Engine, start HTTP server (SSE chat),
// graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/memory"
	"github.com/ai-agent-tut/agent-go/internal/orchestrator"
	"github.com/ai-agent-tut/agent-go/internal/provider/factory"
	"github.com/ai-agent-tut/agent-go/internal/tools"
	agenthttp "github.com/ai-agent-tut/agent-go/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	// --- Wire Provider ---
	prov, err := factory.New(cfg)
	if err != nil {
		slog.Error("provider", "err", err)
		os.Exit(1)
	}

	// --- Wire Tool Registry ---
	registry := tools.NewRegistry()
	registry.Register(tools.NewEchoTool())
	// Thêm tool thật ở các phase sau.

	// --- Wire Memory Store ---
	store := memory.NewStore()

	// --- Wire Orchestrator (multi-agent) ---
	generalEngine := agent.NewEngine(prov, registry)
	generalEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)

	codeEngine := agent.NewEngine(prov, registry)
	codeEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)

	orch := orchestrator.New()
	orch.Register(&orchestrator.AgentSpec{
		Name:            "general",
		Description:     "General-purpose assistant for everyday tasks and conversation",
		Engine:          generalEngine,
		TriggerKeywords: []string{},
		SystemPrompt:    agent.BuildSystemPrompt(nil),
	})
	orch.Register(&orchestrator.AgentSpec{
		Name:            "code",
		Description:     "Code and programming specialist for development tasks",
		Engine:          codeEngine,
		TriggerKeywords: []string{"code", "programming", "function", "bug", "debug",
			"go", "python", "typescript", "javascript", "rust", "refactor", "test"},
		SystemPrompt: agent.BuildSystemPrompt(nil),
	})
	if err := orch.SetDefault("general"); err != nil {
		slog.Error("orchestrator", "err", err)
		os.Exit(1)
	}

	// --- HTTP Routes ---
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", agenthttp.Healthz)
	mux.HandleFunc("POST /chat", agenthttp.NewChatHandler(orch).ServeHTTP)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}

	// Start server
	go func() {
		slog.Info("jarvis listening", "addr", srv.Addr, "provider", prov.Name())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	slog.Info("shutting down…")
	_ = srv.Shutdown(ctx)
}
