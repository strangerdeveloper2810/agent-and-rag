package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// collectEmit ghi lại mọi event node phát ra để test xác nhận emit.
func collectEmit() (agent.EmitFunc, *[]agent.Event) {
	events := &[]agent.Event{}
	return func(e agent.Event) { *events = append(*events, e) }, events
}

func TestRecallNode_NoUserMessage(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()

	next, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "system prompt"},
			{Role: provider.RoleAssistant, Content: "chào bạn"},
		},
	}, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want không emit", *events)
	}
}

func TestRecallNode_KeywordCascade(t *testing.T) {
	store := NewStore()
	store.Set("user_name", "Linh")
	store.Set("email", "linh@example.com")

	emit, events := collectEmit()
	next, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tên của tôi là gì? và email của tôi?"},
		},
	}, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %v, want 1 event", *events)
	}
	msg := (*events)[0].Message
	if !strings.Contains(msg, "user_name: Linh") || !strings.Contains(msg, "email: linh@example.com") {
		t.Fatalf("event message = %q, want chứa user_name và email", msg)
	}
}

func TestRecallNode_KeywordEnglish(t *testing.T) {
	store := NewStore()
	store.Set("user_name", "Alex")
	store.Set("user_job", "engineer")

	emit, events := collectEmit()
	_, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "what is my name and my job?"},
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %v, want 1 event", *events)
	}
	msg := (*events)[0].Message
	if !strings.Contains(msg, "user_name: Alex") || !strings.Contains(msg, "user_job: engineer") {
		t.Fatalf("event message = %q, want chứa name và job", msg)
	}
}

func TestRecallNode_FullTextFallback(t *testing.T) {
	store := NewStore()
	store.Set("random_key", "banana smoothie")

	emit, events := collectEmit()
	_, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "banana"},
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*events) != 1 || !strings.Contains((*events)[0].Message, "random_key: banana smoothie") {
		t.Fatalf("events = %v, want chứa random_key", *events)
	}
}

func TestRecallNode_SemanticSearch(t *testing.T) {
	store := NewStore()
	store.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{
		"pizza margherita": {1, 0},
		"món ăn Ý":         {1, 0},
	}})
	store.Set("food", "pizza margherita")

	// Query không khớp keyword map lẫn substring — chỉ semantic tìm được.
	emit, events := collectEmit()
	_, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "món ăn Ý yêu thích"},
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*events) != 1 || !strings.Contains((*events)[0].Message, "food: pizza margherita") {
		t.Fatalf("events = %v, want chứa food qua semantic search", *events)
	}
}

func TestRecallNode_SemanticErrorFallsBackToKeyword(t *testing.T) {
	store := NewStore()
	store.SetEmbedder(&stubEmbedder{err: errors.New("embed down")})
	store.Set("user_name", "Linh")

	emit, events := collectEmit()
	next, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tên tôi là gì"},
		},
	}, emit)

	// Semantic lỗi chỉ log warn — keyword vẫn trả kết quả.
	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	if len(*events) != 1 || !strings.Contains((*events)[0].Message, "user_name: Linh") {
		t.Fatalf("events = %v, want chứa user_name", *events)
	}
}

func TestRecallNode_SemanticDoesNotOverrideKeyword(t *testing.T) {
	store := NewStore()
	store.SetEmbedder(&stubEmbedder{vectors: map[string][]float64{
		"Linh":        {1, 0},
		"tên là Linh": {1, 0},
	}})
	store.Set("user_name", "Linh")

	// Keyword ("tên") + fulltext ("linh") + semantic đều trả user_name —
	// chỉ 1 lần trong kết quả, không ghi đè.
	emit, events := collectEmit()
	_, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "tên là Linh"},
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*events) != 1 || strings.Count((*events)[0].Message, "user_name") != 1 {
		t.Fatalf("events = %v, want user_name xuất hiện đúng 1 lần", *events)
	}
}

func TestRecallNode_NoResults(t *testing.T) {
	store := NewStore()
	store.Set("user_name", "Linh")

	emit, events := collectEmit()
	next, err := RecallNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "trời hôm nay đẹp nhỉ"},
		},
	}, emit)

	if err != nil || next != agent.NodeModel {
		t.Fatalf("next/err = (%q, %v), want (NodeModel, nil)", next, err)
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want không emit khi không có kết quả", *events)
	}
}

func TestLastUserContent(t *testing.T) {
	tests := []struct {
		name     string
		messages []provider.Message
		want     string
	}{
		{
			name:     "rỗng",
			messages: nil,
			want:     "",
		},
		{
			name: "không có user",
			messages: []provider.Message{
				{Role: provider.RoleSystem, Content: "sys"},
				{Role: provider.RoleAssistant, Content: "hi"},
			},
			want: "",
		},
		{
			name: "lấy user cuối",
			messages: []provider.Message{
				{Role: provider.RoleUser, Content: "câu 1"},
				{Role: provider.RoleAssistant, Content: "trả lời"},
				{Role: provider.RoleUser, Content: "câu 2"},
			},
			want: "câu 2",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastUserContent(&agent.State{Messages: tc.messages}); got != tc.want {
				t.Fatalf("lastUserContent = %q, want %q", got, tc.want)
			}
		})
	}
}
