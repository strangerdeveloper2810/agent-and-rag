package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai-agent-tut/agent-go/internal/agent"
	"github.com/ai-agent-tut/agent-go/internal/eval"
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

func TestRun_EvalWithoutPath(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run([]string{"eval"}, &out, &errOut); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "jarvis eval") {
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
	for _, want := range []string{"jarvis serve", "jarvis ask", "jarvis chat", "jarvis eval", "jarvis help"} {
		if !strings.Contains(usageText, want) {
			t.Errorf("usageText thiếu %q", want)
		}
	}
}

// --- eval ---

// echoRunner phát 1 câu trả lời cố định (không phụ thuộc input) cho mỗi
// lượt Run, để test runEvalCases xác định được pass/fail/error mà không cần
// LLM thật.
type echoRunner struct {
	reply string
	err   error
	calls []agent.RunInput
}

func (r *echoRunner) Run(_ context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	r.calls = append(r.calls, in)
	if r.err != nil {
		return provider.Usage{}, r.err
	}
	emit(agent.TextEvent(r.reply))
	return provider.Usage{}, nil
}

func TestLoadEvalCases_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.json")
	content := `[
		{"name":"c1","input":"hi","expected":"hi","mode":0},
		{"name":"c2","input":"world","expected":"wor","mode":1,"tags":["smoke"]}
	]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cases, err := loadEvalCases(path)
	if err != nil {
		t.Fatalf("loadEvalCases: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}
	if cases[0].Name != "c1" || cases[1].Name != "c2" {
		t.Errorf("cases = %+v", cases)
	}
	if cases[1].Mode != eval.MatchContains {
		t.Errorf("cases[1].Mode = %v, want MatchContains", cases[1].Mode)
	}
	if len(cases[1].Tags) != 1 || cases[1].Tags[0] != "smoke" {
		t.Errorf("cases[1].Tags = %v", cases[1].Tags)
	}
}

func TestLoadEvalCases_MissingFile(t *testing.T) {
	if _, err := loadEvalCases(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadEvalCases_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := loadEvalCases(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEvalRunnerAdapter_CollectsTextEvents(t *testing.T) {
	runner := &stubRunner{events: []agent.Event{
		agent.TextEvent("xin "),
		agent.TextEvent("chào"),
		agent.MemoryEvent("bo qua"),
	}}
	adapter := &evalRunnerAdapter{runner: runner}

	got, err := adapter.Run(context.Background(), "hoi gi do")
	if err != nil {
		t.Fatalf("adapter.Run: %v", err)
	}
	if got != "xin chào" {
		t.Errorf("got = %q, want %q", got, "xin chào")
	}
	if len(runner.inputs) != 1 || runner.inputs[0].UserMessage != "hoi gi do" {
		t.Errorf("inputs = %+v", runner.inputs)
	}
	if runner.inputs[0].MaxSteps != 12 {
		t.Errorf("MaxSteps = %d, want 12", runner.inputs[0].MaxSteps)
	}
}

func TestEvalRunnerAdapter_PropagatesError(t *testing.T) {
	runner := &stubRunner{err: errors.New("engine chet")}
	adapter := &evalRunnerAdapter{runner: runner}

	if _, err := adapter.Run(context.Background(), "hoi"); err == nil {
		t.Fatal("expected error from adapter.Run")
	}
}

func TestRunEvalCases_AllPass(t *testing.T) {
	runner := &echoRunner{reply: "hello world"}
	cases := []eval.EvalCase{
		{Name: "c1", Input: "hi", Expected: "hello", Mode: eval.MatchContains},
		{Name: "c2", Input: "hi", Expected: "world", Mode: eval.MatchContains},
	}

	var out bytes.Buffer
	code := runEvalCases(context.Background(), runner, cases, &out)
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if strings.Count(out.String(), "[PASS]") != 2 {
		t.Errorf("output = %q, want 2 PASS entries", out.String())
	}
	if !strings.Contains(out.String(), "2/2 passed") {
		t.Errorf("output = %q, want summary line", out.String())
	}
}

func TestRunEvalCases_SomeFail(t *testing.T) {
	runner := &echoRunner{reply: "hello world"}
	cases := []eval.EvalCase{
		{Name: "ok", Input: "hi", Expected: "hello", Mode: eval.MatchContains},
		{Name: "boom", Input: "hi", Expected: "nope", Mode: eval.MatchContains},
	}

	var out bytes.Buffer
	code := runEvalCases(context.Background(), runner, cases, &out)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "[FAIL] boom") {
		t.Errorf("output = %q, want FAIL entry for 'boom'", out.String())
	}
	if !strings.Contains(out.String(), "1/2 passed") {
		t.Errorf("output = %q, want summary line", out.String())
	}
}

func TestRunEvalCases_RunnerError(t *testing.T) {
	runner := &echoRunner{err: errors.New("engine chet")}
	cases := []eval.EvalCase{
		{Name: "c1", Input: "hi", Expected: "x", Mode: eval.MatchContains},
	}

	var out bytes.Buffer
	code := runEvalCases(context.Background(), runner, cases, &out)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "[ERROR] c1") {
		t.Errorf("output = %q, want ERROR entry", out.String())
	}
	if !strings.Contains(out.String(), "engine chet") {
		t.Errorf("output = %q, want error message included", out.String())
	}
}
