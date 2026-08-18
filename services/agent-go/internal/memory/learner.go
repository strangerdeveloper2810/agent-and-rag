package memory

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/ai-agent-tut/agent-go/internal/middleware"
	"github.com/ai-agent-tut/agent-go/internal/mongo"
	"github.com/ai-agent-tut/agent-go/internal/provider"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Learner handles continuous background reflection and knowledge persistence.
type Learner struct {
	store       *Store
	mongoClient *mongo.Client
	provider    provider.Provider
	model       string
	embedder    Embedder
}

// NewLearner creates a continuous memory & knowledge learner.
func NewLearner(store *Store, mongoClient *mongo.Client, p provider.Provider, model string, embedder Embedder) *Learner {
	return &Learner{
		store:       store,
		mongoClient: mongoClient,
		provider:    p,
		model:       model,
		embedder:    embedder,
	}
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

	// Lượt tán gẫu không có gì để học, nhưng reflection vẫn là một lượt gọi LLM
	// đầy đủ — tức mỗi câu "xin chào" đang trả tiền hai lần. Bỏ qua sớm.
	if !worthLearning(messages) {
		slog.Debug("learner: bỏ qua lượt tán gẫu (không có gì để học)")
		return
	}

	tenantID := middleware.GetTenantID(ctx)

	// Copy messages to avoid race conditions with parent caller
	msgsCopy := make([]provider.Message, len(messages))
	copy(msgsCopy, messages)

	go func() {
		bgCtx := context.WithValue(context.Background(), middleware.TenantIDKey, tenantID)
		bgCtx, cancel := context.WithTimeout(bgCtx, 45*time.Second)
		defer cancel()

		res, err := ReflectAndExtract(bgCtx, l.provider, l.model, msgsCopy)
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

	_, _ = coll.UpdateOne(ctx, filter, update)
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

	_, _ = coll.UpdateOne(ctx, filter, update)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
