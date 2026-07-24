package eval

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeRunner implements AgentRunner with canned responses.
type fakeRunner struct {
	responses map[string]string
	err       error
}

func (f *fakeRunner) Run(_ context.Context, input string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if resp, ok := f.responses[input]; ok {
		return resp, nil
	}
	return "fallback", nil
}

func TestEvalExactMatch(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"hello": "hi there",
	}}
	harness := NewEvalHarness(runner, nil)

	result := harness.RunEval(context.Background(), EvalCase{
		Name:     "greeting",
		Input:    "hello",
		Expected: "hi there",
		Mode:     MatchExact,
	})

	if !result.Passed {
		t.Errorf("exact match should pass, got reason: %s", result.Reason)
	}
	if result.Actual != "hi there" {
		t.Errorf("Actual = %q, want %q", result.Actual, "hi there")
	}
	if result.Duration <= 0 {
		t.Error("duration should be > 0")
	}
}

func TestEvalExactMatch_Fails(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"q": "wrong answer",
	}}
	harness := NewEvalHarness(runner, nil)

	result := harness.RunEval(context.Background(), EvalCase{
		Name:     "test",
		Input:    "q",
		Expected: "right answer",
		Mode:     MatchExact,
	})

	if result.Passed {
		t.Error("exact match should fail for wrong answer")
	}
	if result.Actual != "wrong answer" {
		t.Errorf("Actual = %q", result.Actual)
	}
}

func TestEvalContains(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"weather": "The weather today is sunny with a high of 25C",
	}}
	harness := NewEvalHarness(runner, nil)

	result := harness.RunEval(context.Background(), EvalCase{
		Name:     "weather",
		Input:    "weather",
		Expected: "sunny",
		Mode:     MatchContains,
	})

	if !result.Passed {
		t.Errorf("contains should pass: %s", result.Reason)
	}

	// Test failure
	result = harness.RunEval(context.Background(), EvalCase{
		Name:     "weather",
		Input:    "weather",
		Expected: "snowing",
		Mode:     MatchContains,
	})
	if result.Passed {
		t.Error("contains should fail for missing substring")
	}
}

func TestEvalRegex(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"date": "Today is 2026-07-24",
	}}
	harness := NewEvalHarness(runner, nil)

	result := harness.RunEval(context.Background(), EvalCase{
		Name:     "date",
		Input:    "date",
		Expected: `\d{4}-\d{2}-\d{2}`,
		Mode:     MatchRegex,
	})

	if !result.Passed {
		t.Errorf("regex should pass: %s", result.Reason)
	}

	// Test invalid regex
	result = harness.RunEval(context.Background(), EvalCase{
		Name:     "bad-regex",
		Input:    "date",
		Expected: `[invalid`,
		Mode:     MatchRegex,
	})
	if result.Passed {
		t.Error("invalid regex should fail")
	}
	if !strings.Contains(result.Reason, "invalid regex") {
		t.Errorf("reason should mention invalid regex: %s", result.Reason)
	}
}

func TestEvalAgentError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("agent crash")}
	harness := NewEvalHarness(runner, nil)

	result := harness.RunEval(context.Background(), EvalCase{
		Name:     "error-case",
		Input:    "anything",
		Expected: "anything",
		Mode:     MatchExact,
	})

	if result.Passed {
		t.Error("should not pass when agent errors")
	}
	if result.Error != "agent crash" {
		t.Errorf("Error = %q, want %q", result.Error, "agent crash")
	}
}

func TestRunAll(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"a": "response A",
		"b": "response B",
		"c": "wrong C",
	}}
	harness := NewEvalHarness(runner, nil)

	cases := []EvalCase{
		{Name: "case-a", Input: "a", Expected: "response A", Mode: MatchExact},
		{Name: "case-b", Input: "b", Expected: "response B", Mode: MatchExact},
		{Name: "case-c", Input: "c", Expected: "right C", Mode: MatchExact},
	}

	report := harness.RunAll(context.Background(), cases)

	if report.Total != 3 {
		t.Errorf("Total = %d, want 3", report.Total)
	}
	if report.Passed != 2 {
		t.Errorf("Passed = %d, want 2", report.Passed)
	}
	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}
	if report.Errored != 0 {
		t.Errorf("Errored = %d, want 0", report.Errored)
	}
	if report.Duration <= 0 {
		t.Error("duration should be > 0")
	}
	if len(report.Results) != 3 {
		t.Errorf("Results len = %d, want 3", len(report.Results))
	}
}

func TestRunAllParallel(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"1": "one",
		"2": "two",
		"3": "three",
	}}
	harness := NewEvalHarness(runner, nil)

	cases := make([]EvalCase, 10)
	for i := range cases {
		key := string(rune('1' + (i % 3)))
		cases[i] = EvalCase{
			Name:     "p",
			Input:    key,
			Expected: runner.responses[key],
			Mode:     MatchExact,
		}
	}

	report := harness.RunAllParallel(context.Background(), cases)

	if report.Total != 10 {
		t.Errorf("Total = %d, want 10", report.Total)
	}
	if report.Passed != 10 {
		t.Errorf("Passed = %d, want 10", report.Passed)
	}
}

func TestRunAll_ContextCancel(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{"x": "y"}}
	harness := NewEvalHarness(runner, nil)

	cases := []EvalCase{
		{Name: "c1", Input: "x", Expected: "y", Mode: MatchExact},
		{Name: "c2", Input: "x", Expected: "y", Mode: MatchExact},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure ctx is expired

	report := harness.RunAll(ctx, cases)
	// With expired context, cases may or may not run depending on timing
	t.Logf("Passed: %d, Failed: %d, Errored: %d", report.Passed, report.Failed, report.Errored)
}

func TestFilterByTag(t *testing.T) {
	cases := []EvalCase{
		{Name: "c1", Tags: []string{"critical"}},
		{Name: "c2", Tags: []string{"nice-to-have"}},
		{Name: "c3", Tags: []string{"critical", "regression"}},
		{Name: "c4", Tags: []string{}},
	}

	critical := FilterByTag(cases, "critical")
	if len(critical) != 2 {
		t.Errorf("critical filter len = %d, want 2", len(critical))
	}
	if critical[0].Name != "c1" || critical[1].Name != "c3" {
		t.Errorf("filtered names = %v, want [c1 c3]", []string{critical[0].Name, critical[1].Name})
	}

	empty := FilterByTag(cases, "nonexistent")
	if len(empty) != 0 {
		t.Errorf("nonexistent filter len = %d, want 0", len(empty))
	}
}

func TestMatchModeString(t *testing.T) {
	tests := []struct {
		m    MatchMode
		want string
	}{
		{MatchExact, "exact"},
		{MatchContains, "contains"},
		{MatchRegex, "regex"},
		{MatchSemantic, "semantic"},
		{MatchMode(99), "unknown"},
	}
	for _, tt := range tests {
		got := tt.m.String()
		if got != tt.want {
			t.Errorf("MatchMode(%d).String() = %q, want %q", tt.m, got, tt.want)
		}
	}
}

func TestRunAll_PassRate(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{
		"1": "A", "2": "B", "3": "wrong",
	}}
	harness := NewEvalHarness(runner, nil)

	cases := []EvalCase{
		{Name: "t1", Input: "1", Expected: "A", Mode: MatchExact},
		{Name: "t2", Input: "2", Expected: "B", Mode: MatchExact},
		{Name: "t3", Input: "3", Expected: "C", Mode: MatchExact},
	}

	report := harness.RunAll(context.Background(), cases)

	expectedRate := 2.0 / 3.0 * 100.0
	diff := report.PassRate - expectedRate
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.01 {
		t.Errorf("PassRate = %f, want ~%f", report.PassRate, expectedRate)
	}
}

func TestSemanticMode_NoJudge(t *testing.T) {
	runner := &fakeRunner{responses: map[string]string{"q": "answer"}}
	harness := NewEvalHarness(runner, nil) // no judge

	result := harness.RunEval(context.Background(), EvalCase{
		Name:     "sem-no-judge",
		Input:    "q",
		Expected: "answer",
		Mode:     MatchSemantic,
	})

	if result.Passed {
		t.Error("semantic without judge should fail")
	}
	if !strings.Contains(result.Reason, "no LLM judge") {
		t.Errorf("reason = %q", result.Reason)
	}
}
