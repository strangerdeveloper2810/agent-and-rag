package fallback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestFallback_SuccessOnPrimary(t *testing.T) {
	primary := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "primary"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	fallback := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "fallback"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	fb, err := New(time.Second, primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for chunk := range stream {
		if chunk.Kind == provider.ChunkText {
			texts = append(texts, chunk.Text)
		}
	}
	if len(texts) != 1 || texts[0] != "primary" {
		t.Errorf("got %v, want [primary]", texts)
	}

	// All providers should be healthy
	for _, s := range fb.Status() {
		if s.Failures != 0 || s.CoolingDown {
			t.Errorf("provider %s should be healthy: %+v", s.Name, s)
		}
	}
}

type errorProvider struct {
	name string
	err  error
}

func (e *errorProvider) Name() string { return e.name }
func (e *errorProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	return nil, e.err
}

func TestFallback_FailoverToFallback(t *testing.T) {
	primary := &errorProvider{name: "broken", err: errors.New("429 Too Many Requests")}
	fallback := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "saved"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	fb, err := New(time.Second, primary, fallback)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for chunk := range stream {
		if chunk.Kind == provider.ChunkText {
			texts = append(texts, chunk.Text)
		}
	}
	if len(texts) != 1 || texts[0] != "saved" {
		t.Errorf("got %v, want [saved]", texts)
	}

	// Primary should be cooling down
	status := fb.Status()
	if !status[0].CoolingDown {
		t.Error("primary should be cooling down after failure")
	}
	if status[0].Failures != 1 {
		t.Errorf("primary failures = %d, want 1", status[0].Failures)
	}
}

func TestFallback_AllFail(t *testing.T) {
	p1 := &errorProvider{name: "a", err: errors.New("503 Service Unavailable")}
	p2 := &errorProvider{name: "b", err: errors.New("429 Rate Limit Exceeded")}
	fb, _ := New(time.Second, p1, p2)

	_, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestFallback_NonRetryableErrors(t *testing.T) {
	primary := &errorProvider{name: "a", err: errors.New("context canceled")}
	fallback := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	fb, _ := New(time.Second, primary, fallback)

	_, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err == nil {
		t.Fatal("expected error to propagate without failover")
	}
}

func TestFallback_RecoveryAfterCooldown(t *testing.T) {
	// Primary fails once, then recovers
	callCount := 0
	flaky := &flakyProvider{
		name: "flaky",
		fn: func() error {
			callCount++
			if callCount == 1 {
				return errors.New("500 Internal Server Error")
			}
			return nil
		},
	}
	fallback := provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	)
	fb, _ := New(10*time.Millisecond, flaky, fallback)

	// First call: primary fails → fallback succeeds
	_, err := fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !fb.Status()[0].CoolingDown {
		t.Error("flaky should be cooling down")
	}

	// Wait for cooldown
	time.Sleep(20 * time.Millisecond)

	// Second call: primary should be tried again and succeed
	_, err = fb.Generate(context.Background(), provider.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if fb.Status()[0].CoolingDown {
		t.Error("flaky should have recovered")
	}
}

func TestFallback_NeedsTwoProviders(t *testing.T) {
	p := provider.NewFake(provider.StreamChunk{Kind: provider.ChunkDone})
	_, err := New(time.Second, p)
	if err == nil {
		t.Fatal("expected error with only 1 provider")
	}
}

// flakyProvider fails based on fn() return value.
type flakyProvider struct {
	name string
	fn   func() error
}

func (f *flakyProvider) Name() string { return f.name }
func (f *flakyProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	if err := f.fn(); err != nil {
		return nil, err
	}
	return provider.NewFake(
		provider.StreamChunk{Kind: provider.ChunkText, Text: "ok"},
		provider.StreamChunk{Kind: provider.ChunkDone},
	).Generate(ctx, req)
}
