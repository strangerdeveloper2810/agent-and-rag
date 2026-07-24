package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// VersionTool checks latest version of npm packages and GitHub releases.
// Uses official APIs (registry.npmjs.org, api.github.com) — no API key needed.
type VersionTool struct {
	httpClient *http.Client
}

// NewVersionTool creates a version checker tool.
func NewVersionTool() *VersionTool {
	return &VersionTool{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *VersionTool) Name() string { return "version" }
func (t *VersionTool) Kind() Kind   { return KindRead }
func (t *VersionTool) Description() string {
	return "Check latest version of npm packages or GitHub releases. Args: {source: 'npm'|'github', package: 'react', owner: 'facebook', repo: 'react'}"
}
func (t *VersionTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"source": {"type": "string", "enum": ["npm", "github"], "description": "Source to check"},
			"package": {"type": "string", "description": "npm package name (for source=npm)"},
			"owner": {"type": "string", "description": "GitHub owner (for source=github)"},
			"repo": {"type": "string", "description": "GitHub repo name (for source=github)"}
		},
		"required": ["source"]
	}`)
}

func (t *VersionTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	var params struct {
		Source  string `json:"source"`
		Package string `json:"package"`
		Owner   string `json:"owner"`
		Repo    string `json:"repo"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return Result{}, fmt.Errorf("version: parse args: %w", err)
	}

	switch params.Source {
	case "npm":
		return t.checkNPM(ctx, params.Package)
	case "github":
		return t.checkGitHub(ctx, params.Owner, params.Repo)
	default:
		return Result{}, fmt.Errorf("version: unknown source %q", params.Source)
	}
}

func (t *VersionTool) checkNPM(ctx context.Context, pkg string) (Result, error) {
	url := "https://registry.npmjs.org/" + pkg + "/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("npm: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		Version string `json:"version"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Result{}, fmt.Errorf("npm: parse: %w", err)
	}

	return Result{
		Content: fmt.Sprintf(`{"name":"%s","latest":"%s","source":"npm"}`, data.Name, data.Version),
	}, nil
}

func (t *VersionTool) checkGitHub(ctx context.Context, owner, repo string) (Result, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Result{}, fmt.Errorf("github: parse: %w", err)
	}

	return Result{
		Content: fmt.Sprintf(`{"repo":"%s/%s","latest":"%s","name":"%s","source":"github"}`, owner, repo, data.TagName, data.Name),
	}, nil
}
