# go-chan-broadcast

# Go Broadcast Library

A high-performance, lock-free broadcaster implementation for Go that allows one goroutine to efficiently broadcast messages to multiple subscribers.

## Features

- **Lock-free Design**: Uses atomic operations and CAS loops instead of heavy mutexes
- **Object Pooling**: Minimizes GC pressure through efficient resource reuse
- **Multiple Send Strategies**: Non-blocking, blocking, and context-aware sends
- **Memory Safe**: Automatic channel management and leak prevention
- **Generic**: Works with any data type using Go generics
- **Thread-Safe**: All methods are safe for concurrent use

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
    // Create a broadcaster with buffer size 10
    bc := broadcast.New[int](10)
    
    // Create subscribers
    sub1 := bc.Subscribe()
    sub2 := bc.Subscribe()
    
    // Start readers
    go func() {
        // Cleanup sub1 when this goroutine exits
        defer sub1.Dispatch()
		
        for msg := range sub1.Read() {
            fmt.Printf("Subscriber 1 received: %d\n", msg)
        }
        fmt.Println("Subscriber 1 closed")
    }()
    
    go func() {
        // Cleanup sub2 when this goroutine exits
        defer sub2.Dispatch()
		
        for msg := range sub2.Read() {
            fmt.Printf("Subscriber 2 received: %d\n", msg)
        }
        fmt.Println("Subscriber 2 closed")
    }()
    
    // Broadcast messages
    bc.WriteNonBlock(1)    // Non-blocking send
    bc.WriteBlock(2)       // Blocking send
    bc.WriteCtx(context.Background(), 3) // Context-aware send
    
    // Asynchronous send
    bc.DetachAndWrite(context.Background(), 4)
    
    time.Sleep(100 * time.Millisecond)
}
```
### Channel Publisher Pattern
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
    
    // Create publisher with context for graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    publisher := bc.PublisherCtx(ctx)
    defer close(publisher) // Always close the publisher channel
    
    // Use familiar channel patterns
    go func() {
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
    
    // Create subscriber
    sub := bc.Subscribe()
    go func() {
        defer sub.Dispatch()
        for msg := range sub.Read() {
            fmt.Printf("Received: %s\n", msg)
        }
    }()
    
    time.Sleep(100 * time.Millisecond)

    sub.Dispatch()
}
```

## API Reference

### Broadcaster Interface

```go
type Broadcaster[T any] interface {
    // Publisher returns a send-only channel for convenient broadcasting
    // Uses context.Background(). Close the channel to prevent goroutine leaks.
    Publisher() chan<- T

    // PublisherCtx returns a context-aware send-only channel  
    // Respects context cancellation and channel closure.
    PublisherCtx(ctx context.Context) chan<- T

    // Subscribe creates a new subscriber
    Subscribe() Subscriber[T]
    
    // WriteNonBlock - non-blocking send (may drop messages)
    WriteNonBlock(T)
    
    // WriteBlock - blocking send (may block indefinitely)
    WriteBlock(T)
    
    // WriteCtx - context-aware blocking send (recommended)
    WriteCtx(context.Context, T)
    
    // DetachAndWrite - asynchronous send (fire-and-forget)
    DetachAndWrite(ctx context.Context, data T)
}
```

### Subscriber Interface

```go
type Subscriber[T any] interface {
    // Read returns the receive channel for reading broadcast messages
    // Channel closes when Dispatch() is called
    Read() <-chan T

    // Dispatch unsubscribes, closes the channel, and returns resources to pool 
    //Idempotent - safe to call multiple times
    Dispatch()
}
```

## Performance Characteristics

- **Subscribe/Unsubscribe**: O(n) due to slice copying, but uses object pooling
- **Broadcast**: O(n) with concurrent sends to all subscribers
- **Memory**: Efficient object reuse minimizes allocations
- **Concurrency**: No lock contention during broadcasts

## Use Cases

- **Event Systems**: Broadcasting events to multiple handlers
- **Pub/Sub Patterns**: One-to-many message distribution
- **Service Coordination**: Notifying multiple goroutines of state changes
- **Real-time Updates**: Distributing updates to connected clients

## Best Practices

1. **Always call Dispatch()**, **close Publishers** and **use Defer for Cleanup** when done to prevent leaks
```go
// ✅ Recommended - use defer for guaranteed cleanup
go func() {
    defer sub.Dispatch() // Cleanup even if goroutine panics
    for msg := range sub.Read() {
    // process message
    }
}()

// ✅ Recommended for publishers  
publisher := bc.PublisherCtx(ctx)
defer close(publisher) // Prevent goroutine leaks

// ❌ Avoid - may leak resources
go func() {
    for msg := range sub.Read() {
    // process message
    }
    sub.Dispatch() // Might not execute if goroutine exits unexpectedly
}()
```
2. **Choose the Right Send Method**
```go
// For high-throughput, loss-tolerant scenarios
bc.WriteNonBlock(data)

// For guaranteed delivery with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
bc.WriteCtx(ctx, data)

// For fire-and-forget scenarios
bc.DetachAndWrite(context.Background(), data)

// Familiar channel interface for existing codebases
publisher := bc.Publisher()
publisher <- data
```
3. **Choose appropriate buffer sizes** based on your throughput requirements
4. **Handle context cancellation** for proper resource cleanup

## When to use this library vs alternatives

### Use this library when:
- You need one-to-many message broadcasting
- You want different send strategies (blocking/non-blocking)
- You need per-subscriber buffering
- You prefer channel-based APIs with Publisher()
- Performance under high concurrency is critical

### Use sync.Cond when:
- You need thread synchronization with wait/signal semantics
- You're building low-level concurrency primitives
- Simple condition variable pattern is sufficient

### Use channels directly when:
- You have simple one-to-one communication
- The overhead of broadcaster isn't justified
- You're building simple producer-consumer patterns

## Implementation notes

### Channel Closing Semantics Note:
This library intentionally violates the Go principle "only senders should close channels" for practical reasons.
In the broadcaster pattern, the broadcaster (sender) cannot know when all subscribers are done receiving.
Therefore, subscribers control their own channel lifecycle via `Dispatch()`.
This is a necessary trade-off for the broadcast pattern to work effectively.

### Performance Characteristics Clarification:
- **Subscribe/Unsubscribe**: O(n) - due to copying the entire slice of subscribers during CAS operations
- **Broadcast**: O(n) - must send to n subscribers concurrently
- **Memory**: O(n) - stores n channels in the slice

Where n = number of active subscribers.
The copy-on-write approach with atomic operations avoids locks but requires full slice copies on modifications.

## License

BSD 2-Clause License - see LICENSE file for details.
