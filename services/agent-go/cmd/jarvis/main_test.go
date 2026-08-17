package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/provider"
)

// stubRunner phát sẵn event, ghi lại input mỗi lượt (để kiểm tra history).
type stubRunner struct {
	events []agent.Event
	err    error
	inputs []agent.RunInput
}

func (s *stubRunner) Run(_ context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	s.inputs = append(s.inputs, in)
	for _, e := range s.events {
		emit(e)
	}
	return provider.Usage{}, s.err
}

// --- run (dispatch subcommand) ---

func TestRun_NoArgsShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run(nil, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "usage:") {
		t.Errorf("stdout = %q, want hướng dẫn dùng", out.String())
	}
}

func TestRun_Help(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		var out, errOut bytes.Buffer

		if code := run([]string{arg}, &out, &errOut); code != 0 {
			t.Errorf("%s: exit code = %d, want 0", arg, code)
		}
		if !strings.Contains(out.String(), "jarvis ask") {
			t.Errorf("%s: stdout = %q", arg, out.String())
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run([]string{"bay-len-troi"}, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("stderr = %q", errOut.String())
	}
	if !strings.Contains(out.String(), "usage:") {
		t.Error("lệnh lạ vẫn phải in hướng dẫn")
	}
}

func TestRun_AskWithoutQuestion(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run([]string{"ask"}, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), `jarvis ask`) {
		t.Errorf("stderr = %q", errOut.String())
	}
}

// --- askOnce ---

func TestAskOnce_PrintsTextToStdout(t *testing.T) {
	runner := &stubRunner{events: []agent.Event{
		agent.TextEvent("xin "),
		agent.TextEvent("chào"),
		agent.MemoryEvent("đã nhớ"),
	}}

	var out, errOut bytes.Buffer
	if err := askOnce(context.Background(), runner, "hỏi gì đó", &out, &errOut); err != nil {
		t.Fatalf("askOnce: %v", err)
	}

	if !strings.HasPrefix(out.String(), "xin chào") {
		t.Errorf("stdout = %q, want bắt đầu bằng 'xin chào'", out.String())
	}
	// Event phụ đi stderr để stdout chỉ chứa câu trả lời (pipe được).
	if !strings.Contains(errOut.String(), "[memory] đã nhớ") {
		t.Errorf("stderr = %q", errOut.String())
	}
	if len(runner.inputs) != 1 || runner.inputs[0].UserMessage != "hỏi gì đó" {
		t.Errorf("input = %+v", runner.inputs)
	}
	if runner.inputs[0].MaxSteps != 12 {
		t.Errorf("MaxSteps = %d, want 12", runner.inputs[0].MaxSteps)
	}
}

func TestAskOnce_ErrorEventGoesToStderr(t *testing.T) {
	runner := &stubRunner{events: []agent.Event{agent.ErrorEvent("hỏng rồi")}}

	var out, errOut bytes.Buffer
	if err := askOnce(context.Background(), runner, "x", &out, &errOut); err != nil {
		t.Fatalf("askOnce: %v", err)
	}
	if !strings.Contains(errOut.String(), "[error] hỏng rồi") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestAskOnce_PropagatesRunnerError(t *testing.T) {
	runner := &stubRunner{err: errors.New("engine chết")}

	var out, errOut bytes.Buffer
	err := askOnce(context.Background(), runner, "x", &out, &errOut)
	if err == nil {
		t.Fatal("askOnce = nil error, want lỗi từ runner")
	}
}

// --- chatLoop ---

func TestChatLoop_KeepsHistoryAcrossTurns(t *testing.T) {
	runner := &stubRunner{events: []agent.Event{agent.TextEvent("đáp")}}

	var out, errOut bytes.Buffer
	chatLoop(context.Background(), runner, strings.NewReader("câu 1\ncâu 2\n/exit\n"), &out, &errOut)

	if len(runner.inputs) != 2 {
		t.Fatalf("số lượt = %d, want 2", len(runner.inputs))
	}
	// Lượt 1 chưa có history; lượt 2 phải có user + assistant của lượt trước.
	if len(runner.inputs[0].History) != 0 {
		t.Errorf("lượt 1 History = %+v, want rỗng", runner.inputs[0].History)
	}
	if len(runner.inputs[1].History) != 2 {
		t.Fatalf("lượt 2 History = %+v, want 2 message", runner.inputs[1].History)
	}
	if runner.inputs[1].History[0].Content != "câu 1" ||
		runner.inputs[1].History[1].Content != "đáp" {
		t.Errorf("History = %+v", runner.inputs[1].History)
	}
	if !strings.Contains(out.String(), "tam biet!") {
		t.Errorf("stdout = %q, want lời chào tạm biệt", out.String())
	}
}

func TestChatLoop_SkipsBlankLines(t *testing.T) {
	runner := &stubRunner{events: []agent.Event{agent.TextEvent("ok")}}

	var out, errOut bytes.Buffer
	chatLoop(context.Background(), runner, strings.NewReader("\n   \nthật sự hỏi\n/quit\n"), &out, &errOut)

	if len(runner.inputs) != 1 {
		t.Errorf("số lượt = %d, want 1 (bỏ dòng trống)", len(runner.inputs))
	}
}

func TestChatLoop_StopsAtEOF(t *testing.T) {
	runner := &stubRunner{events: []agent.Event{agent.TextEvent("ok")}}

	var out, errOut bytes.Buffer
	chatLoop(context.Background(), runner, strings.NewReader("một câu"), &out, &errOut)

	if len(runner.inputs) != 1 {
		t.Errorf("số lượt = %d, want 1", len(runner.inputs))
	}
}

// Lỗi engine không được làm sập REPL, và lượt lỗi không vào history.
func TestChatLoop_ContinuesAfterError(t *testing.T) {
	runner := &stubRunner{err: errors.New("engine chết")}

	var out, errOut bytes.Buffer
	chatLoop(context.Background(), runner, strings.NewReader("câu 1\ncâu 2\n"), &out, &errOut)

	if len(runner.inputs) != 2 {
		t.Fatalf("số lượt = %d, want 2 (REPL phải chạy tiếp sau lỗi)", len(runner.inputs))
	}
	if len(runner.inputs[1].History) != 0 {
		t.Errorf("lượt lỗi không được vào history: %+v", runner.inputs[1].History)
	}
	if !strings.Contains(errOut.String(), "engine chết") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

// Trả lời rỗng thì chỉ lưu message user, không lưu assistant rỗng.
func TestChatLoop_EmptyResponseNotStored(t *testing.T) {
	runner := &stubRunner{}

	var out, errOut bytes.Buffer
	chatLoop(context.Background(), runner, strings.NewReader("câu 1\ncâu 2\n"), &out, &errOut)

	if len(runner.inputs) != 2 {
		t.Fatalf("số lượt = %d, want 2", len(runner.inputs))
	}
	if len(runner.inputs[1].History) != 1 {
		t.Errorf("History = %+v, want chỉ 1 message user", runner.inputs[1].History)
	}
}

func TestUsageText(t *testing.T) {
	for _, want := range []string{"jarvis serve", "jarvis ask", "jarvis chat", "jarvis help"} {
		if !strings.Contains(usageText, want) {
			t.Errorf("usageText thiếu %q", want)
		}
	}
}
