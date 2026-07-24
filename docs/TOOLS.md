# JARVIS Tool Development Guide

Huong dan tao va dang ky cong cu (tool) moi cho JARVIS agent. Tool la cach agent tuong tac voi the gioi ben ngoai — tim kiem file, goi API, quan ly du lieu.

Guide to creating and registering new tools for the JARVIS agent. Tools are how the agent interacts with the outside world.

---

## Tool Interface

Moi tool trong JARVIS implements interface sau (defined in `internal/tools/tool.go`):

Every tool implements this interface:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() json.RawMessage   // JSON Schema cho args
    Kind() Kind
    Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
```

### Field-by-Field Explanation

| Method | Returns | Purpose |
|--------|---------|---------|
| `Name()` | `string` | Unique tool identifier (e.g., `"web.search"`, `"file.read"`). Convention: `namespace.action`. |
| `Description()` | `string` | Natural language description sent to the LLM. The LLM uses this to decide WHEN to call the tool. Be specific: "Tim kiem file theo pattern (glob) trong thu muc duoc phep." |
| `Schema()` | `json.RawMessage` | JSON Schema describing the `args` parameter. Sent to the LLM so it knows HOW to call the tool. Dung `additionalProperties: false` de LLM khong bia them field. |
| `Kind()` | `Kind` | Phan loai muc do nguy hiem: `KindRead` (an toan), `KindWrite` (co side-effect), `KindDestructive` (can xac nhan). See [Kind enum](#kind-enum) below. |
| `Execute(ctx, args)` | `(Result, error)` | Actual tool logic. Nhan `ctx` de ton trong cancel/timeout. Args la JSON da duoc validate. Tra ve `Result{Content: string}` de dua vao LLM context. |

### Kind Enum (Guardrail Implications)

```go
type Kind int

const (
    KindRead        Kind = iota // an toan: ragSearch, listDocuments, readDocument, listTasks, recallMemory
    KindWrite                   // tao/sua: createTask, updateTask, saveMemory
    KindDestructive             // pha huy: deleteTask -> can HITL xac nhan
)
```

Guardrail behavior (from `internal/guardrails/guard.go`):

| Kind | Auto-Execute? | Logged? | HITL Required? |
|------|:------------:|:-------:|:--------------:|
| `KindRead` | Yes | No | No — read-only operations are safe |
| `KindWrite` | Yes | Info log | No — mutating but not destructive |
| `KindDestructive` | **No** | Warning | **Yes** — engine emits `interrupt` event, pauses loop, waits for user confirmation |

When `KindDestructive` is detected, `guard.CheckTool()` returns `*NeedConfirmationError`. The engine catches this, emits an `interrupt` event, and stops the loop. The user must explicitly approve before the tool executes.

---

## How to Create a New Tool (Step by Step)

### Step 1: Define the tool struct

Create a new file in `services/agent-go/internal/tools/` (e.g., `calculator.go`).

```go
package tools

type calculatorTool struct {
    // Put any dependencies here (e.g., http.Client, database handle)
}

func NewCalculatorTool() Tool {
    return &calculatorTool{}
}
```

### Step 2: Implement the Tool interface

```go
func (t *calculatorTool) Name() string {
    return "calculator.eval"  // namespace.action convention
}

func (t *calculatorTool) Description() string {
    return "Tinh toan bieu thuc toan hoc (cong, tru, nhan, chia, luy thua). " +
           "Ho tro: +, -, *, /, ^, sqrt(). Vi du: '2 + 3 * 4'"
}

func (t *calculatorTool) Schema() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "expression": {
                "type": "string",
                "description": "Bieu thuc toan hoc can tinh, vi du: '2 + 3 * 4'"
            }
        },
        "required": ["expression"],
        "additionalProperties": false
    }`)
}

func (t *calculatorTool) Kind() Kind {
    return KindRead  // Calculator is read-only (no side effects)
}

func (t *calculatorTool) Execute(ctx context.Context, rawArgs json.RawMessage) (Result, error) {
    // 1. Parse args
    var args struct {
        Expression string `json:"expression"`
    }
    if err := json.Unmarshal(rawArgs, &args); err != nil {
        return Result{}, fmt.Errorf("calculator.eval: invalid args: %w", err)
    }
    if args.Expression == "" {
        return Result{}, fmt.Errorf("calculator.eval: expression is required")
    }

    // 2. Respect context (timeout/cancellation)
    select {
    case <-ctx.Done():
        return Result{}, ctx.Err()
    default:
    }

    // 3. Execute logic
    result, err := evaluateExpression(args.Expression)
    if err != nil {
        return Result{}, fmt.Errorf("calculator.eval: %w", err)
    }

    // 4. Return structured result
    out, _ := json.Marshal(map[string]any{
        "expression": args.Expression,
        "result":     result,
    })
    return Result{Content: string(out)}, nil
}
```

### Step 3: Register the tool

In `cmd/server/main.go` (or `cmd/jarvis/main.go`), add your tool to the registry:

```go
registry := tools.NewRegistry()
registry.Register(tools.NewEchoTool())
registry.Register(tools.NewFileSearchTool(allowedPaths))
registry.Register(tools.NewFileReadTool(allowedPaths))
registry.Register(tools.NewWebSearchTool(nil))
registry.Register(tools.NewWebFetchTool(nil))
registry.Register(tools.NewCalculatorTool())  // <-- NEW
```

That's it! The tool will automatically appear in `registry.ToolDefs()` and be sent to the LLM on every model call.

---

## JSON Schema Best Practices

The `Schema()` method returns a JSON Schema object that the LLM uses to understand tool parameters. Key guidelines:

### DO:
```go
// Tot: ro rang, day du description, required fields
func (t *myTool) Schema() json.RawMessage {
    return json.RawMessage(`{
        "type": "object",
        "properties": {
            "query": {
                "type": "string",
                "description": "search query string"
            }
        },
        "required": ["query"],
        "additionalProperties": false
    }`)
}
```

### DON'T:
```go
// Xau: khong co description, allow additional properties
func (t *myTool) Schema() json.RawMessage {
    return json.RawMessage(`{"type":"object"}`)
}
```

### Schema writing tips:
1. **Always set `additionalProperties: false`** — prevents the LLM from hallucinating extra fields
2. **Always set `required`** — tells the LLM which fields are mandatory
3. **Write descriptions in the language the agent uses** — if your agent speaks Vietnamese, write descriptions in Vietnamese
4. **Use `enum` for constrained values** — e.g., `"status": {"type": "string", "enum": ["todo", "doing", "done"]}`
5. **Use `format` hints** — e.g., `"format": "uri"` for URLs, `"format": "date"` for dates

---

## Result Format

Tool results are fed back to the LLM as text. Choose your format wisely:

```go
// Tot: structured JSON — LLM hieu ro cau truc
return Result{Content: `{"query":"xin chao","count":3,"results":[...]}`}

// OK: plain text — LLM can read but parsing is harder
return Result{Content: "Found 3 files: main.go, router.go, state.go"}

// Xau: too verbose, wastes tokens
return Result{Content: "I searched the filesystem and after careful analysis found..."}
```

The `Result.Meta` field is optional and can carry additional metadata (e.g., citations for RAG):

```go
return Result{
    Content: `{"documents": [...]}`,
    Meta:    json.RawMessage(`{"citations": [{"source": "doc.pdf", "score": 0.95}]}`),
}
```

---

## Parallel Execution

When the LLM calls multiple tools in one response, the registry executes them concurrently using `errgroup`:

```go
// internal/tools/registry.go
func (r *Registry) RunParallel(ctx context.Context, calls []provider.ToolCall) []CallResult {
    results := make([]CallResult, len(calls))  // pre-allocate = order preservation

    var g errgroup.Group
    for i, call := range calls {
        i, call := i, call  // pin loop variables
        results[i].Call = call
        g.Go(func() error {
            res, err := r.runOne(ctx, call)
            results[i].Result = res
            results[i].Err = err
            return nil  // tool errors don't cancel siblings
        })
    }
    g.Wait()
    return results
}
```

Key properties:
- **Order preservation**: results[i] always matches calls[i] (pre-allocated slice)
- **No shared state**: each goroutine writes to a unique index — data-race free by construction
- **Error isolation**: a failing tool does not cancel sibling tools
- **Context propagation**: `ctx` is passed to every `Execute` call — if the client disconnects, all tools stop

---

## Testing Tools

Tools are tested with table-driven Go tests. No mocking framework needed — the tool interface is the contract.

### Example: Echo Tool Test

```go
// internal/tools/echo_test.go
package tools

import (
    "context"
    "encoding/json"
    "testing"
)

func TestEchoTool(t *testing.T) {
    tool := NewEchoTool()

    // Verify interface compliance
    if tool.Name() != "echo" {
        t.Errorf("Name() = %q, want %q", tool.Name(), "echo")
    }
    if tool.Kind() != KindRead {
        t.Errorf("Kind() = %v, want KindRead", tool.Kind())
    }

    // Test execution
    ctx := context.Background()
    result, err := tool.Execute(ctx, json.RawMessage(`{"hello":"world"}`))
    if err != nil {
        t.Fatalf("Execute() error: %v", err)
    }
    if result.Content != `{"hello":"world"}` {
        t.Errorf("Content = %q, want %q", result.Content, `{"hello":"world"}`)
    }
}

func TestEchoTool_InvalidJSON(t *testing.T) {
    tool := NewEchoTool()
    ctx := context.Background()
    _, err := tool.Execute(ctx, json.RawMessage(`not json`))
    // Echo tool doesn't validate — but your tool should
    if err != nil {
        t.Logf("expected: error on invalid JSON: %v", err)
    }
}
```

### Example: File Search Tool Test (with safety check)

```go
func TestFileSearchTool_PathRestriction(t *testing.T) {
    tool := NewFileSearchTool([]string{"/tmp/allowed"})

    // Attempt to access disallowed path should fail
    ctx := context.Background()
    args, _ := json.Marshal(FileSearchArgs{Pattern: "*", Path: "/etc"})
    _, err := tool.Execute(ctx, args)
    if err == nil {
        t.Error("expected error for disallowed path, got nil")
    }
}

func TestFileSearchTool_NoPattern(t *testing.T) {
    tool := NewFileSearchTool([]string{"/tmp"})
    ctx := context.Background()
    args, _ := json.Marshal(FileSearchArgs{Pattern: ""})
    _, err := tool.Execute(ctx, args)
    if err == nil {
        t.Error("expected error for empty pattern, got nil")
    }
}
```

---

## Built-in Tools Reference

### `echo`
| Property | Value |
|----------|-------|
| **Kind** | `KindRead` |
| **Description** | Tra lai nguyen van args dau vao (dung de hoc/test luong tool). |
| **Schema** | `{"type":"object","additionalProperties":true}` — accepts any object |
| **File** | `internal/tools/echo.go` |

### `file.search`
| Property | Value |
|----------|-------|
| **Kind** | `KindRead` |
| **Description** | Tim file theo pattern (glob) trong thu muc duoc phep. Tra danh sach JSON duong dan khop. |
| **Schema** | `pattern: string (required), path: string (optional)` |
| **Safety** | Path restricted to `allowedPaths` whitelist |
| **File** | `internal/tools/files.go` |

### `file.read`
| Property | Value |
|----------|-------|
| **Kind** | `KindRead` |
| **Description** | Doc noi dung file text trong thu muc duoc phep. Tra ve text (cat bot neu qua dai). |
| **Schema** | `path: string (required)` |
| **Safety** | Path restricted to `allowedPaths` whitelist; max 24,000 chars |
| **File** | `internal/tools/files.go` |

### `web.search`
| Property | Value |
|----------|-------|
| **Kind** | `KindRead` |
| **Description** | Tim kiem web qua DuckDuckGo Instant Answer API. Tra ve title + abstract + URL. Mien phi, khong can API key. |
| **Schema** | `query: string (required)` |
| **Dependency** | DuckDuckGo API (no key needed) |
| **Timeout** | 10 seconds |
| **File** | `internal/tools/web.go` |

### `web.fetch`
| Property | Value |
|----------|-------|
| **Kind** | `KindRead` |
| **Description** | Tai noi dung text cua mot URL (gioi han 10000 ky tu). |
| **Schema** | `url: string (required, format: uri)` |
| **Timeout** | 10 seconds; max 1MB body read |
| **File** | `internal/tools/web.go` |

---

## Adding Tools to the Agent

Tools are registered once and available to all agents that share the registry:

```go
// cmd/server/main.go
func main() {
    // 1. Create registry
    registry := tools.NewRegistry()

    // 2. Register all tools
    registry.Register(tools.NewEchoTool())
    registry.Register(tools.NewFileSearchTool([]string{".", "/tmp"}))
    registry.Register(tools.NewFileReadTool([]string{".", "/tmp"}))
    registry.Register(tools.NewWebSearchTool(nil))   // nil = use default HTTP client
    registry.Register(tools.NewWebFetchTool(nil))

    // 3. Create engine with this registry
    engine := agent.NewEngine(provider, registry)

    // 4. Tool definitions are automatically sent to the LLM
    //    via registry.ToolDefs() in nodeModel()
}
```

The tool definitions (`[]provider.ToolDef`) are sent to the LLM on every model call as part of `GenerateRequest.Tools`. The LLM sees them as function declarations and decides when to call them.

---

## Safety Checklist for New Tools

Before registering a new tool in production, verify:

- [ ] `Name()` is unique across the registry
- [ ] `Kind()` is correctly assigned (Read/Write/Destructive)
- [ ] `Schema()` uses `additionalProperties: false`
- [ ] `Schema()` lists `required` fields
- [ ] `Execute()` validates all inputs (no panics)
- [ ] `Execute()` respects `ctx.Done()` for cancellation
- [ ] `Execute()` returns structured JSON in `Result.Content`
- [ ] Error messages are prefixed with `"toolname: "` for traceability
- [ ] Timeout is set (via `context.WithTimeout`) for any I/O
- [ ] Path/directory access is restricted (for file tools)
- [ ] Test file exists with both happy-path and error cases
