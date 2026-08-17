package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

func userMsg(content string) provider.Message {
	return provider.Message{Role: provider.RoleUser, Content: content}
}

func TestExtractNode_Patterns(t *testing.T) {
	tests := []struct {
		name  string
		msg   provider.Message
		key   string
		value string
	}{
		{"tôi tên là", userMsg("tôi tên là Linh"), "user_name", "Linh"},
		{"tôi là", userMsg("tôi là Nam"), "user_name", "Nam"},
		{"gọi tôi là", userMsg("gọi tôi là Alex"), "user_name", "Alex"},
		{"thích", userMsg("tôi thích trà đào"), "like", "trà đào"},
		{"rất thích", userMsg("tôi rất thích cà phê"), "like", "cà phê"},
		{"cực thích", userMsg("tôi cực thích trà sữa"), "like", "trà sữa"},
		{"siêu thích", userMsg("tôi siêu thích matcha"), "like", "matcha"},
		{"không thích", userMsg("tôi không thích hành tây"), "dislike", "hành tây"},
		{"ghét", userMsg("tôi ghét muộn giờ"), "dislike", "muộn giờ"},
		{"không ưa", userMsg("tôi không ưa ồn ào"), "dislike", "ồn ào"},
		{"nhớ", userMsg("nhớ là mai họp lúc 9h"), "fact", "mai họp lúc 9h"},
		{"nhớ rằng", userMsg("nhớ rằng chị Lan thích hoa hồng"), "fact", "chị Lan thích hoa hồng"},
		{"nhớ giúp tôi", userMsg("nhớ giúp tôi mua sữa"), "fact", "mua sữa"},
		{"hãy nhớ", userMsg("hãy nhớ sinh nhật em 20/3"), "fact", "sinh nhật em 20/3"},
		{"ở", userMsg("tôi ở Hà Nội"), "user_location", "Hà Nội"},
		{"đang ở", userMsg("tôi đang ở Đà Nẵng"), "user_location", "Đà Nẵng"},
		{"sống ở", userMsg("tôi sống ở Huế"), "user_location", "Huế"},
		{"làm việc tại", userMsg("tôi làm việc tại FPT"), "user_job", "tại FPT"},
		{"làm ở", userMsg("tôi làm ở Google"), "user_job", "Google"},
		{"là engineer", userMsg("tôi là software engineer"), "user_job", "software engineer"},
		{"là sinh viên", userMsg("tôi là sinh viên"), "user_job", "sinh viên"},
		{"muốn", userMsg("tôi muốn đi Đà Lạt"), "want", "đi Đà Lạt"},
		{"cần", userMsg("tôi cần học Go"), "need", "học Go"},
		{"email", userMsg("địa chỉ email của tôi là a@b.com"), "email", "là a@b.com"},
		{"email không tiền tố", userMsg("địa chỉ email a@b.com"), "email", "a@b.com"},
		{"phone", userMsg("số điện thoại của tôi là 0123456789"), "phone", "là 0123456789"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := NewStore()
			emit, _ := collectEmit()
			next, err := ExtractNode(store)(context.Background(), &agent.State{
				Messages: []provider.Message{tc.msg},
			}, emit)

			if err != nil || next != agent.NodeEnd {
				t.Fatalf("next/err = (%q, %v), want (NodeEnd, nil)", next, err)
			}
			v, ok := store.Get(tc.key)
			if !ok {
				t.Fatalf("store không có key %q sau extract; có %d mục", tc.key, store.Len())
			}
			if v != tc.value {
				t.Fatalf("store[%q] = %q, want %q", tc.key, v, tc.value)
			}
		})
	}
}

func TestExtractNode_EmitsEvents(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()
	_, err := ExtractNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			userMsg("tôi thích trà"),
			userMsg("tôi ở Hà Nội"),
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*events) != 2 {
		t.Fatalf("events = %v, want 2 events", *events)
	}
	for _, e := range *events {
		if e.Type != "memory" || !strings.HasPrefix(e.Message, "extracted: ") {
			t.Fatalf("event = %+v, want memory event dạng extracted:", e)
		}
	}
	if !strings.Contains((*events)[0].Message, "like = trà") {
		t.Fatalf("event 0 = %q, want chứa like = trà", (*events)[0].Message)
	}
}

func TestExtractNode_DedupAcrossMessages(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()
	_, err := ExtractNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			userMsg("tôi thích trà"),
			userMsg("tôi thích cà phê"),
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if v, _ := store.Get("like"); v != "trà" {
		t.Fatalf("like = %q, want trà (chỉ lưu lần đầu)", v)
	}
	likeEvents := 0
	for _, e := range *events {
		if strings.Contains(e.Message, "extracted: like") {
			likeEvents++
		}
	}
	if likeEvents != 1 {
		t.Fatalf("số event like = %d, want 1", likeEvents)
	}
}

func TestExtractNode_SkipsNonConversationalRoles(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()
	_, err := ExtractNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "tôi thích trà"},
			{Role: provider.RoleTool, Content: "gọi tôi là Alex"},
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.Len() != 0 {
		t.Fatalf("store có %d mục, want 0 (bỏ qua system/tool)", store.Len())
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want 0", *events)
	}
}

func TestExtractNode_AssistantExtracted(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()
	_, err := ExtractNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			{Role: provider.RoleAssistant, Content: "tôi thích trà"},
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if v, ok := store.Get("like"); !ok || v != "trà" {
		t.Fatalf("like = (%q, %v), want (trà, true)", v, ok)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %v, want 1", *events)
	}
}

func TestExtractNode_ValueTooLongSkipped(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()
	_, err := ExtractNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{
			userMsg("tôi thích " + strings.Repeat("a", 201)),
		},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, ok := store.Get("like"); ok {
		t.Fatal("value > 200 ký tự nên bị bỏ qua")
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want 0", *events)
	}

	// Biên 200 ký tự → vẫn lưu.
	store2 := NewStore()
	_, err = ExtractNode(store2)(context.Background(), &agent.State{
		Messages: []provider.Message{userMsg("tôi thích " + strings.Repeat("b", 200))},
	}, emit)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if v, ok := store2.Get("like"); !ok || len(v) != 200 {
		t.Fatalf("like = (%q, %v), want 200 ký tự", v, ok)
	}
}

func TestExtractNode_TrimsWhitespace(t *testing.T) {
	store := NewStore()
	emit, _ := collectEmit()
	_, err := ExtractNode(store)(context.Background(), &agent.State{
		Messages: []provider.Message{userMsg("tôi thích   trà đào  ")},
	}, emit)

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if v, _ := store.Get("like"); v != "trà đào" {
		t.Fatalf("like = %q, want trà đào (đã trim)", v)
	}
}

func TestExtractNode_NoMessages(t *testing.T) {
	store := NewStore()
	emit, events := collectEmit()
	next, err := ExtractNode(store)(context.Background(), &agent.State{}, emit)

	if err != nil || next != agent.NodeEnd {
		t.Fatalf("next/err = (%q, %v), want (NodeEnd, nil)", next, err)
	}
	if len(*events) != 0 {
		t.Fatalf("events = %v, want 0", *events)
	}
}
