package memory

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

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
func (l *Learner) LearnFromConversation(messages []provider.Message, conversationID string) {
	if l == nil || l.provider == nil || len(messages) < 2 {
		return
	}

	// Copy messages to avoid race conditions with parent caller
	msgsCopy := make([]provider.Message, len(messages))
	copy(msgsCopy, messages)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		res, err := ReflectAndExtract(ctx, l.provider, l.model, msgsCopy)
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

			// Store in active in-memory / local Store
			if l.store != nil {
				l.store.Set(fact.Key, fact.Value)
			}

			// Persist to MongoDB `memories` collection if available
			if l.mongoClient != nil {
				l.saveFactToMongo(ctx, fact, conversationID)
			}

			slog.Info("learner: learned user fact",
				"category", fact.Category,
				"key", fact.Key,
				"value", fact.Value,
				"confidence", fact.Confidence,
			)
		}

		// 2. Process Knowledge Items
		for _, ki := range res.KnowledgeItems {
			if strings.TrimSpace(ki.Title) == "" || strings.TrimSpace(ki.Content) == "" {
				continue
			}

			if l.mongoClient != nil {
				l.saveKnowledgeItemToMongo(ctx, ki, conversationID)
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

	var emb []float64
	if l.embedder != nil {
		vecs, err := l.embedder.Embed(ctx, []string{fact.Key + ": " + fact.Value})
		if err == nil && len(vecs) > 0 {
			emb = vecs[0]
		}
	}

	filter := bson.M{"key": fact.Key}
	update := bson.M{
		"$set": bson.M{
			"type":           fact.Category,
			"key":            fact.Key,
			"value":          fact.Value,
			"source":         "autonomous_reflection",
			"confidence":     fact.Confidence,
			"embedding":      emb,
			"conversationId": conversationID,
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

	filter := bson.M{"documentId": docID}
	update := bson.M{
		"$set": bson.M{
			"documentId": docID,
			"source":     source,
			"version":    1,
			"chunkIndex": 0,
			"text":       fullText,
			"embedding":  emb,
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
