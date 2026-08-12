// Command jarvis là CLI entrypoint cho JARVIS agent.
//
// Subcommands:
//
//	serve                start HTTP server (SSE chat)
//	ask "câu hỏi"        one-shot question, in kết quả ra stdout
//	chat                 interactive chat (REPL)
//
//	stdlib only — không dùng thư viện CLI ngoài.
//
// Usage:
//
//	jarvis serve
//	jarvis ask "thời tiết hôm nay thế nào?"
//	jarvis chat
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/config"
	"github.com/ai-agent-tut/agent-go/internal/memory"
	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/orchestrator"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/provider/factory"
	"github.com/ai-agent-tut/agent-go/internal/provider/ollama"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
	agenthttp "github.com/ai-agent-tut/agent-go/internal/transport/http"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "serve":
		runServe()
	case "ask":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: jarvis ask \"câu hỏi\"")
			os.Exit(1)
		}
		question := strings.Join(os.Args[2:], " ")
		runAsk(question)
	case "chat":
		runChat()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`JARVIS — AI assistant CLI

usage:
  jarvis serve              start HTTP server
  jarvis ask "câu hỏi"      one-shot question
  jarvis chat               interactive chat (REPL)
  jarvis help               show this help`)
}

// --- serve ---

func runServe() {
	cfg, prov, orch := setup()
	_ = prov

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", agenthttp.Healthz)
	mux.HandleFunc("POST /chat", agenthttp.NewChatHandler(orch).ServeHTTP)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: middleware.CORSMiddleware(mux)}

	go func() {
		slog.Info("jarvis listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	slog.Info("shutting down…")
	_ = srv.Shutdown(ctx)
}

// --- ask ---

func runAsk(question string) {
	_, _, orch := setup()
	runner := agent.Runner(orch)

	input := agent.RunInput{
		UserMessage: question,
		MaxSteps:    12,
	}

	var output strings.Builder
	emit := func(e agent.Event) {
		switch e.Type {
		case "text":
			output.WriteString(e.Text)
			fmt.Print(e.Text)
		case "error":
			fmt.Fprintf(os.Stderr, "\n[error] %s\n", e.Message)
		case "memory":
			fmt.Fprintf(os.Stderr, "[memory] %s\n", e.Message)
		}
	}

	if _, err := runner.Run(context.Background(), input, emit); err != nil {
		fmt.Fprintf(os.Stderr, "\nengine: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
}

// --- chat ---

func runChat() {
	_, _, orch := setup()
	runner := agent.Runner(orch)

	fmt.Fprintf(os.Stderr, "JARVIS chat. go /exit de thoat.\n\n")

	var history []provider.Message
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			fmt.Println("tam biet!")
			break
		}

		input := agent.RunInput{
			History:     history,
			UserMessage: line,
			MaxSteps:    12,
		}

		var assistantContent strings.Builder
		emit := func(e agent.Event) {
			switch e.Type {
			case "text":
				assistantContent.WriteString(e.Text)
				fmt.Print(e.Text)
			case "error":
				fmt.Fprintf(os.Stderr, "\n[error] %s\n", e.Message)
			case "memory":
				fmt.Fprintf(os.Stderr, "[memory] %s\n", e.Message)
			}
		}

		_, err := runner.Run(context.Background(), input, emit)
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "engine: %v\n", err)
			continue
		}

		// Maintain conversation history across turns.
		history = append(history, provider.Message{
			Role:    provider.RoleUser,
			Content: line,
		})
		if resp := assistantContent.String(); resp != "" {
			history = append(history, provider.Message{
				Role:    provider.RoleAssistant,
				Content: resp,
			})
		}
	}
}

// --- wiring ---

// setup tạo config, provider, tools, memory store, engines, orchestrator.
// Dùng chung cho serve/ask/chat.
func setup() (config.Config, provider.Provider, *orchestrator.Orchestrator) {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	prov, err := factory.New(cfg)
	if err != nil {
		slog.Error("provider", "err", err)
		os.Exit(1)
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewEchoTool())

	store := memory.NewStore()

	// Wire embedding provider for semantic memory recall.
	if cfg.OllamaURL != "" {
		embedClient, err := ollama.New(cfg.OllamaURL, "nomic-embed-text")
		if err != nil {
			slog.Warn("memory: ollama embed client creation failed", "err", err)
		} else {
			store.SetEmbedder(memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
				vecs32, err := embedClient.Embed(ctx, texts)
				if err != nil {
					return nil, err
				}
				result := make([][]float64, len(vecs32))
				for i, v := range vecs32 {
					result[i] = make([]float64, len(v))
					for j, val := range v {
						result[i][j] = float64(val)
					}
				}
				return result, nil
			}))
			slog.Info("memory: semantic embedding enabled", "url", cfg.OllamaURL)
		}
	}

	// Load skills for progressive disclosure in system prompt.
	var skillSummaries []skills.SkillSummary
	var skillLoader *skills.Loader
	if loader, err := skills.NewLoader(cfg.SkillsDir); err == nil {
		skillLoader = loader
		skillSummaries = loader.ListSkills()
		slog.Info("skills loaded", "count", len(skillSummaries), "dir", cfg.SkillsDir)
	} else {
		slog.Warn("skills not loaded", "err", err)
	}

	// General agent.
	generalEngine := agent.NewEngine(prov, registry)
	generalEngine.SetSkillLoader(skillLoader)
	generalEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(),
	)

	// Code agent.
	codeEngine := agent.NewEngine(prov, registry)
	codeEngine.SetSkillLoader(skillLoader)
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
		SystemPrompt:    agent.BuildSystemPrompt(nil, skillSummaries),
	})
	orch.Register(&orchestrator.AgentSpec{
		Name:        "code",
		Description: "Code and programming specialist for development tasks",
		Engine:      codeEngine,
		TriggerKeywords: []string{"code", "programming", "function", "bug", "debug",
			"go", "python", "typescript", "javascript", "rust", "refactor", "test"},
		SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries),
	})
	if err := orch.SetDefault("general"); err != nil {
		slog.Error("orchestrator", "err", err)
		os.Exit(1)
	}

	return cfg, prov, orch
}
