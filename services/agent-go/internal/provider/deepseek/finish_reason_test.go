package deepseek

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func TestMapFinishReason(t *testing.T) {
	cases := map[string]provider.FinishReason{
		"length":              provider.FinishLength,
		"tool_calls":          provider.FinishToolCalls,
		"stop":                provider.FinishStop,
		"":                    "",
		"content_filter":      "",
		"unknown_from_future": "",
	}
	for raw, want := range cases {
		if got := mapFinishReason(raw); got != want {
			t.Errorf("mapFinishReason(%q) = %q, want %q", raw, got, want)
		}
	}
}

// collectStream drains the SSE body through streamSSE and returns the chunks.
func collectStream(t *testing.T, sse string) []provider.StreamChunk {
	t.Helper()
	c := &Client{}
	out := make(chan provider.StreamChunk)
	go c.streamSSE(context.Background(), io.NopCloser(strings.NewReader(sse)), out)

	var chunks []provider.StreamChunk
	for chunk := range out {
		chunks = append(chunks, chunk)
	}
	return chunks
}

func lastDone(t *testing.T, chunks []provider.StreamChunk) provider.StreamChunk {
	t.Helper()
	for i := len(chunks) - 1; i >= 0; i-- {
		if chunks[i].Kind == provider.ChunkDone {
			return chunks[i]
		}
	}
	t.Fatal("no ChunkDone in stream")
	return provider.StreamChunk{}
}

func TestStreamSSE_TruncatedByLength(t *testing.T) {
	sse := `data: {"choices":[{"index":0,"delta":{"content":"một câu dài"}}]}
data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}
data: [DONE]
`
	chunks := collectStream(t, sse)
	done := lastDone(t, chunks)
	if done.FinishReason != provider.FinishLength {
		t.Errorf("done.FinishReason = %q, want %q", done.FinishReason, provider.FinishLength)
	}
}

func TestStreamSSE_NormalStop(t *testing.T) {
	sse := `data: {"choices":[{"index":0,"delta":{"content":"xong"}}]}
data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}
data: [DONE]
`
	done := lastDone(t, collectStream(t, sse))
	if done.FinishReason != provider.FinishStop {
		t.Errorf("done.FinishReason = %q, want %q", done.FinishReason, provider.FinishStop)
	}
}

// Không có marker [DONE]: finish_reason vẫn phải theo tới chunk Done cuối.
func TestStreamSSE_LengthWithoutDoneMarker(t *testing.T) {
	sse := `data: {"choices":[{"index":0,"delta":{"content":"cụt"},"finish_reason":"length"}]}
`
	done := lastDone(t, collectStream(t, sse))
	if done.FinishReason != provider.FinishLength {
		t.Errorf("done.FinishReason = %q, want %q", done.FinishReason, provider.FinishLength)
	}
}
