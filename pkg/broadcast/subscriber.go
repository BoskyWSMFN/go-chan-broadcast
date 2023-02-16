package broadcast

import (
	"sync"
	"sync/atomic"
)

type (
	// Subscriber allows goroutine to read written to Broadcaster T value.
	Subscriber[T any] interface {
		// Read from channel. Read returns closed channel if dispatched.
		Read() <-chan T
		// Dispatch removes Subscriber's active channel from Broadcaster's active channels slice, drains it and
		// puts Subscriber itself to Broadcaster's Subscribers pool.
		Dispatch()
	}

	channelManagement[T any] struct {
		activeChan                chan T
		activeChannelBuffer       int
		chanRemoveFromBroadcaster func(chan T)
		subscribersPool           *sync.Pool
		closedChan                chan T
		isClosed                  *atomic.Bool
	}
)

func newSubscriber[T any](
	subscribersPool *sync.Pool,
	chanRemoveFromBroadcaster func(chan T),
	activeChannelBuffer int) *channelManagement[T] {
	ch := &channelManagement[T]{
		activeChan:                make(chan T, activeChannelBuffer),
		activeChannelBuffer:       activeChannelBuffer,
		chanRemoveFromBroadcaster: chanRemoveFromBroadcaster,
		subscribersPool:           subscribersPool,
		closedChan:                make(chan T),
		isClosed:                  new(atomic.Bool),
	}

	close(ch.closedChan)

	return ch
}

// Read from channel. Read returns closed channel if dispatched.
func (s *channelManagement[T]) Read() <-chan T {
	if s.isClosed.Load() {
		return s.closedChan
	}

	return s.activeChan
}

// Dispatch removes Subscriber's active channel from Broadcaster's active channels slice, drains it and
// puts Subscriber itself to Broadcaster's Subscribers pool.
func (s *channelManagement[T]) Dispatch() {
	if s.isClosed.Swap(true) {
		return
	}

	s.chanRemoveFromBroadcaster(s.activeChan)
	s.drainActiveChannel()
	s.subscribersPool.Put(s)
}

// drainActiveChannel to prevent infinite write blocks
func (s *channelManagement[T]) drainActiveChannel() {
	for ctr := 0; ctr <= s.activeChannelBuffer; ctr++ {
		select {
		case <-s.activeChan:
		default:
		}
	}
}

// open closed with Dispatch method Subscriber
func (s *channelManagement[T]) open() {
	s.isClosed.Store(false)
}
