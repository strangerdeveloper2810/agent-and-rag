# Go Production Patterns — Từ Frontend (JS/TS) sang Go Backend

> **Audience:** Frontend engineers learning Go backend. Every concept mapped to JavaScript/TypeScript equivalent.
> **Context:** All examples from `services/agent-go` — a real production agent runtime, not a tutorial project.

---

## 0. Go vs TypeScript: Bảng Ánh Xạ Nhanh

| Concept | JavaScript/TypeScript | Go | Ghi chú |
|---|---|---|---|
| Biến | `let`/`const` | `var`/`:=` | Go `:=` = khai báo + gán (chỉ trong hàm) |
| Hàm | `const fn = (x) => x` | `func fn(x int) int { return x }` | Go: kiểu SAU tên biến |
| Export | `export const` / `export default` | Viết HOA chữ cái đầu: `func Do()`, `type State struct` | lowercase = private (package-scoped) |
| Null | `null` / `undefined` | `nil` | Chỉ pointer, interface, map, slice, chan, func mới nil được |
| Array | `const arr = [1,2,3]` | `arr := []int{1,2,3}` (slice) hoặc `[3]int{1,2,3}` (array cố định) | Slice = dynamic array; Array = fixed size |
| Object | `{name: "x", age: 1}` | `type Person struct { Name string; Age int }` | Go: phải define struct TRƯỚC |
| Map/Dict | `{key: value}` hoặc `Map<K,V>` | `map[string]int{"a": 1}` | Zero value của map là nil (không ghi được) |
| Callback | `fn((x) => ...)` | `type EmitFunc func(Event)` | Go: function type = first-class, nhưng không có arrow function |
| Class | `class X { ... }` | `type X struct { ... }` + methods | Go không có class, chỉ có struct + receiver methods |
| Interface | `interface X { fn(): void }` | `type X interface { Do() error }` | Go interface = IMPLICIT (không cần `implements`) |
| Promise | `await promise` | Channel hoặc callback | Go không có async/await; dùng goroutine + channel |
| Try/catch | `try { ... } catch(e) {}` | `if err != nil { return err }` | Go không có exception; error là return value thứ 2 |
| This/Self | `this.name` | Receiver: `func (s *State) LastAssistant()` | Go dùng receiver thay `this` |
| Generic | `Array<T>` / `type Foo<T>` | `type Registry[T any] struct { ... }` | Go 1.18+ mới có generics |
| Package | `import { X } from './x'` | `import "github.com/..." ` | Go import qua module path, không có relative import |
| Underscore | `_` (unused var warning lint) | `_` (blank identifier — bắt buộc cho biến không dùng) | Go: compile ERROR nếu khai báo biến mà không dùng |

---

## 1. Biến, Hàm, Package — Những Thứ Cơ Bản Nhất

### 1.1 Khai báo biến: `:=` vs `var`

```go
// ✅ := (short declaration) — dùng 90% thời gian, CHỈ trong hàm
name := "hello"
count := 0
state := &State{MaxSteps: 12}

// ✅ var — khi cần zero value hoặc khai báo ngoài hàm
var defaultPort string = "3002"
var (
    port    = "3002"
    timeout = 30 * time.Second
)

// ❌ := không dùng được ở package level
// ❌ name := "x" → "no new variables on left side of :=" (nếu name đã có)
```

### 1.2 Function: Kiểu SAU tên biến (ngược với TS!)

```go
// TypeScript:     function add(a: number, b: number): number
// Go:             func add(a int, b int) int

// Nhiều return value (pattern CỰC PHỔ BIẾN trong Go):
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, errors.New("không chia được cho 0")
    }
    return a / b, nil
}

// Named return (nên tránh trong hàm dài, dùng cho helper ngắn):
func newState(in RunInput) *State {
    // ...
    return &State{...}
}
```

### 1.3 Export: CHỮ HOA = Public, chữ thường = Private

```go
// TypeScript:
//   export const MAX_STEPS = 12       → dùng được từ file khác
//   const maxSteps = 12               → chỉ trong file này

// Go:
//   const MAX_STEPS = 12              → dùng được từ package khác (exported)
//   const maxSteps = 12               → chỉ trong package này (unexported)
//   func NewState() *State { ... }    → public constructor
//   func newState() *State { ... }    → private (chỉ test nội bộ dùng)
```

**Quy tắc:** Hầu hết mọi thứ là private (chữ thường). Chỉ export những gì package khác THỰC SỰ cần.

---

## 2. Struct & Receiver Methods — "Class" Trong Go

### 2.1 Struct = Object có type cố định

```go
// TypeScript:
//   type Person = { name: string; age: number }

// Go:
type Person struct {
    Name string  // HOA = export
    age  int     // thường = private
}

// Khởi tạo:
p1 := Person{Name: "An", age: 25}             // positional: phải nhớ thứ tự field
p2 := Person{Name: "Bình"}                     // named: age = zero value (0)
p3 := &Person{Name: "Chi"}                     // con trỏ (dùng khi muốn sửa sau)
```

### 2.2 Receiver Methods = Method Của "Class"

```go
// TypeScript:
//   class Person {
//     name: string
//     greet(): string { return `Hi ${this.name}` }
//   }

// Go:
type Person struct {
    Name string
}

// Value receiver (KHÔNG sửa được struct gốc) — như đọc property
func (p Person) Greet() string {
    return "Hi " + p.Name
}

// Pointer receiver (SỬA ĐƯỢC struct gốc) — đây mới là cái dùng nhiều
func (p *Person) SetName(name string) {
    p.Name = name  // p là con trỏ → sửa trực tiếp
}
```

**Khi nào dùng con trỏ (`*Person`) vs value (`Person`)?**

| Dùng `*Person` (pointer) khi | Dùng `Person` (value) khi |
|---|---|
| Cần SỬA struct | Chỉ ĐỌC, không sửa |
| Struct LỚN (>100 bytes) → tránh copy | Struct nhỏ (vài field) |
| Muốn thể hiện "có thể nil" | Không bao giờ nil |
| **90% thời gian dùng pointer** | Helper thuần túy, immutable |

### 2.3 Constructor Function (Thay Constructor Của Class)

```go
// TypeScript:  constructor(input: RunInput) { ... }
// Go:          function NewX(...) *X

func newState(in RunInput) *State {
    maxSteps := in.MaxSteps
    if maxSteps <= 0 {
        maxSteps = 12  // default
    }
    return &State{
        Messages: append(in.History, provider.Message{
            Role:    provider.RoleUser,
            Content: in.UserMessage,
        }),
        MaxSteps: maxSteps,
    }
}
```

---

## 3. Error Handling — KHÔNG CÓ Try/Catch

### 3.1 Pattern Chuẩn: Return Error Là Giá Trị Thứ 2

```go
// TypeScript:
//   try {
//     const result = await fetch(url)
//   } catch (err) {
//     console.error(err)
//   }

// Go:
result, err := doSomething()
if err != nil {
    return fmt.Errorf("làm X thất bại: %w", err)  // wrap + propagate
}
// dùng result tiếp...
```

### 3.2 `%w` vs `%v` — Cực Kỳ Quan Trọng

```go
// %w = wrap lỗi (giữ nguyên chain để errors.Is/errors.As hoạt động)
return fmt.Errorf("gemini: %w", err)    // ✅ DÙNG CÁI NÀY

// %v = chỉ copy text (mẤT chain)
return fmt.Errorf("gemini: %v", err)    // ❌ TRÁNH cái này
```

### 3.3 Sentinel Errors + Custom Types

```go
// Sentinel error — so sánh bằng ==
var ErrNotFound = errors.New("not found")
if errors.Is(err, ErrNotFound) { ... }

// Custom error type — dùng errors.As để unpack
type NotFoundError struct {
    Name string
}
func (e *NotFoundError) Error() string {
    return "tools: tool not found: " + e.Name
}

var nf *NotFoundError
if errors.As(err, &nf) {
    fmt.Println("missing tool:", nf.Name)
}
```

---

## 4. Concurrency — Thứ Làm Go Đặc Biệt

### 4.1 Goroutine = "Siêu Nhẹ Thread"

```go
// TypeScript:
//   await Promise.all([task1(), task2(), task3()])

// Go:
var g errgroup.Group
g.Go(func() error { return task1() })
g.Go(func() error { return task2() })
g.Go(func() error { return task3() })
if err := g.Wait(); err != nil {
    // ít nhất 1 task lỗi
}
```

**So sánh với JS:**

| JS/TS | Go |
|---|---|
| `Promise` | Goroutine |
| `await` | Không có — goroutine chạy độc lập |
| `Promise.all` | `errgroup` hoặc `sync.WaitGroup` |
| Single-threaded (event loop) | Multi-threaded (GMP scheduler) |
| 1 request = 1 async context | 1 request = 1 goroutine (2KB stack!) |

### 4.2 Channel = Pipe Giữa Các Goroutine

```go
// Channel như cái ống: 1 bên gửi, 1 bên nhận
ch := make(chan string)  // unbuffered: gửi BLOCK đến khi có người nhận

// Gửi (producer):
go func() {
    ch <- "hello"  // gửi vào ống
    ch <- "world"
    close(ch)      // đóng ống → người nhận biết đã hết
}()

// Nhận (consumer):
for msg := range ch {  // tự động dừng khi channel đóng
    fmt.Println(msg)
}
```

**Tưởng tượng như JS:** Channel giống như `ReadableStream` hoặc `AsyncIterator` — nhưng synchronized và type-safe.

### 4.3 Pattern Phổ Biến: select Với Context

```go
// Đây là pattern XUẤT HIỆN KHẮP NƠI trong code agent-go:
select {
case ch <- chunk:      // gửi thành công
case <-ctx.Done():     // người dùng đã cancel (đóng tab, mất mạng...)
    return             // dừng ngay, không leak goroutine
}
```

**Tại sao quan trọng:** Nếu không check `ctx.Done()`, goroutine cứ chạy mãi dù client đã ngắt kết nối → leak memory.

### 4.4 errgroup — Promise.all Của Go

```go
// Đây là code THẬT từ tools/registry.go:
func (r *Registry) RunParallel(ctx context.Context, calls []provider.ToolCall) []CallResult {
    results := make([]CallResult, len(calls))  // pre-allocate = giữ thứ tự

    var g errgroup.Group
    for i, call := range calls {
        i, call := i, call  // PIN BIẾN — quan trọng!
        results[i].Call = call
        g.Go(func() error {
            res, err := r.runOne(ctx, call)
            results[i].Result = res  // mỗi goroutine ghi index RIÊNG → không data race
            results[i].Err = err
            return nil  // lỗi tool KHÔNG làm hỏng tool khác
        })
    }
    g.Wait()
    return results
}
```

**Giải thích từng dòng cho FE dev:**

| Dòng | Ý nghĩa | JS equivalent |
|---|---|---|
| `results := make([]CallResult, len(calls))` | Tạo array đúng size TRƯỚC | `new Array(calls.length)` |
| `i, call := i, call` | Copy biến vòng lặp (closure trap!) | Giống `for (let i=0;...)` nhưng Go cần pin thủ công trước 1.22 |
| `results[i].Result = res` | Ghi vào index RIÊNG → thread-safe | Mỗi promise resolve ghi vào vị trí riêng |
| `return nil` | Lỗi tool KHÔNG cancel tool khác | Khác `Promise.all` (fail-fast), giống `Promise.allSettled` |

---

## 5. Interface — Implicit, Không Cần `implements`

### 5.1 So Sánh Với TypeScript

```typescript
// TypeScript: interface ĐƯỢC KHAI BÁO TRƯỚC, class IMPLEMENTS nó
interface Provider {
    generate(ctx: Context, req: GenerateRequest): AsyncIterable<StreamChunk>
    name(): string
}

class GeminiProvider implements Provider { ... }  // PHẢI viết implements
```

```go
// Go: interface ĐỊNH NGHĨA Ở NƠI DÙNG, struct tự động thoả mãn
type Provider interface {
    Generate(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    Name() string
}

// GeminiClient KHÔNG CẦN viết "implements Provider"
type GeminiClient struct { ... }
func (c *GeminiClient) Generate(...) (<-chan StreamChunk, error) { ... }
func (c *GeminiClient) Name() string { return "gemini" }

// Compile-time check (optional, nên có):
var _ Provider = (*GeminiClient)(nil)  // nếu GeminiClient thiếu method → compile error
```

### 5.2 Quy Tắc Vàng: Interface Ở Call Site

```go
// ✅ Interface trong package DÙNG nó (agent package cần provider)
// File: internal/agent/engine.go
type modelEngine interface {
    getProvider() provider.Provider
    getRegistry() *tools.Registry
}

// ❌ KHÔNG define interface trong package IMPLEMENT nó
// Không nên: internal/provider/provider.go đã có Provider interface
// Nhưng internal/agent có modelEngine interface RIÊNG (nhỏ hơn, đúng nhu cầu)
```

---

## 6. Pointer & Memory — Thứ FE Dev Hay Vấp

### 6.1 Con Trỏ (`*`) và Địa Chỉ (`&`)

```go
// Giống JS: mọi object là reference (trừ primitive)
// Go: phân biệt RÕ value vs pointer

x := 42       // x là int (value)
p := &x       // p là *int (pointer to x)
*p = 100      // sửa x thông qua pointer → x giờ là 100

// Với struct:
state := State{Step: 0}
s1 := state    // COPY toàn bộ struct
s2 := &state   // pointer — không copy

// Trong thực tế: LUÔN dùng pointer cho struct
func process(s *State) { ... }  // ✅
func process(s State) { ... }   // ❌ copy mỗi lần gọi, không sửa được gốc
```

### 6.2 Slice = "View" Của Array Bên Dưới

```go
// Slice giống array trong JS: dynamic, có thể grow
msgs := []Message{}                         // empty slice
msgs = append(msgs, Message{Role: "user"})  // append = push

// Pre-allocate để tránh realloc (performance):
msgs := make([]Message, 0, 100)  // capacity=100, length=0

// Slice LÀ reference type — truyền vào hàm không copy data:
func addMessage(msgs []Message, m Message) []Message {
    return append(msgs, m)  // append CÓ THỂ tạo backing array mới nếu vượt capacity
}
```

### 6.3 Map Phải Khởi Tạo Mới Ghi Được

```go
// ❌ PANIC: nil map
var m map[string]int
m["key"] = 1  // panic: assignment to entry in nil map

// ✅ Khởi tạo trước
m := make(map[string]int)
// hoặc
m := map[string]int{}
m["key"] = 1  // ok
```

### 6.4 defer = "Cleanup Khi Hàm Thoát"

```go
// Giống try...finally hoặc useEffect cleanup trong React
func readFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()  // CHẮC CHẮN chạy khi hàm return (dù lỗi hay không)

    return io.ReadAll(f)
}
```

---

## 7. Testing — Không Cần Jest

### 7.1 Table-Driven Test (Pattern Chuẩn Go)

```go
// Thay vì viết 5 test riêng → gom vào 1 table
func TestRoute(t *testing.T) {
    tests := []struct {
        name string
        s    *State
        want NodeID
    }{
        {"tool calls → tools", &State{...}, NodeTools},
        {"final answer → end", &State{...}, NodeEnd},
        {"maxSteps → end",    &State{Step: 12, MaxSteps: 12}, NodeEnd},
        {"interrupt → halt",  &State{Interrupt: &Interrupt{...}}, NodeInterrupt},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := route(tt.s); got != tt.want {
                t.Errorf("route() = %q, want %q", got, tt.want)
            }
        })
    }
}
```

### 7.2 So Sánh Testing Go vs Jest

| Jest (JS/TS) | Go `testing` |
|---|---|
| `test('name', () => {})` | `func TestXxx(t *testing.T) {}` |
| `expect(x).toBe(y)` | `if got != want { t.Errorf(...) }` |
| `describe` / `it` | `t.Run(name, func(t *testing.T) {})` |
| `beforeEach` / `afterEach` | `setup()` / `defer teardown()` |
| `jest.mock('./module')` | Fake struct (không cần mock framework) |
| `test.only` | `go test -run TestName` |
| Coverage | `go test -cover` |

---

## 8. Go Module & Dependency — Không Có `node_modules`

### 8.1 go.mod = package.json

```go
module github.com/ai-agent-tut/agent-go  // tên module

go 1.24  // phiên bản Go tối thiểu

require (
    go.mongodb.org/mongo-driver/v2 v2.2.0
    google.golang.org/genai v0.6.0
    golang.org/x/sync v0.17.0
)
```

### 8.2 Import Path = Module Path + Package Path

```go
import (
    "github.com/ai-agent-tut/agent-go/internal/provider"         // cùng module
    "go.mongodb.org/mongo-driver/v2/bson"                         // external
)
```

Không có `../` relative import! Mọi thứ từ module root.

---

## 9. "Go Way" — Triết Lý Code Go

### 9.1 Những Điều Làm Code Go "Có Mùi" (Idiomatic)

| Làm thế này | Đừng làm thế này |
|---|---|
| `if err != nil { return err }` explicit | `try { ... } catch` |
| Interface NHỎ (1-3 methods) | Interface lớn (10+ methods) |
| Package phẳng, ít tầng | Deep directory nesting |
| `context.Context` là param ĐẦU TIÊN | Context để ở giữa/cuối |
| Error là giá trị | Error là exception |
| Concurrent qua channel | Shared memory + mutex (chỉ khi cần) |
| `gofmt` tự động format | Tranh luận về style |

### 9.2 Một File Go "Chuẩn"

```go
// 1. Package declaration
package agent

// 2. Imports (grouped: stdlib trước, external sau, internal cuối)
import (
    "context"
    "fmt"

    "github.com/ai-agent-tut/agent-go/internal/provider"
)

// 3. Types (exported trước)
type NodeID string

// 4. Constants
const (
    NodeModel  NodeID = "model"
    NodeEnd    NodeID = "end"
)

// 5. Functions (exported trước, unexported sau)
func route(s *State) NodeID { ... }        // public
func countUnanswered(s *State) int { ... }  // private

// 6. Methods (trên struct tương ứng)
func (s *State) LastAssistant() *provider.Message { ... }
```

---

## 10. Các Lỗi FE Dev Hay Mắc Khi Viết Go

### 🔴 #1: Quên check error

```go
// ❌ Bỏ qua error
data, _ := json.Marshal(obj)

// ✅ Check error (LUÔN LUÔN)
data, err := json.Marshal(obj)
if err != nil {
    return fmt.Errorf("marshal: %w", err)
}
```

### 🔴 #2: Closure bắt nhầm biến vòng lặp (Go < 1.22)

```go
// ❌ Tất cả goroutine dùng CÙNG 1 biến i, call
for i, call := range calls {
    g.Go(func() error {
        process(i, call)  // i và call = giá trị CUỐI CÙNG của vòng lặp
        return nil
    })
}

// ✅ Pin biến (Go < 1.22) hoặc dùng Go 1.22+
for i, call := range calls {
    i, call := i, call  // tạo bản copy
    g.Go(func() error {
        process(i, call)  // OK: mỗi goroutine có bản copy riêng
        return nil
    })
}
```

### 🔴 #3: Nil pointer dereference

```go
var s *State       // s = nil
s.Step = 1         // PANIC: nil pointer dereference

// ✅ Luôn khởi tạo
s := &State{MaxSteps: 12}
// hoặc
s := new(State)
```

### 🔴 #4: Gửi vào channel đã đóng

```go
close(ch)
ch <- "data"  // PANIC: send on closed channel

// ✅ Chỉ producer đóng channel; consumer chỉ đọc
```

### 🔴 #5: Dùng `+=` trên string trong loop

```go
// ❌ Mỗi lần += tạo string mới → O(n²)
result := ""
for _, chunk := range chunks {
    result += chunk.Text
}

// ✅ Dùng strings.Builder
var b strings.Builder
for _, chunk := range chunks {
    b.WriteString(chunk.Text)
}
result := b.String()
```

---

## 11. Cách Debug Go (Không Có Console.log)

```go
// "console.log" của Go:
fmt.Println("debug:", value)       // stdout
fmt.Printf("state: %+v\n", state)  // in struct có field name
log.Printf("error: %v", err)       // có timestamp

// slog (structured logging) — dùng trong production:
slog.Info("agent started", "step", s.Step, "provider", prov.Name())
slog.Error("model failed", "err", err)

// Test: dùng t.Log (chỉ hiện khi -v hoặc test fail)
t.Logf("last = %+v", last)
t.Logf("unanswered = %d", unanswered)
```

---

## 12. Lộ Trình Học Go Cho FE Dev

| Giai đoạn | Nội dung | Thời gian dự kiến |
|---|---|---|
| **Tuần 1-2** | Syntax cơ bản + error handling + struct/methods | Làm tour.golang.org |
| **Tuần 3-4** | Interface + testing + package organization | Đọc code agent-go P0-P1 |
| **Tuần 5-6** | Goroutine + channel + context | Đọc code agent-go P2 |
| **Tuần 7-8** | errgroup + concurrency patterns + race detector | Đọc code agent-go P2.4-P2.5 |
| **Tuần 9+** | MongoDB driver + HTTP server + production patterns | Đọc code agent-go P4+ |

**Nguyên tắc:** Học Go BẰNG CÁCH ĐỌC CODE THẬT của agent-go, không phải tutorial. Mỗi task là 1 bài học.

---

## 13. Tài Liệu Tham Khảo

| Resource | Link | Dùng khi |
|---|---|---|
| Go Tour | tour.golang.org | Mới bắt đầu |
| Effective Go | go.dev/doc/effective_go | Đã viết được cơ bản |
| Go by Example | gobyexample.com | Tra cứu nhanh |
| Go Proverbs | go-proverbs.github.io | Hiểu triết lý |
| 100 Go Mistakes | 100go.co | Tránh lỗi phổ biến |
