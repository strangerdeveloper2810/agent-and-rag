# Kiến trúc Deep-Dive: Subsystem của Agent Runtime "Jarvis" (services/agent-go)

> Tài liệu này đi sâu vào **tất cả subsystem của `services/agent-go` NGOẠI TRỪ core engine** (`internal/agent/*`, có tài liệu riêng lo phần đó). Mục tiêu: đọc lại để hiểu hệ thống đến từng dòng code, và chuẩn bị phỏng vấn vị trí "agentic coding". Nguyên tắc xuyên suốt tài liệu: **code thật luôn được ưu tiên hơn comment/doc cũ** — mọi chỗ tên gọi, comment, hoặc tài liệu design cũ (`docs/architecture/*.md`, `docs/TOOLS.md`, `docs/SKILLS.md`, `docs/DEPLOY.md`) lệch với hành vi thật của code đều được chỉ rõ, kèm trích dẫn `file:line`.
>
> Phạm vi: `internal/memory`, `internal/guardrails`, `internal/provider` (+ 4 sub-package provider), `internal/orchestrator`, `internal/personality`, `internal/proactive`, `internal/mcp`, `internal/skills`, `internal/tools` (25 tool + registry), `internal/rag`, `internal/storage/{chroma,sqlite}`, `internal/transport/http/suggestions.go`, `internal/middleware`, `internal/metrics`, `internal/observability`, `internal/config`, `internal/eval`.
>
> Một lưu ý kỹ thuật quan trọng cho người đọc lại tài liệu này: worktree dùng để viết tài liệu này được tạo từ commit `9d23615` — **trước** commit `ee520ea` ("feat(users,agent-go,web): user avatar, agent logo, MCP servers and skills management") trên `master`, nên thiếu `internal/mcp/sse.go`/`sse_test.go`. Phần MCP (mục 6) được đọc trực tiếp từ checkout `master` mới nhất để đảm bảo phản ánh đúng code hiện tại — các phần khác đọc từ worktree.

**Tổng quan phát hiện đáng chú ý nhất** (chi tiết ở từng mục tương ứng):

| # | Phát hiện | Mục |
|---|---|---|
| 1 | 2 pipeline "học" memory chạy song song: regex đồng bộ (miễn phí) vs LLM nền có gate (đắt, có kiểm soát) | 1 |
| 2 | `guardrails.CircuitBreaker` KHÔNG phải circuit breaker 3-state chuẩn — nó là bộ đếm lặp phát hiện stuck-loop. Circuit breaker thật (có cooldown/backoff) nằm ở `provider/fallback` | 2, 3.4 |
| 3 | Routing multi-agent 100% keyword-matching, không có LLM classifier như design doc ban đầu hình dung | 4 |
| 4 | `personality` và `proactive` là 2 package hoàn chỉnh, có test đầy đủ, nhưng **chưa từng được wire vào `cmd/server/main.go`** | 5 |
| 5 | MCP discovery cho server SSE remote chạy **per-request** (không phải 1 lần lúc khởi động) để tránh rò tool giữa user | 6 |
| 6 | RAG "rerank" mặc định KHÔNG dùng cross-encoder — chỉ là keyword-overlap heuristic; "hybrid search" và "HyDE" thì có thật | 9 |
| 7 | Package `storage/chroma` KHÔNG phải ChromaDB client — là in-memory map tự viết, không persist, không HNSW | 10 |
| 8 | 5/25 tool (`calendar`, `http`, `json`, `timer`, `weather`) có code + test nhưng chưa từng được đăng ký vào registry production | 8 |
| 9 | `internal/metrics` và `internal/observability` (tracing) đều chưa wire vào `main.go` — code "chết" dù có test tốt | 12 |
| 10 | `shell.exec` không sandbox, allowlist command luôn được gọi với `nil` ở production — mitigation thật nằm ở tầng guardrail/owner-tenant phía ngoài | 15 |

---

## 1. Memory 3-tier

### 1.1 Khái niệm chung

Trong thiết kế AI agent hiện đại, "memory" thường được chia làm 3 tầng theo vòng đời sống của thông tin:

- **Short-term memory (context window / history)**: toàn bộ chuỗi message của một conversation, sống trong RAM/State của một lần chạy (run), mất đi khi request kết thúc nếu không được persist. Đây chính là `agent.State.Messages` trong Jarvis — history được client gửi lại mỗi lượt gọi API (agent-go là **stateless per-request**, không tự giữ session ở server).
- **Working memory**: thông tin "đang được xử lý" trong một turn/task cụ thể — ví dụ kết quả tool call vừa chạy, plan đang thực hiện, hoặc (như trong Jarvis) danh sách fact vừa được `RecallNode` tìm ra và tạm gắn vào `State.RecalledMemories` để `nodeModel` dệt vào system prompt của đúng lượt đó. Working memory không tồn tại ngoài phạm vi một lần `Engine.Run()`.
- **Long-term memory**: thông tin **persist qua nhiều session/nhiều ngày**, thường lưu ngoài context window (DB/vector store) và được truy xuất lại bằng recall khi cần — đây là phần "agent nhớ user" theo đúng nghĩa sản phẩm (tên, sở thích, tech stack, quy ước làm việc...).

Hai ý tưởng kinh điển hay được đối chiếu khi nói về long-term memory của agent:

- **MemGPT** (Packer et al., 2023) coi context window của LLM như RAM của một hệ điều hành: dung lượng hữu hạn, nên cần một "OS ảo" chủ động **page thông tin vào/ra** giữa context (main memory) và một kho ngoài (disk) — tự quyết định khi nào evict phần cũ ra ngoài và khi nào load lại phần liên quan vào context. Jarvis có một bản rút gọn của ý tưởng này: `SummarizeNode` (evict — nén phần message cũ khi vượt ngưỡng) và `RecallNode` (load lại — nạp fact liên quan vào system prompt mỗi turn), nhưng không có cơ chế "page fault" chủ động do LLM tự gọi function để tìm nạp thêm — recall ở đây là fixed pipeline chạy trước mọi request, không phải một tool LLM tự quyết định gọi.
- **Generative Agents** (Park et al., 2023, "Generative Agents: Interactive Simulacra of Human Behavior") đề xuất một memory stream gồm các observation thô, và định kỳ chạy **reflection**: tổng hợp nhiều memory thô thành insight bậc cao hơn (ví dụ nhiều observation nhỏ về việc học → insight "X đang đam mê nghiên cứu AI"), có chấm **importance score**, kết hợp với **recency** và **relevance** để xếp hạng khi retrieve. Jarvis có một node tên chính xác là "reflection" (`reflection.go` — `ReflectAndExtract`), nhưng bản chất khác Generative Agents: nó không tính importance/recency/relevance score hay chạy theo lịch định kỳ độc lập — mà là một **lượt gọi LLM để trích xuất fact có cấu trúc (structured extraction)** từ N tin nhắn cuối, chạy sau mỗi lượt trả lời. Tên "reflection" ở đây gần với nghĩa "agent tự nhìn lại hội thoại vừa xảy ra để rút ra bài học", nhưng cơ chế implement là extraction-by-LLM-with-JSON-schema, không phải multi-memory synthesis có importance score như bản gốc.

Điểm khác biệt lớn nhất của Jarvis so với hai mô hình lý thuyết trên: **có tới hai pipeline "học" chạy song song, độc lập, với chi phí khác nhau hẳn** — một pipeline rẻ, đồng bộ, dùng regex (`ExtractNode`, chạy trong graph, mọi request) và một pipeline đắt, bất đồng bộ, dùng LLM thật (`Learner.LearnFromConversation` → `ReflectAndExtract`, chạy nền sau khi response đã trả về, có gate để không tốn tiền mỗi lượt chat). Đây là một quyết định kỹ thuật rất thực tế xuất phát từ **ngân sách LLM hẹp** (xem phần learner_gate.go), khác hẳn thiết kế "muốn chính xác nhất, cứ gọi LLM" của nhiều bài paper học thuật.

### 1.2 Embedder

`internal/memory/embedder.go` chỉ định nghĩa **interface trừu tượng**, không tự implement gọi API nào:

```go
// services/agent-go/internal/memory/embedder.go:5-15
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// EmbedderFunc wraps a function as an Embedder.
type EmbedderFunc func(ctx context.Context, texts []string) ([][]float64, error)

func (f EmbedderFunc) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return f(ctx, texts)
}
```

`EmbedderFunc` là một adapter kiểu "function-as-interface" (giống `http.HandlerFunc`) — cho phép package `memory` không phải phụ thuộc trực tiếp vào bất kỳ SDK embedding cụ thể nào (Voyage/Ollama/OpenAI). Việc "cắm" một embedder thật vào được làm ở **lớp wiring** (`cmd/server/main.go`), không nằm trong package `memory`.

Implementation thật nằm ở `internal/rag/voyage.go` — dùng **Voyage AI** (`voyage-3`, 1024 chiều, khớp `numDimensions` của Atlas vector index dùng cho RAG):

```go
// services/agent-go/internal/rag/voyage.go:15-19, 49-57, 65-76
const (
	voyageURL   = "https://api.voyageai.com/v1/embeddings"
	voyageModel = "voyage-3" // 1024 chiều — khớp numDimensions của Atlas vector_index
	batchSize   = 96         // an toàn dưới giới hạn số text/request của Voyage
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, httpClient: &http.Client{Timeout: 30 * time.Second}}
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
```

Đây gọi HTTP REST trực tiếp (không SDK), header `Authorization: Bearer <apiKey>`.

Wiring thực tế trong `cmd/server/main.go` — có **fallback theo provider**: ưu tiên Voyage nếu có `VoyageKey`, nếu không thì fallback sang Ollama (local embedding, phù hợp môi trường không có key trả phí):

```go
// services/agent-go/cmd/server/main.go:117-144 (rút gọn)
if cfg.VoyageKey != "" {
	vc := rag.NewClient(cfg.VoyageKey)
	store.SetEmbedder(memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
		return vc.Embed(ctx, texts, "document")
	}))
	slog.Info("memory: semantic embedding enabled", "provider", "voyage")
} else if cfg.Provider == "ollama" || os.Getenv("OLLAMA_URL") != "" {
	embedClient, err := ollama.New(cfg.OllamaURL, cfg.EmbedModel)
	// ... convert []float32 -> []float64, SetEmbedder tương tự
}
```

Nếu cả hai đều không cấu hình, `store.embedder == nil` → mọi `SemanticSearch` trả về rỗng ngay, hệ thống fallback thuần về keyword/full-text (xem 1.5) — **không lỗi, không chặn khởi động**, chỉ mất khả năng semantic.

Đáng chú ý: `Learner` (1.4) dùng **một embedder khác, được tạo riêng** tại dòng 323-332 của `main.go` (cùng logic Voyage, nhưng instance HTTP client riêng) để embed fact/knowledge trước khi lưu Mongo — tách biệt với embedder gắn vào `Store` cho recall trong RAM.

### 1.3 Extract

**Kết luận quan trọng đầu tiên**: `extract.go` **KHÔNG gọi LLM** — nó là **thuần heuristic dựa trên regex**, chạy đồng bộ trong node graph của mọi request, không tốn token, không có latency gọi API.

```go
// services/agent-go/internal/memory/extract.go:14-39
type extractRule struct {
	re  *regexp.Regexp
	key string // meaningful key, e.g. "user_name", "like", "user_location"
}

var extractPatterns = []extractRule{
	{re: regexp.MustCompile(`(?i)tôi (?:tên|là) (?:tên |là )?(.+)$`), key: "user_name"},
	{re: regexp.MustCompile(`(?i)gọi tôi là (.+)`), key: "user_name"},
	{re: regexp.MustCompile(`(?i)tôi thích (.+)`), key: "like"},
	{re: regexp.MustCompile(`(?i)tôi không thích (.+)`), key: "dislike"},
	{re: regexp.MustCompile(`(?i)nhớ (?:là |rằng |giúp tôi |cho tôi )?(.+)$`), key: "fact"},
	{re: regexp.MustCompile(`(?i)tôi ở (.+)`), key: "user_location"},
	{re: regexp.MustCompile(`(?i)tôi làm (?:việc|ở|tại) (.+)`), key: "user_job"},
	{re: regexp.MustCompile(`(?i)tôi muốn (.+)`), key: "want"},
	{re: regexp.MustCompile(`(?i)tôi cần (.+)`), key: "need"},
	{re: regexp.MustCompile(`(?i)địa chỉ email (?:của tôi |là )?(.+)`), key: "email"},
	{re: regexp.MustCompile(`(?i)số điện thoại (?:của tôi |là )?(.+)`), key: "phone"},
	// ... tổng 15 pattern, cả tiếng Việt (có biến thể "thích/rất thích/cực thích/siêu thích")
}
```

Hàm chính, chữ ký `ExtractNode(store *Store) agent.Node` — factory trả về một `agent.Node` (closure) để engine gọi trong pipeline:

```go
// services/agent-go/internal/memory/extract.go:41-75
func ExtractNode(store *Store) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		tenantID := middleware.GetTenantID(ctx)

		extracted := 0
		seen := make(map[string]bool)

		for _, msg := range s.Messages {
			if msg.Role != provider.RoleUser && msg.Role != provider.RoleAssistant {
				continue
			}
			for _, rule := range extractPatterns {
				matches := rule.re.FindStringSubmatch(msg.Content)
				if len(matches) < 2 {
					continue
				}
				value := strings.TrimSpace(matches[1])
				if value == "" || len(value) > 200 {
					continue
				}
				if seen[rule.key] {
					continue
				}
				seen[rule.key] = true
				store.Set(tenantID, rule.key, value)
				extracted++
				emit(agent.MemoryEvent(fmt.Sprintf("extracted: %s = %s", rule.key, value)))
			}
		}

		return agent.NodeEnd, nil
	}
}
```

Luồng xử lý:
1. Lấy `tenantID` từ context (multi-tenant — mỗi tenant có namespace riêng trong `Store`).
2. Duyệt **toàn bộ `s.Messages`** (cả history + turn hiện tại), chỉ xét role `user`/`assistant` (bỏ `system`/`tool` — test `TestExtractNode_SkipsNonConversationalRoles` xác nhận).
3. Với mỗi message, thử match lần lượt 15 pattern; group đầu tiên (`(.+)`) là value.
4. Validation nhẹ: value rỗng hoặc > 200 ký tự thì bỏ (`TestExtractNode_ValueTooLongSkipped`); mỗi `key` chỉ lưu **lần đầu tiên gặp trong 1 lượt gọi node** (map `seen`) — dedup **trong-turn**, không phải dedup toàn cục (2 turn khác nhau nói "tôi thích X" hai lần vẫn ghi đè bình thường qua `store.Set`).
5. Ghi trực tiếp vào `Store` (in-memory, xem 1.7) qua `store.Set(tenantID, key, value)`, và emit `agent.MemoryEvent(...)` — event này chảy ra SSE stream cho UI hiển thị "đã ghi nhớ: ...".
6. Luôn trả về `agent.NodeEnd` — đây là **node cuối cùng** của graph (không đi tiếp đâu nữa).

Vì đây là regex thuần, độ chính xác phụ thuộc hoàn toàn vào việc câu nói của user có khớp đúng cấu trúc câu định sẵn hay không (ví dụ "tôi là X" khớp, nhưng diễn đạt khác đi kiểu "mọi người hay gọi tôi X" thì không khớp bất kỳ pattern nào). Đây chính là lý do tồn tại một pipeline thứ hai mạnh hơn — `Learner`/`ReflectAndExtract` (1.4/1.6) — dùng LLM để bắt được các cách diễn đạt tự nhiên mà regex không cover được, đánh đổi bằng chi phí và độ trễ.

### 1.4 Learner & Gate

"Learner" (`learner.go`) là tên gọi trong Jarvis cho **pipeline học nền (background) chạy LLM thật** — khác hẳn `ExtractNode` (đồng bộ, regex, miễn phí). `Learner` không tự "quyết định memory nào đáng lưu" bằng importance score kiểu Generative Agents; việc "đáng lưu hay không" được LLM (qua `ReflectAndExtract`, xem 1.6) tự trả lời bằng field `confidence` trong JSON output. Vai trò gate thực sự nằm ở `learner_gate.go` — nhưng gate ở đây gác **"có đáng gọi LLM không"**, tức là một **cost gate**, không phải "content quality gate" theo nghĩa academic.

**Cấu trúc `Learner`:**

```go
// services/agent-go/internal/memory/learner.go:18-36
type Learner struct {
	store       *Store
	mongoClient *mongo.Client
	provider    provider.Provider
	model       string
	embedder    Embedder
}

func NewLearner(store *Store, mongoClient *mongo.Client, p provider.Provider, model string, embedder Embedder) *Learner {
	return &Learner{store: store, mongoClient: mongoClient, provider: p, model: model, embedder: embedder}
}
```

**Điểm vào** `LearnFromConversation` — được gọi từ `chat.go` **SAU KHI response đã trả về hoàn tất** (không nằm trong node graph của lượt chạy, không chặn TTFB của user):

```go
// services/agent-go/internal/memory/learner.go:50-73 (rút gọn)
func (l *Learner) LearnFromConversation(ctx context.Context, messages []provider.Message, conversationID string) {
	if l == nil || l.provider == nil || len(messages) < 2 {
		return
	}
	if !worthLearning(messages) {
		slog.Debug("learner: bỏ qua lượt tán gẫu (không có gì để học)")
		return
	}

	tenantID := middleware.GetTenantID(ctx)
	msgsCopy := make([]provider.Message, len(messages))
	copy(msgsCopy, messages)

	go func() {
		bgCtx := context.WithValue(context.Background(), middleware.TenantIDKey, tenantID)
		bgCtx, cancel := context.WithTimeout(bgCtx, 45*time.Second)
		defer cancel()

		res, err := ReflectAndExtract(bgCtx, l.provider, l.model, msgsCopy)
		// ... process res.UserFacts + res.KnowledgeItems (xem dưới)
	}()
}
```

Chi tiết kỹ thuật quan trọng ở đây: `ctx` gốc (request context từ HTTP handler) **chỉ được dùng để lấy `tenantID` NGAY LÚC ĐÓ**, rồi bị bỏ — goroutine nền dựng một `context.Background()` mới mang theo tenantID, vì `net/http` sẽ cancel request context ngay khi handler return, còn goroutine học vẫn cần sống tiếp tới 45 giây.

**Gate — `learner_gate.go` — `worthLearning`:**

```go
// services/agent-go/internal/memory/learner_gate.go:15-52
const (
	trivialUserRunes      = 25
	trivialAssistantRunes = 400
)

func worthLearning(messages []provider.Message) bool {
	lastUser, lastAssistant := lastByRole(messages)

	if lastUser == "" {
		return false
	}

	lower := strings.ToLower(lastUser)
	for keyword := range keywordToKeys {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	if len([]rune(lastUser)) > trivialUserRunes {
		return true
	}
	return len([]rune(lastAssistant)) > trivialAssistantRunes
}
```

Thuật toán gate — **không dùng score liên tục, dùng OR của 3 điều kiện rời rạc**:
1. Câu user cuối chứa **bất kỳ keyword** nằm trong `keywordToKeys` (bảng dùng CHUNG với `RecallNode`, xem 1.5 — "một nguồn sự thật") → **luôn học**.
2. Câu user cuối dài hơn **25 rune** → học (câu dài thường mang thông tin/yêu cầu thật).
3. Câu trả lời assistant cuối dài hơn **400 rune** → học (câu trả lời dài thường là giải pháp kỹ thuật đáng ghi thành knowledge item).
4. Ngược lại (user ngắn + assistant ngắn + không keyword) → **bỏ qua, không gọi LLM**.

Comment trong code giải thích rõ triết lý: gate cố tình đặt **bảo toàn phía học** (false negative rẻ hơn false positive) — bỏ sót một lượt đáng học chỉ mất 1 fact (lượt sau nhắc lại vẫn học được), nhưng học mọi lượt tán gẫu ("xin chào", "cảm ơn nhé") thì **nhân đôi hoá đơn LLM của toàn hệ thống**, vì mỗi lượt chat đã có 1 lượt gọi model chính, nếu học không gate thì cộng thêm 1 lượt gọi LLM nữa cho reflection.

**Sau khi qua gate**, `LearnFromConversation` xử lý kết quả `ReflectAndExtract` theo 2 nhánh:

```go
// services/agent-go/internal/memory/learner.go:83-123 (rút gọn)
// 1. Process User Facts
for _, fact := range res.UserFacts {
	if strings.TrimSpace(fact.Key) == "" || strings.TrimSpace(fact.Value) == "" {
		continue
	}
	if l.store != nil {
		l.store.Set(tenantID, fact.Key, fact.Value) // ghi vào Store RAM
	}
	if l.mongoClient != nil {
		l.saveFactToMongo(bgCtx, fact, conversationID) // ghi bền xuống Mongo
	}
}
// 2. Process Knowledge Items
for _, ki := range res.KnowledgeItems {
	if strings.TrimSpace(ki.Title) == "" || strings.TrimSpace(ki.Content) == "" {
		continue
	}
	if l.mongoClient != nil {
		l.saveKnowledgeItemToMongo(bgCtx, ki, conversationID)
	}
}
```

`saveFactToMongo` dùng `UpdateOne` với `SetUpsert(true)`, filter theo **cả `key` và `tenantId`** (bắt buộc — nếu thiếu `tenantId` trong filter, 2 tenant học cùng key `user_name` sẽ upsert đè lên cùng 1 document, rò rỉ dữ liệu chéo tenant):

```go
// services/agent-go/internal/memory/learner.go:143-160 (rút gọn)
filter := bson.M{"key": fact.Key, "tenantId": tenantID}
update := bson.M{
	"$set": bson.M{
		"type": fact.Category, "key": fact.Key, "value": fact.Value,
		"source": "autonomous_reflection", "confidence": fact.Confidence,
		"embedding": emb, "conversationId": conversationID,
		"tenantId": tenantID, "updatedAt": now,
	},
	"$setOnInsert": bson.M{"_id": bson.NewObjectID(), "createdAt": now},
}
coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
```

`saveKnowledgeItemToMongo` thì lưu knowledge item vào collection **`documents`** — CHUNG với collection RAG dùng cho `rag.search` — dưới dạng một "tài liệu tự sinh" (`documentId = "learned-<slug-title>"`), để agent có thể recall lại kiến thức đã học qua đúng con đường RAG search, không cần một collection/API riêng.

### 1.5 Recall

`recall.go` implement recall theo mô hình **3 bước fallback tăng dần chi phí** — chỉ đi tới bước đắt hơn khi bước rẻ không ra kết quả:

```go
// services/agent-go/internal/memory/recall.go:39-109 (rút gọn, giữ các đoạn quan trọng)
func RecallNode(store *Store) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		tenantID := middleware.GetTenantID(ctx)

		query := s.LastUserContent()
		if query == "" {
			return agent.NodeModel, nil
		}

		results := make(map[string]string)

		// Step 1: Direct key lookup from keyword mapping (fast, accurate).
		lower := strings.ToLower(query)
		for keyword, keys := range keywordToKeys {
			if strings.Contains(lower, keyword) {
				for _, k := range keys {
					if v, ok := store.Get(tenantID, k); ok {
						results[k] = v
					}
				}
			}
		}

		// Step 2: Full-text substring search as fallback.
		fullResults := store.Search(tenantID, query)
		for k, v := range fullResults {
			results[k] = v
		}

		// Step 3: Embedding-based semantic search — CHỈ khi 2 bước rẻ ở trên
		// không ra gì.
		if len(results) == 0 {
			semResults, err := store.SemanticSearch(tenantID, query, 5)
			if err != nil {
				slog.Warn("memory: semantic search failed, continuing with keyword results", "err", err)
			}
			for _, item := range semResults {
				if _, exists := results[item.Key]; !exists {
					results[item.Key] = item.Value
				}
			}
		}

		if len(results) == 0 {
			return agent.NodeModel, nil
		}

		items := make([]string, 0, len(results))
		for k, v := range results {
			items = append(items, fmt.Sprintf("%s: %s", k, v))
		}
		s.RecalledMemories = items // feed vào State — nodeModel sẽ dệt vào system prompt
		emit(agent.MemoryEvent("recalled: " + strings.Join(items, " | ")))

		return agent.NodeModel, nil
	}
}
```

**3 bước cụ thể:**

1. **Keyword-to-key lookup** — bảng tĩnh `keywordToKeys` (viết tay, có cả tiếng Việt + tiếng Anh) map keyword trong câu hỏi sang key trong Store:
   ```go
   // services/agent-go/internal/memory/recall.go:13-37
   var keywordToKeys = map[string][]string{
       "tên": {"user_name"}, "name": {"user_name"},
       "thích": {"like"}, "like": {"like"},
       "ghét": {"dislike"}, "ở": {"user_location"}, "sống": {"user_location"},
       "làm": {"user_job"}, "job": {"user_job"}, "email": {"email"},
       "số điện thoại": {"phone"}, "muốn": {"want"}, "cần": {"need"},
       "nhớ": {"fact"}, "remember": {"fact"},
       // ...
   }
   ```
   Đây là bảng **dùng chung với `worthLearning`** (1.4) — comment ghi rõ "một nguồn sự thật" (single source of truth) để tránh 2 bảng lệch nhau.
2. **Full-text substring search** — `store.Search(tenantID, query)`: so khớp substring (không phân biệt hoa/thường) trên cả `key` và `value` của mọi entry trong tenant đó (xem `Store.Search`, 1.7).
3. **Semantic search (cosine similarity qua embedding)** — chỉ chạy khi bước 1+2 đều rỗng. Gọi `store.SemanticSearch(tenantID, query, 5)` — **top-K = 5**, không có ngưỡng threshold tối thiểu cứng (lấy top-5 theo điểm số dù điểm thấp, không loại theo cutoff). Kết quả bị bảo vệ khỏi ghi đè (`if _, exists := results[item.Key]; !exists`) — nếu 1 key đã có từ bước 1/2 thì semantic không đè lên (test `TestRecallNode_SemanticDoesNotOverrideKeyword`).

Lý do thiết kế 3-bước-fallback được giải thích rõ trong comment và được test chứng minh bằng `cost_reduction_test.go` (`TestRecallNode_KhongGoiEmbeddingKhiKeywordDaTimDuoc` / `TestRecallNode_VanGoiEmbeddingKhiKeywordKhongRaGi`): gọi Voyage API tốn tiền + ~100-300ms latency mỗi lần, nên chỉ nên trả giá đó khi thật sự cần (câu hỏi diễn đạt khác cách fact được lưu, keyword+fulltext không bắt được).

Kết quả recall được ghi vào `s.RecalledMemories []string` (field trong `agent.State`) — đây chính là điểm nối giữa memory pipeline và model pipeline: **không chỉ emit ra SSE cho UI xem, mà thực sự được `nodeModel` dệt vào system prompt gửi cho LLM** (xem 1.9 và trích dẫn ở `node_model.go` dưới):

```go
// services/agent-go/internal/agent/node_model.go:112-119
if len(s.RecalledMemories) > 0 {
	var mb strings.Builder
	mb.WriteString("\n\n[BỘ NHỚ] — Các quy ước, sở thích và kinh nghiệm kỹ thuật đã học từ người dùng (ưu tiên tuân thủ khi đưa ra giải pháp):\n")
	for _, m := range s.RecalledMemories {
		mb.WriteString("- " + m + "\n")
	}
	systemPrompt += mb.String()
}
```

### 1.6 Reflection

`reflection.go` **có** implement một dạng "reflection" theo nghĩa "LLM tự nhìn lại hội thoại", nhưng **không giống Generative Agents (Park et al.)** ở phần cốt lõi: không có importance score, không có recency/relevance ranking, không tổng hợp NHIỀU memory thô cũ thành 1 insight bậc cao hơn. Thứ nó làm thực chất là **structured extraction bằng LLM** — một lượt gọi LLM với system prompt cố định, ép output theo JSON schema, để trích ra 2 loại dữ liệu: `user_facts` và `knowledge_items`. Về bản chất kỹ thuật, nó gần với "function calling để trích xuất dữ liệu có cấu trúc" hơn là "reflection" học thuật.

**System prompt thật** (rút gọn phần đầu, giữ đủ cấu trúc JSON và ghi chú escaping quan trọng):

```go
// services/agent-go/internal/memory/reflection.go:97-129 (nguyên văn, rút bớt phần lặp)
const reflectionSystemPrompt = `Bạn là hệ thống Trích Xuất & Học Tri Thức Tự Động (Autonomous Knowledge & Memory Learner) cho AI Assistant J.A.R.V.I.S.
Nhiệm vụ của bạn là phân tích đoạn hội thoại vừa diễn ra giữa Người dùng (User) và Trợ lý (Assistant) để trích xuất:

1. "user_facts": Các thông tin, sở thích, tech stack, quy ước làm việc hoặc luật mới mà người dùng đề cập (chỉ lấy thông tin rõ ràng và có ích cho các phiên sau).
   - Category: "tech_stack" | "coding_preference" | "user_profile" | "rule"
   - Key: tên định danh ngắn gọn bằng tiếng Anh (vd: "web_framework", "css_style", "user_role")
   - Value: giá trị chi tiết
   - Confidence: độ tin cậy từ 0.7 đến 1.0

2. "knowledge_items": Các bài học kinh nghiệm, giải pháp kỹ thuật vừa sửa lỗi thành công, hoặc quy chuẩn kiến trúc quan trọng được giải quyết trong hội thoại (chỉ tạo khi có vấn đề kỹ thuật hoặc bài học thực sự có giá trị).
   - Title: Tiêu đề rõ ràng
   - Summary: Tóm tắt 1-2 câu
   - Tags: Mảng các từ khóa liên quan
   - Content: Nội dung bằng Markdown giải thích vấn đề và cách giải quyết — SÚC TÍCH, tối đa khoảng 120 từ
   - TỐI ĐA 2 knowledge_items mỗi lượt. Chọn 2 cái giá trị nhất, bỏ phần còn lại.

BẮT BUỘC trả về định dạng JSON thuần túy ...
Nếu không có thông tin hay bài học nào mới đáng nhớ, hãy trả về:
{"user_facts": [], "knowledge_items": []}

QUAN TRỌNG về escaping JSON: mọi dấu ngoặc kép (") xuất hiện BÊN TRONG một
giá trị chuỗi ... BẮT BUỘC phải escape thành \". ...`
```

Hàm chính, chữ ký `ReflectAndExtract(ctx context.Context, p provider.Provider, model string, messages []provider.Message) (*ReflectionResult, error)`:

```go
// services/agent-go/internal/memory/reflection.go:136-201 (rút gọn, giữ luồng chính)
func ReflectAndExtract(ctx context.Context, p provider.Provider, model string, messages []provider.Message) (*ReflectionResult, error) {
	if len(messages) == 0 || p == nil {
		return &ReflectionResult{}, nil
	}

	// Chỉ lấy các tin nhắn CUỐI (lọc role user/assistant TRƯỚC khi cắt, để tool
	// message không chiếm slot): learner chạy sau mỗi câu trả lời nên lượt cũ
	// đã được reflect ở lần gọi trước — gửi lại là trả tiền lặp.
	var dialogue []provider.Message
	for _, m := range messages {
		if m.Role == provider.RoleUser || m.Role == provider.RoleAssistant {
			dialogue = append(dialogue, m)
		}
	}
	if len(dialogue) > maxReflectionMessages { // = 4 (2 cặp trao đổi)
		dialogue = dialogue[len(dialogue)-maxReflectionMessages:]
	}

	// build convText, cắt theo RUNE tại maxReflectionConvRunes (= 2500)

	var lastErr error
	maxTokens := reflectionMaxTokens // = 16384
	for attempt := 1; attempt <= maxReflectionAttempts; attempt++ { // = 2
		res, err := reflectOnce(ctx, p, model, trimmedConv, maxTokens)
		if err == nil {
			return res, nil
		}
		switch {
		case errors.Is(err, errReflectionTimeout):
			return &ReflectionResult{}, nil // hết thời gian → KHÔNG retry
		case errors.Is(err, errReflectionTruncated):
			maxTokens *= 2 // chạm trần token → retry với ngân sách GẤP ĐÔI
			lastErr = err
		case errors.Is(err, errReflectionParseFailed):
			lastErr = err // JSON lỗi cú pháp → retry cùng ngân sách
		default:
			return nil, err // lỗi Generate() thật (network/API) → không retry, trả lỗi luôn
		}
	}
	return &ReflectionResult{}, nil // hết attempt → bỏ qua êm
}
```

Vài chi tiết kỹ thuật đáng chú ý (đúng kiểu "hardening sau khi vấp lỗi thật trong log dev" — thấy rõ qua comment):

- **Phân loại lỗi rất kỹ** để quyết định retry đúng cách: lỗi timeout (context hết hạn giữa stream, channel đóng không có `ChunkDone`/`ChunkError`) → **không retry** (ngân sách thời gian đã cạn, retry chắc chắn cũng timeout); lỗi bị cắt do chạm `MaxTokens` (`FinishReason == FinishLength`) → retry với **maxTokens x2**; lỗi parse JSON thông thường → retry với **cùng** ngân sách.
- `ThinkingLevel: provider.ThinkingOff` — bắt buộc tắt "thinking" của model (comment ghi rõ đã verify bằng API thật với `deepseek-v4-flash`: token suy luận (reasoning) bị tính vào `max_tokens`, nên nếu bật thinking cho một task extract theo schema cố định thì dễ chạm trần và bị cắt output).
- `repairTruncatedJSON` — cơ chế "vá" JSON bị cắt cụt giữa chừng (đóng string/bracket còn dở theo LIFO, bỏ trailing comma) để **cứu được phần dữ liệu đã hoàn chỉnh trước điểm cắt** thay vì mất trắng toàn bộ theo kiểu all-or-nothing của `json.Unmarshal`.

### 1.7 Store

`store.go` là kho lưu memory **in-memory, thread-safe, partition theo tenant**, có tuỳ chọn mirror ra MongoDB:

```go
// services/agent-go/internal/memory/store.go:14-38
type storeEntry struct {
	value     string
	embedding []float64
}

type Store struct {
	mu       sync.RWMutex
	data     map[string]map[string]storeEntry // tenantID -> key -> entry
	embedder Embedder
}

func NewStore() *Store {
	return &Store{data: make(map[string]map[string]storeEntry)}
}
```

Schema thực tế của "một mẩu memory" tồn tại ở **2 chỗ khác nhau** trong codebase, không hợp nhất:

1. **`storeEntry`** (dùng nội bộ `Store` trong RAM) — chỉ có `value` + `embedding`; `key` là key của map ngoài, `tenantID` là key của map ngoài cùng. Không có `type`/`confidence`/`source` — những field này chỉ tồn tại ở tầng Mongo.
2. **`Item`** (định nghĩa trong `memory.go`, dùng cho merge/validate/API serialize — không phải struct lưu trong `Store`):
   ```go
   // services/agent-go/internal/memory/memory.go:22-29
   type Item struct {
       Type       MemoryType `bson:"type"                json:"type"`
       Key        string     `bson:"key"                 json:"key"`
       Value      string     `bson:"value"               json:"value"`
       Confidence float64    `bson:"confidence"          json:"confidence"`
       Source     string     `bson:"source"              json:"source"`
       Embedding  []float64  `bson:"embedding,omitempty" json:"embedding,omitempty"`
   }
   ```
   `MemoryType` gồm 3 giá trị: `preference` | `fact` | `entity`. `memory.go` cũng có `MergeMemories` (gộp 2 nguồn theo `Type+Key`, ưu tiên confidence cao hơn) và `ValidateItem` — đây là các hàm thuần (pure function), theo comment đầu file là phần khai báo trước cho "phase Mongo sau", **hiện KHÔNG thấy được gọi** ở `ExtractNode`/`RecallNode`/`Learner` trong luồng chính (các luồng đó dùng trực tiếp `Store.Set`/`Get`, không đi qua `Item`/`MergeMemories`) — có dấu hiệu đây là phần thiết kế "để dành" chưa được tích hợp hết vào pipeline thật.
3. **Document Mongo thật** (ghi bởi `Learner.saveFactToMongo`, collection `memories`): `key`, `value`, `type` (= `fact.Category`), `confidence`, `source` (= `"autonomous_reflection"`), `embedding`, `conversationId`, `tenantId`, `createdAt`/`updatedAt` — gần khớp `Item` nhưng thêm `conversationId`/`tenantId`/timestamps.

**Nơi lưu**: mặc định **RAM** (`map[string]map[string]storeEntry`, singleton process-wide, wiring 1 lần trong `main.go`). Nếu Mongo được cấu hình, `Learner` mirror fact/knowledge item xuống 2 collection MongoDB (`memories` cho user facts, `documents` — CHUNG với RAG — cho knowledge items), và `Store.LoadFromMongo` nạp lại từ `memories` vào RAM lúc server khởi động:

```go
// services/agent-go/internal/memory/store.go:221-239 (rút gọn)
func (s *Store) LoadFromMongo(ctx context.Context, mongoClient *mongo.Client) (int, error) {
	if mongoClient == nil {
		return 0, nil // Mongo không cấu hình → no-op, không chặn khởi động
	}
	coll := mongoClient.Collection("memories")
	cursor, err := coll.Find(ctx, bson.M{})
	// ... decode vào []memoryDoc, rồi applyLoadedDocs (merge vào s.data, GHI ĐÈ giá trị RAM cũ)
}
```

Không có Chroma/SQLite trong package này (`chroma` chỉ xuất hiện ở tầng RAG — `internal/tools/rag.go` — cho tài liệu, không liên quan tới long-term memory của user). Cosine similarity cho `SemanticSearch` được implement thuần Go, không qua thư viện vector DB:

```go
// services/agent-go/internal/memory/store.go:241-255
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

`SemanticSearch` duyệt toàn bộ entry của 1 tenant (không có index ANN — brute-force O(n)), tính cosine similarity với query vector, **sort bằng insertion sort thủ công** (comment giải thích: "simple insertion sort since topK là small" — chấp nhận O(n×k) vì k rất nhỏ, tránh phụ thuộc `sort` package cho một selection nhỏ).

### 1.8 Summarize

`summarize.go` — trigger là **số lượng message**, không phải token count: `summarizeThreshold = 15`.

```go
// services/agent-go/internal/memory/summarize.go:11-58
const summarizeThreshold = 15

func SummarizeNode(prov provider.Provider, model string) agent.Node {
	return func(ctx context.Context, s *agent.State, emit agent.EmitFunc) (agent.NodeID, error) {
		if len(s.Messages) <= summarizeThreshold {
			return agent.NodeModel, nil
		}

		dropCount := agent.SafeDropBoundary(s.Messages, len(s.Messages)-summarizeThreshold)
		dropped := s.Messages[:dropCount]

		var noteContent string
		if summary, ok := agent.SummarizeMessages(ctx, prov, model, dropped); ok {
			noteContent = fmt.Sprintf("[Tóm tắt %d tin nhắn trước]: %s", dropCount, summary)
		} else {
			noteContent = fmt.Sprintf("[%d tin nhắn trước đã bị lược bỏ do hội thoại quá dài — không tóm tắt được]", dropCount)
		}

		kept := make([]provider.Message, 0, len(s.Messages)-dropCount+1)
		kept = append(kept, provider.Message{Role: provider.RoleUser, Content: noteContent})
		kept = append(kept, s.Messages[dropCount:]...)
		s.Messages = kept

		emit(agent.MemoryEvent(fmt.Sprintf("summarized: condensed %d messages, keeping %d", dropCount, len(s.Messages)-1)))
		return agent.NodeModel, nil
	}
}
```

Điểm kỹ thuật quan trọng:

- **`dropCount = agent.SafeDropBoundary(...)`** — không cắt cứng đúng `len - 15`, mà dịch biên cắt tới khi tin đầu tiên giữ lại **không phải role `tool`** (tránh cắt giữa cặp `tool_call` (assistant) / `tool_result` (role=tool) — nếu giữ lại 1 `tool_result` mồ côi, provider như Anthropic sẽ từ chối request vì yêu cầu mọi `tool_result` phải khớp 1 `tool_use` đứng trước).
- Note thay thế phần bị drop được gắn **`Role: provider.RoleUser`**, không phải `RoleSystem` — comment giải thích: adapter Anthropic **bỏ qua hoàn toàn** message có `role=system` nằm trong `Messages` (system prompt thật đi qua field `System` riêng của request) — nên nếu note tóm tắt gắn `RoleSystem`, nó sẽ **biến mất âm thầm** mỗi khi Anthropic đang phục vụ request.
- Việc tóm tắt thật được **delegate cho `agent.SummarizeMessages`** (nằm ở package `agent`, file `compaction.go`, không phải trong package `memory`) — gọi 1 lượt LLM **rẻ/nhanh** (model do caller truyền vào, thường là "fast model" cấu hình riêng cho task phụ trợ), với system prompt riêng cho việc nén ngữ cảnh:
  ```go
  // services/agent-go/internal/agent/compaction.go:16, 22, 26, 28
  const compactionMaxTokens = 800
  const compactionTimeout = 12 * time.Second        // ngắn hơn hẳn reflection (45s) vì NẰM TRÊN hot path
  const maxCompactionInputRunes = 6000
  const compactionSystemPrompt = `Bạn là hệ thống nén ngữ cảnh nội bộ cho AI Assistant. Nhiệm vụ DUY NHẤT: đọc đoạn tin nhắn cũ dưới đây và viết một đoạn tóm tắt NGẮN GỌN (3-6 câu), giữ lại thông tin QUAN TRỌNG mà các lượt trả lời SAU có thể cần: tên/thông tin người dùng đã cung cấp, quyết định đã chốt, kết quả tra cứu/tool cụ thể, số liệu/ID quan trọng. Bỏ qua chào hỏi xã giao và nội dung lặp lại. Trả lời THẲNG bằng đoạn tóm tắt, không thêm tiêu đề, không dùng markdown.`
  ```
- `SummarizeMessages` là **best-effort, KHÔNG retry** (khác `ReflectAndExtract` có retry) — vì nằm ngay trên hot path của request (model chính phải đợi nó xong mới chạy tiếp), retry sẽ cộng thêm latency thấy được cho user. Nếu thất bại (`ok=false`), `SummarizeNode` bắt buộc dùng fallback **trung thực**: nói rõ "đã bị lược bỏ — không tóm tắt được", không giả vờ đã tóm tắt (đây là một fix so với "bản cũ" — comment trong code nhắc lại điều này nhiều lần, cho thấy từng có bug hiển thị sai).
- `ThinkingLevel: provider.ThinkingOff` cũng được set ở đây, cùng lý do như reflection (model bật thinking mặc định có thể ăn vào `MaxTokens=800` nhỏ, khiến output rỗng/cắt).

### 1.9 Pipeline tổng hợp

Toàn bộ node graph của một lượt chạy (`Engine.Run`, `internal/agent/engine.go`) đi theo thứ tự cố định sau (rút từ `dispatch()` + `router.go`):

```
NodeRecall → NodeSummarize → NodePlan → NodeModel ⇄ NodeTools (lặp nếu có tool_call)
                                              │
                                   (route: hết tool_call, hết plan)
                                              ▼
                                        NodeReflect (planning, KHÁC memory reflection)
                                              ▼
                                        NodeExtract (memory.ExtractNode — regex)
                                              ▼
                                            NodeEnd
```

Trích dẫn `dispatch()` cho đúng thứ tự transition:

```go
// services/agent-go/internal/agent/engine.go:290-335 (rút gọn)
switch node {
case NodeRecall:
	if e.recallFn != nil { return e.recallFn(ctx, s, emit) }
	return NodeSummarize, nil
case NodeSummarize:
	if e.summarizeFn != nil { return e.summarizeFn(ctx, s, emit) }
	return NodePlan, nil
case NodePlan:
	// ...
case NodeModel:
	return nodeModel(ctx, e, s, emit) // trả NodeTools hoặc NodeReflect/NodeExtract tuỳ router.go
case NodeTools:
	return nodeTools(ctx, e, s, emit) // luôn quay lại NodeModel
case NodeReflect:
	if e.reflectFn != nil { return e.reflectFn(ctx, s, emit) }
	return NodeExtract, nil
case NodeExtract:
	if e.extractFn != nil { return e.extractFn(ctx, s, emit) }
	return NodeEnd, nil
}
```

Lưu ý: **`NodeReflect` ở đây là planning-reflect** (`node_reflect.go` — đánh giá tiến độ multi-step plan), **hoàn toàn khác** với "memory reflection" (`ReflectAndExtract` trong `reflection.go`) — trùng tên "reflect" nhưng 2 khái niệm độc lập, đừng nhầm khi đọc code.

**Wiring thực tế** (`cmd/server/main.go`, mỗi agent — general/code/research — đều gọi `SetMemoryNodes` với 3 node factory dùng chung 1 `Store`):

```go
// services/agent-go/cmd/server/main.go:186-189
generalEngine.SetMemoryNodes(
	memory.RecallNode(store),
	memory.ExtractNode(store),
	memory.SummarizeNode(prov, fastModel(cfg)),
	// ...
)
```

**Luồng đầy đủ của một turn, ghép cả phần đồng bộ (trong graph) và bất đồng bộ (Learner, ngoài graph):**

1. **Request tới** → `chat.go` build `RunInput` (history + user message mới) → gọi `engine.Run()`.
2. **`NodeRecall`** (đầu graph, TRƯỚC khi gọi LLM): `RecallNode` lấy `s.LastUserContent()`, chạy 3-bước fallback (keyword → full-text → semantic nếu 2 bước trên rỗng) trên `Store` (đã namespace theo tenant), ghi kết quả vào `s.RecalledMemories`.
3. **`NodeSummarize`**: nếu `len(s.Messages) > 15`, nén phần đầu bằng 1 lượt LLM rẻ (`agent.SummarizeMessages`), thay bằng 1 note; luôn chạy TRƯỚC khi gọi model chính (vì phải chốt xong `s.Messages` cuối cùng trước khi build request gửi LLM).
4. **`NodeModel`**: build system prompt = static base prompt + `[BỘ NHỚ]` dệt từ `s.RecalledMemories` (bước 2) + override ngôn ngữ per-request → gọi LLM chính, model trả lời (có thể kèm tool_call).
5. Nếu có tool_call → **`NodeTools`** chạy tool, quay lại `NodeModel` (loop tới khi hết tool_call hoặc hết plan).
6. Model trả lời cuối (final answer, không tool_call) → router chuyển tới **`NodeExtract`**: `ExtractNode` quét lại **toàn bộ** `s.Messages` (cả history + turn mới) bằng regex, ghi trực tiếp fact match được vào `Store` (đồng bộ, trong graph, **KHÔNG gọi LLM**) → `NodeEnd` → response SSE hoàn tất trả về client.
7. **NGOÀI graph, SAU KHI response đã stream xong** (`chat.go`, sau `h.runner.Run(...)`): nếu có `h.learner` (đã wiring) và có nội dung trả lời, `chat.go` build lại full message list (`history + userMessage + assistantContent`) rồi gọi `h.learner.LearnFromConversation(r.Context(), fullMsgs, conversationID)`.
8. `LearnFromConversation` chạy `worthLearning` gate (1.4) — nếu qua gate, spawn **goroutine nền** (context riêng, timeout 45s, không phụ thuộc request context đã có thể bị cancel) gọi `ReflectAndExtract` (1.6) — 1 lượt LLM thật, retry tối đa 2 lần theo phân loại lỗi, trả về `user_facts` + `knowledge_items`.
9. Kết quả từ bước 8: `user_facts` được ghi vào **cả** `Store` RAM (`store.Set`, cùng namespace tenant) **và** MongoDB collection `memories` (upsert theo `key+tenantId`, kèm embedding riêng cho fact đó); `knowledge_items` được ghi vào collection `documents` (chung với RAG) dưới dạng tài liệu tự sinh, để lần sau agent tìm lại được qua `rag.search` bình thường.
10. **Lượt chat kế tiếp** của cùng tenant: `NodeRecall` (bước 2) sẽ nhìn thấy fact mới học được ở bước 9 (nếu key/keyword khớp) — đây là điểm khép vòng "học ở lượt N, nhớ lại ở lượt N+1" của long-term memory. Nếu server restart, `Store.LoadFromMongo` (chạy 1 lần lúc khởi động trong `main.go`) nạp lại toàn bộ fact từ collection `memories` vào RAM, nên fact học được không mất khi deploy lại.

Tổng kết bằng một bảng đối chiếu ngắn giữa 2 pipeline "học" song song:

| | `ExtractNode` (1.3) | `Learner`/`ReflectAndExtract` (1.4 + 1.6) |
|---|---|---|
| Vị trí | Trong node graph, node cuối (đồng bộ) | Ngoài graph, chạy nền sau response (bất đồng bộ) |
| Cơ chế | Regex pattern match | LLM call với JSON schema output |
| Chi phí | ~0 (CPU thuần) | 1 lượt LLM/turn (có gate để giảm tần suất) |
| Gate | Không — luôn chạy | `worthLearning` (keyword OR độ dài câu) |
| Nơi lưu | `Store` RAM only | `Store` RAM + Mongo (`memories`, `documents`) |
| Độ chính xác | Thấp (chỉ bắt câu đúng khuôn mẫu) | Cao hơn (LLM hiểu ngữ nghĩa tự do) |
| Độ trễ cảm nhận bởi user | Có (trong graph, trước khi response kết thúc) | Không (chạy sau khi đã trả lời xong) |
## 2. Guardrails

Package `internal/guardrails` (services/agent-go/internal/guardrails/) chịu trách nhiệm cho 3 lớp an toàn khác nhau của agent runtime: phát hiện vòng lặp kẹt (circuit breaker), chặn tool nguy hiểm (guard), và làm sạch input người dùng (input guard). Đây là 3 khái niệm khác nhau bị gộp chung một package, nên trước khi đọc code cần tách rõ khái niệm chuẩn ra khỏi cái thực sự được implement — vì (spoiler) tên "CircuitBreaker" trong code này **không** làm đúng những gì circuit breaker pattern kinh điển làm.

### 2.1 Circuit Breaker

#### Khái niệm chuẩn (lý thuyết)

Circuit breaker là một resilience pattern mượn từ ngành điện: khi dòng điện (hay ở đây là lỗi) vượt ngưỡng, "ngắt mạch" để bảo vệ hệ thống phía sau, thay vì tiếp tục đập vào một service đang chết. Pattern chuẩn (Netflix Hystrix, Polly, Martin Fowler's CircuitBreaker) có đúng 3 state:

1. **Closed** (đóng mạch) — trạng thái bình thường. Mọi request đi qua, đồng thời bộ đếm lỗi được theo dõi (đếm lỗi liên tiếp hoặc tỷ lệ lỗi trong sliding window). Khi số lỗi vượt threshold → chuyển sang **Open**.
2. **Open** (ngắt mạch) — mọi request bị chặn **ngay lập tức** (fail fast), không gọi service phía sau nữa, trả lỗi tức thì. Sau một khoảng **cooldown/timeout**, chuyển sang **Half-Open**.
3. **Half-Open** (thử mở lại) — cho một (hoặc một số nhỏ) request thật đi qua để "dò" xem service đã hồi phục chưa. Nếu request đó thành công → quay lại **Closed** (reset bộ đếm). Nếu vẫn lỗi → quay lại **Open**, cooldown tiếp (thường tăng theo cấp số nhân — exponential backoff).

Mục đích: (a) fail fast — không để user/caller phải chờ timeout của một service chắc chắn sẽ lỗi; (b) bảo vệ service đang gặp sự cố khỏi bị dội thêm traffic trong lúc nó đang cố hồi phục; (c) tự động dò và phục hồi mà không cần con người can thiệp.

Trong AI agent, "input guardrail" là một khái niệm hoàn toàn khác — nó không liên quan tới lỗi hạ tầng, mà là lớp kiểm duyệt nội dung *trước khi* nội dung đó chạm vào LLM hoặc tool thật. Các mối lo kinh điển:
- **Prompt injection**: user (hoặc nội dung lấy từ web/tool) chứa chỉ dẫn cố tình đánh lừa LLM bỏ qua system prompt ("ignore previous instructions", "you are now DAN"...).
- **Rate limiting**: chặn spam / lạm dụng API bằng cách giới hạn số request trong một đơn vị thời gian, thường bằng token bucket (nạp token đều theo thời gian, mỗi request tiêu 1 token, hết token thì chặn — cho phép burst có kiểm soát) hoặc sliding window (đếm request trong một cửa sổ thời gian trượt, chính xác hơn fixed window nhưng tốn bộ nhớ hơn).
- **PII / nội dung nhạy cảm**: regex hoặc NER để phát hiện số thẻ, email, SSN... trước khi log hoặc gửi ra ngoài.
- **Length validation**: chặn input quá dài để tránh tốn token/tiền hoặc DoS.

#### Code thật: circuit_breaker.go — KHÔNG phải 3-state pattern

Đọc `services/agent-go/internal/guardrails/circuit_breaker.go`, đây **không** phải circuit breaker 3 trạng thái theo đúng định nghĩa kinh điển. Đây là một **bộ đếm lỗi lặp liên tiếp (consecutive-repeat counter)** để phát hiện "stuck loop" — tức là khi agent (LLM) gọi lại đúng một tool với đúng args giống hệt lần trước, nhiều lần liên tiếp (một dạng vòng lặp vô hạn/kẹt logic của chính LLM, không phải lỗi hạ tầng).

Comment đầu file đã tự gọi tên đúng bản chất:

```go
// services/agent-go/internal/guardrails/circuit_breaker.go:1-2
// Package guardrails provides safety checks for the agent runtime:
// circuit breaker (stuck loop detection) and tool guard (read/write/destructive).
```

Struct chỉ có 2 khái niệm: `count` (đang đếm bao nhiêu lần liên tiếp) và `maxRepeats` (threshold) — **không có state enum nào cho open/closed/half-open**:

```go
// services/agent-go/internal/guardrails/circuit_breaker.go:22-44
// callKey identifies a unique (tool, args) pair for dedup tracking.
type callKey struct {
	tool string
	args string // string(json.RawMessage) — stable comparison
}

type CircuitBreaker struct {
	mu         sync.Mutex
	last       callKey
	count      int
	maxRepeats int
}
```

`Record` — hàm lõi — chỉ làm một việc: so key (tool+args) hiện tại với key lần gọi trước; nếu **giống** thì tăng `count`, nếu **khác** thì reset `count=1`. Khi `count >= maxRepeats` thì trả lỗi:

```go
// services/agent-go/internal/guardrails/circuit_breaker.go:64-80
func (cb *CircuitBreaker) Record(toolName string, args json.RawMessage) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	key := callKey{tool: toolName, args: string(args)}
	if key == cb.last {
		cb.count++
	} else {
		cb.last = key
		cb.count = 1
	}

	if cb.count >= cb.maxRepeats {
		return &StuckLoopError{Tool: toolName, Count: cb.count}
	}
	return nil
}
```

Ngưỡng lỗi (threshold) = `maxRepeats`, mặc định **3** nếu constructor nhận giá trị <= 0:

```go
// services/agent-go/internal/guardrails/circuit_breaker.go:54-60
func NewCircuitBreaker(maxRepeats int) *CircuitBreaker {
	if maxRepeats <= 0 {
		maxRepeats = 3
	}
	return &CircuitBreaker{maxRepeats: maxRepeats}
}
```

Điểm quan trọng nhất khác biệt với circuit breaker chuẩn: **không có cooldown theo thời gian, không có state Open tồn tại độc lập, không có Half-Open tự động dò lại**. Sau khi đã báo lỗi (`count >= maxRepeats`), nếu vẫn tiếp tục gọi đúng key đó, `count` cứ tăng tiếp và tiếp tục lỗi mãi — test xác nhận rõ điều này:

```go
// services/agent-go/internal/guardrails/circuit_breaker_test.go:139-169 (TestCircuitBreaker_KeepsErroring)
err := cb.Record("t", args)
// ...
if loopErr.Count != 3 {
    t.Fatalf("Count = %d, want 3", loopErr.Count)
}
// Sau khi đã lỗi, count tiếp tục tăng → vẫn lỗi.
err = cb.Record("t", args)
loopErr, ok = err.(*StuckLoopError)
// ...
if loopErr.Count != 4 {
    t.Fatalf("Count = %d, want 4", loopErr.Count)
}
```

Cách "hồi phục" duy nhất là: (1) gọi một tool/args **khác** (tự nhiên reset counter, vì key đổi), hoặc (2) gọi `Reset()` thủ công:

```go
// services/agent-go/internal/guardrails/circuit_breaker.go:82-88
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.last = callKey{}
	cb.count = 0
}
```

Nói cách khác, đây gần với pattern **"loop guard" / "dedup-based stuck detector"** hơn là circuit breaker thật. Nó không bảo vệ khỏi lỗi hạ tầng (network, 5xx, timeout) mà bảo vệ khỏi lỗi *hành vi của LLM* (LLM cứ lặp lại một tool call giống hệt nhau — dấu hiệu điển hình của reasoning loop bị kẹt).

Một chi tiết thiết kế quan trọng khác nằm ở comment tiếng Việt trong file (dòng 32-38): scope của `CircuitBreaker` là **một lượt chạy (một Run)**, không phải instance global chia sẻ giữa nhiều request đồng thời — vì nếu share, 2 user hỏi trùng câu sẽ bị tính chung một counter (false positive), hoặc 2 run song song ghi đè `last` của nhau khiến loop thật không bị phát hiện (false negative). Vì vậy `Engine.Run` tạo **một CircuitBreaker mới cho mỗi Run**; instance nào truyền qua `SetCircuitBreaker` chỉ đóng vai "mẫu cấu hình" để lấy `MaxRepeats()`.

**Điểm thú vị để trả lời phỏng vấn**: nếu được hỏi "circuit breaker thật (có cooldown, exponential backoff, tự phục hồi) nằm ở đâu trong codebase này?" — câu trả lời đúng là **không phải** `guardrails.CircuitBreaker`, mà là `provider/fallback/fallback.go` (xem mục 3.4): nơi đó có `coolUntil` (thời điểm hết cooldown), `failures` (đếm lỗi liên tiếp), và backoff tăng theo cấp số nhân `cooldown * (1 << min(fails-1, 4))` — đúng chất "circuit breaker cho LLM provider" hơn hẳn, chỉ là không đặt tên `CircuitBreaker`.

### 2.2 Guard

`guard.go` guard **tool call** — cụ thể là quyết định một tool có được phép **thực thi** hay không, dựa trên `Kind()` của tool (không phải guard output/text của LLM). Đây là kiểm soát ở tầng hành động (action-level), tương tự khái niệm "human-in-the-loop" (HITL) cho các thao tác nguy hiểm.

```go
// services/agent-go/internal/guardrails/guard.go:10-38
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
```

3 mức phân loại theo `tools.Kind`:
- `KindRead` — luôn cho qua, không log gì thêm (đọc dữ liệu là an toàn, không có side-effect).
- `KindWrite` — cho qua nhưng ghi log `slog.Info` để có audit trail (có side-effect nhưng không phá hủy dữ liệu).
- `KindDestructive` — **luôn chặn**, trả `NeedConfirmationError` để buộc tầng gọi (engine/UI) phải xin xác nhận người dùng trước khi thực thi thật (ví dụ xoá task, xoá file).
- Giá trị `Kind` không nằm trong 3 case trên (test dùng `tools.Kind(42)`) → lỗi rõ ràng, không âm thầm cho qua ("fail closed" — mặc định từ chối khi không nhận diện được, đây là nguyên tắc an toàn đúng: unknown → deny, không phải unknown → allow).

`CheckTool` được gọi trước khi Execute thật của tool chạy (nằm trong luồng `node_tools`/engine — không thuộc phạm vi file được yêu cầu đọc ở đây, nhưng vị trí logic là: LLM sinh ToolCall → engine gọi `guardrails.CheckTool` → nếu lỗi `NeedConfirmationError` thì dừng lại chờ user confirm, không phải lỗi kiểu retry/fallback như ở provider layer).

### 2.3 Input Guard

`input.go` implement `ValidateUserInput` — hàm duy nhất, chặn **đúng 3 loại vi phạm cụ thể**, theo thứ tự kiểm tra: length → prompt injection → XSS. **Không có rate limiting nào trong file này** (không token bucket, không sliding window) — và grep toàn bộ `services/agent-go/internal` cũng không thấy bất kỳ implementation rate limiter/throttle nào trong cả service. Vậy câu trả lời rõ ràng: **agent-go hiện tại không có rate limiting ở tầng guardrail** (nếu có rate limit thật thì nó phải nằm ở BFF Node/Fastify hoặc infra/gateway phía trước, không phải trong package này).

```go
// services/agent-go/internal/guardrails/input.go:10-24
// Common prompt injection patterns targeting LLM instruction override.
// Does NOT block legitimate SQL/code questions — only explicit hijack attempts.
var promptInjectionPattern = regexp.MustCompile(
	`(?i)(?:` +
		`\bignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|messages?|conversations?|text|context)\b|` +
		`\byou\s+are\s+now\s+(DAN|jailbreak|a\s+different)\b|` +
		`\bforget\s+(everything|all\s+(previous|prior)\s+instructions?)\b|` +
		`\bsystem\s*:\s*(override|prompt|instruction|you\s+are)\b|` +
		`\bprint\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\b|` +
		`\breveal\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\b|` +
		`\bwhat\s+(is|are)\s+(your|the)\s+(system\s+)?(instructions?|prompts?|rules?)\??\s*$|` +
		`\bnew\s+instructions?\s*:|` +
		`\bfrom\s+now\s+on\s+you\s+are\b` +
		`)`,
)
```

Đây **chính là prompt injection detection** thật (case-insensitive `(?i)`), nhắm cụ thể vào các mẫu câu "hijack" hệ thống, gồm 9 nhóm pattern:
1. `ignore (all) previous/prior/above instructions/prompts/messages/conversations/text/context` — kiểu injection kinh điển nhất.
2. `you are now DAN/jailbreak/a different [model]` — jailbreak persona switch.
3. `forget everything / forget all previous instructions`.
4. `system: override/prompt/instruction/you are` — giả lập một message role=system giữa nội dung user.
5. `print your/the (system) instructions/prompts/rules`.
6. `reveal your/the (system) instructions/prompts/rules`.
7. `what is/are your/the (system) instructions/prompts/rules?` — pattern này neo `$` ở cuối chuỗi (`\s*$`) để **không** chặn câu hỏi chung chung như "what is a system prompt in LLMs?" (test `TestValidateUserInput_Benign` xác nhận câu này KHÔNG bị chặn, vì nó không kết thúc đúng ngay sau "rules/prompts/instructions").
8. `new instructions:` — chèn "chỉ thị mới" giả danh hệ thống.
9. `from now on you are` — đổi persona.

Comment đầu quy định rõ ranh giới: **không chặn** câu hỏi hợp pháp về SQL/code (test benign có "how do I write a SQL SELECT statement?", "explain the eval function in Python" — đều pass), chỉ chặn hijack attempt rõ ràng nhờ dùng word-boundary (`\b`) và anchor chặt (ví dụ đòi có dấu `:` sau "system" hoặc "new instructions", để câu như "system design interview questions" không khớp).

Pattern XSS/script injection — tách riêng, không liên quan injection LLM mà là injection kiểu web (phòng trường hợp nội dung này bị echo ra HTML ở phía client):

```go
// services/agent-go/internal/guardrails/input.go:27-31
var xssPattern = regexp.MustCompile(
	`(?i)<script[\s>]|</script>|javascript\s*:|on\w+\s*=\s*["']|` +
		`onerror\s*=|onload\s*=|onclick\s*=|eval\s*\(|` +
		`document\.cookie|document\.write|<iframe|<object|<embed`,
)
```

Chặn: `<script>`/`</script>`, `javascript:` URI scheme, mọi `on*="..."` event-handler attribute (generic `on\w+\s*=` + các handler cụ thể onerror/onload/onclick), `eval(...)` (gọi hàm — có `(` để không chặn câu "explain the eval function"), `document.cookie`, `document.write(...)`, và các tag nhúng nguy hiểm `<iframe>`/`<object>`/`<embed>`.

Cuối cùng, length limit — check **đầu tiên**, trước cả injection/XSS (test `TestValidateUserInput_LengthLimit` xác nhận thứ tự này: input vừa dài vừa có injection → trả `ErrInputTooLong`, không phải `ErrPromptInjection`):

```go
// services/agent-go/internal/guardrails/input.go:33-55
var (
	ErrPromptInjection = errors.New("input contains disallowed prompt injection pattern")
	ErrXSSInjection    = errors.New("input contains disallowed XSS/script pattern")
	ErrInputTooLong    = errors.New("input exceeds maximum allowed length")
)

const MaxInputLength = 4000

// ValidateUserInput checks a user message for prompt injection, XSS, and
// length limits. Returns nil if the input is safe, or an error describing
// the violation.
func ValidateUserInput(input string) error {
	if len(input) > MaxInputLength {
		return ErrInputTooLong
	}
	if promptInjectionPattern.MatchString(input) {
		return ErrPromptInjection
	}
	if xssPattern.MatchString(input) {
		return ErrXSSInjection
	}
	return nil
}
```

`MaxInputLength = 4000` (ký tự, không phải token). Không có validate format nào khác (không kiểm tra encoding, không kiểm tra PII/số thẻ/email). Tóm lại: `ValidateUserInput` = length guard + regex-based prompt injection detector + regex-based XSS detector, theo đúng thứ tự đó — không có rate limiting, không có PII detection.

---

## 3. Provider Layer

### 3.1 Strategy + Factory Pattern

Toàn bộ `internal/provider/` là một ví dụ giáo trình của việc kết hợp 2 pattern kinh điển:

- **Strategy pattern**: nhiều "chiến lược" (ở đây là 4 nhà cung cấp LLM — Anthropic, DeepSeek, Gemini, Ollama, cộng thêm `fake.go` cho test) cùng implement một interface chung `provider.Provider`. Phần code gọi (engine, node_model.go) chỉ biết đến interface, không biết — và không cần biết — mình đang nói chuyện với API nào. Có thể đổi "chiến lược" lúc runtime mà không đụng vào logic gọi.
- **Factory pattern**: `provider/factory/factory.go` chịu trách nhiệm **tạo** đúng instance Provider dựa vào config (`cfg.Provider`: "gemini"/"anthropic"/"deepseek"/"auto"), giấu hết chi tiết khởi tạo (API key nào, model nào, cấu hình timeout...) khỏi code gọi. Code gọi (`cmd/server/main.go`, `cmd/jarvis/main.go`) chỉ gọi `factory.New(cfg)` và nhận về một `provider.Provider` — không quan tâm nó là 1 client đơn hay một chuỗi `fallback.Provider` bọc nhiều client.

Bằng chứng interface được tôn trọng triệt để: `fallback.Provider` (mục 3.4) **chính nó cũng implement `provider.Provider`** — tức là một "Strategy" có thể được cấu thành từ nhiều "Strategy" khác thông qua **Decorator/Composite pattern** chồng lên Strategy. Engine hoàn toàn không biết có bao nhiêu client thật đứng sau — nó chỉ gọi `Generate()` một lần.

Xác nhận điểm gọi thực tế trong `internal/agent/node_model.go` (qua grep, không đọc sâu logic engine):

```go
// services/agent-go/internal/agent/node_model.go — các dòng liên quan tới provider
19:	getProvider() provider.Provider
41:	prov := eng.getProvider()
188:	req := provider.GenerateRequest{ ... }
192:		Options: provider.ProviderOptions{ ... }
214:	var toolCalls []provider.ToolCall
216:	var finish provider.FinishReason
220-251: switch theo provider.ChunkText/ChunkToolCall/ChunkUsage/ChunkError/ChunkDone
261:	s.Truncated = finish == provider.FinishLength
293-294:	s.Messages = append(s.Messages, provider.Message{Role: provider.RoleAssistant, ...})
317:	func trimContext(ctx context.Context, prov provider.Provider, model string, s *State, maxTokens int)
```

`node_model.go` **không import** `provider/fallback` hay `provider/factory` — nó chỉ dùng `provider.Provider` (interface) và `provider.GenerateRequest`/`StreamChunk` (types chuẩn hoá). `eng.getProvider()` trả về `e.prov` (`internal/agent/engine.go:185`), một giá trị được **inject từ ngoài** lúc dựng `Engine` — chính là kết quả của `factory.New(cfg)` được gọi tại composition root (`cmd/server/main.go:40`, `cmd/jarvis/main.go:234`). Đây là minh chứng rõ nhất cho Strategy pattern: engine phụ thuộc vào abstraction (`provider.Provider`), việc "strategy nào được chọn" hoàn toàn do factory quyết định ở lớp ngoài, engine không hề biết.

### 3.2 Interface chung

Interface trung tâm — chỉ 2 method:

```go
// services/agent-go/internal/provider/provider.go:8-11
type Provider interface {
	Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
	Name() string
}
```

`Generate` trả về **channel** (không trả toàn bộ response một lần) — bắt buộc mọi provider phải stream, và phải tôn trọng `ctx` (cancel/timeout), đóng channel khi xong. Đây là điểm thiết kế quan trọng: interface ép mọi implementation về cùng một mô hình streaming, dù API gốc của provider có streaming thật (SSE) hay không (Ollama NDJSON cũng được adapter về channel).

Các type chuẩn hoá trong `types.go` — đây là "ngôn ngữ chung" mà mọi adapter phải dịch qua/lại:

```go
// services/agent-go/internal/provider/types.go:10-15
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)
```

```go
// services/agent-go/internal/provider/types.go:17-34
type Attachment struct {
	Type     string `json:"type"`     // "image" or "file"
	Name     string `json:"name"`     // filename
	Data     string `json:"data"`     // base64 for images, text content for files
	MimeType string `json:"mimeType"` // e.g. "image/png", "text/plain"
}

type Message struct {
	Role        Role
	Content     string
	ToolCalls   []ToolCall   // khi assistant yêu cầu gọi tool
	ToolCallID  string       // khi Role=tool: id của tool_call tương ứng
	Attachments []Attachment // image/file attachments on user messages (multimodal)
}
```

```go
// services/agent-go/internal/provider/types.go:36-49
type ToolDef struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

type ToolCall struct {
	ID               string
	Name             string
	Args             json.RawMessage
	ThoughtSignature []byte // Gemini thought_signature (required for multi-turn tool use)
}
```

Đáng chú ý: `ToolCall.ThoughtSignature` là field **chỉ Gemini dùng** (bắt buộc cho multi-turn tool-use của Gemini 3.x reasoning models) nhưng được đưa thẳng vào struct chuẩn hoá — một sự "rò rỉ" nhỏ của chi tiết provider cụ thể vào abstraction chung, đánh đổi lấy việc không cần thêm một cơ chế side-channel riêng để mang dữ liệu này qua các lượt hội thoại.

```go
// services/agent-go/internal/provider/types.go:51-87
type ChunkKind int

const (
	ChunkText ChunkKind = iota
	ChunkToolCall
	ChunkUsage
	ChunkDone
	ChunkError
)

type FinishReason string

const (
	FinishStop      FinishReason = "stop"
	FinishToolCalls FinishReason = "tool_calls"
	FinishLength    FinishReason = "length"
)

type StreamChunk struct {
	Kind     ChunkKind
	Text     string
	ToolCall *ToolCall
	Usage    *Usage
	Err      error
	FinishReason FinishReason
}
```

`FinishReason` là một ví dụ rõ về "chuẩn hoá khác biệt provider": Anthropic trả `"max_tokens"`, DeepSeek/OpenAI-style trả `"length"`, Gemini trả `genai.FinishReasonMaxTokens` — cả 3 đều được map về **cùng một** `provider.FinishLength`. Nhờ vậy code ở tầng trên (`node_model.go:261`: `s.Truncated = finish == provider.FinishLength`) chỉ cần so sánh với 1 hằng số duy nhất, không cần biết provider nào đang chạy.

```go
// services/agent-go/internal/provider/types.go:89-119
type Usage struct {
	InputTokens  int
	OutputTokens int
}

type ThinkingLevel string

const (
	ThinkingOff    ThinkingLevel = "OFF"
	ThinkingLow    ThinkingLevel = "LOW"
	ThinkingMedium ThinkingLevel = "MEDIUM"
	ThinkingHigh   ThinkingLevel = "HIGH"
)

type ProviderOptions struct {
	Model         string
	MaxTokens     int
	ThinkingLevel ThinkingLevel
	Cache         bool // đánh dấu phần ổn định (system+tools+context) để prompt-cache
}

type GenerateRequest struct {
	System   string
	Messages []Message
	Tools    []ToolDef
	Options  ProviderOptions
}
```

`fake.go` cung cấp `FakeProvider` — implement `Provider` bằng cách "replay" một kịch bản chunk lập trình sẵn qua channel, tôn trọng `ctx.Done()`. Đây chính là kỹ thuật kinh điển để test Strategy pattern: test engine/fallback mà không cần gọi mạng thật, chỉ cần `provider.NewFake(...)`.

### 3.3 So sánh 4 provider

Cả 4 adapter đều tuân theo cùng khuôn: (1) hàm dịch request thuần (pure function, không I/O) từ `[]provider.Message`/`[]provider.ToolDef` sang type riêng của SDK/API; (2) gọi API (SDK hoặc raw HTTP+SSE); (3) hàm dịch response — đọc từng event/chunk của provider, emit `provider.StreamChunk` chuẩn hoá; (4) `mapFinishReason`/`mapStopReason` riêng để chuẩn hoá lý do dừng. Nhưng chi tiết transform khác nhau hoàn toàn vì mỗi API có model dữ liệu riêng.

**Anthropic** (`provider/anthropic/anthropic.go`) — dùng SDK chính chủ `anthropic-sdk-go`, message roles map gần 1:1 nhưng `RoleSystem` **không** nằm trong mảng messages (đi qua field `System` riêng của Anthropic API):

```go
// services/agent-go/internal/provider/anthropic/anthropic.go:61-106 (rút gọn)
func toAnthropicMessages(msgs []provider.Message) []sdk.MessageParam {
	out := make([]sdk.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case provider.RoleUser:
			blocks := ... // text block + image blocks (base64)
			out = append(out, sdk.NewUserMessage(blocks...))
		case provider.RoleAssistant:
			blocks := ... // text block + tool_use block cho mỗi ToolCall
			out = append(out, sdk.NewAssistantMessage(blocks...))
		case provider.RoleTool:
			// tool_result là content của 1 USER message theo API Anthropic
			out = append(out, sdk.NewUserMessage(
				sdk.NewToolResultBlock(m.ToolCallID, m.Content, false),
			))
		case provider.RoleSystem:
			continue // đi qua GenerateRequest.System, không nằm trong messages
		}
	}
	return out
}
```

Điểm riêng của Anthropic: tool name bị "sanitize" (đổi `.` thành `_`) lúc gửi lên và "unsanitize" lúc nhận về (`sanitizeToolName`/`unsanitizeToolName`), vì Anthropic không cho phép `.` trong tên tool. Prompt caching cũng là field riêng của Anthropic (`CacheControl` gắn trên block `System` khi `Options.Cache=true`).

Transform response: Anthropic SSE stream các event `content_block_delta` (text) và `content_block_stop` (đánh dấu 1 block — có thể là `tool_use` — đã hoàn tất, khi đó mới emit `ChunkToolCall` với input đã gom đủ):

```go
// services/agent-go/internal/provider/anthropic/anthropic.go:221-243 (rút gọn)
switch event.Type {
case "content_block_delta":
	if event.Delta.Text != "" {
		send(ctx, out, provider.StreamChunk{Kind: provider.ChunkText, Text: event.Delta.Text})
	}
case "content_block_stop":
	idx := int(event.Index)
	if blk := acc.Content[idx]; blk.Type == "tool_use" {
		tc := provider.ToolCall{ID: blk.ID, Name: unsanitizeToolName(blk.Name), Args: blk.Input}
		send(ctx, out, provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &tc})
	}
}
```

`mapStopReason`: `"max_tokens"` → `FinishLength`, `"tool_use"` → `FinishToolCalls`, `"end_turn"/"stop_sequence"` → `FinishStop` (anthropic.go:269-280).

**DeepSeek** (`provider/deepseek/deepseek.go`) — không dùng SDK, tự implement OpenAI-compatible chat-completions qua raw `net/http` + SSE thủ công (`bufio.Scanner`). Message role gần giống chuẩn OpenAI, nhưng có xử lý đặc biệt: assistant có tool_calls thì **không** được có `content` cùng lúc:

```go
// services/agent-go/internal/provider/deepseek/deepseek.go:184-203 (rút gọn)
case provider.RoleAssistant:
	dm := dsMessage{Role: "assistant"}
	if len(m.ToolCalls) > 0 {
		// Assistant with tool calls — must NOT have content (DeepSeek requirement)
		dm.ToolCalls = make([]dsToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			dm.ToolCalls[i] = dsToolCall{ID: tc.ID, Type: "function",
				Function: dsFunctionCall{Name: sanitizeName(tc.Name), Arguments: string(tc.Args)}}
		}
	} else if m.Content != "" {
		dm.Content = m.Content
	}
```

Transform response tự viết tay parser SSE, phải tự gom **incremental tool-call delta** (OpenAI-style: mỗi chunk chỉ gửi một phần nhỏ của `arguments`, phải nối chuỗi lại theo `index`):

```go
// services/agent-go/internal/provider/deepseek/deepseek.go:433-448 (rút gọn)
for _, tc := range delta.ToolCalls {
	for len(toolCalls) <= tc.Index {
		toolCalls = append(toolCalls, pendingTool{})
	}
	pt := &toolCalls[tc.Index]
	if tc.ID != "" { pt.id = tc.ID }
	if tc.Function.Name != "" { pt.name = tc.Function.Name }
	if tc.Function.Arguments != "" { pt.args.WriteString(tc.Function.Arguments) }
}
```

`mapFinishReason`: `"length"` → `FinishLength`, `"tool_calls"` → `FinishToolCalls`, `"stop"` → `FinishStop` (deepseek.go:492-503). Điểm rất đặc trưng của adapter này: xử lý riêng field `reasoning_content` (chain-of-thought của DeepSeek v4, đến qua delta tách biệt khỏi `content`) — **không** emit ra `ChunkText` (không phải câu trả lời cho user) nhưng vẫn đo độ dài để log cảnh báo khi CoT ăn hết ngân sách `max_tokens` khiến `content` rỗng — một bug thật đã gặp trong production (ghi rõ trong comment dòng 85-90, 124-131, 358-371).

DeepSeek cũng là provider duy nhất trong 4 provider có logic **auto-route giữa 2 model** ngay trong adapter (`pickModel`) — chi tiết ở mục 3.5.

**Gemini** (`provider/gemini/gemini.go`) — dùng SDK `google.golang.org/genai`. Khác biệt lớn nhất: role gọi là `RoleModel` (không phải "assistant"), và tool result (`RoleTool`) map thành `FunctionResponse` gắn trong content role `user`, cần tra cứu lại tên tool + `ThoughtSignature` từ message assistant trước đó (Gemini không tự mang theo các field này trong tool-result message):

```go
// services/agent-go/internal/provider/gemini/gemini.go:66-94 (rút gọn)
case provider.RoleAssistant:
	parts := ...
	for _, tc := range m.ToolCalls {
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{ID: tc.ID, Name: tc.Name, Args: rawToMap(tc.Args)},
			ThoughtSignature: tc.ThoughtSignature,
		})
	}
	out = append(out, &genai.Content{Role: genai.RoleModel, Parts: parts})

case provider.RoleTool:
	out = append(out, &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				ID: m.ToolCallID, Name: findToolName(msgs, m.ToolCallID),
				Response: map[string]any{"output": m.Content},
			},
			ThoughtSignature: findThoughtSignature(msgs, m.ToolCallID),
		}},
	})
```

Transform response: Gemini SDK trả về response object có `Candidates[0].Content.Parts`, phải lặp qua từng `part` để phân loại (function call / text / "thought" — Gemini có field `part.Thought` đánh dấu phần suy luận nội bộ, phải loại trừ khỏi `ChunkText`):

```go
// services/agent-go/internal/provider/gemini/gemini.go:272-295 (rút gọn)
for _, part := range resp.Candidates[0].Content.Parts {
	switch {
	case part.FunctionCall != nil:
		args, _ := json.Marshal(part.FunctionCall.Args)
		emit(provider.StreamChunk{Kind: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: part.FunctionCall.ID, Name: part.FunctionCall.Name, Args: args,
			ThoughtSignature: part.ThoughtSignature,
		}})
	case part.Text != "" && !part.Thought:
		emit(provider.StreamChunk{Kind: provider.ChunkText, Text: part.Text})
	}
}
```

`mapFinishReason`: chỉ map 2 case — `genai.FinishReasonMaxTokens` → `FinishLength`, `genai.FinishReasonStop` → `FinishStop` (gemini.go:335-344; không có case riêng cho tool-call, vì Gemini không set `FinishReason` khi model dừng để gọi tool — hành vi khác 2 provider kia).

Một chi tiết riêng của Gemini: nếu **không** có tool nào (function calling tools rỗng) thì tự động bật `GoogleSearch` như built-in tool (gemini.go:211-217, comment giải thích Gemini không cho trộn 2 loại tool cùng lúc).

**`cache.go` — Gemini context caching**: đây là cơ chế **context caching** (không phải KV-cache runtime, mà là caching phía server của Google) cho phần nội dung ổn định (system prompt + tool declarations) để giảm chi phí. `CreateCachedContent` gọi API `Caches.Create` với TTL 1 giờ, gửi `SystemInstruction` + `Tools`, nhận về một `cached.Name` (resource name):

```go
// services/agent-go/internal/provider/gemini/cache.go:17-52 (rút gọn)
func (c *Client) CreateCachedContent(ctx context.Context, systemPrompt string, toolDefs []provider.ToolDef) (string, error) {
	...
	config := &genai.CreateCachedContentConfig{
		TTL:               1 * time.Hour,
		DisplayName:       "jarvis-system-cache",
		SystemInstruction: sysContent,
		Tools:             tools,
	}
	cached, err := c.client.Caches.Create(ctx, c.model, config)
	if err != nil {
		slog.Warn("gemini: failed to create cached content, falling back to uncached", "err", err)
		return "", nil // graceful degradation
	}
	return cached.Name, nil
}
```

Cơ chế hoạt động: cache name (`c.cacheName`) được lưu lại, và mỗi lần `Generate()` sau đó nếu `c.cacheName != ""` thì set `config.CachedContent = c.cacheName` (gemini.go:220-222) — Google API sẽ tính tiền phần đã cache **rẻ hơn nhiều** so với gửi lại nguyên văn system+tools mỗi request. Đây chính là lý do trong `gemini.go` có đoạn log riêng đọc `u.CachedContentTokenCount` (gemini.go:298-313) — để biết prompt-cache có "ăn" (hit) hay không, vì đây là khoản lớn nhất trong hoá đơn khi system prompt lặp lại ~5k token mỗi request. Nếu `Caches.Create` lỗi, code **graceful-degrade**: trả `("", nil)` để caller tiếp tục chạy uncached thay vì fail cứng.

**Ollama** (`provider/ollama/ollama.go`) — local LLM, raw HTTP tới `/api/chat` với NDJSON streaming (không phải SSE `data:` prefix như DeepSeek, mà mỗi dòng JSON là 1 object riêng, đọc bằng `bufio.Scanner` trực tiếp). Vì Ollama không có role "tool" thật trong API chat cũ, adapter giả bằng cách nhồi kết quả tool vào một **user message** dạng text:

```go
// services/agent-go/internal/provider/ollama/ollama.go:162-171 (rút gọn)
case provider.RoleTool:
	om.Role = "user"
	om.Content = fmt.Sprintf("Tool result (call_id=%s): %s", m.ToolCallID, m.Content)
```

Transform response — parser đơn giản nhất trong 4 provider vì API Ollama trả nguyên object `message` mỗi chunk (không cần gom incremental delta như DeepSeek):

```go
// services/agent-go/internal/provider/ollama/ollama.go:215-247 (rút gọn)
func fromOllamaChunk(line []byte) (provider.StreamChunk, error) {
	var c ollamaChunk
	json.Unmarshal(line, &c)

	if c.Done {
		var finish provider.FinishReason
		switch c.DoneReason {
		case "length": finish = provider.FinishLength
		case "stop":   finish = provider.FinishStop
		}
		return provider.StreamChunk{Kind: provider.ChunkDone, FinishReason: finish}, nil
	}
	if len(c.Message.ToolCalls) > 0 {
		tc := c.Message.ToolCalls[0]
		return provider.StreamChunk{Kind: provider.ChunkToolCall,
			ToolCall: &provider.ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments}}, nil
	}
	if c.Message.Content != "" {
		return provider.StreamChunk{Kind: provider.ChunkText, Text: c.Message.Content}, nil
	}
	return provider.StreamChunk{}, nil
}
```

Ollama cũng là provider duy nhất có thêm `Embed()` (`embed.go`) — không thuộc interface `provider.Provider` (không có `Embed` trong interface chung), gọi riêng `/api/embed` với model cứng `"nomic-embed-text"` để phục vụ RAG/vector search local, tách hẳn khỏi luồng chat.

**Tóm tắt khác biệt 4 provider ở transform layer**:

| Provider | Cơ chế gọi | Role đặc biệt | Tool-call streaming | Field riêng |
|---|---|---|---|---|
| Anthropic | SDK chính chủ, SSE qua `stream.Next()` | System tách riêng khỏi messages | Emit khi `content_block_stop` (đã gom đủ input) | `CacheControl` (prompt cache), sanitize tên tool `.`→`_` |
| DeepSeek | raw HTTP + tự parse SSE | Assistant có tool_calls thì không kèm content | Gom incremental delta theo `index`, flush khi finish_reason | `reasoning_content` (CoT) đo riêng, không emit text |
| Gemini | SDK `genai`, iterator `range GenerateContentStream` | `RoleModel` thay "assistant"; tool-result phải tra lại tên tool + ThoughtSignature | Emit ngay khi có `FunctionCall` part | `ThoughtSignature`, `part.Thought` (ẩn CoT), context cache |
| Ollama | raw HTTP + NDJSON | Tool result giả làm user message text | Chunk đã là tool call đầy đủ, không cần gom | `Embed()` riêng ngoài interface chung |

### 3.4 Fallback chain

`provider/fallback/fallback.go` implement chính `provider.Provider` — tức tự nó là một "provider" trong mắt code gọi, nhưng bên trong bọc **nhiều** provider thật theo thứ tự ưu tiên cố định (Decorator/Composite trên Strategy). Comment đầu file mô tả đúng bản chất:

```go
// services/agent-go/internal/provider/fallback/fallback.go:1-8
// Package fallback implements automatic provider failover: when the primary
// LLM provider fails (rate limit, timeout, server error), requests are
// transparently retried on the next provider in the chain.
//
// Pattern: Primary → Fallback1 → Fallback2 → ... → error if all fail.
// Recovery: failed providers are retried after a cooldown period.
// Health tracking: counts consecutive failures and cooldown per provider.
```

**Thứ tự fallback là theo đúng thứ tự tham số truyền vào `New(...)`** (không có logic chọn ngẫu nhiên hay theo tải) — tức là do caller (factory.go, mục 3.5) quyết định, `fallback.go` chỉ thi hành đúng thứ tự đó tuần tự:

```go
// services/agent-go/internal/provider/fallback/fallback.go:36-50
func New(cooldown time.Duration, providers ...provider.Provider) (*Provider, error) {
	if len(providers) < 2 {
		return nil, fmt.Errorf("fallback: need at least 2 providers, got %d", len(providers))
	}
	if cooldown < 0 {
		cooldown = 30 * time.Second
	}
	chain := make([]namedProvider, len(providers))
	for i, p := range providers {
		chain[i] = namedProvider{name: p.Name(), prov: p}
	}
	return &Provider{chain: chain, cooldown: cooldown}, nil
}
```

**Điều kiện chuyển sang provider kế tiếp** — hàm `isRetryable` là "luật" quyết định lỗi nào được coi là tạm thời (retry qua provider khác) và lỗi nào là lỗi vĩnh viễn (trả thẳng cho caller, không failover):

```go
// services/agent-go/internal/provider/fallback/fallback.go:289-319
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Rate limit / server errors → retry
	retryable := []string{
		"429", "rate limit", "too many requests",
		"resource_exhausted", "quota exceeded",
		"503", "502", "500", "internal server error",
		"timeout", "deadline exceeded", "connection refused",
		"temporarily unavailable", "service unavailable",
	}
	for _, p := range retryable {
		if strings.Contains(msg, p) {
			return true
		}
	}

	// Non-retryable → don't failover
	nonRetryable := []string{"400", "401", "403", "invalid", "context canceled"}
	for _, p := range nonRetryable {
		if strings.Contains(msg, p) {
			return false
		}
	}

	// Unknown error → retry (safety: rather retry than fail silently)
	return true
}
```

Vậy: **rate limit (429), server error (5xx), timeout, connection refused** → retryable → failover sang provider kế tiếp. **400/401/403 (client error, auth lỗi), input invalid, context bị cancel** → non-retryable → trả lỗi ngay, không thử provider khác (vì thử lại cũng vô nghĩa — lỗi do request sai, không phải do provider chết). Lỗi lạ/không nhận diện được → mặc định **retryable** ("an toàn hơn là cứ thử", theo comment).

**Không có retry cùng một provider trước khi fallback** — không có logic "thử lại provider hiện tại N lần rồi mới chuyển" — hễ lỗi retryable là **chuyển ngay sang provider kế tiếp trong chain** (retry ở cấp độ *chain*, không phải retry ở cấp độ *từng provider*):

```go
// services/agent-go/internal/provider/fallback/fallback.go:64-89 (rút gọn, nhánh lỗi ngay lúc Generate())
for i := range p.chain {
	np := &p.chain[i]

	if coolUntil := np.coolUntil.Load(); coolUntil > 0 {
		if time.Now().UnixNano() < coolUntil {
			continue // đang cooldown → bỏ qua, sang provider kế
		}
		np.coolUntil.Store(0)
	}

	scoped := scopeModel(req, np.name)
	stream, err := np.prov.Generate(ctx, scoped)
	if err != nil {
		if !isRetryable(err) {
			return nil, err // lỗi vĩnh viễn → trả ngay, KHÔNG failover
		}
		lastErr = err
		p.recordFailure(np, err)
		p.logFailure(np, scoped, i, err, "generate")
		continue // sang provider kế tiếp — không retry lại provider này
	}
	...
```

Một cơ chế thú vị khác: lỗi rate-limit của một số provider **không** đến ngay khi gọi `Generate()` mà đến ở **chunk đầu tiên của stream** (vì API trả HTTP 200 rồi mới báo lỗi qua SSE). `fallback.go` xử lý bằng cách "peek" chunk đầu trước khi coi là thành công:

```go
// services/agent-go/internal/provider/fallback/fallback.go:91-121 (rút gọn)
wrapped := make(chan provider.StreamChunk, 1)
go func() {
	defer close(wrapped)
	first := true
	for chunk := range stream {
		if first && chunk.Kind == provider.ChunkError && isRetryable(chunk.Err) {
			wrapped <- chunk
			return
		}
		first = false
		wrapped <- chunk
	}
}()

firstChunk, ok := <-wrapped
...
if firstChunk.Kind == provider.ChunkError && isRetryable(firstChunk.Err) {
	lastErr = firstChunk.Err
	p.recordFailure(np, firstChunk.Err)
	p.logFailure(np, scoped, i, firstChunk.Err, "stream")
	for range wrapped { } // drain
	continue // sang provider kế tiếp
}
```

Nếu chunk đầu **không** phải lỗi retryable, `replayStream` phát lại chunk đó rồi tiếp tục forward toàn bộ stream còn lại cho caller — caller không hề biết có việc "peek" này xảy ra phía sau.

**Cơ chế cooldown/backoff (đây mới là phần "circuit breaker" thật của cả hệ thống)** — mỗi provider trong chain có `failures` (đếm lỗi liên tiếp) và `coolUntil` (mốc thời gian hết cấm). Khi lỗi, `recordFailure` tăng backoff theo cấp số nhân, chặn trần ở 5 phút, và có "day-lock" riêng 2 giờ nếu lỗi là do **hết quota theo ngày**:

```go
// services/agent-go/internal/provider/fallback/fallback.go:229-242
func (p *Provider) recordFailure(np *namedProvider, err error) {
	fails := np.failures.Add(1)
	if p.cooldown <= 0 {
		return
	}
	cd := p.cooldown * (1 << min(int(fails)-1, 4))
	if isDailyQuotaExhausted(err) {
		// Day-lock: 2 hours cooldown when daily quota is exhausted
		cd = 2 * time.Hour
	} else if cd > 5*time.Minute {
		cd = 5 * time.Minute
	}
	np.coolUntil.Store(time.Now().Add(cd).UnixNano())
}
```

Với `cooldown` cơ sở = 15s (giá trị factory.go truyền vào, xem 3.5), backoff là 15s, 30s, 60s, 120s, rồi chặn trần 5 phút từ lần lỗi thứ 5 (`1<<4 = 16`, nhưng bị cap ở `5*time.Minute` trước khi nhân, thực ra là `cd` được tính trước rồi so với cap). Khi `coolUntil` còn hiệu lực, provider đó bị **skip hoàn toàn** ở vòng lặp `Generate` kế tiếp (không hề gọi tới nó) — test `TestGenerate_SkipsCoolingProvider` xác nhận provider đang cooldown có `called == 0`. Khi cooldown hết hạn, `Generate` tự reset `coolUntil` về 0 và thử lại provider đó — đây chính là hành vi tương đương "Half-Open → thử lại" của circuit breaker chuẩn, chỉ khác là không có state machine tường minh, mà suy ra từ so sánh timestamp mỗi lần gọi.

`isDailyQuotaExhausted` phát hiện lỗi hết quota **theo ngày** (khác quota theo phút/giây — cái này cần cooldown dài hơn nhiều vì phải chờ tới hôm sau mới reset thật, 2 giờ chỉ là "khoá tạm" để tránh dội request vô ích suốt cả ngày):

```go
// services/agent-go/internal/provider/fallback/fallback.go:321-331
func isDailyQuotaExhausted(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "per day") ||
		strings.Contains(msg, "daily") ||
		strings.Contains(msg, "day limit") ||
		strings.Contains(msg, "free_tier_requests_per_day") ||
		strings.Contains(msg, "rpd")
}
```

**Chi tiết dễ bị bỏ sót nhưng quan trọng — `scopeModel`**: vì cả chain dùng chung một `GenerateRequest`, nếu caller set `Options.Model` cho một model cụ thể (ví dụ ép dùng `"deepseek-v4-flash"` cho task rẻ) thì **không được** rò tên model đó sang provider khác họ (ví dụ Gemini) — vì Gemini sẽ tôn trọng override và gọi API với một model không tồn tại, tốn 1 request lỗi vô ích trước khi rớt xuống DeepSeek đúng:

```go
// services/agent-go/internal/provider/fallback/fallback.go:186-227
func modelFamily(model string) string {
	m := strings.ToLower(strings.TrimPrefix(model, "models/"))
	switch {
	case m == "":
		return ""
	case strings.HasPrefix(m, "gemini"), strings.HasPrefix(m, "gemma"):
		return "gemini"
	case strings.HasPrefix(m, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(m, "claude"):
		return "anthropic"
	}
	return ""
}

func scopeModel(req provider.GenerateRequest, providerName string) provider.GenerateRequest {
	fam := modelFamily(req.Options.Model)
	if fam == "" || fam == providerName {
		return req
	}
	scoped := req
	scoped.Options.Model = ""
	return scoped
}
```

Đây là một bugfix thật đã được ghi lại rất chi tiết trong comment (dòng 203-218): trước fix, mỗi lượt gọi learner/summarize tốn thêm 2 request Gemini vô ích, vừa hao quota free-tier vừa làm circuit-breaker/cooldown đánh lỗi oan cho provider chính.

Cuối cùng, `Status()` expose health hiện tại của toàn chain (dùng cho endpoint debug/observability), và `logFailure`/log "provider phục vụ sau khi bỏ qua" (mức WARN khi có lỗi, INFO chỉ khi phải bỏ qua ít nhất 1 provider) — thiết kế có chủ đích để log production không bị rác ở đường thành công bình thường (test `TestFallback_KhongLogKhiProviderDauThanhCong` xác nhận không log gì khi provider đầu trả lời ngay).

### 3.5 Factory & routing

`provider/factory/factory.go` là **factory theo config tĩnh** — đọc `cfg.Provider` (chuỗi từ env `LLM_PROVIDER`) và dựng đúng provider tương ứng, **không** có heuristic dựa trên nội dung task hay câu hỏi của user ở tầng này:

```go
// services/agent-go/internal/provider/factory/factory.go:25-38
func New(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case "gemini":
		return newGemini(cfg)
	case "anthropic":
		return newAnthropic(cfg)
	case "deepseek":
		return newDeepSeek(cfg)
	case "auto":
		return newAuto(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (use gemini, anthropic, deepseek, or auto)", cfg.Provider)
	}
}
```

Chế độ `"auto"` (`newAuto`) là factory dựng **fallback chain** thay vì 1 provider đơn — nhưng thứ tự chain vẫn là **tĩnh, quyết định bởi thứ tự code, không phải bởi phân loại task lúc runtime**: toàn bộ pool Gemini (primary model → secondary model → các fallback model theo đúng thứ tự khai trong config) → DeepSeek → Anthropic (chốt chặn cuối):

```go
// services/agent-go/internal/provider/factory/factory.go:65-136 (rút gọn)
func newAuto(cfg config.Config) (provider.Provider, error) {
	hasGemini := cfg.GeminiKey != ""
	hasDeepSeek := cfg.DeepSeekKey != ""
	hasClaude := cfg.AnthropicKey != ""

	providers := make([]provider.Provider, 0, 10)

	if hasGemini {
		// 1. Primary Gemini model, 2. Secondary (nếu có), 3. toàn bộ Fallback pool
		for _, m := range models { // đã dedup theo thứ tự ưu tiên
			mCfg := cfg
			mCfg.GeminiModel = m
			g, err := newGemini(mCfg)
			if err == nil {
				providers = append(providers, g)
			} else if m == cfg.GeminiModel {
				return nil, err // lỗi constructor của primary → báo lỗi ngay
			}
		}
	}
	if hasDeepSeek {
		d, err := newDeepSeek(cfg)
		if err != nil { return nil, err }
		providers = append(providers, d)
	}
	if hasClaude {
		c, err := newAnthropic(cfg)
		if err != nil { return nil, err }
		providers = append(providers, c)
	}

	switch len(providers) {
	case 0:
		return nil, fmt.Errorf("auto provider: need at least one of GEMINI_API_KEY, DEEPSEEK_API_KEY, or ANTHROPIC_API_KEY")
	case 1:
		return providers[0], nil // chỉ 1 provider → KHÔNG bọc fallback (test xác nhận)
	default:
		// 15s base cooldown — kết hợp exponential backoff + day-lock 2h ở fallback.go
		return fallback.New(15*time.Second, providers...)
	}
}
```

Test `TestNew_AutoChainOrder` xác nhận thứ tự chain thật: với đủ 3 key, `p.Name() == "fallback[gemini→deepseek→anthropic]"`. Test `TestNew_AutoFallbackGeminiPool` xác nhận nếu có nhiều Gemini fallback model, chuỗi sẽ có nhiều mắt-xích "gemini" liên tiếp trước khi tới deepseek/anthropic (ví dụ `fallback[gemini→gemini→gemini→gemini→deepseek→anthropic]`). Lý do đặt thứ tự Gemini trước: Gemini có free tier (rẻ nhất khi còn quota); DeepSeek đứng thứ 2 vì "siêu rẻ" (pay-as-you-go, comment factory.go dòng 3 gọi là "immediate fallback rẻ tiền"); Anthropic/Claude đứng cuối làm "chốt chặn" (đắt nhất nhưng đáng tin cậy nhất).

**Vậy có "DeepSeek auto-route" ở factory.go không? Không — ở factory.go đây chỉ là factory theo config tĩnh, thứ tự chain cố định, không đọc nội dung task để quyết định route.** Nhưng có một cơ chế **auto-route thật** — chỉ khác là nó nằm **bên trong** `deepseek.Client` (`provider/deepseek/deepseek.go`), không phải ở factory: hàm `pickModel` chọn giữa 2 model (`flashModel` rẻ/nhanh và `proModel` mạnh hơn) dựa trên heuristic đọc **nội dung của chính request đó** — số lượng message, độ dài message, và có yêu cầu "thinking"/reasoning hay không:

```go
// services/agent-go/internal/provider/deepseek/deepseek.go:60-77
// pickModel routes to flash or pro based on request complexity and reasoning config.
func (c *Client) pickModel(req provider.GenerateRequest) string {
	if req.Options.Model != "" {
		return req.Options.Model
	}
	if req.Options.ThinkingLevel != "" && req.Options.ThinkingLevel != provider.ThinkingOff {
		return c.proModel
	}
	if len(req.Messages) > 10 {
		return c.proModel
	}
	for _, m := range req.Messages {
		if m.Role == provider.RoleUser && len(m.Content) > 1000 {
			return c.proModel
		}
	}
	return c.flashModel
}
```

Heuristic cụ thể: (1) nếu caller đã ép model rõ ràng (`Options.Model`) → tôn trọng override, không đoán; (2) nếu yêu cầu `ThinkingLevel` khác OFF (cần suy luận sâu) → dùng `proModel`; (3) nếu hội thoại đã dài hơn 10 message → dùng `proModel` (context phức tạp hơn); (4) nếu bất kỳ message user nào dài hơn 1000 ký tự → dùng `proModel`; nếu không rơi vào case nào trên → dùng `flashModel` (rẻ, nhanh, mặc định cho task đơn giản). Đây đúng là "route heuristic dựa vào độ phức tạp của task", nhưng phạm vi chỉ trong 2 model của **cùng một** provider DeepSeek, không phải route giữa các provider khác nhau — việc route giữa các provider (Gemini/DeepSeek/Anthropic) hoàn toàn dựa vào thứ tự chain tĩnh + tình trạng lỗi/cooldown (mục 3.4), không dựa vào phân loại nội dung task.

Cuối cùng, một hành vi factory quan trọng cho vận hành: nếu `"auto"` chỉ có **đúng 1** key được cấu hình, factory trả về **provider đơn thuần**, không bọc `fallback.Provider` (vì `fallback.New` đòi tối thiểu 2 provider — factory tự né việc gọi `fallback.New` với 1 provider bằng nhánh `case 1: return providers[0], nil`) — test `TestNew_AutoSingleKeyReturnsPlainProvider` xác nhận điều này. Đây là ví dụ tốt về nguyên tắc "chỉ trả complexity khi thực sự cần" — không tạo lớp bọc thừa khi không có gì để fallback.
## 4. Orchestrator: Routing

### 4.1. Kết luận trước, chứng minh sau

Đọc thẳng vào hàm routing chính (`internal/orchestrator/orchestrator.go`): **JARVIS hiện tại route 100% bằng keyword matching, KHÔNG có bất kỳ lệnh gọi LLM nào trong đường đi routing**. Không có intent classifier, không có prompt "phân loại câu này thuộc domain nào". Toàn bộ quyết định "agent nào xử lý request này" giải quyết bằng `strings.Contains` + `regexp`, tốn ~micro-giây, không tốn 1 token.

Điều thú vị: **tài liệu thiết kế ban đầu của dự án lại hình dung một kiến trúc khác** — có fallback LLM. Đây là bằng chứng code thật đã đơn giản hoá đi so với design doc.

### 4.2. Code thật: `route()` và `matchTrigger()`

```go
// internal/orchestrator/orchestrator.go:93-108
func (o *Orchestrator) Run(ctx context.Context, in agent.RunInput, emit agent.EmitFunc) (provider.Usage, error) {
	// 1. Route: chọn agent dựa trên keyword matching
	spec := o.route(in.UserMessage)
	slog.Info("orchestrator: routed", "agent", spec.Name, "input_preview", truncate(in.UserMessage, 100))
	emit(agent.Event{Type: "agent", Node: spec.Name})
	// 3. Chạy engine của agent được chọn (GIỮ NGUYÊN Engine.Run)
	return spec.Engine.Run(ctx, in, emit)
}

// route chọn agent dựa trên keyword matching.
// Duyệt theo thứ tự đăng ký → agent đầu tiên match keyword được chọn.
func (o *Orchestrator) route(input string) *AgentSpec {
	lower := strings.ToLower(input)
	for _, name := range o.order {
		spec := o.agents[name]
		for _, kw := range spec.TriggerKeywords {
			if matchTrigger(lower, kw) { return spec }
		}
	}
	return o.agents[o.defaultAgent]
}
```

`Run()` không gọi `provider.Provider.Generate(...)` để classify — LLM call ĐẦU TIÊN trong toàn lượt chạy là LLM call của agent đã được chọn, không có "vòng classify" tốn thêm round-trip.

Trước một lần fix, `strings.Contains` thô gây false positive nghiêm trọng — chính comment trong code ghi lại:

```go
// internal/orchestrator/orchestrator.go:129-164
var asciiWordRe = regexp.MustCompile(`^[a-z0-9]+$`)
var (
	triggerRegexMu    sync.RWMutex
	triggerRegexCache = map[string]*regexp.Regexp{}
)

// matchTrigger khớp trigger keyword với input: keyword ASCII đơn từ dùng word
// boundary, còn lại (tiếng Việt có dấu, cụm nhiều từ) dùng substring.
//
// Trước fix, route() dùng strings.Contains thô cho MỌI keyword nên keyword "go"
// của agent code khớp cả "golang", "goroutine", "mongo", "django", "google",
// "algorithm"; "test" khớp "latest" (khiến mọi câu hỏi có "latest" bị agent
// code cướp trước research); "bug" khớp "debug".
func matchTrigger(s, kw string) bool {
	if !asciiWordRe.MatchString(kw) {
		return strings.Contains(s, kw)
	}
	triggerRegexMu.RLock()
	re, ok := triggerRegexCache[kw]
	triggerRegexMu.RUnlock()
	if !ok {
		re = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
		triggerRegexMu.Lock()
		triggerRegexCache[kw] = re
		triggerRegexMu.Unlock()
	}
	return re.MatchString(s)
}
```

Test khoá hành vi (`orchestrator_test.go:103-110`):
```go
{"'go' riêng vẫn khớp code", "viết cho tôi hàm go", "code"},
{"'mongo' KHÔNG khớp 'go'", "cấu hình mongo replica set thế nào", "general"},
{"'django' KHÔNG khớp 'go'", "django có gì hay", "general"},
{"'latest' về research, không bị 'test' cướp", "latest AI news", "research"},
{"'debug' KHÔNG khớp 'bug' (word riêng)", "debug hộ tôi", "general"},
```

Đây là "brittle" đúng nghĩa: sai một quyết định kỹ thuật nhỏ (Contains thay vì word-boundary) là route sai âm thầm, không exception, không log lỗi — chỉ trả lời sai agent. Không có LLM ở giữa để "hiểu ngữ cảnh" và tự sửa.

### 4.3. `Register()`: một dead-code bug đã fix, liên quan trực tiếp routing

```go
// internal/orchestrator/orchestrator.go:60-82
// Register thêm một agent vào orchestrator. Agent đăng ký trước có độ ưu
// tiên cao hơn trong keyword matching.
//
// Nếu spec.SystemPrompt khác rỗng, nó được áp vào engine NGAY TẠI ĐÂY. Trước
// đây field này là dead code: nó được gán ở cmd/server/main.go nhưng không
// hàm nào trong orchestrator đọc tới, nên toàn bộ prompt riêng của agent
// (vd 39 dòng hướng dẫn quy trình của research agent) chưa bao giờ tới LLM.
func (o *Orchestrator) Register(spec *AgentSpec) {
	name := spec.Name
	if _, exists := o.agents[name]; !exists {
		o.order = append(o.order, name)
	}
	o.agents[name] = spec
	if spec.SystemPrompt != "" && spec.Engine != nil {
		spec.Engine.SetSystemPrompt(spec.SystemPrompt)
	}
	if o.defaultAgent == "" {
		o.defaultAgent = name
	}
}
```

Bug này im lặng — không panic, không test fail — cho tới khi có test `TestOrchestrator_RegisterAppliesSystemPrompt` bắt provider giả ghi lại `GenerateRequest.System` thực sự gửi đi:

```go
// orchestrator_test.go:155-173
func TestOrchestrator_RegisterAppliesSystemPrompt(t *testing.T) {
	orch := New()
	eng, cap := newCapturingEngine()
	const want = "[BẠN LÀ RESEARCH AGENT] quy trình nghiên cứu riêng"
	orch.Register(&AgentSpec{Name: "research", Engine: eng, SystemPrompt: want})
	orch.Run(context.Background(), agent.RunInput{UserMessage: "hi", MaxSteps: 2}, func(agent.Event) {})
	if got := cap.LastRequest.System; got != want {
		t.Errorf("system prompt gửi cho LLM = %q, want %q", got, want)
	}
}
```

Bài học: routing đúng nhưng "wiring" của agent chưa đúng thì hệ thống vẫn sai lặng lẽ — test kiểu keyword-matching (`spec.Name == "code"`) không đủ, phải test tới tận request thực gửi cho provider.

### 4.4. Production wiring thật: `cmd/server/main.go`

3 agent: `general`, `code`, `research`. `general` đăng ký TRƯỚC với `TriggerKeywords: []string{}` (rỗng) → không bao giờ tự thắng qua keyword, chỉ là **default agent**:

```go
// cmd/server/main.go:237-276 (rút gọn)
orch := orchestrator.New()
orch.Register(&orchestrator.AgentSpec{
	Name: "general", Engine: generalEngine, TriggerKeywords: []string{},
	SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries, "vi"),
})
orch.Register(&orchestrator.AgentSpec{
	Name: "code", Engine: codeEngine,
	TriggerKeywords: []string{
		"code", "coding", "programming", "function", "func", "bug", "debug",
		"go", "golang", "python", "typescript", "javascript", "rust", "java",
		"refactor", "test", "unit test", "compile", "build", "deploy",
		"react", "hook", "hooks", "component", "redux", "vue", "angular",
		"nestjs", "nodejs", "node", "express", "fastify", "css", "tailwind",
		"html", "sql", "query", "api", "endpoint", "struct", "interface",
		"class", "method", "regex", "docker", "kubernetes",
		"viết hàm", "viết code", "sửa lỗi", "lỗi", "hàm", "biến", "thư viện",
		"triển khai", "tối ưu", "mã nguồn",
	},
	SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries, "vi"),
})
orch.Register(&orchestrator.AgentSpec{
	Name: "research", Engine: researchEngine,
	TriggerKeywords: []string{
		"search", "research", "tìm hiểu", "tra cứu", "find out", "look up",
		"what is", "who is", "when did", "how to", "latest", "tin tức",
		"news", "kiến thức", "cho biết", "giải thích", "why", "tại sao",
	},
	SystemPrompt: agent.BuildSystemPrompt(nil, skillSummaries, "vi") + `[BẠN LÀ RESEARCH AGENT]...`,
})
```

Vì `route()` duyệt tuần tự theo `o.order` và trả về NGAY khi khớp — nếu 1 câu vừa khớp `"code"` vừa khớp `"research"`, **agent đăng ký sớm hơn luôn thắng, không phải agent khớp "tốt hơn"**. Đây là "first-match-wins", khác hẳn `skills.Loader.MatchSkill` (Mục 7) dùng scoring — một sự bất nhất trong design giữa 2 module.

### 4.5. So sánh design doc ban đầu: kế hoạch có LLM fallback, thực tế thì không

`docs/architecture/multi-agent-orchestrator-design.md` mô tả `IntentRouter` 2 tầng — keyword trước, LLM nhẹ (Gemini Flash) **fallback** khi không khớp:

```go
type IntentRouter struct {
    classifier provider.Provider  // LLM nhẹ (Gemini Flash) để classify
    agents     []*AgentSpec
}
func (r *IntentRouter) Route(ctx context.Context, input string) (*AgentSpec, error) {
    // 1. Keyword matching
    for _, a := range r.agents { ... }
    // 2. Nếu không match keyword → gọi LLM nhẹ để classify
    domain := r.callClassifier(ctx, input)
    if a, ok := r.agents[domain]; ok { return a, nil }
    // 3. Default → GeneralAgent
    return r.agents["general"], nil
}
```

**Đây KHÔNG phải code đã triển khai** — `IntentRouter`, `classifier`, `callClassifier` không tồn tại ở bất kỳ đâu trong `internal/orchestrator/`. Route thật dừng ngay ở bước 1 — không khớp keyword nào → rơi thẳng về `o.agents[o.defaultAgent]`, bỏ qua hoàn toàn bước LLM classify. Quyết định có chủ đích: đơn giản hoá so với kế hoạch — keyword-matching + default agent đủ tốt, rẻ hơn, nhanh hơn, dễ debug hơn.

### 4.6. Trade-off: keyword matching vs LLM-based classification

| | Keyword matching (ĐANG DÙNG) | LLM-based classification (design doc, chưa triển khai) |
|---|---|---|
| Latency thêm | ~0 (thuần Go) | +1 round-trip LLM (100-500ms) |
| Cost thêm | 0 token | Tốn token mỗi request |
| Chính xác với câu rõ ràng | Cao | Cao tương đương |
| Chính xác với câu mơ hồ/đa domain | Thấp — first-match-wins, không "hiểu" ý định | Cao hơn — LLM cân nhắc ngữ cảnh |
| Debug | Rất dễ — deterministic | Khó hơn — nondeterministic |
| Maintain | Phải tay thêm keyword mỗi domain mới | Tự nhiên mở rộng hơn |
| Rủi ro | Substring match sai, quên keyword | Hallucination domain, prompt injection ảnh hưởng classify |
| Ngân sách LLM hẹp (bối cảnh dự án) | Phù hợp | Không phù hợp — tăng cost tuyến tính theo traffic |

**Kết luận**: routing hiện tại nằm hẳn về đầu "rẻ — nhanh — dễ debug — brittle" của spectrum. Đổi lấy latency=0 & cost=0, JARVIS chấp nhận rủi ro misroute ở ranh giới 2 domain, xử lý bằng word-boundary + keyword tay chứ không "hiểu ngữ nghĩa". `general` (không keyword) luôn là lưới an toàn cuối — đảm bảo không crash vì "không route được", chỉ có nguy cơ route "chưa tối ưu".

### 4.7. Handoff / Delegate: routing lần hai, agent-to-agent

`Delegate()` cho phép 1 agent đang chạy chuyển task sang agent khác — chỉ định tường minh `To: "code"`, không qua `route()`/keyword:

```go
// internal/orchestrator/orchestrator.go:221-231
func (o *Orchestrator) Delegate(ctx context.Context, req HandoffRequest) (*HandoffResult, error) {
	if req.Depth >= o.maxDelegationDepth {
		return nil, &DelegationDepthExceededError{From: req.From, To: req.To, Depth: req.Depth, Max: o.maxDelegationDepth}
	}
	spec := o.agents[req.To]
	if spec == nil {
		return nil, fmt.Errorf("orchestrator: agent %q not found for handoff from %q", req.To, req.From)
	}
	...
}
```

Chống đệ quy vô hạn (A→B→A→...) dựa vào `HandoffRequest.Depth` tăng dần, `defaultMaxDelegationDepth = 4` — chốt an toàn state machine, không liên quan LLM, khoá bởi test `TestDelegate_DepthLimitDefault`.

---

## 5. Personality & Proactive

### 5.1. Personality: `profile.go` — cơ chế build prompt

`AdaptPrompt()` là hàm build prompt DUY NHẤT, prepend header `[TÍNH CÁCH]` phía trước base prompt:

```go
// internal/personality/profile.go:146-189
func (e *PersonalityEngine) AdaptPrompt(base string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var b strings.Builder
	b.WriteString("[TÍNH CÁCH]\n")
	b.WriteString(fmt.Sprintf("Bạn là %s.\n", e.profile.Name))
	switch e.profile.Formality {
	case FormalityCasual:  b.WriteString("- Phong cách: thân mật, gần gũi...\n")
	case FormalityNeutral: b.WriteString("- Phong cách: trung tính, lịch sự...\n")
	case FormalityFormal:  b.WriteString("- Phong cách: trang trọng, chuyên nghiệp...\n")
	}
	switch e.profile.Humor { /* ... */ }
	switch e.profile.Verbosity { /* ... */ }
	b.WriteString("\n")
	b.WriteString(base)
	return b.String()
}
```

`Learn()` chỉ tích luỹ SỐ LẦN người dùng dùng cụm từ liên quan preference vào `Stats.Preferences map[string]int` — **KHÔNG tự động thay đổi `e.profile`**:

```go
// internal/personality/profile.go:191-215
func (e *PersonalityEngine) Learn(input, response string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats.Interactions++
	lower := strings.ToLower(input)
	prefs := map[string]string{
		"ngắn gọn": "concise", "brief": "concise", "concise": "concise",
		"chi tiết": "detailed", "detailed": "detailed",
		"thân mật": "casual", "casual": "casual",
		"trang trọng": "formal", "formal": "formal",
	}
	for keyword, pref := range prefs {
		if strings.Contains(lower, keyword) { e.stats.Preferences[pref]++ }
	}
}
```

`Learn()` + `Update()` (ghi đè profile, do caller gọi) là 2 nửa "quan sát → điều chỉnh" nhưng **không có "keo" nối 2 nửa** — không có logic "nếu Preferences vượt ngưỡng N thì tự Update()". `docs/ARCHITECTURE_VI.md` mô tả `Learn()` như thể tự mutate profile ngay — không khớp code thật (aspirational doc vs actual code).

**Phát hiện quan trọng nhất**: package `personality` **hoàn toàn KHÔNG được wiring vào request path**. Không có `personality.New()`/`AdaptPrompt()` nào được gọi ngoài chính `profile.go`/`profile_test.go`. So với `skills.Loader` (đã wire đầy đủ, Mục 7): `personality` là module hoàn chỉnh về logic + test (228 dòng test, coverage mọi nhánh) nhưng chưa đi vào production traffic — phân biệt "module hoàn chỉnh" với "module đã chạy thật" là bài học phỏng vấn tốt.

### 5.2. Proactive: `scheduler.go` — trigger THẬT là cron, không phải Ticker/event-driven

Dùng `github.com/robfig/cron/v3`, KHÔNG phải `time.Ticker`, KHÔNG event-driven:

```go
// internal/proactive/scheduler.go:39-55
type ProactiveEngine struct {
	cron    *cron.Cron
	runner  PromptRunner
	tasks   map[string]*Task
	results []TaskResult
	mu      sync.RWMutex
}
func NewProactiveEngine(runner PromptRunner) *ProactiveEngine {
	return &ProactiveEngine{cron: cron.New(cron.WithSeconds()), runner: runner, tasks: make(map[string]*Task)}
}
```

`cron.WithSeconds()` bật cron 6-field (giây-phút-giờ-ngày-tháng-thứ) — test dùng `"* * * * * *"` (mỗi giây) và `"0 0 8 * * *"` (8h sáng).

```go
// internal/proactive/scheduler.go:61-85
func (e *ProactiveEngine) AddTask(name, cronExpr, prompt string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.tasks[name]; exists { return &TaskExistsError{Name: name} }
	task := &Task{Name: name, CronExpr: cronExpr, Prompt: prompt}
	e.tasks[name] = task
	_, err := e.cron.AddFunc(cronExpr, func() { e.runTask(task) })
	if err != nil { delete(e.tasks, name); return err }
	return nil
}
```

`RemoveTask()` chỉ xoá khỏi map, KHÔNG hủy được cron job (comment tự thừa nhận `robfig/cron/v3` không cho remove job theo ID):

```go
// internal/proactive/scheduler.go:87-99
func (e *ProactiveEngine) RemoveTask(name string) error {
	// cron v3 does not support removing individual jobs by name directly.
	// We mark it as removed and skip in runTask.
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.tasks[name]; !exists { return &TaskNotFoundError{Name: name} }
	delete(e.tasks, name)
	return nil
}
```

Đọc `runTask()` thì thấy comment "skip in runTask" **không khớp code thật** — không có điều kiện kiểm tra task còn tồn tại trong map:

```go
// internal/proactive/scheduler.go:142-171
func (e *ProactiveEngine) runTask(task *Task) {
	e.mu.Lock()
	task.LastRun = time.Now()
	task.RunCount++
	e.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	response, err := e.runner.RunPrompt(ctx, task.Prompt)
	// ghi TaskResult, giới hạn e.results tối đa 1000 phần tử gần nhất
}
```

Hệ quả: sau `RemoveTask("x")`, `Tasks()` không còn hiển thị "x" nhưng cron entry bên trong `e.cron` **vẫn tồn tại và vẫn fire đúng lịch** — vẫn gọi `runner.RunPrompt` thật (tốn LLM call!), chỉ là không ai theo dõi được bằng tên nữa tới khi restart process. Khoảng lệch thật giữa comment và hành vi — minh chứng "đọc code thật quan trọng hơn đọc comment".

`PromptRunner` là dependency inversion (không phụ thuộc trực tiếp `*agent.Engine`):
```go
type PromptRunner interface {
	RunPrompt(ctx context.Context, prompt string) (string, error)
}
```

**Cùng phát hiện như 5.1**: không tìm thấy `proactive.NewProactiveEngine(...)` nào ngoài `scheduler_test.go` trong toàn bộ `cmd/`. Package hoàn chỉnh (cron thật, timeout 5 phút/task, giới hạn 1000 kết quả, error type riêng) nhưng **chưa được kích hoạt trong server đang chạy** — không có task tự động nào thực thi trong production hiện tại, dù hạ tầng sẵn sàng 100%.

---

## 6. MCP (Model Context Protocol)

### 6.1. Khái niệm tổng quát

MCP là chuẩn giao tiếp mở (Anthropic khởi xướng) giải quyết: 1 "agent host" muốn dùng tool từ 1 MCP server ngoài mà không viết integration riêng. Chuẩn hoá 2 hành vi:

1. **Discover** — host gọi `tools/list`, server trả `name`/`description`/`inputSchema` (JSON Schema).
2. **Invoke** — host gọi `tools/call` với `name`+`arguments`, server trả `content`+`isError`.

Giao tiếp qua **JSON-RPC 2.0**, 2 transport:
- **stdio**: host spawn subprocess, giao tiếp qua stdin/stdout — phù hợp server local, admin cấu hình.
- **SSE / Streamable HTTP**: HTTP POST tới server remote, response JSON thuần hoặc SSE-framed — phù hợp server user tự thêm (chỉ URL+token, không RCE risk).

Cả 2 bắt đầu bằng handshake: `initialize` → server trả kết quả → client gửi `notifications/initialized`.

JARVIS triển khai **cả hai transport**: `MCPClient` (stdio, admin-only, YAML) và `SSEClient` (Streamable HTTP, user-configurable, DB per-user).

### 6.2. `discovery.go`: auto-discovery qua stdio + YAML

```go
// internal/mcp/discovery.go:27-53
type jsonRPCRequest struct {
	JSONRPC string; ID int64; Method string; Params interface{} `json:"params,omitempty"`
}
type jsonRPCResponse struct {
	JSONRPC string; ID int64; Result json.RawMessage; Error *jsonRPCError
}
type listToolsResult struct { Tools []mcpToolDef `json:"tools"` }
type mcpToolDef struct {
	Name string; Description string; InputSchema json.RawMessage
}
```

Cấu hình YAML admin-only (arbitrary executable):
```go
// internal/mcp/discovery.go:247-257
type MCPToolConfig struct {
	Name string `yaml:"name"`; Command string `yaml:"command"`; Args []string `yaml:"args"`
}
type MCPConfig struct { Servers []MCPToolConfig `yaml:"servers"` }
```

`Discover(configDir)` quét mọi `.yaml`/`.yml` (không đệ quy), parse, rồi `connectServer()` cho mỗi server khai báo:

```go
// internal/mcp/discovery.go:373-399
func (r *MCPRegistry) connectServer(cfg MCPToolConfig) error {
	client := &MCPClient{}
	if err := client.Connect(cfg.Command, cfg.Args...); err != nil { return ... }
	defs, err := client.ListTools()
	if err != nil { client.Close(); return ... }
	for _, def := range defs {
		adapter := &mcpAdapter{
			name: qualifiedToolName(cfg.Name, def.Name), rawName: def.Name,
			description: def.Description, schema: def.Schema, client: client,
		}
		r.reg.Register(adapter)
	}
	r.clients = append(r.clients, client)
	return nil
}
```

Chuỗi discovery: (1) spawn subprocess (`exec.Command` + StdinPipe/StdoutPipe, gửi `initialize` + `notifications/initialized`); (2) gọi `tools/list`, parse thành `[]provider.ToolDef`; (3) bọc mỗi tool thành `mcpAdapter` (implement `tools.Tool`, dùng chung registry với tool native); (4) đăng ký thẳng vào `tools.Registry` chia sẻ.

**Không phải cache TTL** — chạy 1 lần lúc khởi động, tool definitions cố định tới khi restart hoặc gọi lại `Discover()` tay. Không polling định kỳ.

Namespace tool tránh đụng độ (`mcp__<server>__<raw>`):
```go
// internal/mcp/discovery.go:294-320
const maxToolNameLen = 64
func qualifiedToolName(server, raw string) string {
	name := "mcp__" + invalidToolNameChar.ReplaceAllString(server, "_") + "__" + invalidToolNameChar.ReplaceAllString(raw, "_")
	if len(name) <= maxToolNameLen { return name }
	sum := sha256.Sum256([]byte(server + "\x00" + raw))
	suffix := "-" + hex.EncodeToString(sum[:])[:8]
	keep := max(maxToolNameLen-len(suffix), 0)
	return name[:keep] + suffix
}
```

`mcpAdapter.client` là interface `toolClient` (không phải `*MCPClient` cụ thể) — cho phép cùng adapter dùng chung cho cả stdio và SSE:
```go
// internal/mcp/discovery.go:259-276
type toolClient interface {
	CallTool(name string, args json.RawMessage) (string, error)
}
type mcpAdapter struct {
	name, rawName, description string; schema json.RawMessage; client toolClient
}
```

### 6.3. `sse.go` — transport Streamable HTTP cho MCP server remote (BẢN MỚI NHẤT)

> `sse.go`/`sse_test.go` là uncommitted change trên checkout chính — nội dung dưới đọc trực tiếp bản mới nhất trên đĩa.

```go
// internal/mcp/sse.go:18-64
type ServerConfig struct {
	Name, URL, APIKey string
}
const (
	// mcpProtocolVersion: CỐ TÌNH giữ "2024-11-05" thay vì bump mới hơn.
	// Connect() gửi version này trong "initialize" nhưng KHÔNG đọc/kiểm tra
	// protocolVersion server trả về — client không thực sự "negotiate", chỉ
	// tuyên bố version của mình. Một số MCP server strict validate và trả
	// lỗi nếu gặp version lạ. "2024-11-05" là baseline cũ nhất, hỗ trợ rộng
	// nhất bởi server hiện có → an toàn nhất cho "chạy được với server thật
	// hôm nay" (Notion, GitHub, Linear, Sentry...).
	mcpProtocolVersion = "2024-11-05"
	mcpClientName      = "jarvis-go"
	mcpClientVersion   = "0.1.0"
	mcpHTTPTimeout     = 30 * time.Second
)
type SSEClient struct {
	url, apiKey string
	httpClient  *http.Client
	mu          sync.Mutex
	nextID      int64
	sessionID   string // Mcp-Session-Id do server cấp
}
```

Hardcode protocolVersion là quyết định có chủ đích (tương thích ngược), không phải quên bump — tự nhận là nợ kỹ thuật đã biết.

**Handshake:**
```go
// internal/mcp/sse.go:75-95
func (c *SSEClient) Connect(ctx context.Context) error {
	if _, err := c.sendRequest(ctx, "initialize", map[string]interface{}{
		"protocolVersion": mcpProtocolVersion, "capabilities": map[string]interface{}{},
		"clientInfo": map[string]string{"name": mcpClientName, "version": mcpClientVersion},
	}); err != nil { return fmt.Errorf("mcp: initialize: %w", err) }
	notif := jsonRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}
	_, _ = c.post(ctx, notif, -1)
	return nil
}
```

**`post()` — header + content negotiation:**
```go
// internal/mcp/sse.go:162-212
func (c *SSEClient) post(ctx context.Context, req jsonRPCRequest, expectedID int64) (json.RawMessage, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	resp, err := c.httpClient.Do(httpReq)
	...
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" { c.sessionID = sid }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { ... return nil, fmt.Errorf(...) }
	if expectedID < 0 { return nil, nil }
	data, _ := io.ReadAll(resp.Body)
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return parseSSEResponse(data, expectedID)
	}
	return parseJSONResponse(data, expectedID)
}
```

`Accept: application/json, text/event-stream` = Streamable HTTP: client chấp nhận cả 2 kiểu, server tự chọn. `Mcp-Session-Id` được cấp lần đầu và echo lại các request sau — duy trì session logic trên transport về bản chất stateless.

**Parse SSE — không phải persistent stream:**
```go
// internal/mcp/sse.go:228-251
func parseSSEResponse(data []byte, expectedID int64) (json.RawMessage, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") { continue }
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" { continue }
		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(payload), &resp); err != nil { continue }
		if resp.ID != expectedID { continue }
		if resp.Error != nil { return nil, fmt.Errorf("mcp: rpc error code=%d: %s", resp.Error.Code, resp.Error.Message) }
		return resp.Result, nil
	}
	return nil, fmt.Errorf("mcp: không tìm thấy response id=%d trong SSE stream", expectedID)
}
```

`post()` gọi `io.ReadAll(resp.Body)` đọc TOÀN BỘ response body của 1 request-response HTTP đơn — "SSE" ở đây chỉ là format đóng khung của MỘT response, không phải kết nối 2 chiều dài hạn. `Close()` là no-op:
```go
func (c *SSEClient) Close() error { return nil }
```

**Reconnect/lỗi kết nối**: không có logic reconnect/retry (không backoff, không retry loop) — đúng bản chất "mỗi request độc lập". Lỗi network/timeout (30s) hoặc status ngoài 2xx → trả lỗi ngay lên caller. Graceful degradation xử lý ở tầng GỌI (`agent.Engine.Run`, xem 6.4): chỉ log warning và tiếp tục không có tool MCP, không fail cả request.

### Phần MỚI vừa sửa: Authorization header — bug fix có bằng chứng test

Docstring đầu `sse_test.go`:
```go
// Test cho tính năng MỚI (user tự thêm MCP server có token xác thực): kiểm
// chứng ServerConfig.APIKey THỰC SỰ được gửi thành header
// Authorization: Bearer <token> khi user đã cấu hình token, và KHÔNG được
// gửi header đó khi server không có token.
```

3 test case:
1. `TestSSEClient_SendsAuthorizationHeader_WhenAPIKeySet` — `APIKey` set → MỌI request (initialize, initialized, tools/list) mang `Authorization: Bearer ...`.
2. `TestSSEClient_NoAuthorizationHeader_WhenAPIKeyEmpty` — `APIKey=""` → KHÔNG có header Authorization (hoàn toàn không set, không phải rỗng — vì "một số server strict coi header hiện diện dù rỗng là lỗi định dạng").
3. `TestDiscoverSSE_PropagatesAPIKeyFromServerConfig` — test đường đi ĐẦY ĐỦ từ `ServerConfig.APIKey` qua `DiscoverSSE` tới header thật, đảm bảo token DB không "rơi" ở tầng trung gian.

Code tương ứng chính xác dòng 176-178 của `post()`. Fix có chủ đích cho lớp lỗi cụ thể: MCP server thật (Notion/GitHub/Linear/Sentry) đòi header này, thiếu thì `initialize`/`tools/list` fail 401/403 — khớp migration DB (6.5).

### `DiscoverSSE`: registry riêng cho MỖI lượt chạy

```go
// internal/mcp/sse.go:253-290
// DiscoverSSE kết nối tới từng MCP server remote, discovery tools và đăng ký
// vào một registry RIÊNG cho lượt chạy hiện tại (tránh ghi vào registry dùng
// chung gây data race và rò rỉ tool giữa các user).
func DiscoverSSE(ctx context.Context, servers []ServerConfig) (*tools.Registry, []*SSEClient, error) {
	reg := tools.NewRegistry()
	clients := make([]*SSEClient, 0, len(servers))
	for _, srv := range servers {
		if strings.TrimSpace(srv.URL) == "" { continue }
		client := NewSSEClient(srv.URL, srv.APIKey)
		if err := client.Connect(ctx); err != nil { client.Close(); return reg, clients, fmt.Errorf(...) }
		defs, err := client.ListTools(ctx)
		if err != nil { client.Close(); return reg, clients, fmt.Errorf(...) }
		for _, def := range defs {
			reg.Register(&mcpAdapter{name: qualifiedToolName(srv.Name, def.Name), ...})
		}
		clients = append(clients, client)
	}
	return reg, clients, nil
}
```

Khác `MCPRegistry.Discover` (stdio, registry dùng chung, 1 lần lúc khởi động): `DiscoverSSE` tạo registry MỚI mỗi lần gọi, mỗi lượt chat (per-request) — vì mỗi user có thể cấu hình MCP server khác nhau, dùng chung registry sẽ rò tool giữa user + data race.

### 6.4. Wiring thật vào engine: discovery per-request

```go
// internal/agent/engine.go:236-263 (rút gọn)
if len(in.McpServers) > 0 {
	cfg := make([]mcp.ServerConfig, 0, len(in.McpServers))
	for _, srv := range in.McpServers {
		cfg = append(cfg, mcp.ServerConfig{Name: srv.Name, URL: srv.URL, APIKey: srv.APIKey})
	}
	reg, clients, err := mcp.DiscoverSSE(ctx, cfg)
	if err != nil {
		slog.Warn("engine: MCP discovery thất bại", "err", err)
	} else {
		s.mcpRegistry = reg
		s.mcpClients = ...
	}
}
if len(s.mcpClients) > 0 {
	defer func() { for _, c := range s.mcpClients { c.Close() } }()
}
```

Bằng chứng graceful degradation: lỗi `DiscoverSSE` chỉ `slog.Warn` rồi tiếp tục — user vẫn nhận câu trả lời, chỉ thiếu tool MCP lượt đó.

### 6.5. Auth cho MCP server: token + header, và persistence phía DB

2 tầng: (1) transport — `Authorization: Bearer <APIKey>`; (2) persistence — migration `apps/api/src/database/postgres/migrations/004-mcp-auth-transport.sql` đổi cột `api_key` → `auth_token`, nới constraint transport chấp nhận `'http'`/`'sse'`:

```sql
-- Cột api_key cũ (migration 003) được GIỮ NGUYÊN tại DB (không DROP) để
-- không phá dữ liệu hiện có, nhưng kể từ migration này ứng dụng không còn
-- đọc/ghi cột api_key nữa -- auth_token là nguồn sự thật duy nhất.
--
-- LƯU Ý BẢO MẬT: auth_token đang lưu PLAINTEXT tại rest (giống api_key cũ).
-- Đây là nợ kỹ thuật tạm chấp nhận để không tự chế crypto vội -- bước sau
-- nên mã hoá cột này trước khi dùng với dữ liệu thật/production.
ALTER TABLE user_mcp_servers ADD COLUMN IF NOT EXISTS auth_token TEXT DEFAULT NULL;
```

Token của user cho MCP server thứ ba hiện lưu plaintext trong Postgres — rủi ro nếu DB bị lộ. Team chọn "ship trước, mã hoá sau" có ý thức (ghi rõ trong migration).

---

## 7. Skills: Progressive Disclosure

### 7.1. Khái niệm tổng quát

Nếu nhồi TOÀN BỘ hướng dẫn mọi skill vào system prompt: (1) token bloat — tốn tiền dù skill không dùng; (2) nhiễu ngữ nghĩa — chỉ dẫn xung đột nhiều domain; (3) chi phí lặp lại. Progressive disclosure (Claude Skills): chỉ giữ **danh sách rất nhẹ** (tên/mô tả) trong system prompt, **nội dung đầy đủ** chỉ nạp khi cần, biến mất khi turn kết thúc.

### 7.2. `loader.go`: hai tầng tải — "nhẹ" cho catalogue, "đầy đủ" khi kích hoạt

```go
// internal/skills/loader.go:18-36
type Skill struct {
	Name, Description, WhenToUse string
	Tools    []string
	Content  string // Thân SKILL.md, KHÔNG gồm frontmatter
	Triggers []string
}
type SkillSummary struct { Name, Description string }
```

```go
// internal/skills/loader.go:101-119
func (l *Loader) ListSkills() []SkillSummary { /* chỉ Name+Description */ }
func (l *Loader) LoadSkill(name string) *Skill { return l.skills[name] }
```

JARVIS đi XA HƠN "name+description" chuẩn — bỏ hẳn `Description`, chỉ giữ **tên** trong prompt thật, vì skill không do model tự chọn:

```go
// internal/agent/context.go:15-27
// buildSkillCatalogue liệt kê TÊN skill, KHÔNG kèm description.
//
// Vì sao bỏ được description: skill KHÔNG do model chọn. skills.Loader.MatchSkill
// chấm điểm bằng code Go trên input người dùng rồi node_model nạp NGUYÊN VĂN
// SKILL.md của skill thắng vào prompt. Model không có vai trò gì trong việc
// kích hoạt, nên 30 dòng description gửi kèm MỌI request (~1.100 token, ~21%
// input của 1 lượt chat) không mua được khả năng nào.
//
// Vẫn giữ danh sách trong prompt để phần đầu prompt ổn định, phục vụ prompt
// caching — chèn động theo câu hỏi sẽ phá cache prefix.
func buildSkillCatalogue(summaries []skills.SkillSummary) string { ... }
```

**Khác biệt kiến trúc so với Claude Skills chính thức**: trong Claude Skills, MODEL tự quyết định mở/dùng skill nào. Trong JARVIS, **việc kích hoạt hoàn toàn nằm ở tầng Go code** — `MatchSkill()` chấm điểm deterministic, model chỉ nhận nội dung đã tiêm sẵn, không có "quyền" chọn.

`NewLoader` nạp TẤT CẢ SKILL.md lúc khởi động (không lazy per-request):

```go
// internal/skills/loader.go:65-92
func NewLoader(skillsDir string) (*Loader, error) {
	l := &Loader{skills: make(map[string]*Skill)}
	entries, _ := os.ReadDir(skillsDir)
	for _, entry := range entries {
		if !entry.IsDir() { continue }
		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil { continue }
		skill, err := parseSkill(string(data))
		if err != nil { return nil, fmt.Errorf(...) }
		if _, exists := l.skills[skill.Name]; !exists { l.order = append(l.order, skill.Name) }
		l.skills[skill.Name] = skill
	}
	return l, nil
}
```

"Progressive disclosure" thật nằm ở tầng **PROMPT**, không phải tầng I/O: toàn bộ nội dung đã trong RAM từ lúc khởi động; chỉ `Name` đi vào system prompt CỐ ĐỊNH, `Content` đầy đủ chỉ ghép vào lượt chạy hiện tại khi `MatchSkill()` khớp.

**`MatchSkill()`: scoring, không phải first-match:**
```go
// internal/skills/loader.go:121-190
const (
	skillScoreNameMatch      = 100
	skillScoreTriggerMatch   = 100
	skillScoreWhenToUseMatch = 3
	skillScoreDescMatch      = 1
)
// minSkillActivationScore: đo trên 23 skill thật, điểm khớp skill ĐÚNG và
// SAI đều nằm trong 1-4 (chỉ trùng vài từ kỹ thuật lọt vào câu tiếng Việt).
// Kích hoạt sai KHÔNG vô hại: nhồi cả SKILL.md (api-designer ~3.000 token)
// vào prompt và lái model sai hướng.
const minSkillActivationScore = 6

func (l *Loader) MatchSkill(userInput string) *Skill {
	...
	for _, name := range l.order {
		s := l.skills[name]
		score := 0
		if strings.Contains(normalized, " "+nameLower+" ") || strings.Contains(normalized, " "+nameSpaced+" ") {
			score += skillScoreNameMatch
		}
		for _, trg := range s.Triggers {
			if strings.Contains(lower, trg) { score += skillScoreTriggerMatch; break }
		}
		score += skillScoreWhenToUseMatch * countKeywordMatches(normalized, s.WhenToUse)
		score += skillScoreDescMatch * countKeywordMatches(normalized, s.Description)
		if score > bestScore { best, bestScore = s, score }
	}
	if bestScore < minSkillActivationScore { return nil }
	return best
}
```

Hệ thống chấm điểm đa tín hiệu — khớp tên/trigger (weight 100) > `when_to_use` (weight 3/từ) > `description` (weight 1/từ) — khác triết lý "first match wins" của `orchestrator.route()` (Mục 4). Cả 2 module đều tinh chỉnh dựa trên số liệu đo thật, chỉ khác giải pháp cuối.

Chống false-positive substring (`"use"` khớp `"useMemo"`) bằng word-boundary + danh sách `skillStopWords` (60+ từ) — cùng lớp vấn đề với `matchTrigger()` Mục 4, giải quyết bằng kỹ thuật tương tự.

### 7.3. Điểm kích hoạt thật trong request path: `node_model.go` + `context.go`

Injection thật xảy ra ở `internal/agent/node_model.go`, KHÔNG phải trong package `skills`:

```go
// internal/agent/node_model.go:64-95 (rút gọn)
// Progressive skill disclosure: match user input against skill triggers
// and inject full SKILL.md content into the system prompt on first match.
systemPrompt := eng.getSystemPrompt()
if sl := eng.getSkillLoader(); sl != nil {
	if matched := sl.MatchSkill(userInput); matched != nil && !s.activatedSkills[matched.Name] {
		s.activatedSkills[matched.Name] = true
		body := matched.PromptBody() // gọt vừa ngân sách token (skills.MaxPromptBytes)
		systemPrompt += "\n\n[KỸ NĂNG ĐANG KÍCH HOẠT: " + matched.Name + "]\n" + body
		emit(MemoryEvent("Kích hoạt kỹ năng: " + matched.Name))
		skillTools = matched.Tools
	}
}
```

`s.activatedSkills` chặn inject LẶP trong CÙNG 1 lượt chạy nhưng KHÔNG persist qua lượt khác — "loaded, used, discarded" per-turn. Khi khớp, `matched.Tools` (từ frontmatter `tools:`) đảm bảo nằm trong tool list gửi LLM — allowlist theo skill.

`BuildSystemPrompt()` ghép catalogue vào vị trí CỐ ĐỊNH để tối ưu prompt caching:
```go
// internal/agent/context.go:44-118 (rút gọn)
// Thứ tự: 1.[HỆ THỐNG] 2.[KỸ NĂNG] 3.[CÔNG CỤ] (cacheable) 4.[BỘ NHỚ] 5.[NGỮ CẢNH] (dynamic)
if len(skillSummaries) > 0 {
	b.WriteString("[KỸ NĂNG] — Các kỹ năng có thể kích hoạt khi cần:\n")
	b.WriteString(buildSkillCatalogue(skillSummaries))
}
```

Catalogue (chỉ tên) nằm ở phần "ổn định" của prompt để giữ cache prefix hit; nội dung skill đầy đủ nằm CUỐI, biến động theo lượt — tách bạch cacheable/dynamic ngay từ thiết kế.

### 7.4. `budget.go`: giới hạn ngân sách — cắt theo section, không cắt theo byte thô

```go
// internal/skills/budget.go:5-37
// 5.500 byte (≈1.570 token) chọn theo dữ liệu thật: sau khi viết lại 8
// SKILL.md dài nhất cho gọn, file lớn nhất còn 5.462 byte — KHÔNG skill nào
// bị cắt nữa. Trước đó 13/32 skill bị cắt, mất tổng 29.464 byte hướng dẫn.
const MaxPromptBytes = 5500
const truncationNote = "\n\n[Phần sau của kỹ năng đã được lược bỏ để tiết kiệm ngữ cảnh — dùng phần trên là đủ.]"

// PromptBody trả phần thân skill đã gọt vừa MaxPromptBytes.
// Cắt theo RANH GIỚI SECTION ("## ") chứ không cắt theo số byte.
func (s *Skill) PromptBody() string {
	return truncateToSections(s.Content, MaxPromptBytes)
}
```

Chỉ 1 giới hạn duy nhất (số byte thân skill), không giới hạn số skill kích hoạt đồng thời (vì `node_model.go` chỉ activate TỐI ĐA 1 skill/lượt — lấy điểm cao nhất). Cắt ưu tiên ranh giới `## section`:

```go
// internal/skills/budget.go:55-71
for _, line := range lines {
	if strings.HasPrefix(line, "## ") {
		seenSections++
		if seenSections >= 2 { bestCut = len(kept) }
	}
	next := size + len(line) + 1
	if next > maxBytes { break }
	kept = append(kept, line)
	size = next
}
if bestCut > 0 { kept = kept[:bestCut] }
```

Nếu bị cắt, model được thông báo THẲNG bằng `truncationNote` (không cắt âm thầm) — tránh model tự suy diễn sai phạm vi. Team đo thật (13/32 skill bị cắt) và chọn refactor NỘI DUNG (viết gọn) thay vì tăng trần — context engineering đúng đắn.

### 7.5. Cấu trúc thật của một SKILL.md

Frontmatter (giữa 2 dấu `---`):

| Field | Bắt buộc | Ví dụ |
|---|---|---|
| `name` | Có (lỗi nếu thiếu) | `code-review` |
| `description` | Không bắt buộc về code, luôn có thực tế | `Review code for bugs, security, and best practices` |
| `when_to_use` | Không | `When user asks for code review...` |
| `triggers` | Không, nhưng skill mới đều có | `[review code, rà soát code, code review, pull request]` |
| `tools` | Không | `[file.read, shell.exec, git.diff, git.log]` |

Ví dụ thật (`skills/code-review/SKILL.md:1-7`):
```yaml
---
name: code-review
description: Review code for bugs, security, and best practices
when_to_use: When user asks for code review, PR review, or code quality check
triggers: [review code, rà soát code, kiểm tra code, code review, pull request, pr này]
tools: [file.read, shell.exec, git.diff, git.log]
---
```

Body tổ chức heading `##`/`###` lặp lại: mở đầu `# <Tên> Skill` → section chuyên môn (`## Review Checklist`, `### 1. Correctness`, `### 2. Security`...) → kết `## Process` mô tả quy trình step-by-step. Skill phức tạp (`api-designer/SKILL.md`, 137 dòng, ~10.9 KB — loại từng vượt `MaxPromptBytes`) theo khung: `## Nguyên tắc vàng` → nhiều section (`## Resource naming`, `## Status code`, `## Pagination`...) → `## Checklist review` (checkbox) + `## Anti-pattern`. Mẫu chung: ưu tiên checklist/quy trình cụ thể hơn mô tả trừu tượng — khớp best practice `docs/SKILLS.md` đề ra.

Đối chiếu `docs/SKILLS.md` — tự ghi "Status: Skills system is in planning (Phase 9)... Full loading + injection into agent context is upcoming", mô tả thiết kế đơn giản hơn thực tế (matching case-insensitive theo triggers, không scoring, không `budget.go`, field `version` mẫu không được `parseSkill()` đọc trong code thật). Bằng chứng implementation đã vượt xa design doc — hệ thống thật (scoring có trọng số, stop-words, truncate theo section, catalogue chỉ-tên tối ưu cache) tinh chỉnh hơn nhiều so với "Phase 9 planning doc", minh chứng hệ thống agentic trưởng thành qua vận hành thật (log dev, số liệu đo) chứ không dừng ở thiết kế trên giấy.
## 8. Tools Registry

### 8.1 Registry Pattern

`internal/tools/registry.go` implement đúng **Registry pattern** kinh điển: một `map[string]Tool` tra cứu theo tên, cộng thêm một slice `order` để giữ thứ tự đăng ký ổn định (map trong Go không có thứ tự lặp xác định, nên nếu chỉ dùng map thì `ToolDefs()` trả về danh sách tool cho LLM sẽ đảo lộn ngẫu nhiên giữa các lần gọi — ảnh hưởng tới prompt caching phía provider vì thứ tự tool trong request thay đổi sẽ làm cache-key thay đổi).

```go
// services/agent-go/internal/tools/registry.go:14-31
type Registry struct {
	tools map[string]Tool
	order []string // giữ thứ tự đăng ký cho All()/ToolDefs() ổn định
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register thêm (hoặc ghi đè) một tool theo Name(). Ghi đè không thêm lại vào order.
func (r *Registry) Register(t Tool) {
	name := t.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}
```

```go
// services/agent-go/internal/tools/registry.go:34-46
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}
```

`ToolDefs()` (registry.go:49-60) map mỗi `Tool` sang `provider.ToolDef{Name, Description, Schema}` — đây là cấu trúc provider-agnostic được nạp cho LLM; adapter riêng của từng provider (Gemini, DeepSeek, Anthropic...) dịch tiếp `[]provider.ToolDef` sang format function-calling riêng của họ.

**KHÔNG có self-registration qua `init()`.** Không file tool nào trong `internal/tools/*.go` có `func init() { ... }` gọi `Register` — mỗi file tool chỉ export một constructor thuần (`NewEchoTool()`, `NewCalculatorTool()`, `NewFileWriteTool(allowedPaths []string)`...) trả về giá trị implement `Tool`, KHÔNG tự đăng ký vào đâu cả. Việc đăng ký là **tập trung hoàn toàn ở call site**, cụ thể là hàm `buildRegistries()` và `registerRAGAndCodeExtras()` trong `cmd/server/main.go` (services/agent-go/cmd/server/main.go:365-419) — xem chi tiết ở mục 8.3. Đây là design có chủ đích: tool package không biết gì về wiring, wiring biết hết về tool package — dependency injection thủ công (constructor injection), không dùng DI framework hay registry toàn cục kiểu Go `sql.Register`.

Một điểm kiến trúc quan trọng khác: agent-go **không có một Registry duy nhất dùng chung**, mà có **3 Registry riêng biệt theo chuyên môn agent** — `code`, `research`, `general` — được orchestrator chọn theo `TriggerKeywords` (xem `orch.Register(&orchestrator.AgentSpec{...})` trong main.go dòng 238-315). Mỗi agent specialty có registry riêng nên `code` không có `translate`/`weather`, `research` không có `file.*`/`shell.exec`, v.v. — đây là một dạng **capability scoping theo agent**, giảm số tool phải nhồi vào 1 lần gọi LLM (giảm token, giảm rủi ro model gọi nhầm tool không phù hợp).

`Tool` interface (`tool.go:27-33`) rất tối giản — 5 method, không có hook lifecycle nào khác:

```go
// services/agent-go/internal/tools/tool.go:27-33
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema cho args
	Kind() Kind
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
```

`Kind` (tool.go:14-18) là enum 3 giá trị dùng cho guardrail:

```go
const (
	KindRead        Kind = iota // an toàn: ragSearch, listDocuments, readDocument, listTasks, recallMemory
	KindWrite                   // tạo/sửa: createTask, updateTask, saveMemory
	KindDestructive             // phá huỷ: deleteTask → cần HITL xác nhận
)
```

Có một interface tuỳ chọn thứ hai, `TimeoutTool` (tool.go:44-47), theo pattern "optional interface" rất Go-idiomatic (giống `io.Closer`/`sort.Interface` — không bắt buộc implement, registry dùng type-assertion `t.(TimeoutTool)` để phát hiện):

```go
type TimeoutTool interface {
	Tool
	Timeout() time.Duration
}
```

`Registry.runOne` (registry.go:132-164) dùng type-assertion này: nếu tool tự khai `Timeout() > 0` (như `shell.exec`) thì dùng deadline riêng của nó; còn lại — **mọi tool khác trong registry** — tự động được bọc bởi `DefaultToolTimeout = 60 * time.Second` (registry.go:127). Đây là một fix quan trọng được ghi rõ trong comment: trước đây chỉ `shell.exec` có deadline, một tool treo (Mongo/HTTP không phản hồi) sẽ treo luôn cả request vì `nodeTools` chờ `wg.Wait()` vô hạn.

Việc chạy nhiều `tool_call` song song nằm ở `RunParallel`/`RunParallelStreaming` (registry.go:70-114): dùng `sync.WaitGroup` + channel `done` thủ công (không dùng `errgroup` như `docs/TOOLS.md` mô tả — **đây là điểm doc đã lệch code thật**, xem ghi chú cuối mục 8.3), kết quả được viết vào slice `results[i]` đã pre-allocate theo index để giữ thứ tự đầu vào dù goroutine hoàn thành không theo thứ tự. `onResult` callback cho phép caller stream `tool_end` SSE event ngay khi từng tool xong, không cần chờ tất cả — quan trọng cho UX real-time của chat.

Lỗi được đóng gói thành 2 kiểu implement `error` + `Unwrap()` để dùng được với `errors.As`/`errors.Is`: `TimeoutError` (registry.go:167-176, chỉ gán khi CHÍNH context con vừa tạo hết hạn — không gán nhãn sai khi ctx CHA của request chết trước) và `NotFoundError` (registry.go:179-185, khi LLM gọi tên tool không tồn tại trong registry — không panic).

---

### 8.2 Bảng đầy đủ tool

Toàn bộ **25 tool** có source file trong `internal/tools/` (tên tool lấy đúng từ `Name()` trả về trong code, KHÔNG suy đoán):

| # | Tên tool (`Name()`) | File | Chức năng ngắn | Input params chính (Schema) |
|---|---|---|---|---|
| 1 | `echo` | `echo.go:15` | Trả nguyên văn args đầu vào — dùng học/test luồng tool call. `Kind=KindRead` | bất kỳ object (`additionalProperties: true`) |
| 2 | `calculator` | `calculator.go:21` | Đánh giá biểu thức toán học an toàn bằng recursive-descent parser tự viết (KHÔNG `eval`). Hỗ trợ `+ - * / % **`, `sqrt/sin/cos/tan/abs/round/floor/ceil/log/log10/exp`. `Kind=KindRead` | `expression: string (required)` |
| 3 | `calendar` | `calendar.go:29` | Quản lý calendar `.ics` cục bộ trên đĩa: liệt kê event hôm nay hoặc thêm event mới. `Kind=KindRead` | `action: enum[today,add]`, `title`, `date (YYYY-MM-DD)`, `time (HH:MM)`, `duration` |
| 4 | `datetime` | `datetime.go:21` | Giờ hiện tại, đổi timezone, cộng/trừ duration, tính khoảng cách 2 thời điểm. `Kind=KindRead` | `operation: enum[now,convert,add,diff]`, `datetime`, `timezone`, `duration`, `datetime2`, `format` |
| 5 | `file.write` | `file_write.go:29` | Ghi content vào file trong `allowedPaths`, tự tạo parent dir, giới hạn 100KB. `Kind=KindWrite` | `path: string (required)`, `content: string (required)` |
| 6 | `file.search` | `files.go:30` | Tìm file theo glob pattern trong thư mục được phép (dùng `filepath.Walk`). `Kind=KindRead` | `pattern: string (required)`, `path: string (optional)` |
| 7 | `file.read` | `files.go:174` | Đọc nội dung file text trong thư mục được phép, cắt bớt nếu > 24.000 ký tự (`defaultMaxSize`). `Kind=KindRead` | `path: string (required)` |
| 8 | `git` | `git.go:36` | Thực thi **read-only** git subcommand (`log/diff/status/branch/show` — allowlist cứng `readOnlyGitOps`), output cắt 8.000 ký tự. `Kind=KindRead` | `operation: enum[log,diff,status,branch,show]`, `args: []string` |
| 9 | `http` | `http.go:30` | Gọi HTTP request tuỳ ý (GET/POST/PUT/DELETE/PATCH), trả status/headers/body (cắt 10KB). `Kind=KindWrite` | `method: enum`, `url: uri (required)`, `headers: object`, `body: string` |
| 10 | `json` | `json.go:22` | Parse/format/validate/truy vấn JSON theo path dạng `a.b.0.c`. `Kind=KindRead` | `operation: enum[format,get,validate]`, `data: string (required)`, `path: string` |
| 11 | `memory.save` | `memory_tools.go:49` | Lưu key-value vào `memoryBackend` (chung instance với pipeline recall/extract tự động). `Kind=KindWrite` | `key: string (required)`, `value: string (required)` |
| 12 | `memory.recall` | `memory_tools.go:111` | Tìm memory theo keyword (case-insensitive substring). `Kind=KindRead` | `keyword: string (required)` |
| 13 | `memory.list` | `memory_tools.go:174` | Liệt kê toàn bộ memory đã lưu của tenant hiện tại. `Kind=KindRead` | (không tham số) |
| 14 | `notes.search` | `notes.go:32` | Full-text search note markdown trong thư mục của tenant; query rỗng/`"*"` = liệt kê tất cả. `Kind=KindRead` | `query: string (optional)` |
| 15 | `notes.create` | `notes.go:142` | Tạo note markdown mới với tags tuỳ chọn, filename được sanitize. `Kind=KindWrite` | `title: string (required)`, `content: string (required)`, `tags: []string` |
| 16 | `rag.search` | `rag.go:70` | Tìm kiếm semantic (Atlas `$vectorSearch` + tuỳ chọn hybrid RRF/rerank/HyDE/Parent Doc Retrieval) trong knowledge base RAG của tenant. `Kind=KindRead` | `query: string (required)` |
| 17 | `rag.list` | `rag.go:136` | Liệt kê ĐẦY ĐỦ metadata mọi tài liệu RAG của tenant (tối đa 200), không trả nội dung. `Kind=KindRead` | (không tham số) |
| 18 | `rag.read` | `rag.go:263` | Đọc toàn văn 1 tài liệu RAG theo `documentId` hoặc `source`, cắt ở 24.000 ký tự. `Kind=KindRead` | `documentId: string`, `source: string` (1 trong 2) |
| 19 | `shell.exec` | `shell.go:45` | Thực thi shell command qua `os/exec` trực tiếp trên host, timeout mặc định 30s, output cắt 8.000 ký tự. `Kind=KindDestructive` | `command: string (required)`, `args: []string` |
| 20 | `timer` | `timer.go:31` | Đặt/liệt kê/hủy timer nhắc nhở trong RAM (goroutine `time.After`), tối đa 24h. `Kind=KindRead` | `action: enum[set,list,cancel]`, `message`, `duration`, `id` |
| 21 | `translate` | `translate.go:25` | Dịch text qua LibreTranslate API công khai (free, không cần key). `Kind=KindRead` | `text: string (required)`, `source: string`, `target: string (required)` |
| 22 | `version` | `version.go:24` | Tra phiên bản mới nhất của package npm hoặc release GitHub qua API chính thức. `Kind=KindRead` | `source: enum[npm,github]`, `package`, `owner`, `repo` |
| 23 | `weather` | `weather.go:26` | Lấy thời tiết hiện tại theo tên thành phố qua `wttr.in` (free, không cần key). `Kind=KindRead` | `city: string (required)` |
| 24 | `web.search` | `web.go:76` | Tìm kiếm web — race song song 3 backend (Tavily API ưu tiên, fallback Google/Bing HTML scraping), có cache TTL 5 phút. `Kind=KindRead` | `query: string (required)` |
| 25 | `web.fetch` | `web.go:417` | Tải nội dung 1 URL, tự strip HTML thành text thuần, cắt ở 15.000 ký tự. `Kind=KindRead` | `url: uri (required)` |

Ghi chú đối chiếu nhanh với `Kind`: trong toàn bộ 25 tool, **chỉ duy nhất `shell.exec` được gán `KindDestructive`** (xác nhận bằng `grep -rn "KindDestructive" internal/tools/*.go` — chỉ khớp `shell.go:64` và định nghĩa enum trong `tool.go`). `file.write`, `http`, `memory.save`, `notes.create` là `KindWrite` (ghi có side-effect nhưng không cần HITL). Phần còn lại là `KindRead`.

---

### 8.3 Ghost tools — tool bị bỏ quên

Đối chiếu 25 tool ở mục 8.2 với **toàn bộ** danh sách đăng ký thật trong `cmd/server/main.go`:

```go
// services/agent-go/cmd/server/main.go:365-402 — buildRegistries(), TOÀN BỘ danh sách Register
func buildRegistries(cfg config.Config, store *memory.Store) (code, research, general *tools.Registry) {
	code = tools.NewRegistry()
	code.Register(tools.NewFileSearchTool(cfg.AllowedPaths))      // file.search
	code.Register(tools.NewFileReadTool(cfg.AllowedPaths))        // file.read
	code.Register(tools.NewFileWriteTool(cfg.AllowedPaths))       // file.write
	code.Register(tools.NewShellToolWithTimeout(nil, ...))        // shell.exec
	code.Register(tools.NewGitTool("."))                          // git
	code.Register(tools.NewVersionTool())                         // version
	code.Register(tools.NewSaveMemoryTool(store))                 // memory.save
	code.Register(tools.NewRecallMemoryTool(store))               // memory.recall
	code.Register(tools.NewListMemoriesTool(store))               // memory.list

	research = tools.NewRegistry()
	research.Register(tools.NewWebSearchTool(nil))                // web.search
	research.Register(tools.NewWebFetchTool(nil))                 // web.fetch
	research.Register(tools.NewNotesSearchTool("."))               // notes.search
	research.Register(tools.NewNotesCreateTool("."))               // notes.create
	research.Register(tools.NewSaveMemoryTool(store))
	research.Register(tools.NewRecallMemoryTool(store))
	research.Register(tools.NewListMemoriesTool(store))

	general = tools.NewRegistry()
	general.Register(tools.NewEchoTool())                          // echo
	general.Register(tools.NewFileSearchTool(cfg.AllowedPaths))
	general.Register(tools.NewFileReadTool(cfg.AllowedPaths))
	general.Register(tools.NewFileWriteTool(cfg.AllowedPaths))
	general.Register(tools.NewShellToolWithTimeout(nil, ...))
	general.Register(tools.NewWebSearchTool(nil))
	general.Register(tools.NewWebFetchTool(nil))
	general.Register(tools.NewTranslateTool(nil))                  // translate
	general.Register(tools.NewCalculatorTool())                    // calculator
	general.Register(tools.NewDateTimeTool())                      // datetime
	general.Register(tools.NewSaveMemoryTool(store))
	general.Register(tools.NewRecallMemoryTool(store))
	general.Register(tools.NewListMemoriesTool(store))

	return code, research, general
}
```

```go
// services/agent-go/cmd/server/main.go:410-419 — registerRAGAndCodeExtras()
func registerRAGAndCodeExtras(codeRegistry, researchRegistry, generalRegistry *tools.Registry, ragTool, ragReadTool, ragListTool tools.Tool) {
	for _, reg := range []*tools.Registry{codeRegistry, researchRegistry, generalRegistry} {
		reg.Register(ragTool)      // rag.search
		reg.Register(ragReadTool)  // rag.read
		reg.Register(ragListTool)  // rag.list
	}
	codeRegistry.Register(tools.NewWebSearchTool(nil))
	codeRegistry.Register(tools.NewWebFetchTool(nil))
}
```

**Main.go dòng 365-419 (`buildRegistries` + `registerRAGAndCodeExtras`) là TOÀN BỘ nơi `.Register(...)` được gọi trong production code** (đã xác nhận bằng `grep -n "\.Register("` trên toàn file — không còn lệnh `Register` nào khác trong `main.go`, và `grep -rn "tools\.New" cmd/server/main.go` cho danh sách constructor khớp chính xác đoạn trên). Hợp tất cả tên tool xuất hiện trong 2 hàm này (union qua cả 3 registry): `file.search, file.read, file.write, shell.exec, git, version, memory.save, memory.recall, memory.list, web.search, web.fetch, notes.search, notes.create, translate, calculator, datetime, echo, rag.search, rag.read, rag.list` — **20 tool**.

So với 25 tool có source file ở mục 8.2, **5 tool sau có constructor export, có test coverage, nhưng KHÔNG hề được `.Register()` ở bất kỳ đâu trong `main.go`** — tức chúng tồn tại trong binary (đã compile) nhưng **không một agent nào (`code`/`research`/`general`) có thể gọi được chúng lúc runtime**, vì không nằm trong registry nào cả:

| Tool bị bỏ quên | Nơi định nghĩa (constructor) | Xác nhận thiếu ở main.go |
|---|---|---|
| `calendar` | `services/agent-go/internal/tools/calendar.go:21` — `func NewCalendarTool(icsPath string) Tool` | main.go dòng 365-419 (toàn bộ danh sách `.Register`) KHÔNG có `NewCalendarTool` |
| `http` | `services/agent-go/internal/tools/http.go:23` — `func NewHTTPTool(client *http.Client) Tool` | main.go dòng 365-419 KHÔNG có `NewHTTPTool` |
| `json` | `services/agent-go/internal/tools/json.go:18` — `func NewJSONTool() Tool` | main.go dòng 365-419 KHÔNG có `NewJSONTool` |
| `timer` | `services/agent-go/internal/tools/timer.go:25` — `func NewTimerTool() Tool` | main.go dòng 365-419 KHÔNG có `NewTimerTool` |
| `weather` | `services/agent-go/internal/tools/weather.go:19` — `func NewWeatherTool(client *http.Client) Tool` | main.go dòng 365-419 KHÔNG có `NewWeatherTool` |

Đã double-check bằng `grep -rn "NewCalendarTool\|NewHTTPTool\|NewJSONTool\|NewTimerTool\|NewWeatherTool" --include="*.go" .` trên TOÀN repo (không chỉ `main.go`) và loại `_test.go`: **kết quả 0 match** ngoài chính file định nghĩa. Nghĩa là 5 constructor này chỉ được gọi ở đúng 1 nơi khác trong toàn repo: `internal/tools/metadata_test.go:16-40`, trong helper `allTools(t)` — dùng để test metadata (`Name()/Description()/Schema()/Kind()` hợp lệ), **không phải wiring production**. Cũng không có `cmd/jarvis/main.go` hay `cmd/promptsize/main.go` nào đăng ký chúng (đã kiểm tra `find services/agent-go/cmd -type d` → chỉ có `server`, `promptsize`, `jarvis`; grep xác nhận `promptsize/main.go` chỉ đăng ký `shell.exec`).

Hệ quả thực tế: `calendar.go`, `http.go`, `json.go`, `timer.go`, `weather.go` là **dead code ở tầng production** — người dùng JARVIS thật (qua `/chat`) không bao giờ có thể gọi 5 tool này, dù chúng pass test và trông như đã "xong". Đây là dấu hiệu điển hình của tool được viết/test trong 1 sprint học tập rồi quên wiring — một bài học interview tốt về khoảng cách giữa "code tồn tại + có test" và "code thực sự chạy trong production path".

**Ghi chú lệch doc:** `docs/TOOLS.md` (root repo) mục "Built-in Tools Reference" (dòng 334-380) chỉ liệt kê 4 tool (`echo`, `file.search`, `file.read`, `web.search`, `web.fetch`) và mô tả `web.search` dùng "DuckDuckGo Instant Answer API" — thực tế code (`web.go:51-55`) dùng race 3 backend `searchTavily → searchGoogleWeb → searchBingWeb`, không đụng DuckDuckGo. Doc cũng mô tả `RunParallel` dùng `errgroup.Group` (dòng 230-246) — code thật (`registry.go:83-114`) dùng `sync.WaitGroup` + channel thủ công. `docs/TOOLS.md` nên được coi là **hướng dẫn quy trình** (cách viết tool mới) hơn là **nguồn sự thật** về danh sách tool hiện có — một lý do nữa nên tin code hơn doc khi audit.

## 9. RAG Pipeline

### 9.1 Khái niệm chung: Hybrid Search, Rerank, HyDE

Trước khi soi code, cần nắm 4 khái niệm hay bị nhầm lẫn — vì tên gọi trong code/comment của Jarvis dùng đúng các thuật ngữ này, nên phải hiểu đúng bản chất mới đánh giá được implementation có "danh xưng đúng với thực chất" hay không.

**Hybrid Search** — kết hợp hai kiểu retrieval bù trừ cho nhau:
- **Dense retrieval**: câu query và document được embed thành vector (embedding), so khớp bằng cosine similarity / dot product. Bắt được ngữ nghĩa ("xe hơi" ≈ "ô tô") nhưng dễ bỏ lỡ khi câu hỏi chứa từ khoá hiếm/định danh chính xác (mã lỗi, tên biến, số phiên bản).
- **Sparse/lexical retrieval (BM25/keyword)**: so khớp trực tiếp từ (term frequency, inverse document frequency) — mạnh với exact-match, từ khoá hiếm, nhưng không hiểu đồng nghĩa.
- Hybrid = chạy cả hai, rồi **fuse** kết quả (phổ biến nhất: **Reciprocal Rank Fusion — RRF**, công thức `score = Σ 1/(k + rank_i)` cho mỗi hệ thống retrieval, k là hằng số làm mượt, thường 60).

**Reranking** — retrieval thô (dense/sparse) tối ưu cho **recall** (không bỏ lỡ), nên top-K ban đầu thường có nhiều nhiễu. Rerank dùng một model **khác** — kinh điển là **cross-encoder** (nhận đồng thời [query, document] làm input, chấm điểm độ liên quan trực tiếp — chính xác hơn dense vector vì "thấy" cả hai văn bản cùng lúc, nhưng chậm hơn nên chỉ chạy trên top-K nhỏ, không chạy trên toàn bộ corpus). Rerank chuẩn công nghiệp: Cohere Rerank, `bge-reranker`, v.v.

**HyDE (Hypothetical Document Embeddings)** — kỹ thuật: dùng LLM sinh ra một **đoạn văn trả lời giả định** cho câu hỏi (không cần đúng sự thật), rồi **embed đoạn văn giả định đó** thay vì embed câu hỏi gốc để đi search. Lý do: câu hỏi ("Postgres index hoạt động thế nào?") và đoạn trả lời thật trong tài liệu ("B-tree index trong Postgres...") có "kiểu văn phong" khác nhau (câu hỏi vs. câu trả lời khẳng định) — không gian embedding của model thường match tốt hơn giữa hai đoạn **cùng kiểu văn phong** (trả lời ↔ trả lời) hơn là giữa câu hỏi ↔ trả lời.

**PDR** — ban đầu không chắc PDR là viết tắt gì trong ngữ cảnh code. Sau khi đọc `internal/tools/rag.go`, PDR **có xuất hiện thật** trong comment: `EnableParentRetrieval` — **Parent Document Retrieval**. Đây là kỹ thuật: chunk nhỏ (embed chính xác hơn vì ít nhiễu ngữ nghĩa) dùng để **match**, nhưng khi trả kết quả cho LLM thì **mở rộng ngữ cảnh** bằng cách lấy thêm các chunk "cha"/chunk liền kề (window) xung quanh chunk khớp — tránh tình trạng LLM chỉ nhận được một đoạn ~500 ký tự cụt lủn giữa câu.

### 9.2 Voyage Embedder

`internal/rag/voyage.go` **là một HTTP client thật**, gọi thẳng tới Voyage AI REST API — không phải mock/stub.

```go
// services/agent-go/internal/rag/voyage.go:15-19
const (
    voyageURL   = "https://api.voyageai.com/v1/embeddings"
    voyageModel = "voyage-3" // 1024 chiều — khớp numDimensions của Atlas vector_index
    batchSize   = 96         // an toàn dưới giới hạn số text/request của Voyage
)
```

Request body và hàm gọi HTTP thật (POST, header `Authorization: Bearer <apiKey>`):

```go
// services/agent-go/internal/rag/voyage.go:78-99
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
    ...
}
```

Input/output format:
- **Input**: `EmbedRequest{Input []string, Model string, InputType string}` — `input_type` là `"document"` khi nạp tài liệu vào KB, `"query"` khi embed câu hỏi người dùng (Voyage tối ưu embedding khác nhau cho 2 loại này — asymmetric embedding).
- **Output**: `embedResponse{Data []struct{ Embedding []float64 }}` — parse JSON, trích trường `embedding` của mỗi item, giữ đúng thứ tự.
- **Model**: `voyage-3`, 1024 chiều (comment ghi rõ khớp `numDimensions` của Atlas vector index).
- **Batching**: `Embed()` (dòng 66-76) chia text thành các batch ≤ 96 (`batchTexts`), gọi **tuần tự** (không parallel) "để tôn trọng rate limit", rồi gộp kết quả theo đúng thứ tự gốc.

`voyage_test.go` và `client_test.go` xác nhận đây là HTTP client thật bằng cách test qua `httptest.NewServer` + `redirectTransport` (ép request tới URL Voyage hard-code redirect về server test) — test check cả `Authorization` header, lỗi HTTP 429, lỗi decode JSON, lỗi transport/network, context cancellation. Đây là bộ test của một client **gọi network thật**, không phải test một implementation giả lập nội bộ.

### 9.3 Query pipeline thật trong code

Đây là phần quan trọng nhất: **kiểm chứng từng kỹ thuật có thật trong `internal/tools/rag.go` hay chỉ là tên gọi suông**.

**Kết luận nhanh (bảng đối chiếu tên gọi ↔ thực tế):**

| Kỹ thuật | Có bật cờ (`RAGSearchConfig`) | Default | Thực tế trong code |
|---|---|---|---|
| Hybrid search | `EnableHybridSearch` | **true** | THẬT: dense `$vectorSearch` + sparse Mongo `$text`, merge bằng RRF có trọng số |
| Rerank (miễn phí) | `EnableRerank` | **true** | KHÔNG PHẢI cross-encoder — chỉ là keyword-overlap heuristic (đếm số từ trùng) |
| Rerank (LLM) | `EnableLLMRerank` | false (tốn LLM call) | LLM-as-reranker qua prompting, KHÔNG PHẢI model rerank chuyên dụng (cross-encoder) |
| HyDE | `EnableHyDE` | false (tốn LLM call) | THẬT: gọi LLM sinh câu trả lời giả định trước khi embed |
| Parent Document Retrieval | `EnableParentRetrieval` | **true** | THẬT: mở rộng content bằng chunk liền kề (chunkIndex ±1) |

Cấu hình default lấy từ `internal/config/config.go:179-183`:
```go
EnableHybridSearch:    envOr("ENABLE_HYBRID_SEARCH", "true") == "true",
EnableRerank:          envOr("ENABLE_RERANK", "true") == "true",
EnableParentRetrieval: envOr("ENABLE_PARENT_RETRIEVAL", "true") == "true",
EnableLLMRerank:       envOr("ENABLE_LLM_RERANK", "false") == "true",
EnableHyDE:            envOr("ENABLE_HYDE", "false") == "true",
```

**Bước 0 — HyDE (thật, opt-in):**

```go
// services/agent-go/internal/tools/rag.go:389-398
embedInput := parsed.Query
if t.cfg.EnableHyDE && t.prov != nil && t.model != "" {
    if hypo := t.generateHypotheticalAnswer(ctx, parsed.Query); hypo != "" {
        embedInput = hypo
    }
}
```

`generateHypotheticalAnswer` (dòng 799-841) gọi **thật** một LLM (`t.prov.Generate`) với system prompt:
```go
// services/agent-go/internal/tools/rag.go:810-819
req := provider.GenerateRequest{
    System: "Viết 1 đoạn văn NGẮN (2-4 câu) trả lời giả định cho câu hỏi, " +
        "như thể trích từ 1 tài liệu kỹ thuật thật. KHÔNG giải thích, " +
        "KHÔNG hỏi lại, CHỈ trả về đoạn văn.",
    Messages: []provider.Message{{Role: provider.RoleUser, Content: query}},
    Options: provider.ProviderOptions{
        Model:         t.model,
        MaxTokens:     200,
        ThinkingLevel: provider.ThinkingOff,
    },
}
```
Có timeout riêng 8s độc lập với ctx cha (tránh HyDE ăn hết budget rồi làm chết luôn bước embed sau đó), và nếu lỗi/rỗng thì fallback im lặng dùng câu hỏi gốc — đúng như comment mô tả, không phải placeholder giả.

Lưu ý quan trọng: HyDE **chỉ áp dụng cho nhánh vector** (dense). Nhánh text search (sparse/BM25) trong `hybridSearch` vẫn dùng câu hỏi thô `parsed.Query`, không dùng văn bản giả định — điều này đúng với lý thuyết HyDE gốc (HyDE chỉ có ý nghĩa cho dense embedding, không có ý nghĩa cho lexical match).

**Bước 1 — Embed qua Voyage (thật):**

```go
// services/agent-go/internal/tools/rag.go:401-408
vecs, err := t.voyageClient.Embed(ctx, []string{embedInput}, "query")
...
queryVector := vecs[0]
```

**Bước 2 — Retrieve: hybrid THẬT hay chỉ dense đơn giản?**

Trả lời dựa trên code: **có nhánh hybrid thật**, không phải chỉ dense.

```go
// services/agent-go/internal/tools/rag.go:410-419
var candidates []ragSearchResult
if t.cfg.EnableHybridSearch {
    candidates, err = t.hybridSearch(ctx, parsed.Query, queryVector, tenantID)
} else {
    candidates, err = t.vectorSearch(ctx, queryVector, tenantID, 20)
}
```

`vectorSearch` (dòng 466-513) là dense — MongoDB Atlas `$vectorSearch` aggregation stage thật:
```go
// services/agent-go/internal/tools/rag.go:469-477
{Key: "$vectorSearch", Value: bson.D{
    {Key: "index", Value: "vector_index"},
    {Key: "path", Value: "embedding"},
    {Key: "queryVector", Value: queryVector},
    {Key: "numCandidates", Value: 100},
    {Key: "limit", Value: limit},
}},
```

`textSearch` (dòng 517-573) là sparse/lexical — MongoDB `$text`/`$search` (dùng TF-IDF-based textScore của Mongo, tương đương BM25-family, không phải semantic):
```go
// services/agent-go/internal/tools/rag.go:519-523
matchDoc := bson.D{
    {Key: "$text", Value: bson.D{
        {Key: "$search", Value: query},
    }},
}
```
Nếu Mongo chưa tạo text index thì aggregate lỗi có chứa `"text index"` → hàm trả `nil, nil` (degrade êm về dense-only), không phải panic (dòng 561-563).

`hybridSearch` (dòng 578-647) chạy CẢ HAI, rồi merge bằng **Reciprocal Rank Fusion có trọng số**:
```go
// services/agent-go/internal/tools/rag.go:578-579, 626-639
func (t *ragSearchTool) hybridSearch(ctx context.Context, query string, queryVector []float64, tenantID string) ([]ragSearchResult, error) {
    const k = 60 // RRF constant
    ...
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
```
Nếu text search không có kết quả (chưa có text index / không match từ khoá), fallback về dense-only nhưng vẫn dedup theo document (dòng 598-599):
```go
// services/agent-go/internal/tools/rag.go:598-599
if len(textResults) == 0 {
    return dedupeByDocument(vecResults), nil
}
```

→ **Kết luận**: hybrid search ở đây là THẬT — 2 hệ thống retrieval độc lập (Atlas `$vectorSearch` cho dense, Mongo `$text` cho lexical), merge bằng RRF có trọng số 0.7/0.3 nghiêng về vector. Không phải "gọi 1 embedding rồi similarity search" đơn giản như giả định ban đầu.

**Bước 3 — Rerank: có model rerank chuyên dụng (cross-encoder) không?**

Trả lời dựa trên code: **KHÔNG có cross-encoder / model rerank chuyên dụng nào được gọi**. Có 2 cơ chế, loại trừ nhau (LLM rerank ưu tiên hơn keyword rerank nếu cả 2 cùng bật):

```go
// services/agent-go/internal/tools/rag.go:429-440
const maxLLMRerankCandidates = 10
if t.cfg.EnableLLMRerank && t.prov != nil && t.model != "" && len(candidates) > 1 {
    head := candidates
    var tail []ragSearchResult
    if len(head) > maxLLMRerankCandidates {
        head, tail = candidates[:maxLLMRerankCandidates], candidates[maxLLMRerankCandidates:]
    }
    reranked := t.rerankLLM(ctx, parsed.Query, head)
    candidates = append(reranked, tail...)
} else if t.cfg.EnableRerank && len(candidates) > 0 {
    candidates = t.rerankKeyword(parsed.Query, candidates)
}
```

- **`EnableRerank` (default true, "MIỄN PHÍ")** → `rerankKeyword` (dòng 678-705): **không phải rerank model nào cả**, chỉ đếm số từ khoá trùng giữa query và snippet, rồi trộn `0.7*score_gốc + 0.3*overlap_score`:
```go
// services/agent-go/internal/tools/rag.go:693-697
overlapScore := 0.0
if len(queryTerms) > 0 {
    overlapScore = float64(overlap) / float64(len(queryTerms))
}
results[i].Score = 0.7*results[i].Score + 0.3*overlapScore
```
Comment trong code cũng tự thừa nhận điều này: `"This is a lightweight pragmatic reranker: no LLM call, no cross-encoder."` (dòng 676-677).

- **`EnableLLMRerank` (default false)** → `rerankLLM` (dòng 850-898): gọi **LLM tổng quát** (cùng provider dùng cho hội thoại, model rẻ — ví dụ DeepSeek flash), prompt yêu cầu LLM trả về **1 mảng hoán vị index JSON** (`[2,0,3,1]`) xếp theo độ liên quan. Đây là kỹ thuật **"LLM-as-reranker" qua prompting**, KHÔNG PHẢI gọi một cross-encoder model chuyên dụng (như Cohere Rerank hay `bge-reranker`) vốn được **train riêng** cho tác vụ chấm điểm [query, doc]. Có `parseRerankOrder` (dòng 909-933) validate output phải là hoán vị hợp lệ của `[0,n)` (chấp nhận cả 1-based do LLM hay đánh số từ 1), nếu parse fail thì giữ nguyên thứ tự gốc — không silently drop/duplicate kết quả.

→ **Khoảng cách tên gọi vs thực tế**: comment/config gọi đây là "rerank" và về mặt hành vi (re-sort top-K theo độ liên quan tốt hơn) đúng là rerank, nhưng **không có bước nào gọi một model rerank/cross-encoder chuyên dụng** như trong RAG pipeline chuẩn công nghiệp. Nhánh miễn phí là keyword overlap heuristic thuần Go; nhánh LLM rerank là prompt-engineering trên LLM tổng quát.

**Bước 4 — Top 5, rồi PDR (thật):**

```go
// services/agent-go/internal/tools/rag.go:442-451
if len(candidates) > 5 {
    candidates = candidates[:5]
}

if t.cfg.EnableParentRetrieval && len(candidates) > 0 {
    candidates = t.expandParentWindow(ctx, candidates, tenantID)
}
```

`expandParentWindow` (dòng 735-781) query thêm Mongo lấy chunk lân cận (`chunkIndex ± 1`, cùng `documentId`), nối text lại thành `Content` mở rộng — nhưng **giữ nguyên `Snippet`** gốc (đoạn khớp thật) để không đánh lạc hướng preview. Đây đúng là **Parent Document Retrieval**: match trên chunk nhỏ, trả về ngữ cảnh rộng hơn.

### 9.4 Multi-tenant filtering

Filter theo tenant **có thật** và được áp dụng nhất quán ở **mọi** điểm truy vấn Mongo trong luồng RAG — không phải chỉ 1 nơi. Tenant lấy từ `middleware.GetTenantID(ctx)`, quy tắc chung: bỏ qua filter khi tenantID rỗng hoặc `"default"` (chế độ single-user tương thích ngược).

1. **`vectorSearch`** (dense) — filter bằng `$match` **sau** stage `$vectorSearch` (Atlas vector index tự nó không filter theo field thường, nên phải match riêng):
```go
// services/agent-go/internal/tools/rag.go:480-484
if tenantID != "" && tenantID != "default" {
    pipeline = append(pipeline, bson.D{
        {Key: "$match", Value: bson.D{{Key: "tenantId", Value: tenantID}}},
    })
}
```

2. **`textSearch`** (sparse) — filter ngay trong `$match` cùng với `$text`:
```go
// services/agent-go/internal/tools/rag.go:524-526
if tenantID != "" && tenantID != "default" {
    matchDoc = append(matchDoc, bson.E{Key: "tenantId", Value: tenantID})
}
```

3. **`rag.read`** (đọc full document) — `buildRAGReadFilter`:
```go
// services/agent-go/internal/tools/rag.go:299-309
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
```
Comment tại dòng 294-298 nói rõ đây từng là một **lỗ hổng bảo mật thật** ("this is what previously made rag.read leak data across tenants") — tức trước khi có filter này, tenant A biết/đoán được `documentId`/`source` của tenant B là đọc được toàn bộ nội dung tài liệu của B.

4. **`rag.list`** (liệt kê tài liệu) — `buildRAGListPipeline`:
```go
// services/agent-go/internal/tools/rag.go:170-174
func buildRAGListPipeline(tenantID string) []bson.D {
    match := bson.D{}
    if tenantID != "" && tenantID != "default" {
        match = append(match, bson.E{Key: "tenantId", Value: tenantID})
    }
```

5. **Parent Document Retrieval window** — `buildParentWindowFilter` cũng scope theo tenant (dòng 712-723), tránh PDR vô tình mở rộng sang chunk của tenant khác nếu (giả định lý thuyết) có đụng documentId trùng.

Các test (`rag_read_test.go`, `rag_list_test.go`, `rag_advanced_test.go`) đều assert trực tiếp trên các hàm build-filter thuần (không cần Mongo thật) — ví dụ `TestBuildRAGReadFilter_ScopesToTenant`, `TestBuildRAGListPipeline_ScopesToTenant` — với comment giải thích rõ hậu quả nếu thiếu filter là rò rỉ dữ liệu chéo giữa các tenant.

---

## 10. Storage: SQLite & Chroma

### 10.1 SQLite vector search

**Kết luận trước tiên (quan trọng): `internal/storage/sqlite/sqlite.go` KHÔNG có bất kỳ vector search nào** — không có cột lưu embedding, không load extension `sqlite-vec`/`sqlite-vss`, không có hàm tính cosine similarity nào trong package này. Toàn bộ "search" trong package này là **full-text keyword search qua FTS5** (SQLite built-in virtual table), phục vụ `conversations`/`messages`/`memories` (Tier 2/3 của kiến trúc memory 4-tier) — hoàn toàn không liên quan tới RAG document search (RAG dùng MongoDB Atlas `$vectorSearch`, xem mục 9.3).

Package doc comment tự nói rõ phạm vi:
```go
// services/agent-go/internal/storage/sqlite/sqlite.go:1-3
// Package sqlite cung cấp SQLite storage cho JARVIS: conversations, messages, memories.
// Dùng modernc.org/sqlite (pure Go, không CGO). Auto-migrate schema khi mở DB.
```
`modernc.org/sqlite` là driver **pure Go** (compile-to-Go từ C, không dùng CGO) — nghĩa là **không thể** load native extension `.so`/`.dylib` như `sqlite-vec`/`sqlite-vss` (các extension đó cần CGO để load qua `sqlite3_auto_extension` hoặc `sqlite3_enable_load_extension`). Import duy nhất là:
```go
// services/agent-go/internal/storage/sqlite/sqlite.go:10
_ "modernc.org/sqlite"
```
— không có bất kỳ cgo import (`import "C"`) hay lệnh `.LoadExtension(...)` nào trong toàn bộ package.

Schema thật (migrate, dòng 76-115) chỉ có FTS5 virtual table cho text, KHÔNG có cột embedding/vector nào:
```go
// services/agent-go/internal/storage/sqlite/sqlite.go:85-96
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role TEXT NOT NULL CHECK (role IN ('user','assistant','system','tool')),
    content TEXT NOT NULL DEFAULT '',
    tool_calls TEXT DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(content, content=messages, content_rowid=id);
```
Tương tự cho `memories`/`memories_fts` (dòng 98-111). Truy vấn "search" thật là FTS5 `MATCH` + built-in BM25-based `rank`:
```go
// services/agent-go/internal/storage/sqlite/sqlite.go:212-215
rows, err := s.db.Query(
    "SELECT m.id, m.conversation_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.created_at FROM messages m JOIN messages_fts fts ON m.id = fts.rowid WHERE messages_fts MATCH ? ORDER BY rank LIMIT ?",
    query, limit,
)
```

**Complexity**: đây là **keyword/lexical search** dùng chỉ mục FTS5 có sẵn của SQLite (dựa trên inverted index nội bộ + hàm rank BM25-like), **không phải** linear scan tính cosine similarity thuần Go, nhưng cũng **không phải** vector search — nó thuộc lớp bài toán hoàn toàn khác (lexical matching, không có khái niệm "chiều embedding" hay "similarity score" theo nghĩa vector).

**Đối chiếu với kỳ vọng ban đầu**: có 2 khả năng thường gặp ("dùng extension sqlite-vec/sqlite-vss" hay "linear scan cosine similarity thuần Go") — **cả hai đều sai** vì thực tế package này không làm vector search. Đây chính là kiểu "gap giữa kỳ vọng và implementation thật" cần lưu ý khi phỏng vấn: tên file/package gợi ý storage tổng quát, nhưng đọc kỹ code thì nó chỉ là keyword search cho lịch sử chat/memory, và **hoàn toàn tách biệt** khỏi pipeline RAG.

Một điểm bổ sung đáng chú ý: `internal/storage/sqlite` **không được import ở bất kỳ đâu ngoài file test của chính nó** (grep `internal/storage/sqlite` loại `_test.go` → 0 kết quả). `cmd/server/main.go` dùng `memory.NewStore()` (package `internal/memory`, khác hoàn toàn) cho memory Tier 2/3, không dùng `storage/sqlite`. Vậy package `storage/sqlite` hiện là **code chưa được wire vào runtime nào cả** (orphaned/dead code trong build hiện tại) — có thể là phần khung sẵn cho một giai đoạn migrate sau (SQLite làm persistent local store) chưa được nối dây.

### 10.2 Chroma package — client thật hay fake?

**Kết luận dứt khoát: `internal/storage/chroma/chroma.go` là một in-memory implementation TỰ VIẾT, KHÔNG hề gọi network tới ChromaDB thật.** Tên package "chroma" gây hiểu lầm nghiêm trọng.

Bằng chứng trực tiếp — package doc comment tự thừa nhận:
```go
// services/agent-go/internal/storage/chroma/chroma.go:1-3
// Package chroma cung cấp in-memory vector store cho semantic search.
// MVP dùng cosine similarity trên in-memory store.
// Sau này có thể thay bằng Chroma embedded hoặc pgvector.
```
Câu cuối ("sau này có thể thay bằng Chroma embedded hoặc pgvector") tự nói rõ: **hiện tại CHƯA phải ChromaDB** — nó là placeholder đặt tên trước cho một phase sau.

Toàn bộ import của file chỉ có 2 package thuần Go, **không có `net/http`, không có bất kỳ SDK/client Chroma nào**:
```go
// services/agent-go/internal/storage/chroma/chroma.go:6-9
import (
    "math"
    "sort"
)
```

Cấu trúc dữ liệu là `map` trong RAM, không phải kết nối tới server nào:
```go
// services/agent-go/internal/storage/chroma/chroma.go:18-26
type VectorStore struct {
    entries map[string]*entry
}

type entry struct {
    embedding []float32
    metadata  map[string]any
}
```

Hàm `Search` chính là **linear scan O(n)** qua toàn bộ map, tự tính cosine similarity bằng tay, không có bất kỳ index kiểu HNSW/IVF nào:
```go
// services/agent-go/internal/storage/chroma/chroma.go:48-71
func (vs *VectorStore) Search(query []float32, topK int) []SearchResult {
    if topK <= 0 {
        topK = 5
    }

    type scored struct {
        id    string
        score float64
        meta  map[string]any
    }

    var results []scored
    for id, e := range vs.entries {
        sim := cosineSimilarity(query, e.embedding)
        results = append(results, scored{id: id, score: sim, meta: e.metadata})
    }

    sort.Slice(results, func(i, j int) bool {
        return results[i].score > results[j].score
    })

    if len(results) > topK {
        results = results[:topK]
    }
    ...
}
```
`cosineSimilarity` tính dot product/norm hoàn toàn bằng tay (không dùng thư viện tuyến tính nào):
```go
// services/agent-go/internal/storage/chroma/chroma.go:91-105
func cosineSimilarity(a, b []float32) float64 {
    if len(a) != len(b) || len(a) == 0 {
        return 0
    }
    var dot, normA, normB float64
    for i := range a {
        dot += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }
    if normA == 0 || normB == 0 {
        return 0
    }
    return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

`chroma_test.go` cũng xác nhận: test chỉ gọi trực tiếp `vs.Add(...)`/`vs.Search(...)` trong process, không có `httptest.Server`, không có mock HTTP nào — khác hẳn cách `voyage_test.go` test client HTTP thật (mục 9.2).

Bản thân `docs/ARCHITECTURE_VI.md` (mục 4.9) cũng đã cảnh báo sẵn về điều này:
> ⚠️ Tên package là `chroma` nhưng KHÔNG liên quan đến ChromaDB. Đây là vector store tự viết, in-memory.

**Hệ quả cụ thể nếu dùng package này trong production:**
1. **Không persist qua restart** — `entries` chỉ là `map[string]*entry` sống trong heap của process; deploy lại / crash / restart service là **mất toàn bộ vector đã Add**. Không có file, không ghi disk, không kết nối DB nào.
2. **Không có ANN index thật (HNSW/IVF/…)** — `Search` là **linear scan O(n × d)** (n = số entries, d = chiều embedding) qua **toàn bộ** map mỗi lần gọi, không có cấu trúc index nào để cắt giảm số phép so sánh. Với vài trăm/vài nghìn vector thì chấp nhận được, nhưng **không scale** — đây chính xác là lớp vấn đề mà ChromaDB thật (dùng HNSW qua `hnswlib`) được thiết kế để giải quyết, và package này không có.
3. **Tên gọi "chroma" gây hiểu lầm** — một dev đọc lướt `import ".../storage/chroma"` hoặc thấy comment "ChromaDB" trong tài liệu cũ rất dễ tưởng nhầm hệ thống đang dùng ChromaDB server thật (persistent, có HNSW, có collection API `/api/v1/collections`) — trong khi thực chất chỉ là ~90 dòng Go tự viết, không network call.

**Điểm bổ sung quan trọng**: giống `storage/sqlite`, package `storage/chroma` **cũng không được import ở đâu khác ngoài file test của chính nó**. Không có `main.go` hay tool nào gọi `chroma.NewVectorStore()`. RAG search thật của Jarvis (mục 9.3) **hoàn toàn không đi qua package này** — nó dùng MongoDB Atlas `$vectorSearch` (server-side, có HNSW/ANN thật do MongoDB Atlas vận hành) làm vector store chính thức. Vậy `storage/chroma` hiện là **code MVP/scaffold chưa (hoặc không còn) được wire vào runtime**, tồn tại song song với pipeline RAG thật nhưng không phải là nó.

**Tóm tắt câu hỏi phỏng vấn kiểu "tên gọi vs implementation":**

| Được gợi ý bởi tên | Thực tế trong code |
|---|---|
| `storage/chroma` → tưởng là ChromaDB client | In-memory map + cosine similarity tự viết, 0 network call, không persist |
| `storage/sqlite` → tưởng có thể là vector store cục bộ | Chỉ là FTS5 keyword search cho chat history/memory, không có vector nào |
| RAG "rerank" → tưởng có cross-encoder | Keyword-overlap heuristic (default) hoặc LLM-as-reranker qua prompting (opt-in) — không model rerank chuyên dụng |
| RAG "hybrid search" → nghi ngờ chỉ là dense đơn giản | **Thật**: dense (`$vectorSearch`) + sparse (`$text`) merge bằng weighted RRF |
| RAG "HyDE" → nghi ngờ chỉ là tên gọi | **Thật**: có gọi LLM sinh câu trả lời giả định trước khi embed (opt-in, default off) |
| "PDR" → không rõ có tồn tại | **Thật, có tồn tại**: Parent Document Retrieval — mở rộng chunk liền kề, default on |
## 11. Middleware: Tenant Isolation

### 11.1 `tenant.go` — Tenant ID lấy từ đâu THẬT

File `services/agent-go/internal/middleware/tenant.go` (toàn bộ file):

```go
// services/agent-go/internal/middleware/tenant.go:9-26
// contextKey is an unexported type used for context keys to avoid collisions.
type contextKey string

// TenantIDKey is the context key for the tenant ID value.
const TenantIDKey contextKey = "tenant_id"

// TenantMiddleware extracts the X-Tenant-ID header from incoming requests,
// falls back to "default" if not present, and stores it in the request context.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default"
		}
		ctx := context.WithValue(r.Context(), TenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTenantID retrieves the tenant ID from the context.
// Returns "default" if no tenant ID was set.
func GetTenantID(ctx context.Context) string {
	if id, ok := ctx.Value(TenantIDKey).(string); ok {
		return id
	}
	return "default"
}
```

Trả lời thẳng câu hỏi "lấy từ đâu": **HTTP header `X-Tenant-ID`**, không phải JWT claim, không phải query param. Không có bất kỳ decode JWT nào trong middleware này — tenant hoàn toàn do **client tự khai báo qua header**, agent-go tin tưởng tuyệt đối vào header đó (không verify chữ ký, không kiểm tra header này khớp với user đã auth ở đâu). Đây đúng là mô hình: agent-go chạy sau một API gateway (Fastify `apps/api`) — gateway đó chịu trách nhiệm xác thực JWT rồi mới **tự set** header `X-Tenant-ID` khi forward request sang agent-go. Nếu ai gọi trực tiếp `POST /chat` vào agent-go (bỏ qua gateway) và tự đặt `X-Tenant-ID: <tenant khác>`, middleware này **chấp nhận vô điều kiện** — không có access-control nào ở tầng này để chặn giả mạo tenant. Đây là điểm quan trọng cần nêu khi phỏng vấn: tenant isolation ở agent-go là **isolation giữa dữ liệu của các tenant với nhau (multi-tenancy về data)**, không phải **authentication/authorization** — hai khái niệm khác nhau, agent-go chỉ làm cái đầu, còn cái sau nó tin tưởng giao cho gateway phía trước.

Không có header → fallback cứng thành chuỗi `"default"`. Không có branch nào raise lỗi 401/403 khi thiếu header.

### 11.2 Propagate qua context — key có chống collision không?

Có. `contextKey` là **type riêng** (`type contextKey string`), không dùng `string` trần làm key. Đây đúng chuẩn Go idiom (khuyến nghị chính thức của package `context`) để tránh 2 package khác nhau vô tình cùng dùng string `"tenant_id"` làm context key và đè lên nhau. Có test xác nhận:

```go
// services/agent-go/internal/middleware/middleware_test.go:107-113
// Key phải là kiểu riêng (contextKey), không phải string thuần — tránh đụng key.
func TestTenantIDKey_IsUnexportedType(t *testing.T) {
	ctx := context.WithValue(context.Background(), "tenant_id", "raw-string-key") //nolint:staticcheck // cố ý dùng string key
	if got := GetTenantID(ctx); got != "default" {
		t.Errorf("string key không được đụng contextKey: GetTenantID = %q, want default", got)
	}
}
```

`GetTenantID()` cũng an toàn kiểu (type assertion `.(string)` có kiểm tra `ok`, giá trị sai kiểu → rơi về `"default"` thay vì panic).

### 11.3 Enforcement thật — nơi nào ĐỌC LẠI tenant từ context để filter dữ liệu

Middleware **set** tenant vào context không có nghĩa gì cả nếu không có ai **đọc lại** nó để lọc dữ liệu. Grep `tenant` trên `internal/memory/`, `internal/rag/`, `internal/storage/`, `internal/middleware/` cho kết quả **rất lệch nhau giữa 3 khu vực**.

#### 11.3.1 `internal/memory/` — ENFORCE ĐẦY ĐỦ (cả in-memory Store và Mongo)

`Store` (in-memory) partition dữ liệu theo tenant NGAY TỪ CẤU TRÚC DỮ LIỆU — outer map key là tenantID:

```go
// services/agent-go/internal/memory/store.go:21-32
// Data is partitioned by tenantID (outer map key)...
// Callers MUST pass the tenantID resolved via middleware.GetTenantID(ctx) —
// Store itself has no notion of "no tenant"; an empty string is just another
data     map[string]map[string]storeEntry // tenantID -> key -> entry
```

Mọi method public (`Get`, `Set`, `Delete`, `Search`, `SemanticSearch`, `Len`, `All`) đều nhận `tenantID` làm tham số bắt buộc đầu tiên. Nơi gọi thật:

```go
// services/agent-go/internal/memory/recall.go:41,55,63,80
tenantID := middleware.GetTenantID(ctx)
...
if v, ok := store.Get(tenantID, k); ok { ... }
fullResults := store.Search(tenantID, query)
semResults, err := store.SemanticSearch(tenantID, query, 5)
```

```go
// services/agent-go/internal/memory/extract.go:45,67
tenantID := middleware.GetTenantID(ctx)
...
store.Set(tenantID, rule.key, value)
```

Với Mongo (persist bền, dùng bởi autonomous learner), filter Mongo query cũng bắt buộc có `tenantId`:

```go
// services/agent-go/internal/memory/learner.go:130,140-143
tenantID := middleware.GetTenantID(ctx)
...
// Filter MUST include tenantId — otherwise two tenants learning a fact
filter := bson.M{"key": fact.Key, "tenantId": tenantID}
```

```go
// services/agent-go/internal/memory/learner.go:198-202
// Filter MUST include tenantId — otherwise two tenants learning a
// fact with the same key/documentId would upsert the very same document,
// leaking content across tenants and letting rag.search read one tenant's
// learned knowledge as another's.
filter := bson.M{"documentId": docID, "tenantId": tenantID}
```

Có bộ test riêng khẳng định KHÔNG rò rỉ chéo tenant (`memory/mongo_integration_test.go`, `memory/store_test.go:312-365`, `memory/reflection_test.go:549-598` — comment ghi rõ mức độ nghiêm trọng: `"rò rỉ chéo tenant (P0)"`).

#### 11.3.2 `internal/tools/rag.go` — ENFORCE ĐẦY ĐỦ (KHÔNG phải `internal/rag/`)

**Lưu ý quan trọng:** package `internal/rag/` (file `voyage.go`) grep tenant ra **0 kết quả** — vì package đó CHỈ chứa Voyage AI embedding client, không hề chứa logic filter Mongo. Pipeline `$vectorSearch` + `$match tenantId` nằm ở package **`internal/tools`**, file `rag.go` (tool `rag.search`/`rag.read`/`rag.list`), không phải `internal/rag/`.

```go
// services/agent-go/internal/tools/rag.go:466,480-483 (vectorSearch)
func (t *ragSearchTool) vectorSearch(ctx context.Context, queryVector []float64, tenantID string, limit int) ([]ragSearchResult, error) {
	...
	if tenantID != "" && tenantID != "default" {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: bson.D{{Key: "tenantId", Value: tenantID}}},
		})
	}
```

```go
// services/agent-go/internal/tools/rag.go:295-299 (buildRAGReadFilter)
// Without the tenantId clause, any tenant that knows/guesses another tenant's
// documentId or source can read the FULL content of that document —
// this is what previously made rag.read leak data across tenants.
func buildRAGReadFilter(documentID, source, tenantID string) bson.D {
```

```go
// services/agent-go/internal/tools/rag.go:167-174 (buildRAGListPipeline)
func buildRAGListPipeline(tenantID string) []bson.D {
	...
	if tenantID != "" && tenantID != "default" {
		match = append(match, bson.E{Key: "tenantId", Value: tenantID})
	}
```

Comment `"this is what previously made rag.read leak data across tenants"` cho thấy đây là **fix của một lỗ hổng đã từng thật xảy ra** — rất đáng nhắc trong tài liệu học vì nó là ví dụ thật về multi-tenancy bug và cách vá (thêm test `rag_read_test.go`, `rag_list_test.go`, `rag_advanced_test.go` khẳng định 2 tenant khác nhau → filter khác nhau, không đọc chéo được). `textSearch`, `hybridSearch`, và Parent Document Retrieval window đều truyền `tenantID` xuyên suốt theo cùng quy tắc.

#### 11.3.3 `internal/storage/sqlite/` và `internal/storage/chroma/` — KHÔNG có tenant scoping (GAP thật)

Grep `tenant` trên cả hai package này trả về **0 kết quả**. `Conversation`/`Message`/`Memory` struct trong SQLite KHÔNG có field `TenantID`. `chroma.VectorStore.entries` là `map[string]*entry` KHÔNG namespace theo tenant.

**Đây là gap multi-tenancy thật** — nhưng cần nêu đúng phạm vi: hai package này KHÔNG nằm trên đường dẫn chính của production multi-tenant (README mô tả SQLite/Chroma là để chạy offline/local dev, Mongo mới là nguồn thật cho production). Nếu ai wire SQLite hoặc in-memory Chroma cho một deployment nhiều tenant thật, dữ liệu các tenant sẽ **trộn chung**. Kết luận đúng đắn: middleware tenant chỉ được enforce nhất quán ở 2 nơi (`internal/memory` + Mongo, và `internal/tools/rag.go` Mongo Atlas) — tầng storage offline (SQLite, Chroma in-memory) chưa từng được thiết kế multi-tenant.

### 11.4 `cors.go` — cấu hình CORS thật

Toàn bộ file:

```go
// services/agent-go/internal/middleware/cors.go (toàn bộ)
// CORSMiddleware adds permissive CORS headers for local development.
// In production, the API gateway (Fastify) handles CORS; this is for
// direct frontend calls during dev (e.g. GET /suggestions).
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

| Thuộc tính | Giá trị thật |
|---|---|
| Allow-Origin | `*` (wildcard — mọi origin) |
| Allow-Methods | `GET, POST, OPTIONS` |
| Allow-Headers | `Content-Type, Authorization, X-Tenant-ID` |
| Allow-Credentials | **KHÔNG set** |
| Preflight (`OPTIONS`) | Trả `204 No Content` ngay, KHÔNG gọi `next.ServeHTTP` (short-circuit) |

Vì `Access-Control-Allow-Credentials` không được set, wildcard `*` không mở lỗ hổng credential-leak kiểu cookie — nhưng header `Authorization` (bearer token) vẫn được cho phép gửi cross-origin không giới hạn: **bất kỳ website nào** cũng có thể gọi trực tiếp `GET /suggestions` hoặc `POST /chat` kèm token nếu có được token đó. Comment tự thừa nhận đây là **cấu hình chỉ dành cho dev**, production dựa vào Fastify gateway đứng trước — agent-go không tự bảo vệ gì ở lớp này khi expose trực tiếp.

---

## 12. Metrics & Observability

### 12.1 `metrics.go` — Prometheus thật hay chỉ struct tự đếm?

**KHÔNG có Prometheus.** Import block của `metrics.go` chỉ có 3 package thư viện chuẩn:

```go
// services/agent-go/internal/metrics/metrics.go:5-9
import (
	"sync"
	"sync/atomic"
	"time"
)
```

`go.mod` của service **không chứa** `prometheus` ở bất kỳ dòng nào — chỉ có `go.opentelemetry.io/otel` và `otel/trace` liên quan observability, không có Prometheus client hay exporter nào.

`Metrics` là struct tự viết, `atomic.Int64` cho counter, `sync.RWMutex` cho histogram latency (map):

```go
// services/agent-go/internal/metrics/metrics.go:29-56
type Metrics struct {
	requests     atomic.Int64
	toolCalls    atomic.Int64
	errors       atomic.Int64
	inputTokens  atomic.Int64
	outputTokens atomic.Int64
	latencySumUs atomic.Int64

	mu          sync.RWMutex
	latencyDist map[string]int64 // "<100ms", "100-500ms", etc.
}

func New() *Metrics {
	return &Metrics{
		latencyDist: map[string]int64{
			"<100ms": 0, "100-500ms": 0, "500ms-1s": 0, "1-3s": 0, "3-10s": 0, ">10s": 0,
		},
	}
}
```

Public API: `RecordRequest(latency, tokensIn, tokensOut)`, `RecordToolCall(s)`, `RecordError(s)`, `Snapshot() Snapshot`, `Reset()`. Không có "metric name" theo nghĩa Prometheus — chỉ field JSON: `requests`, `tool_calls`, `errors`, `input_tokens`, `output_tokens`, `latency_sum_us`, `taken_at`.

**Phát hiện quan trọng nhất — package này KHÔNG được wire vào runtime thật.** Grep `metrics.New(` trên toàn bộ service (loại `_test.go`) trả về **0 kết quả**. Không file nào trong `cmd/server/main.go`, `cmd/jarvis/main.go` import `internal/metrics`. Không có endpoint `GET /metrics` nào được đăng ký (router chỉ có `/healthz`, `/readyz`, `/chat`, `/suggestions`). Package được viết đầy đủ, test coverage tốt (bao gồm test concurrency 100 goroutine), nhưng hoàn toàn là code "chết" ở production — không instance nào của `Metrics` từng được tạo ngoài test.

### 12.2 `observability.go` — logging thật, tracing chỉ là placeholder

Comment đầu file tự giải thích rõ trạng thái:

```go
// services/agent-go/internal/observability/observability.go:1-24
// Package observability cung cấp logging có cấu trúc (slog) và tracing
// (OpenTelemetry) cho agent-go.
//
// Phase này exporter còn tối giản: SetupTracer chỉ đăng ký một TracerProvider
// "rỗng" (no-op, không phát telemetry) làm global provider. ...
```

**Logging: THẬT** — `SetupLogger()` dùng `slog.NewJSONHandler` ghi JSON ra `os.Stdout`:

```go
// services/agent-go/internal/observability/observability.go:28-35
func SetupLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
```

**Tracing: CHỈ LÀ PLACEHOLDER (noop)** — import `go.opentelemetry.io/otel` (API thật, có trong `go.mod`), nhưng KHÔNG có SDK/exporter đứng sau — chỉ đăng ký `noop.NewTracerProvider()`:

```go
// services/agent-go/internal/observability/observability.go:42-56
func SetupTracer(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	tp := noop.NewTracerProvider()
	otel.SetTracerProvider(tp)
	slog.Default().Info("tracer initialized (noop provider, exporter phase sau)", slog.String("service.name", serviceName))
	shutdown = func(context.Context) error { return nil }
	return shutdown, nil
}
```

Kết luận phân loại chính xác: **"OpenTelemetry API đã cắm vào code, nhưng KHÔNG có OpenTelemetry tracing thật đang chạy"** — build được, gọi `Tracer("name")` không panic, nhưng zero telemetry thoát ra khỏi process. Không có `sdktrace.TracerProvider`, không có `otlptracehttp`/`stdouttrace` exporter nào trong `go.mod`.

### 12.3 Phát hiện chung — cả 2 package đều KHÔNG được gọi ở runtime thật

`SetupLogger()` và `SetupTracer()` **không được gọi ở bất kỳ đâu ngoài file test của chính package `observability`**. Cả `cmd/server/main.go` và `cmd/jarvis/main.go` đều **không import** `internal/observability`.

Hệ quả: khi chạy `go run ./cmd/server`, logger đang hoạt động là **`slog` default handler của Go** (text format, ghi `os.Stderr`) — KHÔNG phải JSON handler ghi `os.Stdout` như `SetupLogger()` định nghĩa. Toàn bộ `slog.Info/Error/Warn` rải khắp `main.go` chạy qua logger mặc định này, không phải logger "thật" mà package `observability` build ra. Đây là nhận định đáng đưa vào phần "gap/nợ kỹ thuật": **`internal/metrics` và `internal/observability` là hai package được viết tử tế, có unit test riêng, nhưng bị "cô lập" — chưa từng được wire vào `cmd/server` hay `cmd/jarvis`.**

---

## 13. Config: Toàn bộ Environment Variables

### 13.1 Cách đọc config — pattern nào?

`internal/config/config.go` dùng pattern **`os.Getenv` thuần + helper tự viết** (`envOr`, `intEnvOr`, `splitCSV`) — KHÔNG dùng `viper`, KHÔNG dùng `envconfig`/`caarlos0/env`. Chỉ 1 thư viện ngoài: `github.com/joho/godotenv` để tự nạp file `.env`:

```go
// services/agent-go/internal/config/config.go:153-156
func Load() (Config, error) {
	_ = godotenv.Load() // không lỗi nếu file không tồn tại
	...
```

```go
// services/agent-go/internal/config/config.go:220-225
func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
```

`intEnvOr` fail-safe: giá trị rác/âm → log warning + rơi về default, không sập server. `Load()` validate đúng 1 điều bắt buộc: provider LLM đã chọn phải có key tương ứng (hoặc `ollama`/local).

`LLM_PROVIDER` hợp lệ có **5 giá trị**: `gemini`, `anthropic`, `deepseek`, `ollama`, `auto`.

### 13.2 Bảng đầy đủ mọi env var đọc trong `config.go`

| Env var | Default value thật trong code | Kiểu | Mô tả ngắn |
|---|---|---|---|
| `PORT` | `"3002"` | string | Cổng HTTP server |
| `LLM_PROVIDER` | `"gemini"` | string (enum) | `gemini`\|`anthropic`\|`deepseek`\|`ollama`\|`auto` |
| `GEMINI_API_KEY` | fallback đọc `GOOGLE_API_KEY` | string | Key Gemini |
| `GOOGLE_API_KEY` | *(không default riêng)* | string | Key thay thế khi `GEMINI_API_KEY` rỗng |
| `GEMINI_MODEL` | `"gemini-3.1-flash-lite"` | string | Model Gemini chính |
| `GEMINI_SECONDARY_MODEL` | `"gemini-3.5-flash-lite"` | string | Model fallback khi primary rate-limit |
| `GEMINI_FALLBACK_MODELS` | `"gemini-3.5-flash-lite,gemini-3.7-flash,gemini-3.6-flash,gemini-3.5-flash,gemini-3-flash,gemini-2.5-flash-lite,gemini-2.5-flash"` | `[]string` (CSV) | Model Gemini dự phòng theo thứ tự |
| `GOOGLE_THINKING_LEVEL` | `"OFF"` | string (enum) | `OFF`\|`LOW`\|`MEDIUM`\|`HIGH` |
| `ANTHROPIC_API_KEY` | `""` | string | Key Anthropic/Claude |
| `CLAUDE_MODEL` | `"claude-haiku-4-5-20251001"` | string | Model Claude |
| `OLLAMA_URL` | `"http://localhost:11434"` | string | URL Ollama (**tên biến `OLLAMA_URL`, KHÔNG phải `OLLAMA_HOST`**) |
| `OLLAMA_MODEL` | `"llama3.1:8b"` | string | Model Ollama local |
| `DEEPSEEK_API_KEY` | `""` | string | Key DeepSeek |
| `DEEPSEEK_FLASH_MODEL` | `"deepseek-v4-flash"` | string | Model rẻ/nhanh (fastModel) |
| `DEEPSEEK_PRO_MODEL` | `"deepseek-v4-pro"` | string | Model reasoning mạnh |
| `JARVIS_DB_PATH` | `"jarvis.db"` | string | Đường dẫn file SQLite |
| `JARVIS_SKILLS_DIR` | `"./skills"` | string | Thư mục SKILL.md |
| `HOME` | *(giá trị hệ thống)* | string | Build `AllowedPaths = [".", $HOME]` |
| `MONGODB_URI` | `""` (rỗng = không dùng Mongo) | string | Connection string MongoDB |
| `MONGODB_DB` | `"ai_agent_tut"` | string | Tên database Mongo |
| `VOYAGE_API_KEY` | `""` | string | Key Voyage AI |
| `EMBED_MODEL` | `"nomic-embed-text"` | string | Model embedding (Ollama) |
| `ENABLE_HYBRID_SEARCH` | `"true"` | bool | Bật hybrid search cho `rag.search` |
| `ENABLE_RERANK` | `"true"` | bool | Bật rerank keyword-overlap (miễn phí) |
| `ENABLE_PARENT_RETRIEVAL` | `"true"` | bool | Parent Document Retrieval |
| `ENABLE_LLM_RERANK` | `"false"` | bool | Rerank bằng 1 lời gọi LLM |
| `ENABLE_HYDE` | `"false"` | bool | Hypothetical Document Embeddings |
| `MAX_OUTPUT_TOKENS` | `8192` | int | Trần output token/lần gọi LLM chính |
| `MAX_CONTEXT_TOKENS` | `100000` | int | Ngân sách token context trước khi trim |
| `MAX_TOTAL_TOOL_OUTPUT` | `60000` | int | Ngân sách ký tự tool-output cộng dồn/lượt |
| `ENABLE_DYNAMIC_THINKING` | `"false"` | bool | Tự điều chỉnh thinking level |
| `ENABLE_PLANNING` | `"false"` | bool | Bật node plan/reflect |
| `ALLOW_DESTRUCTIVE_TOOLS` | `"false"` | bool | Cho `shell.exec` chạy không cần HITL |
| `OWNER_TENANT_IDS` | `""` → `nil` | `[]string` (CSV) | Tenant được coi là chủ hệ thống |
| `ENABLE_LEARNER` | `"false"` | bool | Bật autonomous learner |

**3 field KHÔNG đọc từ env dù comment mô tả như thể có default cấu hình được:**

```go
// services/agent-go/internal/config/config.go:184,187,189
MaxSteps:              12,        // hardcode — KHÔNG có env var
MaxToolOutput:         24000,     // hardcode — dù comment field ghi "default: 24000"
ShellTimeout:          30,        // hardcode — dù comment field ghi "default: 30"
```

Muốn đổi 3 giá trị này phải sửa code, không thể set qua môi trường — cùng loại "bug lịch sử" mà comment trong `config.go` mô tả đã từng xảy ra với `MaxTokens`/`MaxContextTokens` trước khi được fix (gọi là "CONFIG CHẾT" trong comment tiếng Việt của code).

### 13.3 Đối chiếu với `docs/DEPLOY.md`

File thật ở **root repo** `docs/DEPLOY.md`, bảng "Environment Variables Reference".

#### (A) Sai lệch — DEPLOY.md ghi giá trị/tên KHÁC với code thật

| Biến trong DEPLOY.md | DEPLOY.md ghi | Code thật | Vấn đề |
|---|---|---|---|
| `OLLAMA_HOST` | *(liệt kê biến này)* | Code đọc `OLLAMA_URL`, không đọc `OLLAMA_HOST` | Sai tên biến — set `OLLAMA_HOST` không có tác dụng |
| `OLLAMA_MODEL` default | `gemma3:4b` | `"llama3.1:8b"` | Default sai |
| `GOOGLE_THINKING_LEVEL` default | `LOW` | `"OFF"` | Default sai |
| `LLM_PROVIDER` mô tả | "gemini, anthropic, or ollama" | Switch thật có thêm `deepseek` và `auto` | Thiếu 2 giá trị hợp lệ |

#### (B) Có trong `config.go` nhưng KHÔNG document trong DEPLOY.md (22 biến)

`GOOGLE_API_KEY`, `GEMINI_SECONDARY_MODEL`, `GEMINI_FALLBACK_MODELS`, `DEEPSEEK_API_KEY`, `DEEPSEEK_FLASH_MODEL`, `DEEPSEEK_PRO_MODEL`, `JARVIS_DB_PATH`, `JARVIS_SKILLS_DIR`, `EMBED_MODEL`, `ENABLE_HYBRID_SEARCH`, `ENABLE_RERANK`, `ENABLE_PARENT_RETRIEVAL`, `ENABLE_LLM_RERANK`, `ENABLE_HYDE`, `MAX_OUTPUT_TOKENS`, `MAX_CONTEXT_TOKENS`, `MAX_TOTAL_TOOL_OUTPUT`, `ENABLE_DYNAMIC_THINKING`, `ENABLE_PLANNING`, `ALLOW_DESTRUCTIVE_TOOLS`, `OWNER_TENANT_IDS`, `ENABLE_LEARNER`.

7 trong số này thực ra ĐÃ được document — nhưng ở `services/agent-go/README.md` (bảng "Biến env quan trọng"). Vậy `README.md` hiện là nguồn tài liệu env var **mới và chính xác hơn** `docs/DEPLOY.md` (viết ở giai đoạn chưa có DeepSeek/guardrails/learner/context-budget). Kết luận thực dụng: **ưu tiên đọc `config.go` + `README.md`, không tin tuyệt đối `docs/DEPLOY.md` cho danh sách env var.**

---

## 14. Eval Harness

### 14.1 `eval.go` làm gì cụ thể

Là một **bộ test-case-runner** — chạy 1 tập câu hỏi (`Input`) qua agent, so kết quả thật (`Actual`) với kỳ vọng (`Expected`) theo 1 trong 4 chế độ so khớp, rồi tổng hợp báo cáo pass/fail/error.

```go
// services/agent-go/internal/eval/eval.go:44-61
type EvalCase struct {
	Name     string    `json:"name"`
	Input    string    `json:"input"`
	Expected string    `json:"expected"`
	Mode     MatchMode `json:"mode"`
	Tags     []string  `json:"tags,omitempty"`
}

type EvalResult struct {
	Case     EvalCase      `json:"case"`
	Passed   bool          `json:"passed"`
	Actual   string        `json:"actual"`
	Reason   string        `json:"reason,omitempty"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}
```

Agent cần test được abstract qua interface tối giản:

```go
// services/agent-go/internal/eval/eval.go:74-77
type AgentRunner interface {
	Run(ctx context.Context, input string) (string, error)
}
```

### 14.2 4 `MatchMode`

```go
// services/agent-go/internal/eval/eval.go:14-22
const (
	MatchExact    MatchMode = iota
	MatchContains
	MatchRegex
	MatchSemantic // LLM judge
)
```

3 mode đầu so khớp string thuần, không gọi LLM. Mode 4 cần `Judge` (interface 1 method):

```go
// services/agent-go/internal/eval/eval.go:39-42
type Judge interface {
	Evaluate(ctx context.Context, expected, actual string) (bool, string, error)
}
```

Chưa Judge → fail có lý do rõ ràng, không panic:
```go
// services/agent-go/internal/eval/eval.go:234-237
case MatchSemantic:
	if h.judge == nil {
		return false, "no LLM judge configured for semantic evaluation"
	}
```

Trong toàn bộ codebase hiện tại, **chưa có implementation thật nào của `Judge`** — chỉ có interface định nghĩa.

### 14.3 `RunAll` vs `RunAllParallel` — điểm code/comment lệch nhau

`RunAll` chạy tuần tự (comment: để debug + tránh rate-limit). `RunAllParallel` chạy song song bằng goroutine + `WaitGroup`.

```go
// services/agent-go/internal/eval/eval.go:115-143 (rút gọn)
// Chạy tuần tự (không song song) để dễ debug và tránh rate-limit LLM.
func (h *EvalHarness) RunAll(ctx context.Context, cases []EvalCase) EvalReport {
	for i, c := range cases {
		if i > 0 {
			// Small delay between cases to be kind to LLM APIs
			select {
			case <-ctx.Done():
				break
			default:
			}
		}
		r := h.RunEval(ctx, c)
		...
```

Comment nói **"Small delay between cases to be kind to LLM APIs"**, nhưng code **không có `time.Sleep` nào cả** — `select{case <-ctx.Done(): break; default:}` không tạo độ trễ (rơi vào `default` ngay nếu ctx chưa hết hạn). Thêm nữa, `break` trong `select` chỉ thoát `select`, KHÔNG thoát vòng `for` — nên dù `ctx` hết hạn, loop vẫn tiếp tục gọi case tiếp theo. Đây là ví dụ "code thật khác comment": không thực sự rate-limit, không thực sự early-exit khi context hết hạn.

`RunAllParallel` không có giới hạn số goroutine đồng thời (không semaphore/`errgroup.SetLimit`) — phù hợp mục đích ghi trong comment: "dùng cho eval KHÔNG gọi LLM".

### 14.4 Có CLI entrypoint chạy được chưa?

**CHƯA.**
1. Không có `func main()` nào trong `internal/eval/` — package thư viện thuần.
2. Không có `cmd/*/main.go` nào import `internal/eval` (`cmd/server/main.go`, `cmd/jarvis/main.go` đều không import).
3. README service tự xác nhận: `eval/  # EvalHarness (exact/contains/regex/LLM-judge) — thư viện, chưa wire vào CLI`.

→ Package chỉ được exercise qua `go test ./internal/eval/...` với `fakeRunner` giả lập `AgentRunner` — chưa có cách chạy như CLI độc lập để đánh giá agent thật (Gemini/Claude/DeepSeek) qua dataset câu hỏi thật. README còn mô tả một thư mục `eval/` top-level riêng cho dataset — thư mục này **không tồn tại** trong repo hiện tại, càng khẳng định: eval harness hiện tại chỉ là 1 package library có unit test, chưa có dataset thật, chưa có CLI, chưa wire vào agent thật ngoài `go test`.
## 15. Bảo mật tool nguy hiểm (privileged.go, shell.go, file_write.go)

### 15.1 privileged.go

`privileged.go` **không thực thi gì cả** — nó là một file **policy/classification thuần** (pure functions, không side-effect), định nghĩa tập tool được coi là "chỉ chủ hệ thống được dùng":

```go
// services/agent-go/internal/tools/privileged.go:21-30
var privilegedTools = map[string]bool{
	"file.read":   true,
	"file.search": true,
	"file.write":  true,
	"shell.exec":  true,
	"git":         true,
}

func IsPrivilegedTool(name string) bool { return privilegedTools[name] }
```

Comment ngay phía trên (privileged.go:5-18) giải thích rất rõ lý do: các tool này tác động lên **máy chạy agent** (server), không scope theo tenant — `file.read`/`file.search` có `AllowedPaths` mặc định là `[".", $HOME]` (xác nhận tại `config.go:174`: `AllowedPaths: []string{".", os.Getenv("HOME")}`), nghĩa là bất kỳ ai gọi được `file.read` đều có thể đọc `.env` chứa API key, SSH key, hoặc dữ liệu tenant khác trên server.

Gate quyền không nằm ở privileged.go — nó chỉ cung cấp 3 hàm thuần, việc **enforce** nằm ở 2 lớp riêng trong `internal/agent/`:

**Lớp 1 — ẩn khỏi tool list gửi cho LLM**, `node_model.go:174-186`:
```go
if !tools.IsOwnerTenant(middleware.GetTenantID(ctx), eng.getOwnerTenants()) {
	before := len(toolDefs)
	toolDefs = tools.StripPrivilegedTools(toolDefs)
	...
}
```

**Lớp 2 — chặn lại lúc thực thi** (defense-in-depth, phòng trường hợp model tự "bịa" tên tool hoặc step > 0 khiến `FilterToolDefs` trả lại toàn bộ registry), `node_tools.go:56-69`:
```go
isOwner := tools.IsOwnerTenant(middleware.GetTenantID(ctx), eng.getOwnerTenants())
for _, tc := range last.ToolCalls {
	if !isOwner && tools.IsPrivilegedTool(tc.Name) {
		slog.Warn("tools: chặn tool đặc quyền với tenant không phải chủ", "tool", tc.Name)
		emit(ToolEndEvent(tc.Name, false, privilegedDeniedMessage(tc.Name)))
		s.AppendObservation(Observation{CallID: tc.ID, Name: tc.Name, Error: privilegedDeniedMessage(tc.Name)})
		continue
	}
	...
}
```

`IsOwnerTenant` (privileged.go:52-62) áp dụng nguyên tắc **fail-closed** có chủ đích:
```go
func IsOwnerTenant(tenantID string, owners []string) bool {
	if len(owners) == 0 {
		return tenantID == "" || tenantID == defaultTenantID // "default"
	}
	for _, o := range owners {
		if o != "" && o == tenantID {
			return true
		}
	}
	return false
}
```
Nếu `OWNER_TENANT_IDS` chưa cấu hình (`owners` rỗng) thì CHỈ tenant `"default"` (tức request không có header `X-Tenant-ID`, tức chạy local không qua auth) mới được coi là chủ — mọi tenant thật (đã đăng nhập, có tenant ID thật) đều KHÔNG có đặc quyền dù chưa config gì. Đây là lựa chọn an toàn đúng hướng: hậu quả của quên cấu hình chỉ là "chủ máy phải thêm 1 dòng .env", không phải "mở quyền chạy shell cho người lạ".

**Kết luận 15.1:** cơ chế gate ở đây là **role/tenant-based allowlist**, KHÔNG phải allowlist theo command hay theo nội dung tham số, và KHÔNG có bước "require confirmation" (không có HITL) cho nhóm privileged — chỉ có 2 trạng thái: được dùng toàn quyền (owner) hoặc bị chặn hoàn toàn, ẩn khỏi cả tool list (non-owner). Việc yêu cầu xác nhận (HITL) là một cơ chế **khác**, riêng cho `Kind=KindDestructive` (xem 15.2).

---

### 15.2 shell.go

`shell.go` **không có sandbox nào cả** — không container, không chroot, không seccomp, không namespace. Nó gọi trực tiếp `os/exec.CommandContext` trên host nơi tiến trình Go server đang chạy:

```go
// services/agent-go/internal/tools/shell.go:85
cmd := exec.CommandContext(ctx, args.Command, args.Args...)
```

Có một cơ chế allowlist command, nhưng **là optional và bị vô hiệu hoá hoàn toàn trong thực tế**:

```go
// services/agent-go/internal/tools/shell.go:27-43
func NewShellTool(allowedCommands []string) Tool {
	return NewShellToolWithTimeout(allowedCommands, defaultShellTimeout)
}

func NewShellToolWithTimeout(allowedCommands []string, timeout time.Duration) Tool {
	ac := make(map[string]bool, len(allowedCommands))
	for _, cmd := range allowedCommands {
		ac[cmd] = true
	}
	...
	return &shellTool{allowedCommands: ac, timeout: timeout}
}
```

```go
// services/agent-go/internal/tools/shell.go:79-81 — Execute()
if len(t.allowedCommands) > 0 && !t.allowedCommands[args.Command] {
	return Result{}, fmt.Errorf("shell.exec: command %q is not allowed", args.Command)
}
```

Nghĩa là: nếu `allowedCommands` là slice **rỗng hoặc nil**, `len(t.allowedCommands) == 0` → điều kiện `if` bị bỏ qua hoàn toàn → **mọi command được chạy không giới hạn**. Đã grep toàn bộ repo các call site thật (không phải test):

```
cmd/server/main.go:370: NewShellToolWithTimeout(nil, time.Duration(cfg.ShellTimeout)*time.Second)   // registry "code"
cmd/server/main.go:391: NewShellToolWithTimeout(nil, time.Duration(cfg.ShellTimeout)*time.Second)   // registry "general"
cmd/promptsize/main.go:71: NewShellToolWithTimeout(nil, 30*time.Second)
```

**Cả 3 call site production đều truyền `nil` cho `allowedCommands`.** Không có bất kỳ chỗ nào trong toàn bộ codebase gán một danh sách allowlist thật — tham số này tồn tại trong signature nhưng **chưa từng được dùng đúng ý nghĩa của nó**. Kết quả: `shell.exec` khi chạy trong production **cho phép chạy bất kỳ binary nào có trong `$PATH` của tiến trình server**, không có blocklist cho `rm -rf`, `sudo`, `curl | sh`, `kill`, hay bất kỳ pattern nguy hiểm nào — không có kiểm tra nội dung command/args ngoài việc parse JSON args.

Giới hạn resource/timeout: có timeout (cooperative, qua `ctx`, xem `TimeoutTool.Timeout()` — mặc định `defaultShellTimeout = 30s`, `shell.go:22`), có cắt output ở 8.000 ký tự (`shellMaxOutput`, `shell.go:21`). Nhưng **không có giới hạn CPU, memory, số process con, hay network** — một command như `:(){ :|:& };:` (fork bomb) hay chiếm hết RAM vẫn chạy tự do trong 30s trước khi bị cancel (và `exec.CommandContext` cancel bằng SIGKILL cho tiến trình cha, nhưng process con đã fork ra ngoài group thì có thể sống sót — Go không tự động kill process tree).

Mitigation THẬT SỰ tồn tại cho `shell.exec` nằm ở 2 lớp phía ngoài file này, không phải trong `shell.go`:

1. **`Kind() Kind { return KindDestructive }`** (shell.go:64) — là tool DUY NHẤT trong toàn bộ 25 tool được gán `KindDestructive` (đã grep xác nhận). Qua `guardrails.CheckTool()` (`guard.go`), việc gọi `shell.exec` trả về `*NeedConfirmationError`, khiến `node_tools.go` (dòng 84-102) đẩy call vào `destructiveCalls`, emit `InterruptEvent("confirm_destructive", ...)` và **dừng loop, chờ user xác nhận** — TRỪ KHI `cfg.AllowDestructiveTools == true` (mặc định `false`, `config.go:192`: `envOr("ALLOW_DESTRUCTIVE_TOOLS", "false") == "true"`), lúc đó tool chạy tự do không cần confirm.
2. **`IsPrivilegedTool("shell.exec") == true`** — nên còn bị chặn thêm bởi lớp owner-tenant ở mục 15.1: tenant không phải owner không gọi được `shell.exec` dù có bật `AllowDestructiveTools`.

### 15.3 file_write.go

`file_write.go` CÓ giới hạn path thật, dựa trên whitelist `allowedPaths` truyền vào constructor, kiểm tra bằng `filepath.Abs` + so khớp prefix:

```go
// services/agent-go/internal/tools/file_write.go:101-119
func (t *fileWriteTool) isAllowed(path string) bool {
	if len(t.allowedPaths) == 0 {
		return true // no restrictions configured
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, allowed := range t.allowedPaths {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if strings.HasPrefix(abs, absAllowed+string(filepath.Separator)) || abs == absAllowed {
			return true
		}
	}
	return false
}
```

Việc kiểm tra tiền tố có kèm `string(filepath.Separator)` (không phải `HasPrefix(abs, absAllowed)` trần) — đây là chi tiết đúng đắn quan trọng: nếu thiếu dấu separator, `allowedPaths=["/data/sandbox"]` sẽ vô tình cho phép luôn `/data/sandboxEVIL/...` (prefix string khớp nhưng không phải cùng thư mục). Code ở đây tránh được lỗi phổ biến này.

Về path traversal (`../`): code **không có kiểm tra tường minh** kiểu `strings.Contains(path, "..")`, nhưng đã verify bằng thực nghiệm (mô phỏng đúng logic `filepath.Dir/Join/Abs` của file này) rằng path traversal **vẫn bị chặn gián tiếp**, vì `filepath.Join`/`filepath.Abs` tự động `Clean()` các segment `..` TRƯỚC khi so khớp prefix:

```
raw=/data/sandbox/file.txt                 → abs=/data/sandbox/tenant1/file.txt        → allowed=true
raw=/data/sandbox/../../etc/passwd         → abs=/etc/tenant1/passwd                   → allowed=false
raw=/data/sandbox/../sibling/file.txt      → abs=/data/sibling/tenant1/file.txt        → allowed=false
raw=../../../etc/passwd                    → abs=<cwd>/etc/tenant1/passwd              → allowed=false
```
Vì đường dẫn cuối cùng sau khi "escape" bằng `..` không còn có tiền tố là `allowedPaths[i]` nữa, `isAllowed()` trả `false` và request bị từ chối với lỗi `"access denied: ... is outside allowed paths"` (file_write.go:74). Đây là mitigation **thật, đã verify bằng chạy code**, không phải suy đoán — dù nó hoạt động "tình cờ đúng" nhờ hành vi chuẩn của `path/filepath` hơn là một dòng validate `..` tường minh có chủ đích.

Điểm đáng chú ý thứ hai: **tenant isolation được chèn trước bước kiểm tra path**, bằng cách chèn `tenantID` làm subdirectory:
```go
// services/agent-go/internal/tools/file_write.go:65-67
tenantID := middleware.GetTenantID(ctx)
args.Path = filepath.Join(filepath.Dir(args.Path), tenantID, filepath.Base(args.Path))
```
Nghĩa là nếu tenant A và tenant B cùng gọi `file.write` với `path="report.txt"`, chúng thực sự ghi vào 2 vị trí khác nhau (`.../<tenantA>/report.txt` vs `.../<tenantB>/report.txt`) — chống ghi đè chéo giữa tenant. Nhưng cần lưu ý: `file.read`/`file.search` (`files.go`) **KHÔNG có cơ chế chèn tenant này** — chúng đọc đúng path client truyền, không tự thêm tenant subdir. Điều này ít nghiêm trọng hơn tưởng vì cả 3 tool `file.*` đều nằm trong `privilegedTools` (mục 15.1) — chỉ owner-tenant mới gọi được, nên rủi ro "tenant B đọc được file tenant A viết" chỉ xảy ra nếu OWNER_TENANT_IDS bị cấu hình sai (nhiều tenant cùng là owner).

Giới hạn khác: content tối đa 100KB (`file_write.go:61-63`), timeout `context.WithTimeout(ctx, 10*time.Second)` (dòng 69), tạo parent dir tự động qua `os.MkdirAll(dir, 0755)` (dòng 79) — permission `0644` cho file, `0755` cho dir, không có kiểm soát đặc biệt nào về symlink (không dùng `os.OpenFile` với `O_NOFOLLOW`), nên về lý thuyết nếu `allowedPaths` chứa một symlink trỏ ra ngoài, `filepath.Abs` sẽ KHÔNG resolve symlink (khác `filepath.EvalSymlinks`) — đây là một góc chưa được xử lý, nhưng là edge case thấp vì cần kẻ tấn công đã có quyền tạo symlink trong `allowedPaths` từ trước.

`Kind() Kind { return KindWrite }` (file_write.go:47) — **không phải `KindDestructive`**, nên `file.write` **KHÔNG kích hoạt HITL confirmation** dù `AllowDestructiveTools=false`; nó luôn tự động chạy nếu path hợp lệ, chỉ được `guardrails.CheckTool` ghi 1 dòng `slog.Info("guardrails: write tool allowed", ...)` (guard.go). Điều này hợp lý vì file.write ghi trong sandbox path — hậu quả tệ nhất là ghi đè file trong `allowedPaths`, không phải chạy code tuỳ ý như shell.exec.

### 15.4 Kết luận rủi ro

Tổng hợp theo từng tool, phân biệt rõ RỦI RO THẬT (chưa mitigate) và ĐÃ MITIGATE (có bằng chứng code):

**`shell.exec` — RỦI RO THẬT, mức độ cao nếu 2 điều kiện cấu hình sai đồng thời xảy ra:**
- Không sandbox (chạy trực tiếp `os/exec` trên host, không container/chroot/seccomp) — xác nhận `shell.go:85`.
- Command allowlist parameter tồn tại trong code nhưng **luôn được gọi với `nil`** ở cả 3 call site production (`main.go:370`, `main.go:391`, `promptsize/main.go:71`) — tức tính năng allowlist **chưa từng thực sự bật**, không có blocklist cho lệnh nguy hiểm (`rm -rf`, `sudo`, pipe-to-shell...).
- Không giới hạn CPU/memory/process-tree, chỉ có timeout cooperative 30s.
- **Mitigation duy nhất đang hoạt động thật** là 2 lớp bên ngoài `shell.go`: (1) `Kind=KindDestructive` → bắt buộc HITL confirm trừ khi `ALLOW_DESTRUCTIVE_TOOLS=true`; (2) `IsPrivilegedTool` → chỉ owner-tenant (fail-closed nếu chưa cấu hình `OWNER_TENANT_IDS`) mới gọi được. Nếu vận hành đúng 2 biến môi trường này (`OWNER_TENANT_IDS` giữ rỗng hoặc chỉ chứa tenant tin cậy, `ALLOW_DESTRUCTIVE_TOOLS` giữ `false`), rủi ro được khoanh vùng ở mức "owner tự chạy shell trên server của chính họ, có confirm mỗi lần" — chấp nhận được cho use-case single-user/small-team hiện tại của JARVIS, nhưng **sẽ là lỗ hổng nghiêm trọng ngay khi mở multi-tenant thật mà quên set `OWNER_TENANT_IDS`, hoặc khi ai đó bật `ALLOW_DESTRUCTIVE_TOOLS=true` để "cho tiện" (comment trong code đã tự cảnh báo chính xác điều này).**

**`file.write` — ĐÃ CÓ MITIGATION cho path traversal, nhưng KHÔNG có HITL:**
- Path whitelist + kiểm tra prefix có separator hoạt động đúng, đã verify thực nghiệm rằng `../` traversal bị `filepath.Abs`+prefix-check chặn.
- Tenant isolation qua path nesting hoạt động cho `file.write`, nhưng KHÔNG đồng bộ với `file.read`/`file.search` (thiếu nesting) — không phải lỗ hổng độc lập vì cả nhóm `file.*` đều bị khoanh trong `privilegedTools`, nhưng là điểm KHÔNG NHẤT QUÁN đáng sửa nếu sau này tách `file.read` ra khỏi nhóm privileged.
- Rủi ro còn lại: `Kind=KindWrite` (không phải Destructive) nên auto-execute không cần xác nhận người dùng — chấp nhận được vì đã có path sandbox, nhưng nếu `allowedPaths` rỗng (`len(t.allowedPaths)==0`) thì `isAllowed()` trả `true` vô điều kiện ("no restrictions configured") — đây là **fail-open theo thiết kế**, có ghi rõ trong comment (`NewFileWriteTool`: *"An empty allowedPaths means all paths are allowed (use with caution)"*) nhưng vẫn là điểm cấu hình sai một lần là mất toàn bộ mitigation.

**Tổng kết một câu cho phỏng vấn:** JARVIS áp dụng đúng nguyên lý **defense-in-depth theo lớp** (tool-level allowlist → Kind classification → guardrail HITL → tenant/owner gate) thay vì sandbox hệ điều hành (container/seccomp) — đây là lựa chọn hợp lý cho quy mô hiện tại (self-hosted, ít tenant), nhưng điểm yếu nhất về mặt kỹ thuật là **command allowlist của `shell.exec` chưa từng được wiring thật** (luôn `nil`) — nếu phải chọn 1 điều để fix trước khi mở multi-tenant thật, đây là nó.
## 16. Câu hỏi phỏng vấn thường gặp

Mỗi câu gồm: **(a)** khái niệm chung, **(b)** cách trả lời dựa trên code thật của Jarvis (có trích dẫn), **(c)** câu hỏi đào sâu tiếp theo mà người phỏng vấn có thể hỏi tiếp.

### Câu 1 — Thiết kế memory 3-tier cho AI agent

**(a)** Vì sao 1 AI agent cần nhiều tầng memory thay vì chỉ dùng context window? Vì context window có hạn (giá theo token), và thông tin cần "nhớ lâu" (tên user, sở thích, quy ước làm việc) không nên nằm mãi trong history — vừa tốn token lặp lại mỗi request, vừa mất khi conversation quá dài phải cắt bớt (compaction).

**(b)** Jarvis giải quyết bằng: short-term = `agent.State.Messages` (history gửi lại mỗi request, agent-go stateless); working = `State.RecalledMemories` (chỉ sống trong 1 lượt `Run()`); long-term = `memory.Store` (in-memory, namespace theo tenant) + Mongo (persist qua restart, nạp lại bằng `Store.LoadFromMongo` lúc khởi động). Điểm đặc biệt: Jarvis có **2 pipeline ghi vào long-term** — `ExtractNode` (regex, đồng bộ, mọi request, miễn phí — `internal/memory/extract.go:41-75`) và `Learner.LearnFromConversation` (LLM thật, bất đồng bộ sau response, có gate `worthLearning` để không tốn tiền mỗi lượt chat tán gẫu — `internal/memory/learner_gate.go:15-52`). Recall dùng chiến lược 3-bước tăng dần chi phí: keyword-to-key lookup → full-text substring → semantic search bằng embedding (chỉ khi 2 bước trước rỗng) — `internal/memory/recall.go:39-109`.

**(c)** Nếu phải thêm "importance score" kiểu Generative Agents (Park et al.) vào `Learner`, bạn sẽ đặt nó ở đâu trong pipeline hiện tại, và nó ảnh hưởng gì đến gate `worthLearning` (vốn đang dùng luật OR rời rạc, không dùng score liên tục)?

---

### Câu 2 — MemGPT vs implementation thật của Jarvis

**(a)** MemGPT hình dung LLM context như RAM của OS: có "page fault" khi cần tìm thêm dữ liệu, agent tự quyết định paging in/out.

**(b)** Jarvis có 1 nửa ý tưởng này (`SummarizeNode` = evict phần cũ khi `len(Messages) > 15`, `RecallNode` = load fact liên quan vào system prompt) nhưng **không có "page fault" chủ động** — recall là fixed pipeline chạy TRƯỚC mọi request (`NodeRecall` luôn là node đầu graph, `internal/agent/engine.go` dispatch), không phải một tool mà LLM tự gọi khi "thấy thiếu thông tin". Nói cách khác: paging ở đây là **push-based** (hệ thống tự đẩy fact vào prompt) chứ không phải **pull-based** (agent tự pull khi cần) như MemGPT gốc.

**(c)** Muốn chuyển sang pull-based (agent tự gọi 1 tool `memory.recall` khi thấy cần), bạn phải đánh đổi gì? (Gợi ý: tool `memory.recall` đã tồn tại — `internal/tools/memory_tools.go:111` — nhưng `RecallNode` tự động vẫn chạy song song; có nguy cơ double-recall/nhồi trùng thông tin vào prompt không?)

---

### Câu 3 — Circuit breaker pattern: 3-state chuẩn vs "loop guard" của Jarvis

**(a)** Circuit breaker chuẩn (Hystrix/Polly) có 3 state: Closed (bình thường) → Open (chặn hẳn sau khi vượt threshold lỗi, fail-fast) → Half-Open (thử lại sau cooldown, thành công thì về Closed, thất bại thì về Open lại).

**(b)** `guardrails.CircuitBreaker` (`internal/guardrails/circuit_breaker.go:22-44`) **không** có 3 state này — nó chỉ có `count`/`maxRepeats`, phát hiện khi LLM gọi lặp **đúng cùng 1 tool + cùng args** liên tiếp (dấu hiệu stuck reasoning loop), không liên quan lỗi hạ tầng. Không có cooldown theo thời gian, không tự phục hồi theo thời gian (`TestCircuitBreaker_KeepsErroring` xác nhận: sau khi lỗi 1 lần, tiếp tục gọi cùng key vẫn tiếp tục lỗi mãi, `count` cứ tăng). Circuit breaker THẬT theo đúng nghĩa 3-state (có `coolUntil`, backoff cấp số nhân `cooldown * (1 << min(fails-1,4))`, cap 5 phút, day-lock 2 giờ khi hết quota ngày) nằm ở `provider/fallback/fallback.go:229-242` — chỉ khác là không đặt tên "CircuitBreaker".

**(c)** Tại sao dùng cooldown theo TIMESTAMP so sánh (`coolUntil`) mà không dùng state enum tường minh (Closed/Open/HalfOpen) như circuit breaker sách giáo khoa? Cách nào dễ có race condition hơn trong môi trường concurrent (nhiều goroutine gọi `Generate()` đồng thời)?

---

### Câu 4 — Multi-provider fallback: điều kiện chuyển provider

**(a)** Khi có nhiều LLM provider, hệ thống cần quyết định: lỗi nào nên retry qua provider khác (transient), lỗi nào nên trả về ngay (permanent).

**(b)** `provider/fallback/fallback.go` dùng `isRetryable(err)` (dòng 289-319) phân lỗi theo message string: `429/rate limit/503/502/500/timeout/connection refused` → retryable (chuyển provider kế); `400/401/403/invalid/context canceled` → non-retryable (trả lỗi ngay, vì thử provider khác cũng vô nghĩa nếu lỗi do request sai). Lỗi lạ không nhận diện được → mặc định retryable ("an toàn hơn là cứ thử"). Đặc biệt: **không retry cùng 1 provider trước khi fallback** — hễ lỗi retryable là chuyển ngay sang provider kế trong chain (retry ở cấp chain, không phải cấp provider). Một chi tiết dễ bị bỏ sót: lỗi rate-limit có thể đến ở **chunk đầu của stream** (HTTP 200 rồi mới báo lỗi qua SSE) — code phải "peek" chunk đầu trước khi coi là thành công (dòng 91-121).

**(c)** `scopeModel()` (fallback.go:186-227) xoá `Options.Model` nếu family của model không khớp provider hiện tại trong chain (ví dụ ép `deepseek-v4-flash` nhưng chain đang thử Gemini) — tại sao cần bug fix này, và nếu thiếu nó thì hậu quả cụ thể là gì (tính theo số request lỗi phát sinh mỗi lượt gọi learner/summarize)?

---

### Câu 5 — Strategy + Factory pattern cho multi-LLM

**(a)** Khi hệ thống cần hỗ trợ nhiều LLM provider mà code gọi (engine) không nên biết chi tiết từng API, dùng Strategy pattern (interface chung, nhiều implementation) + Factory pattern (tạo đúng implementation theo config).

**(b)** `provider.Provider` (`provider.go:8-11`) chỉ có 2 method (`Generate`, `Name`) — engine chỉ phụ thuộc interface này, không biết đang nói với Anthropic/Gemini/DeepSeek/Ollama. `factory.New(cfg)` (`provider/factory/factory.go:25-38`) tạo instance theo `cfg.Provider`. Điểm hay: `fallback.Provider` **chính nó cũng implement `provider.Provider`** — Composite pattern chồng lên Strategy, engine hoàn toàn không biết có bao nhiêu client thật đứng sau 1 lần gọi `Generate()`.

**(c)** `ToolCall.ThoughtSignature` (chỉ Gemini dùng, cho multi-turn reasoning) được đưa thẳng vào struct `provider.ToolCall` chung — đây là một sự "rò rỉ" chi tiết implementation cụ thể vào abstraction chung. Bạn có đồng ý với quyết định thiết kế này không? Cách nào khác để tránh rò rỉ mà vẫn mang được dữ liệu này qua nhiều turn?

---

### Câu 6 — MCP: chuẩn hoá tool discovery, và bài toán multi-tenant với remote server

**(a)** MCP chuẩn hoá 2 hành vi giữa agent host và external tool server: `tools/list` (discover) và `tools/call` (invoke), qua JSON-RPC 2.0, transport có thể là stdio (local subprocess) hoặc Streamable HTTP/SSE (remote).

**(b)** Jarvis triển khai cả 2: `MCPClient` (stdio, admin cấu hình qua YAML, đăng ký 1 lần lúc khởi động vào registry DÙNG CHUNG — `internal/mcp/discovery.go`) và `SSEClient` (Streamable HTTP, user tự thêm URL+token, discovery **per-request** vào 1 registry RIÊNG cho lượt chạy đó — `mcp.DiscoverSSE`, gọi từ `internal/agent/engine.go:236-262`). Lý do tách biệt: nếu SSE cũng dùng registry chung, 2 user cấu hình 2 MCP server khác nhau sẽ data-race khi ghi registry, và tool của user A có thể rò sang cho user B gọi được — đánh đổi lấy chi phí discovery lặp lại mỗi turn.

**(c)** `sse.go` cố tình giữ `mcpProtocolVersion = "2024-11-05"` (bản cũ nhất) và không đọc protocolVersion server trả về để "negotiate" — comment tự nhận đây là nợ kỹ thuật có chủ đích. Nếu một MCP server tương lai yêu cầu bắt buộc protocol version mới hơn mới cho `initialize` thành công, hệ thống hiện tại sẽ fail thế nào, và bạn sẽ implement "cải tiến đúng đắn hơn" (đọc version server trả, adapt theo) ra sao mà không phá tương thích với server cũ?

---

### Câu 7 — Tool-calling safety: sandbox hay allowlist?

**(a)** Cho AI agent chạy tool nguy hiểm (shell, file write) là rủi ro cổ điển của "agentic coding" — 2 chiến lược chính: sandbox hệ điều hành (container/seccomp/chroot — cách ly hoàn toàn) hoặc allowlist tầng ứng dụng (permission-based, kiểm tra ở code).

**(b)** Jarvis chọn **allowlist tầng ứng dụng, nhiều lớp (defense-in-depth)**, KHÔNG sandbox OS: `shell.exec` gọi trực tiếp `exec.CommandContext` trên host (`shell.go:85`), tham số `allowedCommands` tồn tại nhưng **luôn được gọi với `nil`** ở cả 3 call site production — tính năng allowlist command chưa từng thực sự bật. Mitigation thật nằm ở 2 lớp NGOÀI `shell.go`: (1) `Kind=KindDestructive` → bắt buộc HITL confirm qua `guardrails.CheckTool` trừ khi `ALLOW_DESTRUCTIVE_TOOLS=true`; (2) `IsPrivilegedTool` → chỉ tenant "chủ" (`OWNER_TENANT_IDS`, fail-closed nếu chưa cấu hình — chỉ tenant `"default"` được coi là chủ) mới gọi được. `file.write` thì có path whitelist thật (`isAllowed()` — `file_write.go:101-119`, chặn được path traversal `../` một cách "tình cờ đúng" nhờ hành vi chuẩn của `filepath.Abs`/`Clean`), cộng thêm chèn `tenantID` vào path để cách ly ghi giữa tenant.

**(c)** Nếu phải chọn 1 điểm để fix trước khi mở multi-tenant thật cho `shell.exec`, bạn chọn gì — bật allowlist command thật, thêm resource limit (cgroup/rlimit), hay chuyển sang sandbox process (gVisor/Firecracker)? Đánh đổi chi phí vận hành vs mức độ an toàn của mỗi lựa chọn?

---

### Câu 8 — Multi-tenancy: nơi enforce vs nơi chỉ set mà không dùng

**(a)** Multi-tenancy trong hệ thống agent cần enforce ở MỌI nơi dữ liệu được đọc/ghi, không chỉ ở tầng nhận request — 1 middleware set tenant ID vào context không tự động bảo vệ gì nếu code phía sau không đọc lại nó.

**(b)** Jarvis lấy tenant từ header `X-Tenant-ID` (`middleware/tenant.go:15-24`, KHÔNG decode JWT — tin tưởng gateway phía trước đã xác thực), propagate qua `context.WithValue` với `contextKey` type riêng (chống collision). Enforce **đầy đủ** ở `internal/memory` (Store partition theo tenant từ cấu trúc dữ liệu — `map[string]map[string]storeEntry`, Mongo filter luôn có `tenantId`) và `internal/tools/rag.go` (mọi query Mongo — vector/text/read/list/parent-window — đều có `$match tenantId`, và comment ghi rõ đây là fix cho 1 lỗ hổng đã từng thật xảy ra: "this is what previously made rag.read leak data across tenants"). **KHÔNG enforce** ở `internal/storage/sqlite` và `internal/storage/chroma` (0 field/namespace tenant nào) — nhưng 2 package này cũng không nằm trên đường dẫn production multi-tenant (chỉ dùng cho dev/offline).

**(c)** Middleware tin tưởng tuyệt đối header `X-Tenant-ID` do client set — nếu ai gọi trực tiếp `POST /chat` vào agent-go (bỏ qua BFF gateway) và tự set header đó thành tenant khác, hệ thống chấp nhận vô điều kiện. Đây có phải lỗ hổng thật không, và biện pháp nào (mTLS giữa gateway-agent, network isolation, hay ký request) sẽ khắc phục đúng lớp vấn đề này?

---

### Câu 9 — RAG: khoảng cách giữa tên gọi kỹ thuật và implementation thật

**(a)** Trong RAG production, tên các kỹ thuật (hybrid search, rerank, HyDE) đôi khi được dùng dù implementation thực tế đơn giản hơn nhiều so với ý nghĩa gốc trong literature — cần đọc code để biết "tên gọi có xứng với chất lượng" hay không.

**(b)** Ở Jarvis: **hybrid search là THẬT** — dense (`$vectorSearch` MongoDB Atlas) + sparse (`$text` MongoDB, TF-IDF-based) merge bằng Reciprocal Rank Fusion có trọng số 0.7/0.3 nghiêng dense (`internal/tools/rag.go:578-647`). **HyDE là THẬT** — gọi LLM sinh câu trả lời giả định trước khi embed, chỉ áp dụng cho nhánh dense (đúng lý thuyết gốc), có timeout riêng 8s độc lập (`rag.go:799-841`). Nhưng **"rerank" mặc định KHÔNG dùng cross-encoder** — chỉ là keyword-overlap heuristic thuần Go (đếm từ trùng, trộn `0.7*score_gốc + 0.3*overlap`), comment code tự thừa nhận "no LLM call, no cross-encoder" (`rag.go:676-677`); chế độ `EnableLLMRerank` (opt-in) là LLM-as-reranker qua prompting, cũng không phải model rerank chuyên dụng train riêng như Cohere Rerank/`bge-reranker`.

**(c)** Nếu phải thêm 1 cross-encoder rerank thật vào pipeline này (ví dụ self-host `bge-reranker-base`), bạn sẽ chèn nó vào đâu trong luồng hiện tại (`hybridSearch` → rerank → top-5 → PDR), và đánh đổi latency/cost nào so với 2 lựa chọn hiện có (keyword-overlap miễn phí vs LLM-as-reranker)?

---

### Câu 10 — Storage: đọc tên package, đừng tin tên package

**(a)** Một nguyên tắc audit code quan trọng: tên package/file có thể phản ánh Ý ĐỊNH thiết kế ban đầu, không phải hiện trạng — luôn verify bằng cách đọc import + logic thật.

**(b)** `internal/storage/chroma/chroma.go` gợi ý ChromaDB client, nhưng thực tế: import chỉ có `math`/`sort` (không `net/http`, không SDK Chroma), `VectorStore.entries` là `map[string]*entry` trong RAM, `Search()` là linear scan O(n) tự tính cosine similarity bằng tay — comment đầu file tự thừa nhận "MVP... sau này có thể thay bằng Chroma embedded hoặc pgvector". Hệ quả: không persist qua restart, không có HNSW/ANN index thật, không scale. `internal/storage/sqlite/sqlite.go` cũng dễ gây nhầm — không phải vector store (không cột embedding, không extension `sqlite-vec`, dùng driver pure-Go `modernc.org/sqlite` không hỗ trợ CGO extension), mà chỉ là FTS5 keyword search cho chat history/memory. Cả 2 package **đều không được import ở đâu ngoài file test của chính nó** — RAG search thật của Jarvis đi qua MongoDB Atlas `$vectorSearch`, không đụng 2 package này.

**(c)** Nếu bạn được giao nhiệm vụ "thay Chroma giả bằng ChromaDB thật", bạn sẽ thiết kế lại interface `VectorStore` thế nào để (1) không phải sửa code gọi ở nơi khác (nếu có), và (2) migrate được dữ liệu hiện có (dù hiện tại chưa persist gì)?

---

### Câu 11 — Orchestrator: keyword routing vs LLM-based classification

**(a)** Multi-agent system cần 1 "router" quyết định request nào đi tới agent chuyên biệt nào — 2 hướng chính: keyword/rule-based (rẻ, nhanh, cứng) hoặc LLM-based classification (mềm dẻo, tốn thêm 1 round-trip + token).

**(b)** `docs/architecture/multi-agent-orchestrator-design.md` hình dung một `IntentRouter` 2 tầng (keyword trước, LLM nhẹ fallback khi không khớp — "BƯỚC 4" trong lộ trình). Code thật (`orchestrator.go:110-127`) **chỉ dừng ở keyword-matching, không hề gọi LLM để classify** — route xong ngay, LLM call đầu tiên trong lượt chạy là của agent ĐÃ được chọn. `matchTrigger()` dùng word-boundary regex cho keyword ASCII đơn từ (tránh `"go"` khớp `"golang"/"mongo"`) và substring cho cụm tiếng Việt/nhiều từ — một bug lịch sử đã fix, có test khoá lại hành vi (`orchestrator_test.go`).

**(c)** Với ngân sách LLM hẹp của dự án (một constraint thực tế đã ghi trong project memory), bạn có đề xuất thêm LLM-based classify như design doc ban đầu không? Nếu có, sẽ áp dụng ở đâu để giảm thiểu chi phí (ví dụ: chỉ gọi LLM nhẹ khi keyword KHÔNG match, giống thiết kế gốc) — và làm sao đo được ROI (tỷ lệ misroute giảm được có đáng chi phí tăng thêm)?

---

### Câu 12 — Skills: progressive disclosure và vấn đề activation chính xác

**(a)** "Progressive disclosure" cho skill: không nhồi hết instruction mọi skill vào system prompt (tốn token, nhiễu), chỉ load full content khi cần — nhưng "khi cần" phải được xác định ĐÚNG, kích hoạt sai skill (nhồi sai nội dung vào prompt) tệ hơn không kích hoạt gì.

**(b)** Khác Claude Skills (model tự quyết định mở skill), Jarvis quyết định kích hoạt **hoàn toàn bằng code Go** (`Loader.MatchSkill` chấm điểm — tên/trigger tường minh = 100 điểm, `when_to_use` = 3 điểm/từ khớp, `description` = 1 điểm/từ khớp, ngưỡng tối thiểu `minSkillActivationScore = 6`). Ngưỡng này được hiệu chỉnh từ dữ liệu đo thật: trên 23 skill, điểm của skill ĐÚNG và SAI đều nằm trong khoảng 1-4 nếu chỉ dùng substring match ngây thơ — phải thêm word-boundary + stopwords list (~70 từ) để loại nhiễu như `"use"` khớp `"useMemo"`. Vì thế catalogue trong system prompt bỏ hẳn `description`, chỉ giữ TÊN (model không có vai trò chọn nên description không mua được gì, tốn ~21% token input mỗi request).

**(c)** Nếu có 100 skill thay vì 23, cơ chế scoring theo keyword hiện tại (tuyến tính qua từng skill mỗi request) có scale tốt không? Bạn sẽ chuyển sang phương án nào (ví dụ: embedding similarity giữa query và `when_to_use`, thay cho keyword scoring) — và đánh đổi gì (thêm 1 lần gọi embedding mỗi request vs độ chính xác)?

---

### Câu 13 — Resilience: phân biệt "retry ở tầng nào" trong 1 hệ thống nhiều tầng lỗi

**(a)** Một request LLM có thể lỗi ở nhiều tầng khác nhau (network, rate-limit provider, model hết token, tool timeout, LLM lặp vô hạn tool call) — mỗi tầng cần chiến lược retry/circuit-breaking khác nhau, không nên dùng 1 cơ chế duy nhất cho tất cả.

**(b)** Jarvis có ít nhất 4 lớp resilience độc lập, dễ nhầm với nhau nếu không đọc kỹ: (1) `guardrails.CircuitBreaker` — phát hiện LLM tự lặp tool call (không phải lỗi hạ tầng); (2) `provider/fallback` — cooldown/backoff + failover giữa nhiều LLM provider khi lỗi hạ tầng (429/5xx/timeout); (3) `tools.Registry` timeout per-tool (`DefaultToolTimeout = 60s`, hoặc riêng theo `TimeoutTool.Timeout()` như `shell.exec = 30s`) — chống 1 tool treo làm treo cả request; (4) `ReflectAndExtract`/`SummarizeMessages` có retry-theo-loại-lỗi riêng của chính nó (timeout → không retry, truncated → retry với budget x2, parse lỗi → retry cùng budget) — vì đây là LLM call PHỤ, cần chiến lược khác LLM call CHÍNH.

**(c)** Nếu 1 request thật đi qua CẢ 4 lớp trên trong 1 lượt chạy (ví dụ: agent gọi tool bị treo → timeout tool → agent thử lại tool khác → bị stuck-loop-detector chặn → đồng thời provider chính đang bị rate-limit nên fallback qua provider 2), hãy vẽ lại trình tự log/event mà bạn kỳ vọng thấy, và chỉ ra điểm nào trong hệ thống hiện tại CHƯA có test coverage cho tương tác giữa nhiều lớp resilience cùng lúc (chỉ có test riêng từng lớp).

---

### Câu 14 — "Code thật vs comment/doc": kỹ năng audit quan trọng nhất khi làm agentic coding

**(a)** LLM code assistant (và cả người) có xu hướng tin comment/docstring hơn code thật — nhưng comment có thể lỗi thời, hoặc được viết TRƯỚC khi code thay đổi. Kỹ năng audit đúng là luôn verify hành vi bằng cách đọc logic thật + chạy test, không dừng ở comment.

**(b)** Jarvis có ít nhất 5 ví dụ thật của lớp lỗi này được phát hiện trong tài liệu: (1) `RunAll` trong eval harness — comment nói "small delay to be kind to LLM APIs" nhưng code không có `time.Sleep` nào, và `break` trong `select` không thoát được vòng `for` như ngụ ý; (2) `proactive.RemoveTask` — comment nói "skip in runTask" nhưng `runTask()` không có điều kiện kiểm tra nào, nên task đã "xoá" vẫn fire theo cron thật; (3) `AgentSpec.SystemPrompt` từng là dead code dù đã gán ở `main.go` (không hàm nào đọc) — sau này fix bằng cách apply ngay tại `Register()`; (4) `docs/DEPLOY.md` ghi sai tên biến (`OLLAMA_HOST` thay vì `OLLAMA_URL` thật) và sai default (`GOOGLE_THINKING_LEVEL=LOW` thay vì `"OFF"` thật); (5) `personality.Learn()` tích luỹ preference nhưng không có "keo" nối để tự update profile — đọc lướt dễ tưởng nó tự học thật.

**(c)** Với vai trò dev/reviewer dùng AI coding assistant hàng ngày, bạn sẽ thiết kế quy trình review nào để bắt được lớp lỗi "comment nói A, code làm B" này TRƯỚC khi merge — checklist thủ công, linter tự viết, hay yêu cầu comment luôn kèm test khẳng định đúng hành vi mô tả?
