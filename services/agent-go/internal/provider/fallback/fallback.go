// Package fallback implements automatic provider failover: when the primary
// LLM provider fails (rate limit, timeout, server error), requests are
// transparently retried on the next provider in the chain.
//
// Pattern: Primary → Fallback1 → Fallback2 → ... → error if all fail.
// Recovery: failed providers are retried after a cooldown period.
// Health tracking: counts consecutive failures and cooldown per provider.
package fallback

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// Provider wraps multiple LLM providers with automatic failover.
type Provider struct {
	chain    []namedProvider
	cooldown time.Duration
	mu       sync.RWMutex
}

type namedProvider struct {
	name      string
	prov      provider.Provider
	coolUntil atomic.Int64 // Unix nano — 0 = available
	failures  atomic.Int64 // consecutive failures
}

// New creates a fallback provider chain. Providers are tried in order.
// cooldown: how long to wait before retrying a failed provider (0 = no cooldown, immediate retry).
func New(cooldown time.Duration, providers ...provider.Provider) (*Provider, error) {
	if len(providers) < 2 {
		return nil, fmt.Errorf("fallback: need at least 2 providers, got %d", len(providers))
	}
	if cooldown < 0 {
		cooldown = 30 * time.Second
	}
	chain := make([]namedProvider, len(providers))
	for i, p := range providers {
		chain[i] = namedProvider{name: p.Name(), prov: p}
	}
	return &Provider{chain: chain, cooldown: cooldown}, nil
}

// Name returns combined names of all providers in the chain.
func (p *Provider) Name() string {
	names := make([]string, len(p.chain))
	for i := range p.chain {
		names[i] = p.chain[i].name
	}
	return "fallback[" + strings.Join(names, "→") + "]"
}

// Generate tries each provider in order. If one fails with a retryable error,
// the next provider is tried. Non-retryable errors (context cancel, invalid args)
// are returned immediately without failover.
func (p *Provider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	var lastErr error

	for i := range p.chain {
		np := &p.chain[i]

		// Skip if cooling down
		if coolUntil := np.coolUntil.Load(); coolUntil > 0 {
			if time.Now().UnixNano() < coolUntil {
				continue
			}
			np.coolUntil.Store(0) // cooldown expired
		}

		stream, err := np.prov.Generate(ctx, req)
		if err != nil {
			if !isRetryable(err) {
				return nil, err
			}
			lastErr = err
			p.recordFailure(np, err)
			continue
		}

		// Check first chunk from stream — rate limit errors may arrive on stream, not via Generate()
		wrapped := make(chan provider.StreamChunk, 1)
		go func() {
			defer close(wrapped)
			first := true
			for chunk := range stream {
				if first && chunk.Kind == provider.ChunkError && isRetryable(chunk.Err) {
					// Rate limit on stream → don't forward, try next provider below
					wrapped <- chunk
					return
				}
				first = false
				wrapped <- chunk
			}
		}()

		// Peek at first chunk
		firstChunk, ok := <-wrapped
		if !ok {
			np.failures.Store(0)
			return emptyStream(), nil
		}
		if firstChunk.Kind == provider.ChunkError && isRetryable(firstChunk.Err) {
			lastErr = firstChunk.Err
			p.recordFailure(np, firstChunk.Err)
			// Drain the wrapper goroutine
			for range wrapped {
			}
			continue // try next provider
		}

		// Success — return a stream that replays first chunk then continues
		np.failures.Store(0)
		return replayStream(firstChunk, wrapped), nil
	}

	return nil, fmt.Errorf("fallback: all %d providers failed: %w", len(p.chain), lastErr)
}

func (p *Provider) recordFailure(np *namedProvider, err error) {
	fails := np.failures.Add(1)
	if p.cooldown <= 0 {
		return
	}
	cd := p.cooldown * (1 << min(int(fails)-1, 4))
	if isDailyQuotaExhausted(err) {
		// Day-lock: 2 hours cooldown when daily quota is exhausted
		cd = 2 * time.Hour
	} else if cd > 5*time.Minute {
		cd = 5 * time.Minute
	}
	np.coolUntil.Store(time.Now().Add(cd).UnixNano())
}

// Status returns health status of all providers.
func (p *Provider) Status() []ProviderStatus {
	out := make([]ProviderStatus, len(p.chain))
	for i := range p.chain {
		cu := p.chain[i].coolUntil.Load()
		out[i] = ProviderStatus{
			Name:        p.chain[i].name,
			Failures:    p.chain[i].failures.Load(),
			CoolingDown: cu > 0 && time.Now().UnixNano() < cu,
		}
	}
	return out
}

func emptyStream() <-chan provider.StreamChunk {
	ch := make(chan provider.StreamChunk)
	close(ch)
	return ch
}

func replayStream(first provider.StreamChunk, rest <-chan provider.StreamChunk) <-chan provider.StreamChunk {
	out := make(chan provider.StreamChunk, 16)
	go func() {
		defer close(out)
		select {
		case out <- first:
		case <-time.After(5 * time.Second):
			return
		}
		for chunk := range rest {
			out <- chunk
		}
	}()
	return out
}

// ProviderStatus reports health of a single provider.
type ProviderStatus struct {
	Name        string `json:"name"`
	Failures    int64  `json:"failures"`
	CoolingDown bool   `json:"coolingDown"`
}

// --- Helpers ---

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Rate limit / server errors → retry
	retryable := []string{
		"429", "rate limit", "too many requests",
		"resource_exhausted", "quota exceeded",
		"503", "502", "500", "internal server error",
		"timeout", "deadline exceeded", "connection refused",
		"temporarily unavailable", "service unavailable",
	}
	for _, p := range retryable {
		if strings.Contains(msg, p) {
			return true
		}
	}

	// Non-retryable → don't failover
	nonRetryable := []string{"400", "401", "403", "invalid", "context canceled"}
	for _, p := range nonRetryable {
		if strings.Contains(msg, p) {
			return false
		}
	}

	// Unknown error → retry (safety: rather retry than fail silently)
	return true
}

func isDailyQuotaExhausted(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "per day") ||
		strings.Contains(msg, "daily") ||
		strings.Contains(msg, "day limit") ||
		strings.Contains(msg, "free_tier_requests_per_day") ||
		strings.Contains(msg, "rpd")
}
