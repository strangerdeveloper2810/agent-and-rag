package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type translateTool struct {
	httpClient *http.Client
}

func NewTranslateTool(client *http.Client) Tool {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &translateTool{httpClient: client}
}

func (t *translateTool) Name() string { return "translate" }

func (t *translateTool) Description() string {
	return "Translate text between languages using libretranslate (free, no API key). Specify source language (auto for auto-detect) and target language."
}

func (t *translateTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"text":{"type":"string","description":"text to translate"},
			"source":{"type":"string","description":"source language code (e.g. en, vi, fr) or 'auto'"},
			"target":{"type":"string","description":"target language code (e.g. en, vi, fr)"}
		},
		"required":["text","target"],
		"additionalProperties":false
	}`)
}

func (t *translateTool) Kind() Kind { return KindRead }

func (t *translateTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
	var args struct {
		Text   string `json:"text"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return Result{}, fmt.Errorf("translate: invalid args: %w", err)
	}
	if strings.TrimSpace(args.Text) == "" {
		return Result{}, fmt.Errorf("translate: text is required")
	}
	if args.Target == "" {
		return Result{}, fmt.Errorf("translate: target language is required")
	}
	if args.Source == "" {
		args.Source = "auto"
	}

	if len(args.Text) > 5000 {
		args.Text = args.Text[:5000]
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	reqBody := map[string]string{
		"q":      args.Text,
		"source": args.Source,
		"target": args.Target,
		"format": "text",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://libretranslate.com/translate", bytes.NewReader(bodyBytes))
	if err != nil {
		return Result{}, fmt.Errorf("translate: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agent-go/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("translate: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<19)) // max 512KB
	if err != nil {
		return Result{}, fmt.Errorf("translate: read response: %w", err)
	}

	var result struct {
		TranslatedText string `json:"translatedText"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return Result{}, fmt.Errorf("translate: parse response: %w", err)
	}

	translated := result.TranslatedText
	if len(translated) > 10000 {
		translated = translated[:10000] + "... [truncated]"
	}

	out, _ := json.Marshal(map[string]any{
		"source":     args.Source,
		"target":     args.Target,
		"original":   args.Text,
		"translated": translated,
	})
	return Result{Content: string(out)}, nil
}
