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
	"io"
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
	"github.com/ai-agent-tut/agent-go/internal/rag"
	"github.com/ai-agent-tut/agent-go/internal/skills"
	"github.com/ai-agent-tut/agent-go/internal/tools"
	agenthttp "github.com/ai-agent-tut/agent-go/internal/transport/http"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run điều phối subcommand và TRẢ VỀ exit code thay vì gọi os.Exit trực tiếp,
// để test gọi được mọi nhánh (trừ serve/ask/chat vì chúng cần LLM thật).
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stdout, usageText)
		return 1
	}

	switch cmd := args[0]; cmd {
	case "serve":
		runServe()
	case "ask":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: jarvis ask \"câu hỏi\"")
			return 1
		}
		runAsk(strings.Join(args[1:], " "))
	case "chat":
		runChat()
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, usageText)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		fmt.Fprintln(stdout, usageText)
		return 1
	}
	return 0
}

const usageText = `JARVIS — AI assistant CLI

usage:
  jarvis serve              start HTTP server
  jarvis ask "câu hỏi"      one-shot question
  jarvis chat               interactive chat (REPL)
  jarvis help               show this help`

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

	if err := askOnce(context.Background(), orch, question, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "\nengine: %v\n", err)
		os.Exit(1)
	}
}

// askOnce chạy 1 lượt hỏi-đáp, in text ra stdout và event phụ ra stderr.
// Tách khỏi runAsk để test được với runner giả (không cần LLM thật).
func askOnce(ctx context.Context, runner agent.Runner, question string, stdout, stderr io.Writer) error {
	input := agent.RunInput{
		UserMessage: question,
		MaxSteps:    12,
	}

	emit := func(e agent.Event) {
		switch e.Type {
		case "text":
			fmt.Fprint(stdout, e.Text)
		case "error":
			fmt.Fprintf(stderr, "\n[error] %s\n", e.Message)
		case "memory":
			fmt.Fprintf(stderr, "[memory] %s\n", e.Message)
		}
	}

	if _, err := runner.Run(ctx, input, emit); err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	return nil
}

// --- chat ---

func runChat() {
	_, _, orch := setup()

	fmt.Fprintf(os.Stderr, "JARVIS chat. go /exit de thoat.\n\n")
	chatLoop(context.Background(), orch, os.Stdin, os.Stdout, os.Stderr)
}

// chatLoop đọc từng dòng từ in, chạy runner, giữ history qua các lượt.
// Tách khỏi runChat để test được với stdin giả + runner giả.
func chatLoop(ctx context.Context, runner agent.Runner, in io.Reader, stdout, stderr io.Writer) {
	var history []provider.Message
	scanner := bufio.NewScanner(in)

	for {
		fmt.Fprint(stdout, "> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			fmt.Fprintln(stdout, "tam biet!")
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
				fmt.Fprint(stdout, e.Text)
			case "error":
				fmt.Fprintf(stderr, "\n[error] %s\n", e.Message)
			case "memory":
				fmt.Fprintf(stderr, "[memory] %s\n", e.Message)
			}
		}

		_, err := runner.Run(ctx, input, emit)
		fmt.Fprintln(stdout)
		if err != nil {
			fmt.Fprintf(stderr, "engine: %v\n", err)
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
	if cfg.VoyageKey != "" {
		vc := rag.NewClient(cfg.VoyageKey)
		store.SetEmbedder(memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
			return vc.Embed(ctx, texts, "document")
		}))
		slog.Info("memory: semantic embedding enabled", "provider", "voyage")
	} else if cfg.Provider == "ollama" || os.Getenv("OLLAMA_URL") != "" {
		embedClient, err := ollama.New(cfg.OllamaURL, cfg.EmbedModel)
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
			slog.Info("memory: semantic embedding enabled", "provider", "ollama", "url", cfg.OllamaURL)
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
	generalEngine.SetMaxContextTokens(cfg.MaxContextTokens)
	generalEngine.SetFastModel(fastModel(cfg))
	generalEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(prov, fastModel(cfg)),
	)

	// Code agent.
	codeEngine := agent.NewEngine(prov, registry)
	codeEngine.SetSkillLoader(skillLoader)
	codeEngine.SetMaxContextTokens(cfg.MaxContextTokens)
	codeEngine.SetFastModel(fastModel(cfg))
	codeEngine.SetMemoryNodes(
		memory.RecallNode(store),
		memory.ExtractNode(store),
		memory.SummarizeNode(prov, fastModel(cfg)),
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

// fastModel chọn model rẻ/nhanh cho các tác vụ phụ trợ tốn thêm 1 LLM call
// (nén ngữ cảnh trong trimContext/SummarizeNode) — KHÔNG dùng model chính cho
// hội thoại để tránh đội chi phí. Ưu tiên DeepSeek flash (rẻ nhất) nếu có
// key, rơi về Gemini model chính nếu không.
func fastModel(cfg config.Config) string {
	if cfg.DeepSeekFlashModel != "" && cfg.DeepSeekKey != "" {
		return cfg.DeepSeekFlashModel
	}
	return cfg.GeminiModel
}
