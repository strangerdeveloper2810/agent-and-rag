package memory

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Learner handles continuous background reflection and knowledge persistence.
type Learner struct {
	store       *Store
	mongoClient *mongo.Client
	provider    provider.Provider
	model       string
	embedder    Embedder

	batchTurns atomic.Int32 // số lượt gộp lại trước khi thực sự chạy reflection; xem SetBatchTurns
	turnMu     sync.Mutex
	turnCount  map[string]int // conversationID → số lượt "đáng học" (qua gate worthLearning) đã tích lũy chưa reflect

	// rawTurnsSinceReflect đếm MỌI lượt gọi LearnFromConversation (kể cả lượt
	// tán gẫu bị worthLearning chặn) kể từ lần reflect thực sự gần nhất.
	//
	// Tách riêng khỏi turnCount vì lý do sau: LearnFromConversation luôn nhận
	// TOÀN BỘ lịch sử hội thoại (kể cả các lượt tán gẫu), và windowing
	// (ReflectAndExtractWithWindow) cắt theo "N tin nhắn CUỐI" của lịch sử
	// đó. Nếu tính windowMessages = 2*batchTurns (chỉ đếm lượt "đáng học"),
	// một lượt tán gẫu xen giữa batch (không tăng batchTurns nhưng vẫn nằm
	// trong lịch sử đầy đủ) có thể đẩy nội dung của lượt "đáng học" đầu tiên
	// ra ngoài cửa sổ. Đếm theo số lượt RAW mới phản ánh đúng kích thước
	// lịch sử cần giữ lại.
	rawTurnsSinceReflect map[string]int
}

// NewLearner creates a continuous memory & knowledge learner.
func NewLearner(store *Store, mongoClient *mongo.Client, p provider.Provider, model string, embedder Embedder) *Learner {
	l := &Learner{
		store:                store,
		mongoClient:          mongoClient,
		provider:             p,
		model:                model,
		embedder:             embedder,
		turnCount:            make(map[string]int),
		rawTurnsSinceReflect: make(map[string]int),
	}
	// mặc định KHÔNG batch — giữ hành vi cũ (chạy ngay mỗi lượt). atomic.Int32
	// zero value là 0, PHẢI Store(1) rõ ràng — 0 nghĩa là "luôn gộp, không
	// bao giờ reflect", sai với hợp đồng cũ.
	l.batchTurns.Store(1)
	return l
}

// SetBatchTurns đặt số lượt chat gộp lại trước khi thực sự chạy 1 lần
// reflection (giảm số request LLM nền). n <= 0 → coi như 1 (không batch).
//
// batchTurns dùng atomic.Int32 (thay vì int thường) vì nó được ĐỌC từ goroutine
// nền của LearnFromConversation (không giữ turnMu ở đó) trong khi có thể bị
// SetBatchTurns GHI từ goroutine khác — dù wiring hiện tại (main.go gọi đúng 1
// lần lúc khởi động, trước khi có request nào) khiến race này chưa xảy ra
// trên thực tế, atomic vẫn cần để đúng theo Go memory model, không chỉ "may
// mắn chưa crash".
func (l *Learner) SetBatchTurns(n int) {
	if n <= 0 {
		n = 1
	}
	l.batchTurns.Store(int32(n))
}

// shouldReflectNow tăng bộ đếm lượt của conversationID và trả true khi đã
// đạt batchTurns (rồi reset về 0 cho chu kỳ tiếp theo).
func (l *Learner) shouldReflectNow(conversationID string) bool {
	l.turnMu.Lock()
	defer l.turnMu.Unlock()
	l.turnCount[conversationID]++
	if l.turnCount[conversationID] < int(l.batchTurns.Load()) {
		return false
	}
	l.turnCount[conversationID] = 0
	return true
}

// LearnFromConversation triggers autonomous reflection in a background goroutine.
//
// ctx must be the request's original context (e.g. r.Context() from the HTTP
// handler) so the tenant ID set by middleware.TenantMiddleware can be
// resolved. The tenant ID is captured synchronously BEFORE the goroutine is
// spawned — the HTTP handler returns immediately after calling this method,
// which (per net/http) cancels the request context, so the background
// goroutine cannot rely on ctx staying alive or usable by the time it runs.
// The captured tenant ID is then carried into a fresh, request-independent
// context so the reflection LLM call and every Mongo write below stay
// scoped to the tenant that produced the conversation (see saveFactToMongo /
// saveKnowledgeItemToMongo).
func (l *Learner) LearnFromConversation(ctx context.Context, messages []provider.Message, conversationID string) {
	if l == nil || l.provider == nil || len(messages) < 2 {
		return
	}

	// Đếm MỌI lượt gọi (kể cả tán gẫu bị gate chặn dưới đây) — xem comment ở
	// field rawTurnsSinceReflect để biết vì sao cần tách khỏi turnCount.
	l.turnMu.Lock()
	l.rawTurnsSinceReflect[conversationID]++
	l.turnMu.Unlock()

	// Lượt tán gẫu không có gì để học, nhưng reflection vẫn là một lượt gọi LLM
	// đầy đủ — tức mỗi câu "xin chào" đang trả tiền hai lần. Bỏ qua sớm.
	if !worthLearning(messages) {
		slog.Debug("learner: bỏ qua lượt tán gẫu (không có gì để học)")
		return
	}

	// Batch: gộp N lượt liền trước khi thực sự chạy reflection (giảm số
	// request LLM nền). batchTurns=1 (mặc định) → luôn chạy ngay, giữ đúng
	// hành vi cũ.
	if !l.shouldReflectNow(conversationID) {
		return
	}

	// Cửa sổ tin nhắn đưa vào reflection PHẢI dựa trên số lượt RAW (kể cả tán
	// gẫu) đã tích lũy kể từ lần reflect trước, KHÔNG dựa trực tiếp vào
	// batchTurns — xem comment ở rawTurnsSinceReflect. Sàn ở
	// maxReflectionMessages để trường hợp batchTurns=1 (mặc định, không
	// batch) giữ đúng cửa sổ cũ của ReflectAndExtract (lượt hiện tại + 1 lượt
	// trước làm ngữ cảnh = 4 tin nhắn) thay vì bị co hẹp về 2*1=2.
	l.turnMu.Lock()
	rawTurns := l.rawTurnsSinceReflect[conversationID]
	l.rawTurnsSinceReflect[conversationID] = 0
	l.turnMu.Unlock()

	windowMessages := 2 * rawTurns
	if windowMessages < maxReflectionMessages {
		windowMessages = maxReflectionMessages
	}

	tenantID := middleware.GetTenantID(ctx)

	// Copy messages to avoid race conditions with parent caller
	msgsCopy := make([]provider.Message, len(messages))
	copy(msgsCopy, messages)

	go func() {
		bgCtx := context.WithValue(context.Background(), middleware.TenantIDKey, tenantID)
		bgCtx, cancel := context.WithTimeout(bgCtx, 45*time.Second)
		defer cancel()

		res, err := ReflectAndExtractWithWindow(bgCtx, l.provider, l.model, msgsCopy, windowMessages)
		if err != nil {
			slog.Warn("learner: reflection failed", "err", err)
			return
		}

		if res == nil {
			return
		}

		// 1. Process User Facts
		for _, fact := range res.UserFacts {
			if strings.TrimSpace(fact.Key) == "" || strings.TrimSpace(fact.Value) == "" {
				continue
			}

			// Store in active in-memory / local Store, scoped to the tenant.
			if l.store != nil {
				l.store.Set(tenantID, fact.Key, fact.Value)
			}

			// Persist to MongoDB `memories` collection if available
			if l.mongoClient != nil {
				l.saveFactToMongo(bgCtx, fact, conversationID)
			}

			slog.Info("learner: learned user fact",
				"category", fact.Category,
				"key", fact.Key,
				"value", fact.Value,
				"confidence", fact.Confidence,
				"tenant", tenantID,
			)
		}

		// 2. Process Knowledge Items
		for _, ki := range res.KnowledgeItems {
			if strings.TrimSpace(ki.Title) == "" || strings.TrimSpace(ki.Content) == "" {
				continue
			}

			if l.mongoClient != nil {
				l.saveKnowledgeItemToMongo(bgCtx, ki, conversationID)
			}

			slog.Info("learner: learned knowledge item",
				"title", ki.Title,
				"summary", ki.Summary,
				"tags", ki.Tags,
			)
		}
	}()
}

func (l *Learner) saveFactToMongo(ctx context.Context, fact UserFact, conversationID string) {
	coll := l.mongoClient.Collection("memories")
	now := time.Now()
	tenantID := middleware.GetTenantID(ctx)

	var emb []float64
	if l.embedder != nil {
		vecs, err := l.embedder.Embed(ctx, []string{fact.Key + ": " + fact.Value})
		if err == nil && len(vecs) > 0 {
			emb = vecs[0]
		}
	}

	// Filter MUST include tenantId — otherwise two tenants learning a fact
	// with the same key (e.g. "user_name") would upsert the very same
	// document and silently overwrite each other's data.
	filter := bson.M{"key": fact.Key, "tenantId": tenantID}
	update := bson.M{
		"$set": bson.M{
			"type":           fact.Category,
			"key":            fact.Key,
			"value":          fact.Value,
			"source":         "autonomous_reflection",
			"confidence":     fact.Confidence,
			"embedding":      emb,
			"conversationId": conversationID,
			"tenantId":       tenantID,
			"updatedAt":      now,
		},
		"$setOnInsert": bson.M{
			"_id":       bson.NewObjectID(),
			"createdAt": now,
		},
	}

	// PHẢI có SetUpsert: filter theo {key, tenantId} nên lần học đầu tiên của một
	// key chưa có document nào khớp — thiếu upsert thì UpdateOne không ghi gì cả.
	// Và PHẢI xử lý error: trước đây cả hai chỗ ghi Mongo đều `_, _ =`, nên việc
	// không ghi được diễn ra hoàn toàn im lặng — fact chỉ nằm trong Store
	// in-memory và mất sạch sau mỗi lần restart, trong khi log vẫn báo
	// "learner: learned user fact" như thể đã lưu.
	if _, err := coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true)); err != nil {
		slog.Warn("learner: không ghi được fact vào Mongo",
			"key", fact.Key, "tenant", tenantID, "err", err)
	}
}

func (l *Learner) saveKnowledgeItemToMongo(ctx context.Context, ki KnowledgeItem, conversationID string) {
	coll := l.mongoClient.Collection("documents")
	now := time.Now()
	tenantID := middleware.GetTenantID(ctx)

	slug := slugify(ki.Title)
	docID := fmt.Sprintf("learned-%s", slug)
	source := fmt.Sprintf("learned-%s.md", slug)

	fullText := fmt.Sprintf("# %s\n\n> Summary: %s\n> Tags: %s\n\n%s",
		ki.Title,
		ki.Summary,
		strings.Join(ki.Tags, ", "),
		ki.Content,
	)

	var emb []float64
	if l.embedder != nil {
		vecs, err := l.embedder.Embed(ctx, []string{fullText})
		if err == nil && len(vecs) > 0 {
			emb = vecs[0]
		}
	}

	// Filter MUST include tenantId — otherwise two tenants learning a
	// knowledge item with the same title (same slug → same documentId)
	// would upsert the very same document, leaking content across tenants
	// and letting rag.search read one tenant's learned knowledge as another's.
	filter := bson.M{"documentId": docID, "tenantId": tenantID}
	update := bson.M{
		"$set": bson.M{
			"documentId": docID,
			"source":     source,
			"version":    1,
			"chunkIndex": 0,
			"text":       fullText,
			"embedding":  emb,
			"tenantId":   tenantID,
			"updatedAt":  now,
		},
		"$setOnInsert": bson.M{
			"_id":       bson.NewObjectID(),
			"createdAt": now,
		},
	}

	// Cùng lý do như saveFactToMongo: cần upsert, và error phải được ghi log.
	if _, err := coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true)); err != nil {
		slog.Warn("learner: không ghi được knowledge item vào Mongo",
			"title", ki.Title, "tenant", tenantID, "err", err)
	}
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
