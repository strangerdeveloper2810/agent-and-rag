# JARVIS API Reference

Tai lieu tham khao day du cho HTTP API cua JARVIS agent runtime (Go). API su dung SSE (Server-Sent Events) de stream ket qua real-time.

Full API reference for the JARVIS agent runtime HTTP API. All endpoints use JSON for requests and SSE for streaming responses.

---

## Base URL

```
http://localhost:3002
```

Port mac dinh la `3002`, co the thay doi qua bien moi truong `PORT`.

---

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/chat` | Gui tin nhan va nhan SSE stream phan hoi tu agent |
| `GET` | `/healthz` | Kiem tra liveness cua service |

---

## POST /chat

Send a user message and receive the agent's response as an SSE (Server-Sent Events) stream.

### Request

**Headers:**
```
Content-Type: application/json
```

**Body (JSON):**

```typescript
interface ChatRequest {
  conversationId?: string;  // (tuỳ chọn) ID của cuộc hội thoại — dùng để liên kết memory
  history?: ChatMessage[];  // (tuỳ chọn) lịch sử hội thoại trước đó
  userMessage: string;      // (bắt buộc) tin nhắn của người dùng
  maxSteps?: number;        // (tuỳ chọn) số bước tối đa cho agent loop (mặc định: 12)
}

interface ChatMessage {
  role: "user" | "assistant" | "system";
  content: string;
}
```

**Example Request:**

```bash
curl -X POST http://localhost:3002/chat \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "conversationId": "conv-001",
    "history": [
      {"role": "user", "content": "Xin chao"},
      {"role": "assistant", "content": "Chao ban! Toi co the giup gi cho ban?"}
    ],
    "userMessage": "Thoi tiet hom nay the nao?",
    "maxSteps": 8
  }'
```

### Response

**Headers:**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

**Body:** SSE stream — each line is `data: <JSON>\n\n`.

### SSE Event Types

Moi event la mot dong JSON trong SSE stream. Client phan biet event qua truong `type`.

Each event is a JSON object with a `type` field. The client renders based on `type`.

#### `step` — Node Transition

Phat ra khi engine chuyen sang mot node moi (recall, model, tools, extract, end...).

Emitted when the engine enters a new processing node.

```json
{"type": "step", "node": "recall"}
{"type": "step", "node": "model"}
{"type": "step", "node": "tools"}
{"type": "step", "node": "model"}
{"type": "step", "node": "extract"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"step"` | Event type |
| `node` | `string` | Node ID: `"recall"`, `"summarize"`, `"model"`, `"tools"`, `"extract"`, `"interrupt"`, `"end"` |

#### `text` — LLM Token

Phat ra moi khi LLM sinh ra mot token moi. Ghep cac token nay de co noi dung day du.

Emitted for each generated token from the LLM. Concatenate to build the full response.

```json
{"type": "text", "text": "Chao"}
{"type": "text", "text": " ban"}
{"type": "text", "text": "!"}
{"type": "text", "text": " Toi"}
{"type": "text", "text": " co"}
{"type": "text", "text": " the"}
{"type": "text", "text": " giup"}
{"type": "text", "text": " gi"}
{"type": "text", "text": "?"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"text"` | Event type |
| `text` | `string` | A single token (may be 1 char, 1 word, or a few chars depending on tokenizer) |

**UI rendering:** Accumulate all `text` events in order and display as streaming text. The final accumulated text is the assistant's complete response.

#### `tool_start` — Tool Execution Begins

Phat ra khi agent bat dau goi mot cong cu (tool).

Emitted when a tool invocation starts.

```json
{"type": "tool_start", "name": "web.search"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"tool_start"` | Event type |
| `name` | `string` | Tool name (e.g., `"web.search"`, `"file.read"`, `"echo"`) |

**UI rendering:** Show a "calling tool..." indicator or chip.

#### `tool_end` — Tool Execution Completes

Phat ra khi tool chay xong (thanh cong hoac that bai).

Emitted when a tool invocation finishes.

```json
{"type": "tool_end", "name": "web.search"}
{"type": "tool_end", "name": "file.read", "message": "file.read: access denied"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"tool_end"` | Event type |
| `name` | `string` | Tool name |
| `message` | `string` (optional) | Error detail if the tool failed; absent on success |

#### `citation` — RAG Sources

Phat ra khi agent su dung RAG de tim kiem tai lieu va muon dan nguon.

Emitted when the agent used document search and wants to cite sources.

```json
{"type": "citation", "text": "[{\"documentId\":\"abc123\",\"source\":\"chinh-sach-nghi-phep.pdf\",\"score\":0.92}]"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"citation"` | Event type |
| `text` | `string` | JSON string — array of `{documentId, source, score}` objects |

#### `memory` — Memory Operation

Phat ra khi he thong doc/ghi/xu ly bo nho.

Emitted during memory operations (recall, extract, summarize).

```json
{"type": "memory", "message": "recalled: pref:coffee_preference: den da khong duong | fact:user_name: Trinh"}
{"type": "memory", "message": "extracted: pref:thich ca phe = ca phe den"}
{"type": "memory", "message": "summarized: condensed 5 messages, keeping 15"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"memory"` | Event type |
| `message` | `string` | Human-readable memory operation detail |

#### `agent` — Agent Selected (Multi-Agent)

Phat ra khi orchestrator chon mot specialized agent de xu ly request.

Emitted when the orchestrator routes a request to a specific agent.

```json
{"type": "agent", "node": "code"}
{"type": "agent", "node": "general"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"agent"` | Event type |
| `node` | `string` | Agent name (`"general"`, `"code"`) |

#### `interrupt` — Human-in-the-Loop Pause

Phat ra khi engine dung de cho nguoi dung xac nhan mot hanh dong nguy hiem (vd: deleteTask).

Emitted when the engine pauses for user confirmation before executing a destructive action.

```json
{"type": "interrupt", "name": "deleteTask", "message": "guardrails: destructive tool \"deleteTask\" requires user confirmation"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"interrupt"` | Event type |
| `name` | `string` | Tool name that triggered HITL |
| `message` | `string` | Reason for interruption |

**Client behavior:** When you receive an `interrupt` event, the engine has stopped. You must show a confirmation dialog. The resume flow (POST /chat/resume) is planned for a future phase. Currently, the interrupt is terminal for that turn.

#### `error` — Recoverable Error

Phat ra khi co loi trong qua trinh xu ly (khong lam crash engine).

Emitted when a recoverable error occurs during processing.

```json
{"type": "error", "message": "model: generate: context deadline exceeded"}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"error"` | Event type |
| `message` | `string` | Error description |

#### `done` — Stream Complete

Phat ra KET THUC stream. Day la event CUOI CUNG. Theo sau event nay, SSE connection dong.

Emitted as the FINAL event in every stream. After this event, the SSE connection closes.

```json
{"type": "done", "usage": {"inputTokens": 450, "outputTokens": 120}}
```

**Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `type` | `"done"` | Event type |
| `usage` | `object` | Token usage for this turn |
| `usage.inputTokens` | `number` | Total input tokens consumed |
| `usage.outputTokens` | `number` | Total output tokens generated |

---

## GET /healthz

Kiem tra liveness — service co dang chay khong.

Liveness check — confirms the process is alive and accepting connections.

### Request

```bash
curl http://localhost:3002/healthz
```

### Response

```
HTTP/1.1 200 OK
Content-Type: application/json

{"status":"ok"}
```

**Status codes:**
| Code | Meaning |
|------|---------|
| `200` | Service is healthy and accepting traffic |

---

## Complete SSE Stream Example

Day la mot luong SSE day du cho request "Tim file .go trong thu muc hien tai":

Here is a complete SSE stream for a file search request:

```
data: {"type":"agent","node":"general"}

data: {"type":"step","node":"recall"}

data: {"type":"step","node":"summarize"}

data: {"type":"step","node":"model"}

data: {"type":"text","text":"De"}

data: {"type":"text","text":" tim"}

data: {"type":"text","text":" file"}

data: {"type":"text","text":" ."}

data: {"type":"text","text":"go"}

data: {"type":"text","text":","}

data: {"type":"text","text":" de"}

data: {"type":"text","text":" nghi"}

data: {"type":"text","text":" goi"}

data: {"type":"text","text":" file"}

data: {"type":"text","text":"."}

data: {"type":"text","text":"search"}

data: {"type":"step","node":"tools"}

data: {"type":"tool_start","name":"file.search"}

data: {"type":"tool_end","name":"file.search"}

data: {"type":"step","node":"model"}

data: {"type":"text","text":"Ket"}

data: {"type":"text","text":" qua"}

data: {"type":"text","text":" tim"}

data: {"type":"text","text":" thay"}

data: {"type":"text","text":" 12"}

data: {"type":"text","text":" file"}

data: {"type":"text","text":" ."}

data: {"type":"text","text":"go"}

data: {"type":"step","node":"extract"}

data: {"type":"done","usage":{"inputTokens":520,"outputTokens":85}}
```

---

## Client Implementation Notes

### JavaScript/TypeScript (browser)

Su dung `EventSource` API hoac `fetch` voi `ReadableStream`:

```typescript
// Cach 1: fetch + ReadableStream (khuyen dung — ho tro POST)
async function chat(userMessage: string, history: ChatMessage[] = []) {
  const response = await fetch('http://localhost:3002/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ userMessage, history }),
  });

  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const event: AgentEvent = JSON.parse(line.slice(6));
        handleEvent(event);
      }
    }
  }
}

function handleEvent(event: AgentEvent) {
  switch (event.type) {
    case 'text':
      // Append to displayed text
      appendText(event.text);
      break;
    case 'tool_start':
      // Show tool chip
      showToolChip(event.name, 'running');
      break;
    case 'tool_end':
      // Update tool chip
      updateToolChip(event.name, event.message ? 'error' : 'done');
      break;
    case 'done':
      console.log('Used tokens:', event.usage);
      break;
    case 'error':
      showError(event.message);
      break;
  }
}

// Cach 2: EventSource (chi GET — khong dung cho POST /chat)
// const es = new EventSource('/chat?message=...');
// es.onmessage = (e) => handleEvent(JSON.parse(e.data));
```

### Go (client)

```go
package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    body, _ := json.Marshal(map[string]any{
        "userMessage": "Xin chao",
        "maxSteps":    8,
    })

    resp, _ := http.Post("http://localhost:3002/chat", "application/json", bytes.NewReader(body))
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if len(line) > 6 && line[:6] == "data: " {
            fmt.Println(line[6:]) // JSON event
        }
    }
}
```

---

## Error Responses

Khi request khong hop le, server tra ve JSON error (KHONG phai SSE):

When the request is invalid, the server returns a plain JSON error (NOT SSE):

```json
{"error":"userMessage is required"}
```

```json
{"error":"invalid json: unexpected end of JSON input"}
```

**Status codes:**
| Code | Condition |
|------|-----------|
| `400` | Missing `userMessage` or invalid JSON body |
| `500` | SSE not supported by client (missing http.Flusher) |

---

## Limitations

1. **No authentication** — API is intended for local/internal use behind the Fastify gateway which handles auth
2. **No pagination** — SSE stream is unbounded; large responses may take time
3. **No resume (yet)** — HITL interrupt is terminal for now; `/chat/resume` endpoint planned for future phase
4. **Single-turn stateless** — history is sent in every request; no server-side conversation state between turns (by design, for horizontal scalability)
