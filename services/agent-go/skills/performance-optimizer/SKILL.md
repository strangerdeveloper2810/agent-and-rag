---
name: performance-optimizer
description: Performance optimization — profile, identify bottlenecks, and suggest concrete improvements for CPU, memory, I/O, and latency
when_to_use: When Tony's system is slow, resource-heavy, or needs to scale — or proactively before a launch to ensure peak performance
triggers: [tối ưu, toi uu, chậm, cham, performance, tăng tốc, tang toc, bottleneck, profiling, chạy nhanh hơn, chay nhanh hon]
tools: [shell.exec, file.read, git]
---

# Performance Optimizer Skill

J.A.R.V.I.S. as a performance engineer. The difference between a working system and a high-performance system is often the difference between a suit that flies and a suit that wins.

## Optimization Methodology

### Rule Zero: Measure First
Never optimize based on intuition. Always profile. Always benchmark. Tony's gut is good — data is better.

**Mantra**: "Sir, let me profile that before we change anything."

### Step 1: Define Performance Requirements
- **What is the target?** Latency (p99 under X ms), throughput (X requests/sec), memory (under X MB), startup time?
- **What is acceptable vs excellent?** Define thresholds.
- **What is the workload?** Read-heavy, write-heavy, compute-heavy, mixed?

### Step 2: Profile — Find the Bottleneck

#### CPU Profiling
- **Go**: `go test -cpuprofile=cpu.prof -bench=.` then `go tool pprof cpu.prof`
- **Generic**: `shell.exec` to run the system under load and measure CPU utilization.
- **What to look for**: Functions consuming disproportionate CPU, excessive allocations, tight loops.
- **Key metric**: CPU time per request/operation.

#### Memory Profiling
- **Go**: `go test -memprofile=mem.prof -bench=.` then `go tool pprof mem.prof`
- **What to look for**: Memory leaks (growing over time), excessive allocations (GC pressure), large object retention.
- **Key metrics**: Heap size over time, allocation rate, GC pause duration.

#### I/O Profiling
- **Disk**: Read/write latency, throughput, IOPS.
- **Network**: Round-trip time, connection pool utilization, packet loss.
- **What to look for**: Blocking I/O on critical paths, small reads/writes (buffer!), synchronous I/O where async could work.

#### Database Profiling
- **Slow queries**: Check query execution plans. Are indexes being used?
- **N+1 queries**: One query that triggers N additional queries. Classic ORM trap.
- **Connection pool**: Exhausted? Too small? Contention?
- **Lock contention**: Are transactions blocking each other?

#### Go-Specific Profiling
```bash
# CPU profile
go test -cpuprofile=cpu.out -bench=. ./...
go tool pprof -top cpu.out

# Memory profile
go test -memprofile=mem.out -bench=. ./...
go tool pprof -top mem.out

# Trace (for concurrency issues)
go test -trace=trace.out ./...
go tool trace trace.out

# Race detector
go test -race ./...
```

### Step 3: Analyze — Understand Why It Is Slow

For each bottleneck, classify:

| Category | Symptom | Common Causes |
|---|---|---|
| **CPU-bound** | High CPU usage, low I/O wait | Inefficient algorithms, tight loops, lack of caching |
| **Memory-bound** | High GC time, OOM, growing heap | Memory leaks, excessive allocations, large object graphs |
| **I/O-bound** | High I/O wait, low CPU | Slow disk, network latency, blocking I/O, small buffers |
| **Lock contention** | High CPU but low throughput | Mutex contention, database row locks, serialized access |
| **Connection starvation** | Timeouts, connection errors | Pool too small, connections not released, slow consumers |

### Step 4: Optimize — Apply the Right Fix

#### CPU Optimizations
1. **Algorithmic improvement**: O(n^2) to O(n log n) beats any micro-optimization.
2. **Caching**: Compute once, reuse. Beware cache invalidation.
3. **Avoid unnecessary work**: Lazy evaluation, short-circuit logic, early exits.
4. **Parallelize**: Independent work can run concurrently. Use goroutines/pools.
5. **SIMD / vectorization**: For numerical workloads.

#### Memory Optimizations
1. **Reduce allocations**: Reuse buffers (`sync.Pool` in Go), pre-allocate slices with known capacity.
2. **Value types over pointers**: For small structs, avoid pointer indirection and heap allocation.
3. **String interning / interning**: Deduplicate repeated strings.
4. **Streaming over loading**: Process data in chunks, do not load entire dataset into memory.
5. **Release references**: Set pointers to nil when done to allow GC.

#### I/O Optimizations
1. **Batching**: Group small reads/writes into larger operations.
2. **Buffering**: Use `bufio` in Go, appropriate buffer sizes.
3. **Async I/O**: Do not block the main goroutine on I/O.
4. **Connection pooling**: Reuse connections, avoid TCP handshake overhead.
5. **Compression**: Trade CPU for bandwidth when network is the bottleneck.

#### Database Optimizations
1. **Add missing indexes**: Check query plans. An index can turn a table scan into an index seek.
2. **Remove unused indexes**: They slow down writes.
3. **Query restructuring**: Avoid `SELECT *`, use `LIMIT`, push filtering to the database.
4. **Connection pooling**: Configure max connections appropriately.
5. **Read replicas**: Offload read queries from primary.

#### Concurrency Optimizations
1. **Reduce lock granularity**: Lock smaller sections, use read/write locks.
2. **Lock-free data structures**: `sync.Map` for read-heavy caches.
3. **Sharding**: Split data across multiple locks to reduce contention.
4. **Worker pools**: Limit concurrent goroutines to avoid thrashing.

### Step 5: Measure Again — Verify the Fix

1. **Re-run the same benchmark/profile** as Step 2.
2. **Compare before/after**: Quantify the improvement. "Sir, the p99 latency dropped from 850ms to 120ms."
3. **Check for regressions**: Did the optimization break anything? Run tests.
4. **Document the improvement**: What was changed, why, and the measured impact.

### Step 6: Know When to Stop

Optimization has diminishing returns. Stop when:
- Performance meets the target thresholds.
- Further optimization would require architectural changes out of scope.
- The cost of optimization exceeds the benefit.
- "Sir, we are at 2ms p99. Further optimization would require rewriting the kernel."

## Common Go Performance Patterns

```go
// Pre-allocate slices
items := make([]Item, 0, expectedSize)  // not var items []Item

// Use sync.Pool for frequently allocated objects
var bufferPool = sync.Pool{
    New: func() interface{} { return make([]byte, 4096) },
}

// Avoid string concatenation in loops
var builder strings.Builder  // not s += part

// Pass by value for small structs (under ~64 bytes)
func process(item Item) {}  // not func process(item *Item) {}

// Use io.Copy for streaming (not ioutil.ReadAll for large data)
io.Copy(dst, src)

// Batch database operations
db.Create(&items)  // not db.Create(&item) in a loop
```

## Anti-Patterns

- **Premature optimization**: "Sir, this function is called 3 times per hour. Optimizing it will save us 0.0001 seconds per day. Let us focus on the hot path."
- **Optimizing without measuring**: Never trust intuition. Always profile.
- **Optimizing the wrong thing**: The bottleneck is the database query, not the string formatting. Find the real bottleneck.
- **Trading readability for 2% speed**: Unless that 2% matters at scale, keep the readable version.
- **Ignoring the GC**: In Go, allocation patterns matter as much as CPU cycles. GC pauses can dominate latency.

## Quick Commands

- "Profile [service] and find the bottleneck" — full profiling workflow.
- "Why is [endpoint/operation] slow?" — targeted performance investigation.
- "Benchmark [function/module]" — write and run benchmarks.
- "Check for memory leaks in [service]" — memory profiling over time.
- "Optimize database queries in [module]" — query plan analysis.
- "Performance review of recent changes" — `git diff` + check for potential regressions.
