package broadcast

import (
	"context"
	"sync"
	"sync/atomic"
)

type (
	// Broadcaster allows one goroutine to write same T value to multiple goroutines concurrently.
	// All methods are thread-safe.
	Broadcaster[T any] interface {
		// Publisher returns a send-only channel that converts channel sends into broadcasts.
		// Uses context.Background() internally. For context-aware publishing, use PublisherCtx.
		// The returned channel can be used in standard Go patterns (range loops, select statements).
		//
		// Example:
		//
		//	publisher := bc.Publisher()
		//	defer close(publisher)
		//
		//	publisher <- data1  // Will be broadcast to all subscribers
		//	publisher <- data2  // Will be broadcast to all subscribers
		//
		// WARNING: The goroutine handling the channel will leak if the channel is never closed.
		// Caller is responsible for closing the channel.
		Publisher() chan<- T
		// PublisherCtx returns a send-only channel that converts channel sends into context-aware broadcasts.
		// The publisher respects context cancellation - if the context is canceled, the publisher
		// stops processing messages and the goroutine exits.
		//
		// Example with context:
		//
		//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		//	defer cancel()
		//
		//	publisher := bc.PublisherCtx(ctx)
		//	defer close(publisher)
		//
		//	publisher <- data  // Broadcast with timeout protection
		//
		// Example with graceful shutdown:
		//
		//	publisher := bc.PublisherCtx(ctx)
		//	defer close(publisher)
		//
		//	for i := 0; i < 10; i++ {
		//	    select {
		//	    case publisher <- i:
		//	        // successfully broadcast
		//	    case <-ctx.Done():
		//	        return // context canceled, exit gracefully
		//	    }
		//	}
		//
		// The caller MUST close the returned channel to prevent goroutine leaks, OR ensure
		// the context eventually gets canceled.
		PublisherCtx(ctx context.Context) chan<- T
		// Subscribe creates a new subscriber and adds its channel to the broadcaster's active channels list.
		Subscribe() Subscriber[T]
		// DetachAndWrite asynchronously writes data (equivalent to "go WriteCtx()").
		// WARNING: possible resource leak when using context.Background() or other non-cancellable contexts.
		DetachAndWrite(context.Context, T)
		// WriteNonBlock uses non-blocking send (select-default).
		// Data may be missed by some subscribers when their channels are full.
		// Use WriteCtx or WriteBlock for guaranteed delivery.
		WriteNonBlock(T)
		// WriteBlock uses blocking send with context.Background().
		// May block indefinitely if there are no active readers.
		WriteBlock(T)
		// WriteCtx uses context-aware send with cancellation support.
		// Recommended method for guaranteed delivery with lifecycle control.
		WriteCtx(context.Context, T)
	}

	// subscribersManagement implements the core broadcaster logic using atomic operations
	// and object pools to minimize lock contention and GC pressure.
	subscribersManagement[T any] struct {
		channels            *atomic.Pointer[sliceStorage[T]]
		sliceStoragesPool   *sync.Pool
		subscribersPool     *sync.Pool
		waitGroupsPool      *sync.Pool
		publisherBufferSize int
	}
)

// New creates a channel-like Broadcaster. Buffer length may be passed as first argument. Args[1:] are ignored.
func New[T any](buffer ...int) Broadcaster[T] {
	var buf int
	if len(buffer) > 0 {
		buf = buffer[0]
	}

	c := &subscribersManagement[T]{
		sliceStoragesPool: &sync.Pool{
			New: func() any { return newSliceStorage[T]() },
		},
		channels: new(atomic.Pointer[sliceStorage[T]]),
		waitGroupsPool: &sync.Pool{
			New: func() any {
				return new(sync.WaitGroup)
			},
		},
		publisherBufferSize: buf,
	}
	subscribersPool := new(sync.Pool)
	subscribersPool.New = func() any {
		return newSubscriber[T](
			subscribersPool,
			c.removeChannel,
			buf,
		)
	}
	c.subscribersPool = subscribersPool

	c.channels.Store(c.getSliceStorageFromPool()) // never nil

	return c
}

// Publisher returns a send-only channel that converts channel sends into broadcasts.
// Uses context.Background() internally. For context-aware publishing, use PublisherCtx.
// The returned channel can be used in standard Go patterns (range loops, select statements).
//
// Example:
//
//	publisher := bc.Publisher()
//	defer close(publisher)
//
//	publisher <- data1  // Will be broadcast to all subscribers
//	publisher <- data2  // Will be broadcast to all subscribers
//
// WARNING: The goroutine handling the channel will leak if the channel is never closed.
// Caller is responsible for closing the channel.
func (b *subscribersManagement[T]) Publisher() chan<- T {
	return b.PublisherCtx(context.Background())
}

// PublisherCtx returns a send-only channel that converts channel sends into context-aware broadcasts.
// The publisher respects context cancellation - if the context is canceled, the publisher
// stops processing messages and the goroutine exits.
//
// Example with context:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	publisher := bc.PublisherCtx(ctx)
//	defer close(publisher)
//
//	publisher <- data  // Broadcast with timeout protection
//
// Example with graceful shutdown:
//
//	publisher := bc.PublisherCtx(ctx)
//	defer close(publisher)
//
//	for i := 0; i < 10; i++ {
//	    select {
//	    case publisher <- i:
//	        // successfully broadcast
//	    case <-ctx.Done():
//	        return // context canceled, exit gracefully
//	    }
//	}
//
// The caller MUST close the returned channel to prevent goroutine leaks, OR ensure
// the context eventually gets canceled.
func (b *subscribersManagement[T]) PublisherCtx(ctx context.Context) chan<- T {
	pubChan := make(chan T, b.publisherBufferSize)

	go func(pubChan <-chan T) {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-pubChan:
				if !ok {
					return
				}

				b.WriteCtx(ctx, data)
			}
		}
	}(pubChan)

	return pubChan
}

// Subscribe creates a new subscriber and adds its channel to the broadcaster's active channels list.
func (b *subscribersManagement[T]) Subscribe() Subscriber[T] {
	s := b.subscribersPool.Get().(*channelManagement[T])
	s.open()
	b.appendChannel(s.getChannelFromAtomicValue())

	return s
}

// DetachAndWrite asynchronously writes data (equivalent to "go WriteCtx()").
// WARNING: possible resource leak when using context.Background() or other non-cancellable contexts.
func (b *subscribersManagement[T]) DetachAndWrite(ctx context.Context, data T) {
	b.write(ctx, data, true, true)
}

// WriteNonBlock uses non-blocking send (select-default).
// Data may be missed by some subscribers when their channels are full.
// Use WriteCtx or WriteBlock for guaranteed delivery.
func (b *subscribersManagement[T]) WriteNonBlock(data T) {
	b.write(context.Background(), data, false, false)
}

// WriteBlock uses blocking send with context.Background().
// May block indefinitely if there are no active readers.
func (b *subscribersManagement[T]) WriteBlock(data T) { //nolint: unused
	b.write(context.Background(), data, true, false)
}

// WriteCtx uses context-aware send with cancellation support.
// Recommended method for guaranteed delivery with lifecycle control.
func (b *subscribersManagement[T]) WriteCtx(ctx context.Context, data T) {
	b.write(ctx, data, true, false)
}

// getSliceStorageFromPool retrieves a sliceStorage from the pool
func (b *subscribersManagement[T]) getSliceStorageFromPool() *sliceStorage[T] {
	return b.sliceStoragesPool.Get().(*sliceStorage[T])
}

// putSliceStorageToPool returns a sliceStorage to the pool
func (b *subscribersManagement[T]) putSliceStorageToPool(s *sliceStorage[T]) {
	b.sliceStoragesPool.Put(s)
}

// getCurrentAndCopiedSliceStorages atomically loads current slice storage and copies its contents
// to a new storage retrieved from the pool.
func (b *subscribersManagement[T]) getCurrentAndCopiedSliceStorages() (
	currentStorage *sliceStorage[T],
	copiedStorage *sliceStorage[T],
) {
	currentStorage = b.channels.Load()
	copiedStorage = b.getSliceStorageFromPool()

	currentStorage.copySliceTo(copiedStorage)

	return
}

// compareAndSwapCurrentAndCopiedSliceStorages performs CAS operation and manages pool returns
func (b *subscribersManagement[T]) compareAndSwapCurrentAndCopiedSliceStorages(
	currentStorage *sliceStorage[T],
	copiedStorage *sliceStorage[T],
) (
	swapped bool,
) {
	if swapped = b.channels.CompareAndSwap(currentStorage, copiedStorage); !swapped {
		b.putSliceStorageToPool(copiedStorage) // new (copied) storage must be put back to pool if not success
	}

	return
}

// appendChannel adds a channel to the active channels list using CAS loop
func (b *subscribersManagement[T]) appendChannel(ch chan T) {
	currentStorage, copiedStorage := b.getCurrentAndCopiedSliceStorages()

	if !copiedStorage.appendValue(ch) { // not appending if already contains
		b.putSliceStorageToPool(copiedStorage)

		return
	}

	if !b.compareAndSwapCurrentAndCopiedSliceStorages(currentStorage, copiedStorage) {
		b.appendChannel(ch) // pointer has already been swapped, need one more try to prevent active channels lose
	}
}

// removeChannel removes a channel from the active channels list using CAS loop
func (b *subscribersManagement[T]) removeChannel(ch chan T) {
	currentStorage, copiedStorage := b.getCurrentAndCopiedSliceStorages()

	if !copiedStorage.removeValue(ch) { // not removing if doesn't contain
		b.putSliceStorageToPool(copiedStorage)

		return
	}

	if !b.compareAndSwapCurrentAndCopiedSliceStorages(currentStorage, copiedStorage) {
		b.removeChannel(ch) // pointer has already been swapped, need one more try to prevent active channels lose
	}
}

// write broadcasts data to all subscribers with specified blocking behavior
func (b *subscribersManagement[T]) write(ctx context.Context, data T, block, detach bool) {
	wg, ok := b.waitGroupsPool.Get().(*sync.WaitGroup)
	if !ok {
		wg = new(sync.WaitGroup)
	}

	writeFunc := getWriteFunc[T](ctx, wg, block)
	channels := b.channels.Load()

	wg.Add(len(channels.slice))

	for _, channel := range channels.slice {
		go writeFunc(channel, data)
	}

	if !detach {
		wg.Wait()
		b.waitGroupsPool.Put(wg)
	} else {
		go func(wg *sync.WaitGroup, pool *sync.Pool) {
			wg.Wait()
			pool.Put(wg)
		}(wg, b.waitGroupsPool)
	}
}

// getWriteFunc returns appropriate write function based on blocking behavior
func getWriteFunc[T any](ctx context.Context, wg *sync.WaitGroup, block bool) func(ch chan<- T, v T) {
	if block {
		return func(ch chan<- T, v T) {
			defer func() { recover() }() // to mask send on closed panic.
			defer wg.Done()

			select {
			case ch <- v:
			case <-ctx.Done():
			}
		}
	}

	return func(ch chan<- T, v T) {
		defer func() { recover() }() // to mask send on closed panic.
		defer wg.Done()

		select {
		case ch <- v:
		default:
		}
	}
}
