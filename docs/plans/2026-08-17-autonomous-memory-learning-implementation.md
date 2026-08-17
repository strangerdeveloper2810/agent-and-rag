# Autonomous Memory & Continuous Knowledge Learning — Implementation Guide

**Document ID**: `2026-08-17-autonomous-memory-learning-implementation`  
**Author**: Antigravity AI Pair Programmer  
**Date**: 2026-08-17  
**Status**: Completed & Verified  
**Related Design Doc**: [`2026-08-17-autonomous-memory-learning-design.md`](file:///Users/mdm/Desktop/company/NewPineTech/ai-agent-tut/docs/plans/2026-08-17-autonomous-memory-learning-design.md)

---

## 1. Implementation Overview

This document details the code changes, file structure, configuration parameters, and verification tests implemented for the **Autonomous Continuous Memory & Knowledge Reflection** system in `services/agent-go`.

---

## 2. File Changes & Structure

```
services/agent-go/
├── internal/
│   ├── memory/
│   │   ├── reflection.go          # [NEW] LLM reflection parser and prompt
│   │   ├── learner.go             # [NEW] Background learner & MongoDB/SQLite sync
│   │   ├── reflection_test.go     # [NEW] Unit tests for reflection and learner
│   │   ├── store.go               # SQLite & in-memory key-value storage
│   │   └── recall.go              # Pre-execution keyword and semantic recall
│   ├── agent/
│   │   └── context.go             # [MODIFIED] System prompt with enriched [BỘ NHỚ]
│   └── transport/
│       └── http/
│           └── chat.go            # [MODIFIED] Hook to trigger learner on stream done
└── cmd/
    └── server/
        ├── main.go                # [MODIFIED] Wire Learner into HTTP router
        └── main_test.go           # [MODIFIED] Unit tests with learner parameter
```

---

## 3. Detailed Code Walkthrough

### 3.1 LLM Reflection Engine (`internal/memory/reflection.go`)

```go
func ReflectAndExtract(ctx context.Context, p provider.Provider, model string, messages []provider.Message) (*ReflectionResult, error)
```
* Builds conversation transcript from `RoleUser` and `RoleAssistant` messages.
* Issues structured reflection prompt with JSON schema constraint.
* Strips code fences and unmarshals into `UserFact` and `KnowledgeItem` structures.

### 3.2 Autonomous Learner (`internal/memory/learner.go`)

```go
type Learner struct {
    store       *Store
    mongoClient *mongo.Client
    provider    provider.Provider
    model       string
    embedder    Embedder
}

func (l *Learner) LearnFromConversation(messages []provider.Message, conversationID string)
```
* Executes asynchronously in a dedicated Goroutine (`go func() { ... }()`).
* Upserts user facts into `Store` and MongoDB `memories` collection.
* Formats Knowledge Items as Markdown, embeds with Voyage AI, and upserts into MongoDB `documents` collection with `documentId: "learned-<slug>"`.

### 3.3 HTTP Transport Hook (`internal/transport/http/chat.go`)

```go
var assistantContent strings.Builder
emit := func(e agent.Event) {
    if e.Type == "text" {
        assistantContent.WriteString(e.Text)
    }
    data, _ := json.Marshal(e)
    fmt.Fprintf(w, "data: %s\n\n", data)
    flusher.Flush()
}

_, _ = h.runner.Run(r.Context(), input, emit)

if h.learner != nil && assistantContent.Len() > 0 {
    fullMsgs := make([]provider.Message, 0, len(history)+2)
    fullMsgs = append(fullMsgs, history...)
    fullMsgs = append(fullMsgs, provider.Message{Role: provider.RoleUser, Content: req.UserMessage})
    fullMsgs = append(fullMsgs, provider.Message{Role: provider.RoleAssistant, Content: assistantContent.String()})
    h.learner.LearnFromConversation(fullMsgs, req.ConversationID)
}
```

### 3.4 Wiring in `cmd/server/main.go`

```go
// Wire Autonomous Learner
var embedder memory.Embedder
if cfg.VoyageKey != "" {
    vc := rag.NewClient(cfg.VoyageKey)
    embedder = memory.EmbedderFunc(func(ctx context.Context, texts []string) ([][]float64, error) {
        return vc.Embed(ctx, texts, "document")
    })
}
learnerModel := cfg.GeminiModel
if cfg.DeepSeekFlashModel != "" && cfg.DeepSeekKey != "" {
    learnerModel = cfg.DeepSeekFlashModel
}
learner := memory.NewLearner(store, mongoClient, prov, learnerModel, embedder)

srv := &http.Server{Addr: ":" + cfg.Port, Handler: newHTTPHandler(prov, orch, mongoPinger(mongoClient), learner)}
```

---

## 4. Verification & Testing

### 4.1 Automated Tests Executed
* **`internal/memory/reflection_test.go`**:
  - `TestReflectAndExtract_Success`: Validates extraction of both facts and knowledge items.
  - `TestReflectAndExtract_EmptyMessages`: Validates graceful handling of empty inputs.
  - `TestLearner_LearnFromConversation`: Validates async store update.
* **Full Test Suite Run**:
  ```bash
  go test ./...
  ```
  Result: **PASS 100% (28/28 packages)**.
* **TypeScript Monorepo Verification**:
  ```bash
  pnpm --filter @app/web exec tsc --noEmit
  pnpm --filter @app/api exec tsc --noEmit
  ```
  Result: **PASS (0 errors)**.

---

## 5. Usage & Integration Guide for Other Agents

When creating or modifying sub-agents or chat endpoints:
1. Ensure the `ChatHandler` has `.SetLearner(learner)` configured so conversation turns are passed to the reflection worker.
2. In RAG searches, queries against `documents` will automatically surface both uploaded company documentation and autonomously learned Knowledge Items (`learned-*.md`).
3. To inspect memories during debugging, query the SQLite `memories` table or MongoDB collection `memories` / `documents`.
