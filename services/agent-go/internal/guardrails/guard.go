package guardrails

import (
	"fmt"
	"log/slog"

	"github.com/ai-agent-tut/agent-go/internal/tools"
)

// NeedConfirmationError is returned when a destructive tool requires HITL
// confirmation before execution.
type NeedConfirmationError struct {
	Tool string
}

func (e *NeedConfirmationError) Error() string {
	return fmt.Sprintf("guardrails: destructive tool %q requires user confirmation", e.Tool)
}

// CheckTool validates whether a tool can be executed based on its Kind.
//
// Rules:
//   - KindRead  → allowed (read-only operations are safe)
//   - KindWrite → allowed + info log (mutating but not destructive)
//   - KindDestructive → returns NeedConfirmationError (requires HITL)
func CheckTool(t tools.Tool) error {
	switch t.Kind() {
	case tools.KindRead:
		return nil
	case tools.KindWrite:
		slog.Info("guardrails: write tool allowed", "tool", t.Name())
		return nil
	case tools.KindDestructive:
		return &NeedConfirmationError{Tool: t.Name()}
	default:
		return fmt.Errorf("guardrails: unknown tool kind %d for tool %q", t.Kind(), t.Name())
	}
}
