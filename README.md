# go-chan-broadcast
[![Go Reference](https://pkg.go.dev/badge/github.com/BoskyWSMFN/go-chan-broadcast.svg)](https://pkg.go.dev/github.com/BoskyWSMFN/go-chan-broadcast)

# Go Broadcast Library

A high-performance broadcaster for Go that allows one goroutine to efficiently
broadcast messages to multiple subscribers.

## Features

- **Copy-on-write subscriber list**: active subscribers are stored in an
  immutable snapshot behind `atomic.Pointer`, so write paths never block on
  Subscribe/Dispatch.
- **Object pooling**: objects are reused to minimize GC pressure.
- **Multiple send strategies**: non-blocking, blocking, context-aware, and
  fire-and-forget.
- **Per-subscriber buffering** with configurable size.
- **Configurable per-write timeout** (`WithBlockTimeout`) for bounding how long
  a slow subscriber can stall a broadcast.
- **Generic**: works with any data type using Go generics.
- **Thread-safe**: all methods are safe for concurrent use.

## Installation

```bash
go get github.com/BoskyWSMFN/go-chan-broadcast
```

## Quick Start

### Basic usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/BoskyWSMFN/go-chan-broadcast/pkg/broadcast"
)

func main() {
	// Create a broadcaster with subscriber buffer size 10.
	bc := broadcast.New[int](10)

	sub1 := bc.Subscribe()
	defer sub1.Dispatch()

	sub2 := bc.Subscribe()
	defer sub2.Dispatch()

	go func() {
		for msg := range sub1.Read() {
			fmt.Printf("Subscriber 1 received: %d\n", msg)
		}
		fmt.Println("Subscriber 1 closed")
	}()

	go func() {
		for msg := range sub2.Read() {
			fmt.Printf("Subscriber 2 received: %d\n", msg)
		}
		fmt.Println("Subscriber 2 closed")
	}()

	// Non-blocking send: may skip subscribers whose buffer is full.
	bc.WriteNonBlock(1)

	// Blocking send: waits for every subscriber.
	bc.WriteBlock(2)

	// Context-aware send: recommended for guaranteed delivery with lifecycle control.
	bc.WriteCtx(context.Background(), 3)

	// Fire-and-forget send: returns immediately, delivers in the background.
	bc.DetachAndWrite(context.Background(), 4)

	time.Sleep(100 * time.Millisecond)
}
```

### Reading with a cancellation context

`range sub.Read()` stops only when `Dispatch` closes the channel. If the
reader needs to stop earlier — for example, on application shutdown — use
`select` between `Read()` and a `context.Done()` channel.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/BoskyWSMFN/go-chan-broadcast/pkg/broadcast"
)

func main() {
	bc := broadcast.New[int](10)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sub := bc.Subscribe()

	// Reader goroutine exits when either:
	//   - Dispatch closes the subscriber channel, or
	//   - ctx is canceled (application shutdown, timeout, etc.).
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer sub.Dispatch() // Always release the subscriber.

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Reader stopping: context canceled")
				return
			case msg, ok := <-sub.Read():
				if !ok {
					fmt.Println("Reader stopping: subscriber dispatched")
					return
				}
				fmt.Printf("Received: %d\n", msg)
			}
		}
	}()

	// Broadcast a few messages.
	bc.WriteNonBlock(1)
	bc.WriteNonBlock(2)
	bc.WriteNonBlock(3)

	// Wait for the reader to finish (either by context or by Dispatch below).
	<-done
}
```

Notes:

- `sub.Read()` returns `<-chan T`, so it can be used in `select` like any
  other channel.
- When the subscriber's channel is closed by `Dispatch`, the receive expression
  yields the zero value of `T` and `ok == false`. Always check `ok` before
  using the value.
- Always call `Dispatch()` from the reader goroutine (or ensure it runs after
  the reader exits). Calling `Dispatch` while another goroutine is blocked in
  `<-sub.Read()` is safe — the channel is closed under a lock, and the reader
  observes `ok == false`.

### Channel Publisher pattern

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/BoskyWSMFN/go-chan-broadcast/pkg/broadcast"
)

func main() {
	bc := broadcast.New[string](5)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	publisher := bc.PublisherCtx(ctx)
	defer close(publisher) // Always close the publisher channel.

	sub := bc.Subscribe()
	
	go func() {
		defer sub.Dispatch()
		
		messages := []string{"hello", "world", "broadcast"}
		for _, msg := range messages {
			select {
			case publisher <- msg:
				fmt.Printf("Published: %s\n", msg)
			case <-ctx.Done():
				return
			}
		}
	}()

	for msg := range sub.Read() {
		fmt.Printf("Received: %s\n", msg)
	}
}
```

### Channel Publisher pattern with cancellable reader

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/BoskyWSMFN/go-chan-broadcast/pkg/broadcast"
)

func main() {
	bc := broadcast.New[string](5)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	publisher := bc.PublisherCtx(ctx)
	defer close(publisher) // Always close the publisher channel.

	sub := bc.Subscribe()

	// Reader goroutine stops when either Dispatch closes the subscriber
	// channel, or ctx is canceled.
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer sub.Dispatch()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Reader stopping: context canceled")
				return
			case msg, ok := <-sub.Read():
				if !ok {
					fmt.Println("Reader stopping: subscriber dispatched")
					return
				}
				fmt.Printf("Received: %s\n", msg)
			}
		}
	}()

	// Publisher goroutine stops on context cancellation.
	go func() {
		messages := []string{"hello", "world", "broadcast"}
		for _, msg := range messages {
			select {
			case publisher <- msg:
				fmt.Printf("Published: %s\n", msg)
			case <-ctx.Done():
				fmt.Println("Publisher stopping: context canceled")
				return
			}
		}
	}()

	// Wait for the reader to finish (context cancellation or timeout).
	<-done
}
```

Notes:

- `PublisherCtx` runs a dedicated goroutine that reads from the returned
  channel and calls `WriteCtx` for each message. Closing the channel
  (`close(publisher)`) or canceling `ctx` terminates that goroutine.
- A slow subscriber blocks subsequent sends on `publisher`. The `select`
  on `ctx.Done()` lets the publisher goroutine exit instead of hanging
  forever.
- `defer sub.Dispatch()` runs in the reader goroutine, after the reader
  exits. This avoids `Dispatch` waiting on an in-flight `WriteCtx` while
  the reader is still consuming messages.

### Bounding slow subscribers

```go
bc := broadcast.New[int](16).
	WithBlockTimeout(500 * time.Millisecond)

sub := bc.Subscribe()
defer sub.Dispatch()

// WriteCtx/WriteBlock/DetachAndWrite will wait at most 500ms per subscriber.
// Subscribers that do not accept the value in time are skipped for this write.
bc.WriteCtx(context.Background(), 42)
```

## API Reference

### Broadcaster interface

```go
type Broadcaster[T any] interface {
	// WithBlockTimeout returns a copy of the broadcaster with the given
	// per-write timeout. Zero means no timeout.
	WithBlockTimeout(time.Duration) Broadcaster[T]

	// WithBufferSize returns a copy of the broadcaster with the given
	// subscriber channel buffer size. Applies to subscribers created by
	// the returned broadcaster via Subscribe, and to publisher channels.
	WithBufferSize(int) Broadcaster[T]

	// Publisher returns a send-only channel for convenient broadcasting.
	// Uses context.Background(). Close the channel to prevent goroutine leaks.
	Publisher() chan<- T

	// PublisherCtx returns a context-aware send-only channel.
	// Respects context cancellation and channel closure.
	PublisherCtx(ctx context.Context) chan<- T

	// Subscribe creates a new subscriber. Release it with Dispatch.
	Subscribe() Subscriber[T]

	// WriteNonBlock - non-blocking send (may skip slow subscribers).
	// Returns false if at least one subscriber did not accept the value.
	WriteNonBlock(T) bool

	// WriteBlock - blocking send with context.Background().
	// May block indefinitely if there are no active readers and no timeout.
	WriteBlock(T)

	// WriteCtx - context-aware blocking send (recommended).
	WriteCtx(context.Context, T)

	// DetachAndWrite - asynchronous send (fire-and-forget).
	DetachAndWrite(ctx context.Context, data T)
}
```

### Subscriber interface

```go
type Subscriber[T any] interface {
	// Read returns a channel for reading messages.
	// The channel is closed after Dispatch is called.
	Read() <-chan T

	// Dispatch unsubscribes the subscriber, closes its channel,
	// and returns internal resources to a pool for reuse.
	// Must be called exactly once per subscriber.
	Dispatch()

	// Len returns the number of elements currently buffered in the
	// subscriber's channel. Diagnostic only.
	Len() int

	// Cap returns the buffer capacity of the subscriber's channel.
	// Diagnostic only.
	Cap() int
}
```

## Delivery semantics

| Method             | Blocks? | Guaranteed delivery? | Respects ctx? | Respects timeout? |
|--------------------|---------|----------------------|---------------|-------------------|
| `WriteNonBlock`    | No      | No                   | No            | No                |
| `WriteBlock`       | Yes     | Yes                  | No            | Yes               |
| `WriteCtx`         | Yes     | Yes                  | Yes           | Yes               |
| `DetachAndWrite`   | No      | Background delivery  | Yes           | Yes               |

Notes:
- "Guaranteed delivery" means the call waits for every subscriber to accept
  the value, or for the timeout / context cancellation to fire.
- `DetachAndWrite` returns immediately.

## Performance Characteristics

Approximate costs on a modern x86-64 machine (AMD Ryzen 7 6800H, Go 1.27).
Actual numbers depend on workload, buffer sizes, and scheduler state.

| Operation                                  | Cost       | Allocations |
|--------------------------------------------|-----------:|------------:|
| `WriteNonBlock` (0 subscribers)            | ~3.5 ns    | 0           |
| `WriteNonBlock` (1 subscriber, buf 16)     | ~88 ns     | 0           |
| `WriteNonBlock` (100 subscribers, buf 16)  | ~57 µs     | 0           |
| `WriteCtx` (1 subscriber, buf 16)          | ~1.1 µs    | 4           |
| `WriteCtx` (100 subscribers, buf 16)       | ~83 µs     | 103         |
| `Subscribe` + `Dispatch`                   | ~400–520 ns| 7–8         |

Key observations:

- **`WriteNonBlock` is allocation-free and an order of magnitude cheaper**
  than `WriteCtx`. Prefer it when message loss is acceptable.
- **`WriteCtx` is dominated by goroutine creation** (~1 µs fixed cost per
  call plus ~800 ns per subscriber). The per-subscriber `RWMutex` is in the
  noise (≈5 ns).
- **`Subscribe`/`Dispatch` is the most expensive per-call operation** due to
  channel creation and pool churn. Keep subscribers long-lived when possible.
- **Buffer size affects latency**: large buffers (256+) hurt single-write
  latency due to cache misses; small buffers (16) are a good default.

Complexity:

- **Subscribe/Dispatch**: O(n) — copy-on-write snapshot is copied on every
  modification, then swapped in via CAS.
- **Broadcast**: O(n) — one goroutine per subscriber in `WriteCtx`/`WriteBlock`.
- **Memory**: O(n) — active subscriber list plus per-subscriber channels.

Where n = number of active subscribers.

## Concurrency model

- **Subscribe/Dispatch** mutate the subscriber list using copy-on-write with
  a CAS loop. Writers (`WriteNonBlock`/`WriteCtx`/...) load an immutable
  snapshot and iterate it without locks.
- **Each subscriber** owns an `RWMutex` that serializes channel close
  (`Dispatch`) against channel send (`write`/`writeNonBlock`). `Dispatch`
  closes the internal `done` channel **before** acquiring the lock, so a
  writer blocked in `select` can observe cancellation and release the lock,
  avoiding deadlock.
- **`Dispatch` may block** if a writer is currently blocked in `select` on
  that subscriber. With `WithBlockTimeout`, this wait is bounded.

## Use Cases

- **Event systems**: broadcasting events to multiple handlers.
- **Pub/Sub patterns**: one-to-many message distribution.
- **Service coordination**: notifying multiple goroutines of state changes.
- **Real-time updates**: distributing updates to connected clients.

## Best Practices

1. **Always call `Dispatch()` and close publishers.** Use `defer` for
   guaranteed cleanup.

```go
// ✅ Recommended — use defer for guaranteed cleanup.
go func() {
	defer sub.Dispatch()
	select {
	case msg, ok <- sub.Read():
		// process message
	case <-ctx.Done():
		return
	}
}

// ✅ Recommended for publishers.
publisher := bc.PublisherCtx(ctx)
defer close(publisher)

// ❌ Avoid — may leak resources if the goroutine exits unexpectedly.
go func() {
    select {
    case msg, ok <- sub.Read():
        // process message
        sub.Dispatch()
    case <-ctx.Done():
        return
    }
}()
```

2. **Choose the right send method.**

```go
// High-throughput, loss-tolerant.
bc.WriteNonBlock(data)

// Guaranteed delivery with a timeout.
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
bc.WriteCtx(ctx, data)

// Fire-and-forget.
bc.DetachAndWrite(context.Background(), data)

// Familiar channel interface.
publisher := bc.Publisher()
publisher <- data
```

3. **Pick buffer sizes to match throughput.** Small buffers (e.g. 16) keep
   single-write latency low; larger buffers help absorb bursts but cost more
   per write.

4. **Set `WithBlockTimeout` when subscribers may stall.** Without it, a single
   slow subscriber can block a broadcast indefinitely.

5. **Handle context cancellation** to release resources promptly.

## When to use this library vs alternatives

### Use this library when:
- You need one-to-many message broadcasting.
- You want different send strategies (blocking / non-blocking / context-aware).
- You need per-subscriber buffering.
- You prefer channel-based APIs via `Publisher()`.
- Performance under high concurrency is critical.

### Use `sync.Cond` when:
- You need thread synchronization with wait/signal semantics.
- You are building low-level concurrency primitives.
- A simple condition-variable pattern is sufficient.

### Use channels directly when:
- You have simple one-to-one communication.
- The overhead of a broadcaster is not justified.
- You are building simple producer–consumer patterns.

## Implementation notes

### Channel closing semantics

This library intentionally violates the Go principle "only senders should
close channels" for practical reasons. In a broadcast pattern, the broadcaster
(the sender) cannot know when all subscribers are done receiving. Therefore
subscribers control their own channel lifecycle via `Dispatch()`.

This is a necessary trade-off for the broadcast pattern to work effectively.
If you need strict sender-closes-channel semantics, use a plain channel per
consumer instead.

### Performance characteristics clarification

- **Subscribe/Dispatch**: O(n) — the subscriber list is copied in full on
  every mutation. The list is stored in `sliceStorage` objects that are
  recycled through a `sync.Pool`.
- **Broadcast**: O(n) — one goroutine per subscriber in `WriteCtx`/
  `WriteBlock`. `WriteNonBlock` is synchronous and allocation-free.
- **Memory**: O(n) — the active subscriber slice plus per-subscriber channels.

Where n = number of active subscribers. The copy-on-write approach with atomic
operations avoids locks but requires full slice copies on modifications.

## License

BSD 2-Clause License — see LICENSE file for details.