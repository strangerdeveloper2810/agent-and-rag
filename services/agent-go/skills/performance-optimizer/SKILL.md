---
name: performance-optimizer
description: Performance optimization — profile, identify bottlenecks, and suggest concrete improvements for CPU, memory, I/O, and latency
when_to_use: When the user's system is slow, resource-heavy, or needs to scale — or proactively before a launch to ensure peak performance
triggers: [tối ưu, toi uu, chậm, cham, performance, tăng tốc, tang toc, bottleneck, profiling, chạy nhanh hơn, chay nhanh hon]
tools: [shell.exec, file.read, git]
---

# Performance Optimizer Skill

## Luật số 0: đo trước

Không tối ưu theo cảm giác. Profile trước, benchmark trước. Câu mở đầu đúng là
"để tôi profile đã, rồi hãy sửa".

## 1. Đặt mục tiêu

Latency (p99 dưới X ms)? throughput (X req/s)? memory (dưới X MB)? thời gian
khởi động? Định rõ mức **chấp nhận được** vs **tốt**. Workload là đọc nhiều, ghi
nhiều, hay tính toán nhiều?

## 2. Profile — tìm nút cổ chai

```bash
go test -cpuprofile=cpu.out -bench=. ./... && go tool pprof -top cpu.out
go test -memprofile=mem.out -bench=. ./... && go tool pprof -top mem.out
go test -trace=trace.out ./... && go tool trace trace.out   # vấn đề concurrency
go test -race ./...
```

- **CPU**: hàm nào ăn CPU bất thường, allocation nhiều, vòng lặp chặt. Chỉ số:
  CPU time mỗi request.
- **Memory**: heap tăng theo thời gian (leak), allocation rate cao (áp lực GC),
  giữ object lớn quá lâu. Chỉ số: heap size, allocation rate, GC pause.
- **I/O**: I/O chặn trên đường nóng, đọc/ghi vụn (thiếu buffer), I/O đồng bộ ở
  chỗ có thể async.
- **Database**: query chậm (đọc execution plan — index có được dùng?), N+1
  query, connection pool cạn, lock contention.

## 3. Phân loại nút cổ chai

| Loại | Dấu hiệu | Nguyên nhân thường gặp |
|---|---|---|
| CPU-bound | CPU cao, I/O wait thấp | thuật toán kém, vòng lặp chặt, thiếu cache |
| Memory-bound | GC time cao, OOM, heap phình | leak, allocation quá nhiều, object graph lớn |
| I/O-bound | I/O wait cao, CPU thấp | disk chậm, network latency, I/O chặn, buffer nhỏ |
| Lock contention | CPU cao mà throughput thấp | tranh mutex, lock hàng DB, truy cập bị tuần tự hoá |
| Cạn connection | timeout, lỗi kết nối | pool nhỏ, không trả connection, consumer chậm |

## 4. Sửa đúng loại

**CPU:** cải thiện thuật toán trước (O(n²) → O(n log n) thắng mọi micro-optimization)
· cache (cẩn thận invalidation) · bỏ việc không cần (lazy, short-circuit, early
exit) · song song hoá phần độc lập.

**Memory:** dùng lại buffer (`sync.Pool`) · pre-allocate slice khi biết capacity ·
value type cho struct nhỏ (dưới ~64 byte) thay vì pointer · xử lý theo chunk thay
vì nạp cả tập dữ liệu · giải phóng reference khi xong.

**I/O:** gộp thao tác nhỏ thành lô · buffer (`bufio`) · async, không chặn
goroutine chính · connection pooling · nén khi network là nút cổ chai (đổi CPU
lấy bandwidth).

**Database:** thêm index còn thiếu (biến table scan thành index seek) · bỏ index
không dùng vì nó làm chậm ghi · viết lại query (tránh `SELECT *`, có `LIMIT`, đẩy
filter xuống DB) · cấu hình pool · read replica cho tải đọc.

**Concurrency:** giảm phạm vi lock, dùng RWMutex · `sync.Map` cho cache đọc nhiều
· sharding để giảm tranh chấp · worker pool để giới hạn số goroutine.

## 5. Đo lại

Chạy lại đúng benchmark ở bước 2, so trước/sau bằng số cụ thể ("p99 từ 850ms
xuống 120ms"), chạy test để chắc không có regression, ghi lại đã đổi gì và tác
động đo được.

## 6. Biết lúc dừng

Dừng khi đã đạt ngưỡng mục tiêu, hoặc bước tiếp theo đòi đổi kiến trúc ngoài phạm
vi, hoặc chi phí tối ưu vượt lợi ích.

## Pattern Go hay dùng

```go
items := make([]Item, 0, expectedSize)   // không dùng var items []Item
var bufferPool = sync.Pool{New: func() any { return make([]byte, 4096) }}
var b strings.Builder                    // không dùng s += part trong vòng lặp
func process(item Item) {}               // struct nhỏ: truyền giá trị
io.Copy(dst, src)                        // stream, không ReadAll dữ liệu lớn
db.Create(&items)                        // ghi theo lô, không Create trong loop
```

## Anti-pattern

- **Tối ưu quá sớm**: hàm chạy 3 lần/giờ thì tối ưu nó chẳng cứu được gì — tìm
  đường nóng.
- **Tối ưu mà không đo**: không tin cảm giác.
- **Tối ưu sai chỗ**: nút cổ chai là query DB, không phải chỗ format string.
- **Đổi dễ đọc lấy 2% tốc độ**: chỉ đáng khi 2% đó có ý nghĩa ở quy mô thật.
- **Bỏ qua GC**: trong Go, kiểu allocation quan trọng ngang số nhịp CPU — GC pause
  có thể chi phối latency.
