package broadcast

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type (
	// Broadcaster allows one goroutine to write the same T value to multiple
	// goroutines concurrently.
	//
	// All methods are thread-safe and may be called from multiple goroutines.
	//
	// Delivery semantics depend on the write method:
	//   - WriteNonBlock: non-blocking, may skip slow subscribers.
	//   - WriteBlock:    blocking, waits indefinitely for all subscribers.
	//   - WriteCtx:      blocking with cancellation and optional timeout.
	//   - DetachAndWrite: like WriteCtx, but returns before delivery completes.
	Broadcaster[T any] interface {
		// WithBlockTimeout returns a copy of the broadcaster with the given
		// per-write timeout.
		//
		// The timeout bounds how long WriteCtx/WriteBlock/DetachAndWrite wait
		// for each subscriber before giving up on it. A zero timeout means no
		// timeout (only the write context can cancel the wait).
		//
		// WithBlockTimeout shares state with the receiver: subscribers added
		// via either instance are visible to both. Only the configuration is
		// copied.
		WithBlockTimeout(time.Duration) Broadcaster[T]

		// WithBufferSize returns a copy of the broadcaster with the given
		// subscriber channel buffer size.
		//
		// The new size applies only to subscribers created by the returned
		// broadcaster via Subscribe. Existing subscribers keep their original
		// buffer size. The publisher channel returned by Publisher/PublisherCtx
		// also uses this size.
		//
		// WithBufferSize shares state with the receiver in the same way as
		// WithBlockTimeout.
		WithBufferSize(int) Broadcaster[T]

		// Publisher returns a send-only channel that converts channel sends
		// into broadcasts. Uses context.Background() internally.
		// For context-aware publishing, use PublisherCtx.
		//
		// The returned channel can be used in standard Go patterns
		// (e.g. select statements).
		//
		// Example:
		//
		//	publisher := bc.Publisher()
		//	defer close(publisher)
		//
		//	publisher <- data1  // Will be broadcast to all subscribers
		//	publisher <- data2  // Will be broadcast to all subscribers
		//
		// Each send is handled by a dedicated goroutine that calls WriteCtx
		// with context.Background(). Consequently, a slow subscriber blocks
		// subsequent sends on the returned channel.
		//
		// WARNING: The goroutine handling the channel will leak if the channel
		// is never closed. Caller is responsible for closing the channel.
		Publisher() chan<- T

		// PublisherCtx returns a send-only channel that converts channel sends
		// into context-aware broadcasts. The publisher respects context
		// cancellation - if the context is canceled, the publisher stops
		// processing messages and the goroutine exits.
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
		// Each send is handled by a dedicated goroutine that calls WriteCtx
		// with the provided context. A slow subscriber blocks subsequent sends
		// on the returned channel.
		//
		// The caller MUST close the returned channel to prevent goroutine
		// leaks, OR ensure the context eventually gets canceled.
		PublisherCtx(ctx context.Context) chan<- T

		// Subscribe creates a new subscriber and adds its channel to the
		// broadcaster's active list.
		//
		// The returned Subscriber must be released with Dispatch.
		// The subscriber's channel buffer size is determined by the
		// broadcaster's current configuration (see WithBufferSize).
		Subscribe() Subscriber[T]

		// DetachAndWrite asynchronously writes data to all current subscribers
		// (equivalent to "go WriteCtx(ctx, data)").
		//
		// The call returns immediately; delivery proceeds in the background.
		// The per-subscriber wait is bounded by blockTimeout and ctx. If both
		// are non-cancellable, the background goroutine may leak.
		//
		// WARNING: possible resource leak when using context.Background() or
		// other non-cancellable contexts without a block timeout.
		DetachAndWrite(context.Context, T)

		// WriteNonBlock uses non-blocking send (select-default).
		//
		// Returns false if the data was not accepted by at least one
		// subscriber, either because its buffer was full or because it had
		// already been dispatched. Returns true only if every subscriber
		// accepted the data.
		//
		// Use WriteCtx or WriteBlock for guaranteed delivery.
		WriteNonBlock(T) bool

		// WriteBlock uses blocking send with context.Background().
		//
		// Waits for every subscriber to accept the data, or for blockTimeout
		// to elapse (if configured). May block indefinitely if there are no
		// active readers and blockTimeout is zero.
		WriteBlock(T)

		// WriteCtx uses context-aware send with cancellation support.
		//
		// Waits for every subscriber to accept the data, for ctx to be
		// canceled, or for blockTimeout to elapse (if configured).
		//
		// Recommended method for guaranteed delivery with lifecycle control.
		WriteCtx(context.Context, T)
	}

	// subscribersManagement implements Broadcaster using atomic operations
	// and object pools to minimize lock contention and GC pressure.
	//
	// The active subscriber list is stored as an immutable slice wrapped in
	// atomic.Pointer[sliceStorage]. Mutations (Subscribe/Dispatch) use
	// copy-on-write with a compare-and-swap loop, so readers of the list
	// (write paths) never block and never observe partial updates.
	//
	// Configuration is copied by value when WithBlockTimeout / WithBufferSize
	// are called. The shared state (channelsManagers, pools) is referenced by
	// pointer, so all derived broadcasters observe the same subscriber set.
	subscribersManagement[T any] struct {
		// channelsManagers points to the current immutable snapshot of active
		// subscribers.
		channelsManagers *atomic.Pointer[sliceStorage[T]]

		// sliceStoragesPool reuses sliceStorage objects across CAS iterations
		// and write iterations. Cleared on Put by putSliceStorageToPool.
		sliceStoragesPool *sync.Pool

		// subscribersPool reuses channelManagement objects across
		// Subscribe/Dispatch cycles.
		subscribersPool *sync.Pool

		// waitGroupsPool reuses sync.WaitGroup objects used by write.
		waitGroupsPool *sync.Pool

		// blockTimeout bounds how long write waits for each subscriber.
		// Zero means no timeout.
		blockTimeout time.Duration

		// publisherBufferSize is the buffer size for both publisher channels
		// (Publisher/PublisherCtx) and subscriber channels opened by Subscribe.
		publisherBufferSize int
	}
)

// New creates a channel-like Broadcaster.
//
// Buffer length may be passed as the first variadic argument; it sets the
// buffer size of publisher channels and of subscriber channels created by
// Subscribe. Additional arguments are ignored.
//
// The returned Broadcaster is ready to use. Its subscriber list is empty.
func New[T any](buffer ...int) Broadcaster[T] {
	var buf int
	if len(buffer) > 0 {
		buf = buffer[0]
	}

	c := &subscribersManagement[T]{
		channelsManagers: new(atomic.Pointer[sliceStorage[T]]),
		sliceStoragesPool: &sync.Pool{
			New: func() any { return newSliceStorage[T]() },
		},
		subscribersPool: new(sync.Pool),
		waitGroupsPool: &sync.Pool{
			New: func() any {
				return new(sync.WaitGroup)
			},
		},
		publisherBufferSize: buf,
	}

	// The subscriber pool must produce subscribers bound to this broadcaster,
	// so that Dispatch removes them from the correct subscriber set.
	c.subscribersPool.New = func() any {
		return newSubscriber[T](
			c.subscribersPool,
			c.removeChannelManagement,
		)
	}

	c.channelsManagers.Store(c.getSliceStorageFromPool()) // never nil

	return c
}

// WithBlockTimeout returns a copy of b with the given per-write timeout.
// Shared state (subscribers, pools) is referenced, not copied.
func (b *subscribersManagement[T]) WithBlockTimeout(blockTimeout time.Duration) Broadcaster[T] {
	return &subscribersManagement[T]{
		channelsManagers:    b.channelsManagers,
		sliceStoragesPool:   b.sliceStoragesPool,
		subscribersPool:     b.subscribersPool,
		waitGroupsPool:      b.waitGroupsPool,
		blockTimeout:        blockTimeout,
		publisherBufferSize: b.publisherBufferSize,
	}
}

// WithBufferSize returns a copy of b with the given subscriber buffer size.
// Shared state (subscribers, pools) is referenced, not copied.
func (b *subscribersManagement[T]) WithBufferSize(bufferSize int) Broadcaster[T] {
	return &subscribersManagement[T]{
		channelsManagers:    b.channelsManagers,
		sliceStoragesPool:   b.sliceStoragesPool,
		subscribersPool:     b.subscribersPool,
		waitGroupsPool:      b.waitGroupsPool,
		blockTimeout:        b.blockTimeout,
		publisherBufferSize: bufferSize,
	}
}

// Publisher returns a send-only channel that converts channel sends into
// broadcasts. Uses context.Background() internally.
// For context-aware publishing, use PublisherCtx.
//
// The returned channel can be used in standard Go patterns (e.g.
// select statements).
//
// Example:
//
//	publisher := bc.Publisher()
//	defer close(publisher)
//
//	publisher <- data1  // Will be broadcast to all subscribers
//	publisher <- data2  // Will be broadcast to all subscribers
//
// Each send is handled by a dedicated goroutine that calls WriteCtx with
// context.Background(). Consequently, a slow subscriber blocks subsequent
// sends on the returned channel.
//
// WARNING: The goroutine handling the channel will leak if the channel is
// never closed. Caller is responsible for closing the channel.
func (b *subscribersManagement[T]) Publisher() chan<- T {
	return b.PublisherCtx(context.Background())
}

// PublisherCtx returns a send-only channel that converts channel sends into
// context-aware broadcasts. The publisher respects context cancellation - if
// the context is canceled, the publisher stops processing messages and the
// goroutine exits.
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
// Each send is handled by a dedicated goroutine that calls WriteCtx with the
// provided context. A slow subscriber blocks subsequent sends on the returned
// channel.
//
// The caller MUST close the returned channel to prevent goroutine leaks, OR
// ensure the context eventually gets canceled.
func (b *subscribersManagement[T]) PublisherCtx(ctx context.Context) chan<- T {
	pubChan := make(chan T, b.publisherBufferSize)

	go func(b *subscribersManagement[T], pubChan <-chan T) {
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
	}(b, pubChan)

	return pubChan
}

// Subscribe creates a new subscriber and adds its channel to the
// broadcaster's active list.
//
// The subscriber's channel buffer size is taken from the current
// configuration (see WithBufferSize).
func (b *subscribersManagement[T]) Subscribe() Subscriber[T] {
	s := b.subscribersPool.Get().(*channelManagement[T])

	s.open(b.publisherBufferSize)
	b.appendChannelManagement(s)

	return s
}

// DetachAndWrite asynchronously writes data to all current subscribers
// (equivalent to "go WriteCtx(ctx, data)").
//
// The call returns immediately; delivery proceeds in the background.
// The per-subscriber wait is bounded by blockTimeout and ctx. If both are
// non-cancellable, the background goroutine may leak.
//
// WARNING: possible resource leak when using context.Background() or other
// non-cancellable contexts without a block timeout.
func (b *subscribersManagement[T]) DetachAndWrite(ctx context.Context, data T) {
	b.write(ctx, data, true)
}

// WriteNonBlock uses non-blocking send (select-default).
//
// Returns false if the data was not accepted by at least one subscriber,
// either because its buffer was full or because it had already been
// dispatched. Returns true only if every subscriber accepted the data.
//
// Use WriteCtx or WriteBlock for guaranteed delivery.
func (b *subscribersManagement[T]) WriteNonBlock(data T) bool {
	channelManagers := b.channelsManagers.Load()
	if len(channelManagers.slice) == 0 {
		return false
	}

	ok := true

	for _, channelM := range channelManagers.slice {
		if !channelM.writeNonBlock(data) {
			ok = false
		}
	}

	return ok
}

// WriteBlock uses blocking send with context.Background().
//
// Waits for every subscriber to accept the data, or for blockTimeout to
// elapse (if configured). May block indefinitely if there are no active
// readers and blockTimeout is zero.
func (b *subscribersManagement[T]) WriteBlock(data T) { //nolint: unused
	b.write(context.Background(), data, false)
}

// WriteCtx uses context-aware send with cancellation support.
//
// Waits for every subscriber to accept the data, for ctx to be canceled,
// or for blockTimeout to elapse (if configured).
//
// Recommended method for guaranteed delivery with lifecycle control.
func (b *subscribersManagement[T]) WriteCtx(ctx context.Context, data T) {
	b.write(ctx, data, false)
}

// getSliceStorageFromPool retrieves a sliceStorage from the pool.
func (b *subscribersManagement[T]) getSliceStorageFromPool() *sliceStorage[T] {
	return b.sliceStoragesPool.Get().(*sliceStorage[T])
}

// putSliceStorageToPool clears and returns a sliceStorage to the pool.
func (b *subscribersManagement[T]) putSliceStorageToPool(s *sliceStorage[T]) {
	s.clear()
	b.sliceStoragesPool.Put(s)
}

// getCurrentAndCopiedSliceStorages atomically loads the current slice storage
// and copies its contents into a new storage retrieved from the pool.
//
// The returned copiedStorage is always a distinct object whose backing array
// does not alias currentStorage, so it is safe to mutate before CAS.
func (b *subscribersManagement[T]) getCurrentAndCopiedSliceStorages() (
	currentStorage *sliceStorage[T],
	copiedStorage *sliceStorage[T],
) {
	currentStorage = b.channelsManagers.Load()
	copiedStorage = b.getSliceStorageFromPool()

	currentStorage.copySliceTo(copiedStorage)

	return
}

// appendChannelManagement adds a subscriber to the active list using a CAS loop.
//
// The loop retries until the compare-and-swap succeeds, because concurrent
// Subscribe/Dispatch calls may have replaced the snapshot in the meantime.
// Each failed attempt returns its copiedStorage to the pool.
//
// If the subscriber is already present, the call is a no-op.
func (b *subscribersManagement[T]) appendChannelManagement(chM *channelManagement[T]) {
	for {
		currentStorage, copiedStorage := b.getCurrentAndCopiedSliceStorages()

		if !copiedStorage.appendValue(chM) {
			b.putSliceStorageToPool(copiedStorage)

			return
		}

		if b.channelsManagers.CompareAndSwap(currentStorage, copiedStorage) {
			return
		}

		b.putSliceStorageToPool(copiedStorage)
	}
}

// removeChannelManagement removes a subscriber from the active list using a CAS loop.
//
// The loop retries until the compare-and-swap succeeds, because concurrent
// Subscribe/Dispatch calls may have replaced the snapshot in the meantime.
// Each failed attempt returns its copiedStorage to the pool.
//
// If the subscriber is not present, the call is a no-op.
func (b *subscribersManagement[T]) removeChannelManagement(chM *channelManagement[T]) {
	for {
		currentStorage, copiedStorage := b.getCurrentAndCopiedSliceStorages()

		if !copiedStorage.removeValue(chM) {
			b.putSliceStorageToPool(copiedStorage)

			return
		}

		if b.channelsManagers.CompareAndSwap(currentStorage, copiedStorage) {
			return
		}

		b.putSliceStorageToPool(copiedStorage)
	}
}

// write broadcasts data to all subscribers currently in the active list.
//
// The subscriber set is snapshotted once at the start of the call; subscribers
// added or removed concurrently are not observed. Each subscriber is written
// to from its own goroutine, and the call waits (or does not wait, if detach
// is true) for all of them via a pooled WaitGroup.
//
// The per-subscriber wait is bounded by blockTimeout (if configured) and by
// ctx. If detach is true, the WaitGroup is returned to the pool from a
// background goroutine and the caller does not block on delivery.
func (b *subscribersManagement[T]) write(ctx context.Context, data T, detach bool) {
	channelManagemers := b.channelsManagers.Load()
	if len(channelManagemers.slice) == 0 {
		return
	}

	var (
		waitCtx context.Context
		cancel  context.CancelFunc
	)

	if b.blockTimeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, b.blockTimeout)
	} else {
		waitCtx, cancel = context.WithCancel(ctx)
	}

	wg := b.waitGroupsPool.Get().(*sync.WaitGroup)

	wg.Add(len(channelManagemers.slice))

	for _, channelM := range channelManagemers.slice {
		go channelM.write(waitCtx, wg, data)
	}

	if !detach {
		waitAndPutWGToPool(cancel, wg, b.waitGroupsPool)
	} else {
		go waitAndPutWGToPool(cancel, wg, b.waitGroupsPool)
	}
}

func waitAndPutWGToPool(cancel context.CancelFunc, wg *sync.WaitGroup, pool *sync.Pool) {
	defer cancel()
	defer pool.Put(wg)

	wg.Wait()
}
