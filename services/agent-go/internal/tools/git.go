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

// gitTool executes read-only git commands.
// Kind=KindRead. Only allows: log, diff, status, branch, show.
type gitTool struct {
	workDir string // working directory for git commands
}

const gitMaxOutput = 8_000

// readOnlyGitOps lists all allowed read-only git subcommands.
var readOnlyGitOps = map[string]bool{
	"log":    true,
	"diff":   true,
	"status": true,
	"branch": true,
	"show":   true,
}

// NewGitTool creates a git tool for read-only operations.
// workDir: directory to run git commands in (empty = current directory).
func NewGitTool(workDir string) Tool {
	return &gitTool{workDir: workDir}
}

func (t *gitTool) Name() string { return "git" }

func (t *gitTool) Description() string {
	return "Thực thi read-only git commands: log, diff, status, branch, show. Output tối đa 8000 ký tự."
}

func (t *gitTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"operation":{"type":"string","enum":["log","diff","status","branch","show"],"description":"git subcommand to run"},
			"args":{"type":"array","items":{"type":"string"},"description":"additional arguments for the git subcommand (optional)"}
		},
		"required":["operation"],
		"additionalProperties":false
	}`)
}

func (t *gitTool) Kind() Kind { return KindRead }

func (t *gitTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Operation string   `json:"operation"`
		Args      []string `json:"args,omitempty"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("git: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Operation) == "" {
		return Result{}, fmt.Errorf("git: operation is required")
	}
	if !readOnlyGitOps[args.Operation] {
		return Result{}, fmt.Errorf("git: operation %q is not allowed (read-only: log, diff, status, branch, show)", args.Operation)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmdArgs := append([]string{args.Operation}, args.Args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if t.workDir != "" {
		cmd.Dir = t.workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += "[stderr]\n" + stderr.String()
	}

	if len(output) > gitMaxOutput {
		output = output[:gitMaxOutput] + "\n... [truncated]"
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	out, _ := json.Marshal(map[string]any{
		"operation": args.Operation,
		"args":      args.Args,
		"workDir":   t.workDir,
		"exitCode":  exitCode,
		"output":    output,
	})
	return Result{Content: string(out)}, nil
}
