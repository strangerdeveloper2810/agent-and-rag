// Package router implements RouterProvider: routes each request to either a
// cheap/fast LOCAL model (Ollama, OpenAI-compatible local server) or the
// existing CLOUD fallback chain (Gemini → DeepSeek → Claude, see
// internal/provider/factory.newAuto), depending on how "heavy" the request
// looks.
//
// Motivation: many requests (simple one-shot text generation, no tool use,
// no deep reasoning) don't need a hosted frontier model at all — a local
// model running on the same machine is free and fast enough. Only requests
// that need tool calling or explicit "thinking" are routed to the cloud
// chain, which is more capable but costs money and network round-trips.
package router

import (
	"context"
	"log/slog"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Router implements provider.Provider by routing each Generate call to
// either a local or a cloud provider.
type Router struct {
	local provider.Provider
	cloud provider.Provider
}

var _ provider.Provider = (*Router)(nil)

// New creates a Router. local is the cheap/fast local backend (typically
// ollama.Client or openai_compat.Client pointed at a local server); cloud is
// the existing cloud provider/chain (typically factory's "auto" chain:
// Gemini → DeepSeek → Claude).
func New(local, cloud provider.Provider) *Router {
	return &Router{local: local, cloud: cloud}
}

// Name identifies both branches so logs make it obvious which pair of
// providers a given Router instance was built from.
func (r *Router) Name() string {
	return "router(local=" + r.local.Name() + ",cloud=" + r.cloud.Name() + ")"
}

// Generate routes to local when the request is "cheap": no thinking
// requested AND no tools offered. Any tool use or non-off thinking level
// routes to cloud, since local models are assumed to be weaker at tool
// calling and multi-step reasoning than the hosted cloud chain.
//
// Fail-safe fallback semantics — this is the part most likely to be
// misread, read carefully:
//
//  1. If local.Generate() returns an error IMMEDIATELY — i.e. synchronously,
//     before any channel is handed back to us — that is a SETUP failure
//     (connection refused because the Ollama server isn't running, DNS
//     failure, malformed request, ...). At this point NOTHING has been sent
//     to the caller yet, so it is safe to silently retry the exact same
//     request on cloud instead of failing the whole request. This is the
//     common "forgot to start Ollama" case.
//
//  2. If local.Generate() returns a channel successfully, we hand that
//     channel straight to the caller and never touch it again — even if an
//     error later appears as a chunk on that channel (Kind == ChunkError),
//     at any position, including as the very first chunk. We deliberately
//     do NOT fall back to cloud in this case. Once the channel has been
//     handed off, the caller may already be streaming partial output from
//     the local model to the end user; silently switching providers
//     mid-stream would splice together output from two different models
//     into one Frankenstein response, which is worse than just surfacing
//     the error like every other provider does. So: error before the
//     channel exists → fallback. Error via the channel → never fallback,
//     propagate as-is.
func (r *Router) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	useLocal := req.Options.ThinkingLevel == provider.ThinkingOff && len(req.Tools) == 0

	if !useLocal {
		slog.Info("router: routing to cloud",
			"cloud_provider", r.cloud.Name(),
			"thinking_level", req.Options.ThinkingLevel,
			"tool_count", len(req.Tools),
		)
		return r.cloud.Generate(ctx, req)
	}

	slog.Info("router: routing to local",
		"local_provider", r.local.Name(),
	)
	stream, err := r.local.Generate(ctx, req)
	if err != nil {
		// Case 1 above: setup failed before we got a channel back — nothing
		// sent to the caller yet, safe to fall back to cloud.
		slog.Warn("router: local provider setup failed, falling back to cloud",
			"local_provider", r.local.Name(),
			"cloud_provider", r.cloud.Name(),
			"err", err,
		)
		return r.cloud.Generate(ctx, req)
	}

	// Case 2 above: channel obtained successfully — forward it verbatim,
	// no fallback from here on regardless of what appears on it later.
	return stream, nil
}
