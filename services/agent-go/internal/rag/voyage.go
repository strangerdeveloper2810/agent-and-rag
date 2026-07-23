// Package rag: retrieval — Voyage embedding (REST) + Atlas vector search.
// (Atlas $vectorSearch thêm ở P5; hiện có Voyage client + hàm thuần.)
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	voyageURL   = "https://api.voyageai.com/v1/embeddings"
	voyageModel = "voyage-3" // 1024 chiều — khớp numDimensions của Atlas vector_index
	batchSize   = 96         // an toàn dưới giới hạn số text/request của Voyage
)

// EmbedRequest là body gửi Voyage.
type EmbedRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type"` // "document" (nạp) | "query" (search)
}

// buildEmbeddingRequest dựng body (thuần — test được).
func buildEmbeddingRequest(texts []string, inputType string) EmbedRequest {
	return EmbedRequest{Input: texts, Model: voyageModel, InputType: inputType}
}

// batchTexts chia texts thành các batch <= size (thuần).
func batchTexts(texts []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	var out [][]string
	for i := 0; i < len(texts); i += size {
		end := i + size
		if end > len(texts) {
			end = len(texts)
		}
		out = append(out, texts[i:end])
	}
	return out
}

// Client gọi Voyage embedding API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

type embedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// Embed embed nhiều text theo batch (tuần tự để tôn trọng rate limit), gộp kết quả.
func (c *Client) Embed(ctx context.Context, texts []string, inputType string) ([][]float64, error) {
	var all [][]float64
	for _, batch := range batchTexts(texts, batchSize) {
		vecs, err := c.embedBatch(ctx, batch, inputType)
		if err != nil {
			return nil, err
		}
		all = append(all, vecs...)
	}
	return all, nil
}

func (c *Client) embedBatch(ctx context.Context, texts []string, inputType string) ([][]float64, error) {
	body, err := json.Marshal(buildEmbeddingRequest(texts, inputType))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("voyage error: %d %s", res.StatusCode, string(detail))
	}
	var parsed embedResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([][]float64, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}
	return out, nil
}
