package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"github.com/ai-agent-tut/agent-go/internal/rag"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ragSearchTool implements Tool for MongoDB Atlas $vectorSearch.
type ragSearchTool struct {
	mongoClient        *mongo.Client
	dbName             string
	voyageClient       *rag.Client
	enableHybridSearch bool
	enableRerank       bool
}

// NewRAGSearchTool creates a tool that searches documents via Atlas vector search.
// If mongoClient is nil, the tool returns a graceful error on Execute ("RAG not configured").
// voyageKey is used to create a Voyage embedding client internally.
// enableHybridSearch enables vector + full-text hybrid search; enableRerank enables post-retrieval reranking.
func NewRAGSearchTool(mongoClient *mongo.Client, dbName string, voyageKey string, enableHybridSearch bool, enableRerank bool) Tool {
	var vc *rag.Client
	if voyageKey != "" {
		vc = rag.NewClient(voyageKey)
	}
	return &ragSearchTool{
		mongoClient:        mongoClient,
		dbName:             dbName,
		voyageClient:       vc,
		enableHybridSearch: enableHybridSearch,
		enableRerank:       enableRerank,
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

	// 2. Retrieve results: hybrid (vector + text) or pure vector.
	var candidates []ragSearchResult
	if t.enableHybridSearch {
		candidates, err = t.hybridSearch(ctx, parsed.Query, queryVector, tenantID)
	} else {
		candidates, err = t.vectorSearch(ctx, queryVector, tenantID, 20)
	}
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: %w", err)
	}

	// 3. Rerank if enabled.
	if t.enableRerank && len(candidates) > 0 {
		candidates = t.rerankKeyword(parsed.Query, candidates)
	}

	// 4. Take top 5.
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	if candidates == nil {
		candidates = []ragSearchResult{}
	}

	b, err := json.Marshal(candidates)
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: marshal results: %w", err)
	}
	return Result{Content: string(b)}, nil
}

// vectorSearch runs a pure $vectorSearch aggregation.
// numCandidates and limit control how many results to retrieve.
func (t *ragSearchTool) vectorSearch(ctx context.Context, queryVector []float64, tenantID string, limit int) ([]ragSearchResult, error) {
	coll := t.mongoClient.Collection("documents")
	pipeline := []bson.D{
		{
			{Key: "$vectorSearch", Value: bson.D{
				{Key: "index", Value: "vector_index"},
				{Key: "path", Value: "embedding"},
				{Key: "queryVector", Value: queryVector},
				{Key: "numCandidates", Value: 100},
				{Key: "limit", Value: limit},
			}},
		},
		// Lọc tenant SAU $vectorSearch, không dùng filter trong-stage: filter
		// trong $vectorSearch đòi hỏi field phải được khai báo type "filter"
		// trong Atlas Search index — index "vector_index" hiện tại chưa có,
		// nên dùng filter trong-stage sẽ lỗi "Path 'tenantId' needs to be
		// indexed as filter".
		{
			{Key: "$match", Value: bson.D{{Key: "tenantId", Value: tenantID}}},
		},
		{
			{Key: "$project", Value: bson.D{
				{Key: "_id", Value: 0},
				{Key: "documentId", Value: 1},
				{Key: "source", Value: 1},
				{Key: "content", Value: 1},
				{Key: "snippet", Value: bson.D{
					{Key: "$substrCP", Value: bson.A{"$text", 0, 500}},
				}},
				{Key: "score", Value: bson.D{
					{Key: "$meta", Value: "vectorSearchScore"},
				}},
			}},
		},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vectorSearch: aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var results []ragSearchResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("vectorSearch: cursor decode: %w", err)
	}
	return results, nil
}

// textSearch runs a MongoDB $text search on the query.
// Falls back gracefully if no text index exists.
func (t *ragSearchTool) textSearch(ctx context.Context, query string, tenantID string, limit int) ([]ragSearchResult, error) {
	coll := t.mongoClient.Collection("documents")
	pipeline := bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "$text", Value: bson.D{
				{Key: "$search", Value: query},
			}},
			{Key: "tenantId", Value: tenantID},
		}},
		{Key: "$addFields", Value: bson.D{
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "textScore"}}},
		}},
		{Key: "$sort", Value: bson.D{
			{Key: "score", Value: -1},
		}},
		{Key: "$limit", Value: limit},
		{Key: "$project", Value: bson.D{
			{Key: "documentId", Value: bson.D{{Key: "$toString", Value: "$_id"}}},
			{Key: "source", Value: 1},
			{Key: "content", Value: 1},
			{Key: "snippet", Value: bson.D{
				{Key: "$substrCP", Value: bson.A{"$content", 0, 300}},
			}},
			{Key: "score", Value: 1},
		}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		if strings.Contains(err.Error(), "text index") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, fmt.Errorf("textSearch: aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var results []ragSearchResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("textSearch: cursor decode: %w", err)
	}
	return results, nil
}

// hybridSearch runs vector search + text search, then merges results using
// Reciprocal Rank Fusion (RRF) with weighted scoring.
// Falls back to pure vector search if text search fails or returns nothing.
func (t *ragSearchTool) hybridSearch(ctx context.Context, query string, queryVector []float64, tenantID string) ([]ragSearchResult, error) {
	const k = 60 // RRF constant

	// Phase a: vector search — top 20 candidates.
	vecResults, err := t.vectorSearch(ctx, queryVector, tenantID, 20)
	if err != nil {
		return nil, fmt.Errorf("hybridSearch: vector: %w", err)
	}

	// Phase b: text search — top 20 candidates.
	textResults, err := t.textSearch(ctx, query, tenantID, 20)
	if err != nil {
		return nil, fmt.Errorf("hybridSearch: text: %w", err)
	}

	// If text search returned nothing (no text index or no matches), fall back to vector-only.
	if len(textResults) == 0 {
		return vecResults, nil
	}

	// Merge using RRF: compute score = 1/(k+rank_vec) + 1/(k+rank_text).
	seen := make(map[string]*ragSearchResult)
	add := func(r ragSearchResult, rrf float64) {
		if existing, ok := seen[r.DocumentID]; ok {
			existing.Score += rrf
		} else {
			cp := r
			cp.Score = rrf
			seen[r.DocumentID] = &cp
		}
	}

	for i, r := range vecResults {
		add(r, 1.0/(k+float64(i+1)))
	}
	for i, r := range textResults {
		add(r, 1.0/(k+float64(i+1)))
	}

	var merged []ragSearchResult
	for _, r := range seen {
		merged = append(merged, *r)
	}

	// Weighted RRF: vector 0.7, text 0.3.
	for i := range merged {
		vecRank := rankOf(vecResults, merged[i].DocumentID)
		textRank := rankOf(textResults, merged[i].DocumentID)
		vecScore := 0.0
		textScore := 0.0
		if vecRank > 0 {
			vecScore = 1.0 / (k + float64(vecRank))
		}
		if textRank > 0 {
			textScore = 1.0 / (k + float64(textRank))
		}
		merged[i].Score = 0.7*vecScore + 0.3*textScore
	}

	// Sort by combined RRF score descending.
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged, nil
}

// rankOf returns the 1-based rank of a documentID in results, or 0 if not found.
func rankOf(results []ragSearchResult, docID string) int {
	for i, r := range results {
		if r.DocumentID == docID {
			return i + 1
		}
	}
	return 0
}

// rerankKeyword re-sorts results by keyword overlap between the query and each chunk's content.
// This is a lightweight pragmatic reranker: no LLM call, no cross-encoder.
// It boosts results whose content contains more query terms.
func (t *ragSearchTool) rerankKeyword(query string, results []ragSearchResult) []ragSearchResult {
	queryTerms := tokenizeKeywords(query)
	queryTermSet := make(map[string]struct{}, len(queryTerms))
	for _, tk := range queryTerms {
		queryTermSet[tk] = struct{}{}
	}

	for i := range results {
		contentTerms := tokenizeKeywords(results[i].Snippet)
		overlap := 0
		for _, ct := range contentTerms {
			if _, ok := queryTermSet[ct]; ok {
				overlap++
			}
		}
		overlapScore := 0.0
		if len(queryTerms) > 0 {
			overlapScore = float64(overlap) / float64(len(queryTerms))
		}
		results[i].Score = 0.7*results[i].Score + 0.3*overlapScore
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// tokenizeKeywords splits text into lowercase alphanumeric tokens for keyword matching.
func tokenizeKeywords(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	var cleaned []string
	for _, f := range fields {
		s := strings.TrimFunc(f, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
		})
		if len(s) > 1 {
			cleaned = append(cleaned, s)
		}
	}
	return cleaned
}
