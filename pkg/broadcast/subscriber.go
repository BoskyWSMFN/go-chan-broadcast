package broadcast

import (
	"context"
	"sync"
	"sync/atomic"
)

type (
	// Subscriber allows reading data sent through Broadcaster.
	// Each subscriber receives its own copy of the data.
	//
	// Values are delivered to the channel returned by Read. If the Broadcaster
	// was configured with a value type that implements Cloner[T], each
	// subscriber receives an independent clone; otherwise all subscribers
	// share the same value.
	Subscriber[T any] interface {
		// Read returns a channel for reading data.
		// The channel is closed immediately after Dispatch is called.
		//
		// Calling Read after Dispatch returns a closed channel.
		// Calling Read concurrently with Dispatch is safe, but the
		// returned channel may be closed at any moment.
		//
		// If the delivered values implement Releaser, the caller must invoke
		// Release once the value is no longer needed, to return it to its
		// source (e.g. a pool). Values that do not implement Releaser can be
		// discarded normally; the garbage collector will reclaim them.
		Read() <-chan T

		// Dispatch unsubscribes the subscriber, closes the channel,
		// and returns internal resources to a pool for reuse.
		//
		// Must be called exactly once per Subscriber.
		// Any use of the Subscriber (including additional Dispatch calls)
		// after the first Dispatch may result in undefined behavior due to
		// reuse of the underlying object from sync.Pool.
		//
		// Dispatch may block until any in-flight WriteCtx/WriteBlock call
		// targeting this subscriber finishes (or its context is canceled).
		// With WriteNonBlock, Dispatch does not block.
		//
		// Dispatch does not release values already delivered to Read; the
		// caller remains responsible for calling Release on them if needed.
		Dispatch()

		// Len returns the number of elements currently buffered in the
		// subscriber's channel. For an unbuffered channel it always returns 0.
		//
		// Len is intended for diagnostics only and must not be used for
		// synchronization.
		Len() int

		// Cap returns the buffer capacity of the subscriber's channel.
		//
		// Cap is intended for diagnostics only and must not be used for
		// synchronization.
		Cap() int

		// Release invokes Release on the given Releaser, returning the value
		// to its source (for example, a sync.Pool).
		//
		// Release is a convenience wrapper that lets callers release values
		// without an explicit type assertion:
		//
		//	for v := range sub.Read() {
		//	    handle(v)
		//	    if r, ok := any(v).(Releaser); ok {
		//	        sub.Release(r)
		//	    }
		//	}
		//
		// Release is a no-op if releaser is nil.
		Release(Releaser)
	}

	// channelManagement implements Subscriber using atomic operations and
	// object pooling.
	//
	// Lifecycle:
	//   - A fresh instance is created by newSubscriber with closed ch and done
	//     channels, so any write before open() is a no-op.
	//   - Subscribe calls open(), which installs fresh ch/done channels under mu.
	//   - Dispatch removes the subscriber from the broadcaster, closes ch and
	//     done, and returns the instance to subscribersPool.
	//   - The instance may then be reused by a subsequent Subscribe.
	//
	// Concurrency:
	//   - mu guards the close-vs-send race: Dispatch closes ch under mu.Lock,
	//     while write/writeNonBlock send under mu.RLock and hold it for the
	//     duration of the select.
	//   - done is closed by Dispatch before taking mu.Lock, so that a writer
	//     blocked in select can observe the cancellation and release RLock,
	//     allowing Dispatch to proceed. Closing done under the lock would
	//     deadlock against a writer waiting on done.
	//   - oncer is stored as an atomic pointer because sync.Once cannot be
	//     reset after use, and instances are reused across Subscribe calls.
	channelManagement[T any] struct {
		// mu serializes channel close (Dispatch) against channel send
		// (write/writeNonBlock). It is also held by open() while swapping
		// ch and done to prevent writers from observing a partially
		// initialized state.
		mu sync.RWMutex

		// ch holds the current data channel as a chan T.
		// Accessed via getChannelFromAtomicValue.
		ch atomic.Value

		// chBuffer mirrors the buffer size passed to open, so that Cap()
		// can report it without touching the channel.
		chBuffer atomic.Int32

		// chanRemoveFromBroadcaster removes this subscriber from the
		// broadcaster's active list. Set once at construction.
		chanRemoveFromBroadcaster func(*channelManagement[T])

		// subscribersPool is the pool this instance belongs to. Dispatch
		// returns the instance here.
		subscribersPool *sync.Pool

		// oncer guards Dispatch so that only the first call performs cleanup.
		// It is replaced on every open() and cleared on every Dispatch.
		oncer atomic.Pointer[sync.Once]

		// notifyExitCh is closed by Dispatch to signal writers blocked in
		// select that they should stop. Accessed via done.
		notifyExitCh atomic.Value
	}
)

// newSubscriber creates a new subscriber instance.
//
// The returned instance is in a "dispatched" state: its ch and done channels
// are closed, so any write before open() is a no-op. The instance becomes
// usable only after open() is called.
func newSubscriber[T any](
	subscribersPool *sync.Pool,
	chanRemoveFromBroadcaster func(management *channelManagement[T]),
) *channelManagement[T] {
	ch := &channelManagement[T]{
		chanRemoveFromBroadcaster: chanRemoveFromBroadcaster,
		subscribersPool:           subscribersPool,
	}

	notifyExitCh := make(chan struct{})
	close(notifyExitCh)

	initClosedCh := make(chan T)
	close(initClosedCh)

	ch.notifyExitCh.Store(notifyExitCh)
	ch.ch.Store(initClosedCh)

	return ch
}

// Read returns a channel for reading data.
// Returns a closed channel after Dispatch is called.
// Read does not block and does not acquire mu; it is safe to call
// concurrently with Dispatch, but the returned channel may be closed at
// any moment after Dispatch begins.
func (s *channelManagement[T]) Read() <-chan T {
	return s.getChannelFromAtomicValue()
}

// Len returns the number of elements buffered in the current channel.
func (s *channelManagement[T]) Len() int {
	return len(s.getChannelFromAtomicValue())
}

// Cap returns the buffer capacity of the current channel.
func (s *channelManagement[T]) Cap() int {
	return int(s.chBuffer.Load())
}

// Release invokes Release on the given Releaser, returning the value
// to its source (for example, a sync.Pool).
//
// Release is a convenience wrapper that lets callers release values
// without an explicit type assertion:
//
//	for v := range sub.Read() {
//	    handle(v)
//	    if r, ok := any(v).(Releaser); ok {
//	        sub.Release(r)
//	    }
//	}
//
// Release is a no-op if releaser is nil.
func (s *channelManagement[T]) Release(releaser Releaser) {
	if releaser != nil {
		releaser.Release()
	}
}

// getChannelFromAtomicValue safely retrieves the current data channel.
//
// The channel is never nil: before open() it is a closed channel installed
// by newSubscriber.
func (s *channelManagement[T]) getChannelFromAtomicValue() chan T {
	return s.ch.Load().(chan T)
}

// Dispatch unsubscribes the subscriber, closes the channel, and returns
// internal resources to subscribersPool for reuse.
//
// Must be called exactly once per Subscriber.
// Any use of the Subscriber (including additional Dispatch calls) after
// the first Dispatch may result in undefined behavior due to reuse of the
// underlying object from sync.Pool.
//
// Dispatch may block on mu.Lock if a writer is currently blocked in select
// under mu.RLock. Closing done before taking mu.Lock ensures that such a
// writer is woken up and releases RLock promptly.
func (s *channelManagement[T]) Dispatch() {
	if oncer := s.oncer.Load(); oncer != nil {
		oncer.Do(func() {
			defer func() {
				s.oncer.Store(nil)
				s.subscribersPool.Put(s)
			}()

			s.chanRemoveFromBroadcaster(s)

			// Close done before acquiring mu so that a writer blocked in
			// select can observe cancellation and release RLock. Closing
			// done under mu.Lock would deadlock against such a writer.
			close(s.notifyExitCh.Load().(chan struct{}))

			// Acquire mu to serialize against in-flight sends. After this
			// point no new sends can start, and any in-flight send has
			// already left its select.
			s.mu.Lock()
			close(s.getChannelFromAtomicValue())
			s.mu.Unlock()
		})
	}
}

// open re-activates a previously dispatched subscriber, installing fresh
// ch/done channels and resetting oncer.
//
// open is called by Subscribe after retrieving the instance from the pool.
// It takes mu.Lock so that writers never observe a partially initialized
// state (e.g. new ch with old done, or vice versa).
func (s *channelManagement[T]) open(buffer int) {
	ch := make(chan T, buffer)

	s.mu.Lock()
	s.oncer.Store(new(sync.Once))
	s.chBuffer.Store(int32(buffer))
	s.notifyExitCh.Store(make(chan struct{}))
	s.ch.Store(ch)
	s.mu.Unlock()
}

// isDone reports whether done has been closed by Dispatch.
//
// isDone is only meaningful when called under s.mu.RLock: otherwise the
// result may be stale by the time the caller acts on it.
func (s *channelManagement[T]) isDone() bool {
	select {
	case <-s.done():
		return true
	default:
		return false
	}
}

// done returns the current notification channel.
func (s *channelManagement[T]) done() <-chan struct{} {
	return s.notifyExitCh.Load().(chan struct{})
}

// write performs a blocking send of data to the subscriber's channel.
//
// write holds mu.RLock for the duration of the select, which prevents
// Dispatch from closing ch underneath it. done is closed by Dispatch before
// it acquires mu.Lock, so a writer blocked in select is guaranteed to be
// woken up even if Dispatch is waiting for mu.Lock.
//
// Returns true if the value was accepted by the channel and false
// otherwise.
//
// write always calls wg.Done before returning, even if the send is skipped
// because the subscriber has already been dispatched or the context is done.
//
// *sync.WaitGroup may be nil, in which case no [sync.WaitGroup.Done] is called.
func (s *channelManagement[T]) write(ctx context.Context, wg *sync.WaitGroup, data T) bool {
	if wg != nil {
		defer wg.Done()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.isDone() {
		return false
	}

	select {
	case s.getChannelFromAtomicValue() <- data:
		return true
	case <-s.done():
		return false
	case <-ctx.Done():
		return false
	}
}

// writeNonBlock performs a non-blocking send of data to the subscriber's
// channel.
//
// writeNonBlock holds mu.RLock for the duration of the select, which prevents
// Dispatch from closing ch underneath it. done is closed by Dispatch before
// it acquires mu.Lock, so a writer blocked in select is guaranteed to be
// woken up even if Dispatch is waiting for mu.Lock.
//
// Returns true if the value was accepted by the channel and false
// otherwise.
//
// Like write, writeNonBlock holds mu.RLock for the duration of the select.
func (s *channelManagement[T]) writeNonBlock(data T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.isDone() {
		return false
	}

	select {
	case s.getChannelFromAtomicValue() <- data:
		return true
	case <-s.done():
		return false
	default:
		return false
	}
}
