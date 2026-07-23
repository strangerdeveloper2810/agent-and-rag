// Command server là entrypoint của agent-go: nạp config, dựng HTTP server (SSE),
// graceful shutdown. (Mongo/engine/provider wiring sẽ thêm ở các phase sau.)
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

	"github.com/ai-agent-tut/agent-go/internal/config"
	agenthttp "github.com/ai-agent-tut/agent-go/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", agenthttp.Healthz)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}

	// Graceful shutdown: SIGINT/SIGTERM → tắt server gọn gàng.
	go func() {
		slog.Info("agent-go listening", "addr", srv.Addr, "provider", cfg.Provider)
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
