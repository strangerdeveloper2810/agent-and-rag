package router

import (
	"context"
	"errors"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// namedFake wraps provider.FakeProvider to give it a distinguishable Name()
// (provider.FakeProvider.Name() always returns "fake", which would make
// local vs cloud indistinguishable in these tests).
type namedFake struct {
	*provider.FakeProvider
	name string
}

func newNamedFake(name string, chunks ...provider.StreamChunk) *namedFake {
	return &namedFake{FakeProvider: provider.NewFake(chunks...), name: name}
}

func (n *namedFake) Name() string { return n.name }

// errProvider always fails synchronously (before returning a channel) —
// simulates a local backend that isn't reachable at all (e.g. Ollama server
// not running: http.Client.Do fails before any channel is created).
type errProvider struct {
	name string
	err  error
}

func (e *errProvider) Name() string { return e.name }
func (e *errProvider) Generate(ctx context.Context, req provider.GenerateRequest) (<-chan provider.StreamChunk, error) {
	return nil, e.err
}

func drainText(t *testing.T, stream <-chan provider.StreamChunk) (texts []string, sawErr bool) {
	t.Helper()
	for chunk := range stream {
		switch chunk.Kind {
		case provider.ChunkText:
			texts = append(texts, chunk.Text)
		case provider.ChunkError:
			sawErr = true
		}
	}
	return texts, sawErr
}

func TestRouter_RoutesToLocal_WhenThinkingOffAndNoTools(t *testing.T) {
	local := newNamedFake("local", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-local"}, provider.StreamChunk{Kind: provider.ChunkDone})
	cloud := newNamedFake("cloud", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-cloud"}, provider.StreamChunk{Kind: provider.ChunkDone})

	r := New(local, cloud)
	req := provider.GenerateRequest{
		Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff},
	}

	stream, err := r.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	texts, sawErr := drainText(t, stream)
	if sawErr {
		t.Fatal("unexpected error chunk")
	}
	if len(texts) != 1 || texts[0] != "from-local" {
		t.Errorf("got %v, want [from-local]", texts)
	}
}

func TestRouter_RoutesToCloud_WhenThinkingNotOff(t *testing.T) {
	local := newNamedFake("local", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-local"}, provider.StreamChunk{Kind: provider.ChunkDone})
	cloud := newNamedFake("cloud", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-cloud"}, provider.StreamChunk{Kind: provider.ChunkDone})

	r := New(local, cloud)
	req := provider.GenerateRequest{
		Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingLow},
	}

	stream, err := r.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	texts, sawErr := drainText(t, stream)
	if sawErr {
		t.Fatal("unexpected error chunk")
	}
	if len(texts) != 1 || texts[0] != "from-cloud" {
		t.Errorf("got %v, want [from-cloud]", texts)
	}
}

func TestRouter_RoutesToCloud_WhenToolsPresent(t *testing.T) {
	local := newNamedFake("local", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-local"}, provider.StreamChunk{Kind: provider.ChunkDone})
	cloud := newNamedFake("cloud", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-cloud"}, provider.StreamChunk{Kind: provider.ChunkDone})

	r := New(local, cloud)
	req := provider.GenerateRequest{
		Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff},
		Tools:   []provider.ToolDef{{Name: "rag.search"}},
	}

	stream, err := r.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	texts, sawErr := drainText(t, stream)
	if sawErr {
		t.Fatal("unexpected error chunk")
	}
	if len(texts) != 1 || texts[0] != "from-cloud" {
		t.Errorf("got %v, want [from-cloud]", texts)
	}
}

// Local fails IMMEDIATELY (before any channel is returned, e.g. Ollama
// server not running) → Router must silently fall back to cloud.
func TestRouter_FallsBackToCloud_WhenLocalFailsImmediately(t *testing.T) {
	local := &errProvider{name: "local", err: errors.New("connection refused")}
	cloud := newNamedFake("cloud", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-cloud"}, provider.StreamChunk{Kind: provider.ChunkDone})

	r := New(local, cloud)
	req := provider.GenerateRequest{
		Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff},
	}

	stream, err := r.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v, want fallback to cloud instead of error", err)
	}
	texts, sawErr := drainText(t, stream)
	if sawErr {
		t.Fatal("unexpected error chunk")
	}
	if len(texts) != 1 || texts[0] != "from-cloud" {
		t.Errorf("got %v, want [from-cloud] (fallback)", texts)
	}
}

// Local fails MID-STREAM (after already emitting a real text chunk) → Router
// must NOT fall back to cloud; the error must propagate as-is, exactly like
// any other provider would, because part of the answer may already have
// reached the end user.
func TestRouter_DoesNotFallBack_WhenLocalFailsMidStream(t *testing.T) {
	streamErr := errors.New("local: connection reset mid-stream")
	local := newNamedFake("local",
		provider.StreamChunk{Kind: provider.ChunkText, Text: "partial-from-local"},
		provider.StreamChunk{Kind: provider.ChunkError, Err: streamErr},
	)
	cloud := newNamedFake("cloud", provider.StreamChunk{Kind: provider.ChunkText, Text: "from-cloud"}, provider.StreamChunk{Kind: provider.ChunkDone})

	r := New(local, cloud)
	req := provider.GenerateRequest{
		Options: provider.ProviderOptions{ThinkingLevel: provider.ThinkingOff},
	}

	stream, err := r.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate: %v, want nil error (channel already handed back)", err)
	}

	var gotErr error
	var texts []string
	for chunk := range stream {
		if chunk.Kind == provider.ChunkText {
			texts = append(texts, chunk.Text)
		}
		if chunk.Kind == provider.ChunkError {
			gotErr = chunk.Err
		}
	}

	if len(texts) != 1 || texts[0] != "partial-from-local" {
		t.Errorf("texts = %v, want [partial-from-local] (no splice with cloud output)", texts)
	}
	if gotErr == nil || gotErr.Error() != streamErr.Error() {
		t.Errorf("gotErr = %v, want propagated %v (no silent fallback mid-stream)", gotErr, streamErr)
	}
}

func TestRouter_Name(t *testing.T) {
	local := newNamedFake("local")
	cloud := newNamedFake("cloud")
	r := New(local, cloud)

	want := "router(local=local,cloud=cloud)"
	if r.Name() != want {
		t.Errorf("Name() = %q, want %q", r.Name(), want)
	}
}
