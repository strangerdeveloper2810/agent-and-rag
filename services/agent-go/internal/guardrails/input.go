// Package guardrails provides safety checks for the agent runtime:
// circuit breaker, write-tool confirmation, and input validation.
package guardrails

import (
	"errors"
	"regexp"
)

// Common prompt injection patterns targeting LLM instruction override.
// Does NOT block legitimate SQL/code questions — only explicit hijack attempts.
var promptInjectionPattern = regexp.MustCompile(
	`(?i)(?:` +
		`\bignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|messages?|conversations?|text|context)\b|` +
		`\byou\s+are\s+now\s+(DAN|jailbreak|a\s+different)\b|` +
		`\bforget\s+(everything|all\s+(previous|prior)\s+instructions?)\b|` +
		`\bsystem\s*:\s*(override|prompt|instruction|you\s+are)\b|` +
		`\bprint\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\b|` +
		`\breveal\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\b|` +
		`\bwhat\s+(is|are)\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\??\s*$|` +
		`\bnew\s+instructions?\s*:|` +
		`\bfrom\s+now\s+on\s+you\s+are\b` +
		`)`,
)

// XSS/script injection patterns.
var xssPattern = regexp.MustCompile(
	`(?i)<script[\s>]|</script>|javascript\s*:|on\w+\s*=\s*["']|` +
		`onerror\s*=|onload\s*=|onclick\s*=|eval\s*\(|` +
		`document\.cookie|document\.write|<iframe|<object|<embed`,
)

var (
	ErrPromptInjection = errors.New("input contains disallowed prompt injection pattern")
	ErrXSSInjection    = errors.New("input contains disallowed XSS/script pattern")
	ErrInputTooLong    = errors.New("input exceeds maximum allowed length")
)

const MaxInputLength = 4000

// ValidateUserInput checks a user message for prompt injection, XSS, and
// length limits. Returns nil if the input is safe, or an error describing
// the violation.
func ValidateUserInput(input string) error {
	if len(input) > MaxInputLength {
		return ErrInputTooLong
	}
	if promptInjectionPattern.MatchString(input) {
		return ErrPromptInjection
	}
	if xssPattern.MatchString(input) {
		return ErrXSSInjection
	}
	return nil
}
