package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"github.com/ai-agent-tut/agent-go/internal/rag"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// RAGSearchConfig gom các cờ tính năng cho rag.search — tránh constructor
// nhận quá nhiều tham số bool rời rạc, khó đọc/dễ nhầm thứ tự tại call site.
type RAGSearchConfig struct {
	EnableHybridSearch bool // vector + $text, gộp bằng RRF

	EnableRerank    bool // rerank keyword overlap, MIỄN PHÍ (không gọi LLM)
	EnableLLMRerank bool // rerank bằng LLM — ưu tiên hơn EnableRerank khi cả 2 cùng bật, tốn thêm 1 LLM call

	// Parent Document Retrieval: mở rộng mỗi kết quả trả về bằng chunk liền
	// kề (chunkIndex ±1) cùng tài liệu — chỉ tốn 1 Mongo query, không gọi LLM.
	EnableParentRetrieval bool

	// HyDE: sinh câu trả lời giả định trước khi embed — tốn thêm 1 LLM call.
	EnableHyDE bool

	// Model dùng cho LLM rerank/HyDE — model rẻ/nhanh (vd DeepSeek flash),
	// KHÔNG dùng model chính cho hội thoại để tránh đội chi phí. Bỏ trống thì
	// rerank/HyDE tự tắt dù cờ bật (an toàn, không gọi Generate với model rỗng).
	Model string
}

// ragSearchTool implements Tool for MongoDB Atlas $vectorSearch.
type ragSearchTool struct {
	mongoClient  *mongo.Client
	dbName       string
	voyageClient *rag.Client
	prov         provider.Provider // dùng cho LLM rerank + HyDE, có thể nil (2 tính năng tự tắt)
	model        string
	cfg          RAGSearchConfig
}

// NewRAGSearchTool creates a tool that searches documents via Atlas vector search.
// If mongoClient is nil, the tool returns a graceful error on Execute ("RAG not configured").
// voyageKey is used to create a Voyage embedding client internally.
// prov + cfg.Model cấp năng lực cho LLM rerank/HyDE (bỏ qua an toàn nếu prov
// nil hoặc cfg.Model rỗng, dù cờ tương ứng bật).
func NewRAGSearchTool(mongoClient *mongo.Client, dbName string, voyageKey string, prov provider.Provider, cfg RAGSearchConfig) Tool {
	var vc *rag.Client
	if voyageKey != "" {
		vc = rag.NewClient(voyageKey)
	}
	return &ragSearchTool{
		mongoClient:  mongoClient,
		dbName:       dbName,
		voyageClient: vc,
		prov:         prov,
		model:        cfg.Model,
		cfg:          cfg,
	}
}

func (t *ragSearchTool) Name() string { return "rag.search" }
func (t *ragSearchTool) Kind() Kind   { return KindRead }
func (t *ragSearchTool) Description() string {
	return "Search documents in RAG knowledge base. " +
		"Returns top matching documents with scores, snippets, and relevant chunk content. " +
		"Note: RAG documents are stored in database, NOT local filesystem paths; do NOT use file.read for RAG documents."
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
	Content    string  `json:"content,omitempty"`
	// ChunkIndex dùng nội bộ cho Parent Document Retrieval (tìm chunk liền kề
	// cùng tài liệu) — không hữu ích với model nên không xuất ra JSON.
	ChunkIndex int `json:"-"`
}

// --- RAG Read Tool ---

// ragReadTool reads full content of a RAG document from MongoDB.
type ragReadTool struct {
	mongoClient *mongo.Client
	dbName      string
}

// NewRAGReadTool creates a tool that reads full document content from MongoDB documents collection.
func NewRAGReadTool(mongoClient *mongo.Client, dbName string) Tool {
	return &ragReadTool{
		mongoClient: mongoClient,
		dbName:      dbName,
	}
}

func (t *ragReadTool) Name() string { return "rag.read" }
func (t *ragReadTool) Kind() Kind   { return KindRead }
func (t *ragReadTool) Description() string {
	return "Read the full content of a document from the RAG knowledge base. " +
		"Provide documentId or source filename (e.g. 'go-language.md', 'nestjs.md'). " +
		"Use this instead of file.read when you need to read complete knowledge base documents."
}

func (t *ragReadTool) Schema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"documentId": map[string]any{
				"type":        "string",
				"description": "ID of the document in RAG",
			},
			"source": map[string]any{
				"type":        "string",
				"description": "Source filename of the document (e.g. 'go-language.md')",
			},
		},
	}
	b, _ := json.Marshal(schema)
	return b
}

type ragReadArgs struct {
	DocumentID string `json:"documentId,omitempty"`
	Source     string `json:"source,omitempty"`
}

// buildRAGReadFilter builds the Mongo match filter for rag.read, scoped to
// tenantID the same way ragSearchTool.vectorSearch/textSearch already do.
// Without the tenantId clause, any tenant that knows/guesses another tenant's
// documentId or source filename could read the full content of that document —
// this is what previously made rag.read leak data across tenants.
func buildRAGReadFilter(documentID, source, tenantID string) bson.D {
	matchDoc := bson.D{}
	if documentID != "" {
		matchDoc = append(matchDoc, bson.E{Key: "documentId", Value: documentID})
	} else if source != "" {
		matchDoc = append(matchDoc, bson.E{Key: "source", Value: source})
	}
	if tenantID != "" && tenantID != "default" {
		matchDoc = append(matchDoc, bson.E{Key: "tenantId", Value: tenantID})
	}
	return matchDoc
}

func (t *ragReadTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	if t.mongoClient == nil {
		return Result{Content: "RAG not configured. Set MONGODB_URI to enable document reading."}, nil
	}

	var parsed ragReadArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return Result{}, fmt.Errorf("rag.read: invalid args: %w", err)
	}

	if parsed.DocumentID == "" && parsed.Source == "" {
		return Result{}, fmt.Errorf("rag.read: either documentId or source is required")
	}

	tenantID := middleware.GetTenantID(ctx)
	coll := t.mongoClient.Collection("documents")
	matchDoc := buildRAGReadFilter(parsed.DocumentID, parsed.Source, tenantID)

	type chunkDoc struct {
		Text       string `bson:"text"`
		ChunkIndex int    `bson:"chunkIndex"`
		Source     string `bson:"source"`
	}

	pipeline := []bson.D{
		{{Key: "$match", Value: matchDoc}},
		{{Key: "$sort", Value: bson.D{{Key: "chunkIndex", Value: 1}}}},
		{{Key: "$project", Value: bson.D{
			{Key: "text", Value: 1},
			{Key: "chunkIndex", Value: 1},
			{Key: "source", Value: 1},
		}}},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return Result{}, fmt.Errorf("rag.read: aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var chunks []chunkDoc
	if err := cursor.All(ctx, &chunks); err != nil {
		return Result{}, fmt.Errorf("rag.read: cursor decode: %w", err)
	}

	if len(chunks) == 0 {
		return Result{Content: "Document not found in RAG knowledge base."}, nil
	}

	var parts []string
	for _, c := range chunks {
		parts = append(parts, c.Text)
	}
	fullText := strings.Join(parts, "\n\n")
	const maxChars = 24000
	if len(fullText) > maxChars {
		fullText = fullText[:maxChars] + "\n\n[... Nội dung đã được cắt bớt do vượt quá giới hạn ...]"
	}

	return Result{Content: fullText}, nil
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

	// 0. HyDE: sinh câu trả lời giả định để EMBED thay câu hỏi thô — câu trả
	// lời giả định thường gần nghĩa với đoạn văn thật hơn câu hỏi, tăng độ
	// chính xác dense search. Chỉ áp dụng cho phía vector: hybrid text-search
	// ($text/BM25) vẫn dùng câu hỏi thô của user, không phải văn giả định.
	embedInput := parsed.Query
	if t.cfg.EnableHyDE && t.prov != nil && t.model != "" {
		if hypo := t.generateHypotheticalAnswer(ctx, parsed.Query); hypo != "" {
			embedInput = hypo
		}
	}

	// 1. Embed via Voyage AI.
	vecs, err := t.voyageClient.Embed(ctx, []string{embedInput}, "query")
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: voyage embed: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return Result{Content: "[]"}, nil
	}
	queryVector := vecs[0]

	// 2. Retrieve results: hybrid (vector + text) or pure vector.
	var candidates []ragSearchResult
	if t.cfg.EnableHybridSearch {
		candidates, err = t.hybridSearch(ctx, parsed.Query, queryVector, tenantID)
	} else {
		candidates, err = t.vectorSearch(ctx, queryVector, tenantID, 20)
	}
	if err != nil {
		return Result{}, fmt.Errorf("rag.search: %w", err)
	}

	// 3. Rerank: LLM rerank (nếu bật + có provider/model) ưu tiên hơn keyword
	// rerank miễn phí — 2 cơ chế loại trừ nhau, không chạy cả hai.
	if t.cfg.EnableLLMRerank && t.prov != nil && t.model != "" && len(candidates) > 1 {
		candidates = t.rerankLLM(ctx, parsed.Query, candidates)
	} else if t.cfg.EnableRerank && len(candidates) > 0 {
		candidates = t.rerankKeyword(parsed.Query, candidates)
	}

	// 4. Take top 5.
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	// 5. Parent Document Retrieval: mở rộng CHỈ top-5 (không phải toàn bộ
	// candidates) bằng chunk liền kề để giảm số Mongo query phát sinh.
	if t.cfg.EnableParentRetrieval && len(candidates) > 0 {
		candidates = t.expandParentWindow(ctx, candidates, tenantID)
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
	}

	if tenantID != "" && tenantID != "default" {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: bson.D{{Key: "tenantId", Value: tenantID}}},
		})
	}

	pipeline = append(pipeline, bson.D{
		{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "documentId", Value: 1},
			{Key: "source", Value: 1},
			{Key: "chunkIndex", Value: 1},
			{Key: "content", Value: "$text"},
			{Key: "snippet", Value: bson.D{
				{Key: "$substrCP", Value: bson.A{"$text", 0, 500}},
			}},
			{Key: "score", Value: bson.D{
				{Key: "$meta", Value: "vectorSearchScore"},
			}},
		}},
	})

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
	matchDoc := bson.D{
		{Key: "$text", Value: bson.D{
			{Key: "$search", Value: query},
		}},
	}
	if tenantID != "" && tenantID != "default" {
		matchDoc = append(matchDoc, bson.E{Key: "tenantId", Value: tenantID})
	}

	pipeline := []bson.D{
		{
			{Key: "$match", Value: matchDoc},
		},
		{
			{Key: "$addFields", Value: bson.D{
				{Key: "score", Value: bson.D{{Key: "$meta", Value: "textScore"}}},
			}},
		},
		{
			{Key: "$sort", Value: bson.D{
				{Key: "score", Value: -1},
			}},
		},
		{
			{Key: "$limit", Value: limit},
		},
		{
			{Key: "$project", Value: bson.D{
				{Key: "documentId", Value: 1},
				{Key: "source", Value: 1},
				{Key: "chunkIndex", Value: 1},
				{Key: "content", Value: "$text"},
				{Key: "snippet", Value: bson.D{
					{Key: "$substrCP", Value: bson.A{"$text", 0, 500}},
				}},
				{Key: "score", Value: 1},
			}},
		},
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

// buildParentWindowFilter build Mongo match filter cho Parent Document
// Retrieval: lấy mọi chunk cùng documentId có chunkIndex trong
// [chunkIndex-radius, chunkIndex+radius], scoped theo tenantID giống hệt quy
// tắc buildRAGReadFilter/vectorSearch (bỏ qua tenantId khi rỗng hoặc
// "default" — giữ tương thích ngược cho môi trường không multi-tenant).
func buildParentWindowFilter(documentID string, chunkIndex, radius int, tenantID string) bson.D {
	matchDoc := bson.D{
		{Key: "documentId", Value: documentID},
		{Key: "chunkIndex", Value: bson.D{
			{Key: "$gte", Value: chunkIndex - radius},
			{Key: "$lte", Value: chunkIndex + radius},
		}},
	}
	if tenantID != "" && tenantID != "default" {
		matchDoc = append(matchDoc, bson.E{Key: "tenantId", Value: tenantID})
	}
	return matchDoc
}

// expandParentWindow mở rộng mỗi kết quả bằng chunk liền kề (chunkIndex ±1)
// CÙNG documentId, cùng tenant — bản đơn giản hoá của Parent Document
// Retrieval: vector search vẫn khớp chính xác trên chunk NHỎ (embedding
// chính xác hơn với đoạn ngắn), nhưng LLM nhận được ngữ cảnh RỘNG hơn xung
// quanh đoạn khớp thay vì 1 đoạn ~500 ký tự cụt lủn giữa câu. Không cần
// schema cha/con riêng — tận dụng field chunkIndex đã có sẵn.
// Lỗi khi mở rộng (Mongo lỗi, không tìm thấy hàng xóm) KHÔNG chặn kết quả
// chính — giữ nguyên snippet gốc của candidate đó và tiếp tục các candidate
// còn lại.
func (t *ragSearchTool) expandParentWindow(ctx context.Context, results []ragSearchResult, tenantID string) []ragSearchResult {
	const windowRadius = 1 // lấy thêm chunkIndex-1 và chunkIndex+1
	const maxSnippetRunes = 500

	coll := t.mongoClient.Collection("documents")
	for i := range results {
		r := &results[i]

		matchDoc := buildParentWindowFilter(r.DocumentID, r.ChunkIndex, windowRadius, tenantID)

		pipeline := []bson.D{
			{{Key: "$match", Value: matchDoc}},
			{{Key: "$sort", Value: bson.D{{Key: "chunkIndex", Value: 1}}}},
			{{Key: "$project", Value: bson.D{{Key: "text", Value: 1}}}},
		}

		cursor, err := coll.Aggregate(ctx, pipeline)
		if err != nil {
			continue
		}

		var neighbors []struct {
			Text string `bson:"text"`
		}
		decodeErr := cursor.All(ctx, &neighbors)
		cursor.Close(ctx)
		if decodeErr != nil || len(neighbors) == 0 {
			continue
		}

		parts := make([]string, 0, len(neighbors))
		for _, n := range neighbors {
			parts = append(parts, n.Text)
		}
		windowed := strings.Join(parts, "\n\n")

		r.Content = windowed
		r.Snippet = truncateRunes(windowed, maxSnippetRunes)
	}
	return results
}

// truncateRunes cắt s về tối đa maxRunes CODE POINT (không phải byte) — cắt
// theo byte có thể chẻ đôi 1 ký tự UTF-8 nhiều byte (tiếng Việt có dấu),
// tạo ra chuỗi lỗi encoding ở cuối snippet.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// generateHypotheticalAnswer sinh 1 đoạn trả lời giả định ngắn cho HyDE
// (Hypothetical Document Embeddings) — embed đoạn này thay vì câu hỏi thô vì
// nó thường gần nghĩa với đoạn văn THẬT trong tài liệu hơn là câu hỏi. Đây là
// bước tối ưu tuỳ chọn: lỗi bất kỳ đâu (Generate lỗi, response rỗng) trả về
// "" để Execute() tự fallback dùng câu hỏi gốc, không chặn luồng chính.
func (t *ragSearchTool) generateHypotheticalAnswer(ctx context.Context, query string) string {
	req := provider.GenerateRequest{
		System: "Viết 1 đoạn văn NGẮN (2-4 câu) trả lời giả định cho câu hỏi, " +
			"như thể trích từ 1 tài liệu kỹ thuật thật. KHÔNG giải thích, " +
			"KHÔNG hỏi lại, CHỈ trả về đoạn văn.",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: query}},
		Options: provider.ProviderOptions{
			Model:     t.model,
			MaxTokens: 200,
		},
	}

	chunkChan, err := t.prov.Generate(ctx, req)
	if err != nil {
		return ""
	}

	var raw strings.Builder
	for chunk := range chunkChan {
		if chunk.Kind == provider.ChunkText {
			raw.WriteString(chunk.Text)
		}
	}
	return strings.TrimSpace(raw.String())
}

// rerankLLM chấm điểm lại thứ tự candidates bằng 1 lời gọi LLM. Prompt yêu
// cầu trả về DUY NHẤT 1 mảng số nguyên (index 0-based theo thứ tự liệt kê)
// xếp từ liên quan nhất -> ít liên quan nhất — KHÔNG yêu cầu model sinh lại
// bất kỳ nội dung text nào, để né hẳn lớp lỗi JSON-escaping từng gặp ở
// memory.ReflectAndExtract (dấu ngoặc kép chưa escape trong string value
// dài). Nếu Generate lỗi hoặc response không parse được thành hoán vị hợp
// lệ của [0,n), giữ nguyên thứ tự gốc — đây là bước tối ưu, không bắt buộc.
func (t *ragSearchTool) rerankLLM(ctx context.Context, query string, results []ragSearchResult) []ragSearchResult {
	var b strings.Builder
	fmt.Fprintf(&b, "Câu hỏi: %s\n\nCác đoạn trích (đánh số 0-%d):\n", query, len(results)-1)
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s\n", i, truncateRunes(r.Snippet, 300))
	}

	req := provider.GenerateRequest{
		System: "Bạn là bộ xếp hạng độ liên quan. CHỈ trả về 1 mảng JSON các " +
			"số index (0-based) theo thứ tự liên quan nhất -> ít liên quan " +
			"nhất, ví dụ: [2,0,3,1]. KHÔNG kèm text giải thích, KHÔNG " +
			"markdown code block, KHÔNG có ký tự nào khác ngoài mảng JSON.",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: b.String()}},
		Options: provider.ProviderOptions{
			Model:     t.model,
			MaxTokens: 200,
		},
	}

	chunkChan, err := t.prov.Generate(ctx, req)
	if err != nil {
		return results
	}

	var raw strings.Builder
	for chunk := range chunkChan {
		if chunk.Kind == provider.ChunkText {
			raw.WriteString(chunk.Text)
		}
	}

	order, ok := parseRerankOrder(raw.String(), len(results))
	if !ok {
		return results
	}

	reordered := make([]ragSearchResult, 0, len(results))
	for _, idx := range order {
		reordered = append(reordered, results[idx])
	}
	return reordered
}

// parseRerankOrder parse output rerankLLM thành 1 hoán vị hợp lệ của [0,n):
// đúng n phần tử, mỗi index trong khoảng [0,n) xuất hiện ĐÚNG 1 LẦN. Trả
// ok=false với BẤT KỲ sai lệch nào (thiếu/thừa/trùng/ngoài phạm vi index) —
// an toàn hơn chấp nhận 1 phần rồi âm thầm làm rơi rụng hoặc lặp kết quả.
func parseRerankOrder(raw string, n int) ([]int, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var order []int
	if err := json.Unmarshal([]byte(raw), &order); err != nil {
		return nil, false
	}
	if len(order) != n {
		return nil, false
	}
	seen := make([]bool, n)
	for _, idx := range order {
		if idx < 0 || idx >= n || seen[idx] {
			return nil, false
		}
		seen[idx] = true
	}
	return order, true
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
