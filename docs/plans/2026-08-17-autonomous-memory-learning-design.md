# Autonomous Memory & Continuous Knowledge Learning — Technical Design Document

**Document ID**: `2026-08-17-autonomous-memory-learning-design`  
**Author**: Antigravity AI Pair Programmer  
**Date**: 2026-08-17  
**Status**: Implemented & Approved  
**Target Services**: `services/agent-go`, `apps/api`, `MongoDB Atlas`, `SQLite`

---

## 1. Executive Summary & Problem Statement

### 1.1 The Problem
In standard agent architectures, each conversational turn is stateless or strictly confined to the immediate context window. When a conversation ends or a new session starts:
- The agent loses recollection of user-specific preferences, tech stack constraints (e.g., "Frontend uses Vanilla CSS", "Go uses Fastify backend").
- Technical lessons, debugging solutions, and architecture decisions discovered during complex problem solving are discarded.
- Previous extraction was restricted to rigid regex patterns (`tôi tên là`, `tôi thích`) that failed to capture nuanced technical facts and procedural knowledge.

### 1.2 The Solution
Implement an **Autonomous Continuous Learning & Memory Reflection Architecture** inspired by **Harness Agent, MemGPT, and Antigravity Knowledge Items (KI)**:
1. **Asynchronous Reflection Worker**: Runs non-blocking reflection in the background after the assistant finishes streaming its answer to the user.
2. **Dual-Tier Knowledge Persistence**:
   - **Semantic Facts & User Preferences**: Persisted to SQLite and MongoDB `memories` collection with confidence scoring.
   - **Episodic Knowledge Items (KIs)**: Automatically distilled markdown documents summarizing solutions and best practices, chunked and embedded via Voyage AI into MongoDB `documents` collection for RAG hybrid search.
3. **Smart Semantic Context Injection**: Before each LLM execution, the `RecallNode` searches both SQLite facts and RAG vector store to inject the most relevant learned knowledge directly into the System Prompt `[BỘ NHỚ]`.

---

## 2. Architecture Diagram

```mermaid
flowchart TD
    User([User Prompt]) --> HTTPHandler[HTTP Transport: POST /chat]
    
    subgraph PreExecution [1. Pre-Execution Context Building]
        HTTPHandler --> RecallNode[memory.RecallNode]
        RecallNode --> MemoryStore[(SQLite / Mongo Memories)]
        RecallNode --> RAGStore[(MongoDB Vector Search / RAG)]
        RecallNode --> ContextBuilder[System Prompt + [BỘ NHỚ]]
    end
    
    subgraph Execution [2. ReAct Execution Loop]
        ContextBuilder --> Engine[Agent Engine / Orchestrator]
        Engine --> LLMStream[LLM Streaming Response]
        LLMStream --> SSEOutput([SSE Stream to User])
    end
    
    subgraph PostExecution [3. Async Autonomous Reflection Worker]
        SSEOutput -.->|Asynchronous Goroutine| Learner[memory.Learner]
        Learner --> FastLLM[Fast Model: DeepSeek Flash / Gemini Flash]
        FastLLM --> Extractor[Structured JSON Extractor]
        
        Extractor -->|User Facts & Preferences| SaveFacts[Store.Set + Mongo Memories Collection]
        Extractor -->|Technical Solutions & KIs| SaveKIs[RAG Ingestion + Voyage Embedding]
        
        SaveFacts --> MemoryStore
        SaveKIs --> RAGStore
    end
```

---

## 3. Core Components Specification

### 3.1 LLM Reflection Engine (`internal/memory/reflection.go`)
* **Interface**: `ReflectAndExtract(ctx, provider, model, messages)`
* **Model Selection**: Uses fast, cost-effective models (e.g. `deepseek-v4-flash` or `gemini-2.5-flash`) with strict 1500 max output tokens.
* **Output Schema**:
  ```json
  {
    "user_facts": [
      {
        "category": "tech_stack | coding_preference | user_profile | rule",
        "key": "short_identifier_in_english",
        "value": "detailed_value_string",
        "confidence": 0.95
      }
    ],
    "knowledge_items": [
      {
        "title": "Clear concise title",
        "summary": "1-2 sentence overview",
        "tags": ["tag1", "tag2"],
        "content": "Markdown detailing problem, solution, code snippet, and best practice"
      }
    ]
  }
  ```

### 3.2 Autonomous Learner Manager (`internal/memory/learner.go`)
* **Responsibility**: Orchestrates background reflection, fact deduplication, SQLite/Mongo upsert, and RAG ingestion.
* **Methods**:
  - `NewLearner(store, mongoClient, provider, model, embedder)`
  - `LearnFromConversation(messages, conversationID)`: Launches async goroutine with 45s timeout.
  - `saveFactToMongo(ctx, fact, conversationID)`: Upserts into collection `memories`.
  - `saveKnowledgeItemToMongo(ctx, ki, conversationID)`: Creates a `learned-<slug>.md` document in collection `documents` with Voyage AI vector embeddings.

### 3.3 HTTP Transport Hook (`internal/transport/http/chat.go`)
* **Hook Mechanism**: Accumulates streamed assistant tokens in `strings.Builder`.
* **Trigger Condition**: When `h.runner.Run` finishes and `assistantContent.Len() > 0`, invokes `h.learner.LearnFromConversation` without blocking HTTP response termination.

### 3.4 Context Injection & Prompt Engineering (`internal/agent/context.go`)
* Dynamic `[BỘ NHỚ]` block injected into System Prompt:
  ```markdown
  [BỘ NHỚ] — Các quy ước, sở thích và kinh nghiệm kỹ thuật đã học từ người dùng (ưu tiên tuân thủ khi đưa ra giải pháp):
  - backend_framework: Go + Fastify
  - css_style: Vanilla CSS
  - [Solution]: Tavily AI search used as primary provider with Bing fallback
  ```

---

## 4. Data Models & Schemas

### 4.1 Mongo Collection: `memories`
```go
type Memory struct {
    ID             bson.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
    Type           string        `bson:"type" json:"type"` // "tech_stack", "coding_preference", etc.
    Key            string        `bson:"key" json:"key"`
    Value          string        `bson:"value" json:"value"`
    Source         string        `bson:"source" json:"source"` // "autonomous_reflection"
    Confidence     float64       `bson:"confidence" json:"confidence"`
    Embedding      []float64     `bson:"embedding,omitempty" json:"embedding,omitempty"`
    ConversationID string        `bson:"conversationId,omitempty" json:"conversationId,omitempty"`
    CreatedAt      time.Time     `bson:"createdAt" json:"createdAt"`
    UpdatedAt      time.Time     `bson:"updatedAt" json:"updatedAt"`
}
```

### 4.2 Mongo Collection: `documents` (RAG Knowledge Items)
```go
type DocChunk struct {
    DocumentID string    `bson:"documentId" json:"documentId"` // "learned-<slug>"
    Source     string    `bson:"source" json:"source"`         // "learned-<slug>.md"
    Version    int       `bson:"version" json:"version"`
    ChunkIndex int       `bson:"chunkIndex" json:"chunkIndex"`
    Text       string    `bson:"text" json:"text"`
    Embedding  []float64 `bson:"embedding,omitempty" json:"embedding,omitempty"`
    CreatedAt  time.Time `bson:"createdAt" json:"createdAt"`
}
```

---

## 5. Security, Privacy & Performance

1. **Non-blocking Latency**: Zero impact on First Token Latency (TTFT) or streaming throughput because reflection runs after SSE stream finishes.
2. **Confidence Filtering**: Only facts with confidence score $\ge 0.7$ are persisted.
3. **Prompt Injection Defense**: Input validation in `guardrails.ValidateUserInput` and strict JSON-only reflection prompts prevent prompt injection leaks.
4. **Tenant Isolation**: Memory keys and documents respect tenant namespaces.
