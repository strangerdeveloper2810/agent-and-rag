// Package eval cung cấp EvalHarness để test agent responses một cách có hệ thống.
// Hỗ trợ exact match, contains, regex, và semantic eval (qua LLM judge).
package eval

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// MatchMode xác định cách so khớp kết quả.
type MatchMode int

const (
	MatchExact   MatchMode = iota // so khớp chính xác
	MatchContains                 // chứa substring
	MatchRegex                    // khớp regex
	MatchSemantic                 // LLM judge (cần Judge interface)
)

func (m MatchMode) String() string {
	switch m {
	case MatchExact:
		return "exact"
	case MatchContains:
		return "contains"
	case MatchRegex:
		return "regex"
	case MatchSemantic:
		return "semantic"
	default:
		return "unknown"
	}
}

// Judge là interface cho LLM-as-judge (semantic eval).
type Judge interface {
	Evaluate(ctx context.Context, expected, actual string) (bool, string, error)
}

// EvalCase là một test case đơn lẻ.
type EvalCase struct {
	Name     string    `json:"name"`
	Input    string    `json:"input"`
	Expected string    `json:"expected"`
	Mode     MatchMode `json:"mode"`
	Tags     []string  `json:"tags,omitempty"`
}

// EvalResult là kết quả chạy một EvalCase.
type EvalResult struct {
	Case     EvalCase     `json:"case"`
	Passed   bool         `json:"passed"`
	Actual   string       `json:"actual"`
	Reason   string       `json:"reason,omitempty"`
	Duration time.Duration `json:"duration"`
	Error    string       `json:"error,omitempty"`
}

// EvalReport tổng hợp kết quả chạy toàn bộ test cases.
type EvalReport struct {
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Errored   int           `json:"errored"`
	Duration  time.Duration `json:"duration"`
	Results   []EvalResult  `json:"results"`
	PassRate  float64       `json:"pass_rate"`
}

// AgentRunner là interface agent cần test.
type AgentRunner interface {
	Run(ctx context.Context, input string) (string, error)
}

// EvalHarness chạy EvalCase qua AgentRunner và tổng hợp kết quả.
type EvalHarness struct {
	runner AgentRunner
	judge  Judge // nil nếu không dùng semantic mode
}

// NewEvalHarness tạo harness với agent runner và optional judge.
func NewEvalHarness(runner AgentRunner, judge Judge) *EvalHarness {
	return &EvalHarness{runner: runner, judge: judge}
}

// RunEval chạy một EvalCase đơn lẻ.
func (h *EvalHarness) RunEval(ctx context.Context, c EvalCase) EvalResult {
	start := time.Now()

	actual, err := h.runner.Run(ctx, c.Input)
	if err != nil {
		return EvalResult{
			Case:     c,
			Passed:   false,
			Duration: time.Since(start),
			Error:    err.Error(),
		}
	}

	passed, reason := h.evaluate(c, actual)

	return EvalResult{
		Case:     c,
		Passed:   passed,
		Actual:   actual,
		Reason:   reason,
		Duration: time.Since(start),
	}
}

// RunAll chạy tất cả EvalCase và trả về báo cáo tổng hợp.
// Chạy tuần tự (không song song) để dễ debug và tránh rate-limit LLM.
func (h *EvalHarness) RunAll(ctx context.Context, cases []EvalCase) EvalReport {
	results := make([]EvalResult, len(cases))

	totalStart := time.Now()
	passed, failed, errored := 0, 0, 0

	for i, c := range cases {
		if i > 0 {
			// Small delay between cases to be kind to LLM APIs
			select {
			case <-ctx.Done():
				// Partial report
				break
			default:
			}
		}
		r := h.RunEval(ctx, c)
		results[i] = r

		if r.Error != "" {
			errored++
		} else if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	passRate := 0.0
	total := passed + failed + errored
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100.0
	}

	return EvalReport{
		Total:    total,
		Passed:   passed,
		Failed:   failed,
		Errored:  errored,
		Duration: time.Since(totalStart),
		Results:  results,
		PassRate: passRate,
	}
}

// RunAllParallel chạy tất cả EvalCase song song (dùng cho eval không gọi LLM).
func (h *EvalHarness) RunAllParallel(ctx context.Context, cases []EvalCase) EvalReport {
	results := make([]EvalResult, len(cases))
	var wg sync.WaitGroup

	totalStart := time.Now()

	for i, c := range cases {
		wg.Add(1)
		go func(idx int, ec EvalCase) {
			defer wg.Done()
			results[idx] = h.RunEval(ctx, ec)
		}(i, c)
	}

	wg.Wait()

	passed, failed, errored := 0, 0, 0
	for _, r := range results {
		if r.Error != "" {
			errored++
		} else if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	total := passed + failed + errored
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100.0
	}

	return EvalReport{
		Total:    total,
		Passed:   passed,
		Failed:   failed,
		Errored:  errored,
		Duration: time.Since(totalStart),
		Results:  results,
		PassRate: passRate,
	}
}

// evaluate so khớp expected vs actual theo MatchMode.
func (h *EvalHarness) evaluate(c EvalCase, actual string) (bool, string) {
	switch c.Mode {
	case MatchExact:
		ok := c.Expected == actual
		if !ok {
			return false, fmt.Sprintf("exact match failed: expected=%q, actual=%q", c.Expected, actual)
		}
		return true, "exact match"

	case MatchContains:
		ok := strings.Contains(actual, c.Expected)
		if !ok {
			return false, fmt.Sprintf("does not contain %q", c.Expected)
		}
		return true, fmt.Sprintf("contains %q", c.Expected)

	case MatchRegex:
		re, err := regexp.Compile(c.Expected)
		if err != nil {
			return false, fmt.Sprintf("invalid regex: %s", err)
		}
		if !re.MatchString(actual) {
			return false, fmt.Sprintf("regex /%s/ does not match %q", c.Expected, actual)
		}
		return true, fmt.Sprintf("regex /%s/ matched", c.Expected)

	case MatchSemantic:
		if h.judge == nil {
			return false, "no LLM judge configured for semantic evaluation"
		}
		ok, reason, err := h.judge.Evaluate(context.Background(), c.Expected, actual)
		if err != nil {
			return false, fmt.Sprintf("judge error: %s", err)
		}
		return ok, reason

	default:
		return false, fmt.Sprintf("unknown MatchMode: %d", c.Mode)
	}
}

// FilterByTag lọc EvalCase theo tag.
func FilterByTag(cases []EvalCase, tag string) []EvalCase {
	var filtered []EvalCase
	for _, c := range cases {
		for _, t := range c.Tags {
			if t == tag {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered
}
