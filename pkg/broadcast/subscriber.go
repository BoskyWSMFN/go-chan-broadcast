package broadcast

import (
	"sync"
	"sync/atomic"
)

type (
	// Subscriber allows reading data sent through Broadcaster.
	// Each subscriber receives its own copy of the data.
	Subscriber[T any] interface {
		// Read returns a channel for reading data.
		// The channel is closed immediately after Dispatch is called.
		Read() <-chan T
		// Dispatch unsubscribes the subscriber, closes the channel,
		// and returns internal resources to a pool for reuse.
		//
		// Must be called exactly once per Subscriber.
		// Any use of the Subscriber (including additional Dispatch calls)
		// after the first Dispatch may result in undefined behavior due to reuse
		// of the underlying object from sync.Pool.
		Dispatch()
	}

	// channelManagement implements subscriber using atomic operations and object pooling
	channelManagement[T any] struct {
		ch                        atomic.Value
		chBuffer                  atomic.Int32
		chanRemoveFromBroadcaster func(chan T)
		subscribersPool           *sync.Pool
		oncer                     atomic.Pointer[sync.Once]
	}
)

// newSubscriber creates a new subscriber instance
func newSubscriber[T any](
	subscribersPool *sync.Pool,
	chanRemoveFromBroadcaster func(chan T),
	chBuffer int) *channelManagement[T] {
	ch := &channelManagement[T]{
		chanRemoveFromBroadcaster: chanRemoveFromBroadcaster,
		subscribersPool:           subscribersPool,
	}

	ch.chBuffer.Store(int32(chBuffer))

	return ch
}

// Read returns a channel for reading data.
// Returns a closed channel after Dispatch is called.
func (s *channelManagement[T]) Read() <-chan T {
	return s.getChannelFromAtomicValue()
}

// getChannelFromAtomicValue safely retrieves the channel from atomic.Value
func (s *channelManagement[T]) getChannelFromAtomicValue() chan T {
	ch, ok := s.ch.Load().(chan T)
	if !ok {
		ch = make(chan T)

		close(ch)
	}

	return ch
}

// Dispatch unsubscribes the subscriber, closes the channel,
// and returns internal resources to a pool for reuse.
//
// Must be called exactly once per Subscriber.
// Any use of the Subscriber (including additional Dispatch calls)
// after the first Dispatch may result in undefined behavior due to reuse
// of the underlying object from sync.Pool.
func (s *channelManagement[T]) Dispatch() {
	if oncer := s.oncer.Load(); oncer != nil {
		oncer.Do(func() {
			defer func() {
				s.oncer.Store(nil)
				s.subscribersPool.Put(s)
			}()

			ch := s.getChannelFromAtomicValue()

			s.chanRemoveFromBroadcaster(ch)
			s.drainChannel(ch)

			select {
			case <-ch:
			default:
				close(ch)
			}
		})
	}
}

// drainChannel drains remaining messages to prevent goroutine leaks
func (s *channelManagement[T]) drainChannel(ch chan T) {
	const doubleDrain = 2

	for range s.chBuffer.Load() * doubleDrain {
		select {
		case <-ch:
		default:
		}
	}
}

// open re-activates a previously dispatched subscriber
func (s *channelManagement[T]) open() {
	ch := make(chan T, s.chBuffer.Load())

	s.oncer.Store(new(sync.Once))
	s.ch.Store(ch)
}
