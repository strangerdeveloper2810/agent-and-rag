package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// shellTool executes shell commands via os/exec.
// Kind=KindDestructive because it can modify the system.
type shellTool struct {
	allowedCommands map[string]bool // empty = allow all
}

const shellMaxOutput = 8_000

// NewShellTool creates a shell execution tool.
// allowedCommands: list of allowed command names. Empty or nil = allow all.
func NewShellTool(allowedCommands []string) Tool {
	ac := make(map[string]bool, len(allowedCommands))
	for _, cmd := range allowedCommands {
		ac[cmd] = true
	}
	return &shellTool{allowedCommands: ac}
}

func (t *shellTool) Name() string { return "shell.exec" }

func (t *shellTool) Description() string {
	return "Thực thi shell command qua os/exec. Timeout 30s, output tối đa 8000 ký tự."
}

func (t *shellTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"command":{"type":"string","description":"shell command to execute"},
			"args":{"type":"array","items":{"type":"string"},"description":"command arguments (optional)"}
		},
		"required":["command"],
		"additionalProperties":false
	}`)
}

func (t *shellTool) Kind() Kind { return KindDestructive }

func (t *shellTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("shell.exec: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return Result{}, fmt.Errorf("shell.exec: command is required")
	}

	// Check allowed commands
	if len(t.allowedCommands) > 0 && !t.allowedCommands[args.Command] {
		return Result{}, fmt.Errorf("shell.exec: command %q is not allowed", args.Command)
	}

	// Timeout 30s
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, args.Command, args.Args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Propagate context errors (deadline exceeded, cancelled)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, fmt.Errorf("shell.exec: %w", ctx.Err())
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "[stderr]\n" + stderr.String()
	}

	// Truncate
	if len(output) > shellMaxOutput {
		output = output[:shellMaxOutput] + "\n... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"command":  args.Command,
		"args":     args.Args,
		"exitCode": exitCode(err),
		"output":   output,
	})
	return Result{Content: string(out)}, nil
}

// exitCode extracts exit code from an exec error, returns -1 if not an ExitError.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
