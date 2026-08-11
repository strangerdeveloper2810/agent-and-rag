package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"github.com/ai-agent-tut/agent-go/internal/rag"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ragSearchTool implements Tool for MongoDB Atlas $vectorSearch.
type ragSearchTool struct {
	mongoClient  *mongo.Client
	dbName       string
	voyageClient *rag.Client
}

// NewRAGSearchTool creates a tool that searches documents via Atlas vector search.
// If mongoClient is nil, the tool returns a graceful error on Execute ("RAG not configured").
// voyageKey is used to create a Voyage embedding client internally.
func NewRAGSearchTool(mongoClient *mongo.Client, dbName string, voyageKey string) Tool {
	var vc *rag.Client
	if voyageKey != "" {
		vc = rag.NewClient(voyageKey)
	}
	return &ragSearchTool{
		mongoClient:  mongoClient,
		dbName:       dbName,
		voyageClient: vc,
	}
}

func (t *ragSearchTool) Name() string { return "rag.search" }
func (t *ragSearchTool) Kind() Kind   { return KindRead }
func (t *ragSearchTool) Description() string {
	return "Search local documents using RAG (Retrieval-Augmented Generation). " +
		"Returns top 5 matching documents with scores and snippets. " +
		"Use this when you need to find information in the user's personal documents, " +
		"notes, or knowledge base."
}

func (t *ragSearchTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query to find relevant documents",
			},
		},
		"required": []string{"query"},
	}
	b, _ := json.Marshal(schema)
	return b
}

// ragSearchArgs holds the parsed arguments for rag.search.
type ragSearchArgs struct {
	Query string `json:"query"`
}

// ragSearchResult represents a single search result.
type ragSearchResult struct {
	DocumentID string  `json:"documentId"`
	Source     string  `json:"source"`
	Score      float64 `json:"score"`
	Snippet    string  `json:"snippet"`
}

func (t *ragSearchTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	if t.mongoClient == nil || t.voyageClient == nil {
		return Result{Content: "RAG not configured. Set MONGODB_URI and VOYAGE_API_KEY to enable document search."}, nil
	}

	tenantID := middleware.GetTenantID(ctx)

	var parsed ragSearchArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return Result{}, fmt.Errorf("rag.search: invalid args: %w", err)
	}
	if parsed.Query == "" {
		return Result{}, fmt.Errorf("rag.search: query is required")
	}

	// 1. Embed query via Voyage AI.
	vecs, err := t.voyageClient.Embed(ctx, []string{parsed.Query}, "query")
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: voyage embed: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return Result{Content: "[]"}, nil
	}
	queryVector := vecs[0]

	// 2. MongoDB $vectorSearch aggregation.
	coll := t.mongoClient.Collection("documents")

	pipeline := []bson.D{
		{
			{Key: "$vectorSearch", Value: bson.D{
				{Key: "index", Value: "vector_index"},
				{Key: "path", Value: "embedding"},
				{Key: "queryVector", Value: queryVector},
				{Key: "numCandidates", Value: 100},
				{Key: "limit", Value: 5},
					{Key: "filter", Value: bson.D{{Key: "tenantId", Value: tenantID}}},
			}},
		},
		{
			{Key: "$project", Value: bson.D{
				{Key: "documentId", Value: bson.D{{Key: "$toString", Value: "$_id"}}},
				{Key: "source", Value: 1},
				{Key: "snippet", Value: bson.D{
					{Key: "$substrCP", Value: bson.A{"$content", 0, 300}},
				}},
				{Key: "score", Value: bson.D{
					{Key: "$meta", Value: "vectorSearchScore"},
				}},
			}},
		},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: mongo aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var results []ragSearchResult
	if err := cursor.All(ctx, &results); err != nil {
		return Result{}, fmt.Errorf("rag.search: cursor decode: %w", err)
	}
	if results == nil {
		results = []ragSearchResult{} // ensure JSON "[]" not null
	}

	b, err := json.Marshal(results)
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: marshal results: %w", err)
	}
	return Result{Content: string(b)}, nil
}
