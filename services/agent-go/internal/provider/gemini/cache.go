package gemini

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/provider"
	"google.golang.org/genai"
)

// CreateCachedContent creates a cached content resource for system instruction + tools.
// Returns the cache resource name on success.
// Falls back gracefully (empty string, no error) on failure so the caller can
// continue with uncached requests.
func (c *Client) CreateCachedContent(ctx context.Context, systemPrompt string, toolDefs []provider.ToolDef) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("gemini: client not initialized")
	}
	if systemPrompt == "" {
		return "", nil // nothing to cache
	}

	// 30s timeout for cache creation.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sysContent := genai.NewContentFromText(systemPrompt, genai.RoleUser)
	tools := toGeminiTools(toolDefs)

	config := &genai.CreateCachedContentConfig{
		TTL:               1 * time.Hour,
		DisplayName:       "jarvis-system-cache",
		SystemInstruction: sysContent,
		Tools:             tools,
	}

	cached, err := c.client.Caches.Create(ctx, c.model, config)
	if err != nil {
		slog.Warn("gemini: failed to create cached content, falling back to uncached",
			"err", err)
		return "", nil // graceful degradation
	}

	slog.Info("gemini: cached content created",
		"name", cached.Name,
		"model", c.model,
		"expire", cached.ExpireTime.Format(time.RFC3339),
	)
	return cached.Name, nil
}

// SetCacheName stores a previously created cache name for use in subsequent
// Generate calls. Set to "" to disable caching.
func (c *Client) SetCacheName(name string) {
	c.cacheName = name
}

// CacheName returns the current cache name (empty if no cache is active).
func (c *Client) CacheName() string {
	return c.cacheName
}
