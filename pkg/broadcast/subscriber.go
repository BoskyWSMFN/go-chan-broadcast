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
		ch                        atomic.Value
		chBuffer                  int
		chanRemoveFromBroadcaster func(chan T)
		subscribersPool           *sync.Pool
		oncer                     atomic.Pointer[sync.Once]
	}
)

func newSubscriber[T any](
	subscribersPool *sync.Pool,
	chanRemoveFromBroadcaster func(chan T),
	chBuffer int) *channelManagement[T] {
	ch := &channelManagement[T]{
		chBuffer:                  chBuffer,
		chanRemoveFromBroadcaster: chanRemoveFromBroadcaster,
		subscribersPool:           subscribersPool,
	}

	return ch
}

// Read from channel. Read returns closed channel if dispatched.
func (s *channelManagement[T]) Read() <-chan T {
	return s.getChannelFromAtomicPointer()
}

func (s *channelManagement[T]) getChannelFromAtomicPointer() chan T {
	ch, ok := s.ch.Load().(chan T)
	if !ok {
		ch = make(chan T)

		close(ch)
	}

	return ch
}

// Dispatch removes Subscriber's active channel from Broadcaster's active channels slice, drains it and
// puts Subscriber itself to Broadcaster's Subscribers pool.
func (s *channelManagement[T]) Dispatch() {
	defer func() {
		closedCh := make(chan T)

		close(closedCh)

		s.ch.Store(closedCh)
		s.oncer.Store(nil)
		s.subscribersPool.Put(s)
	}()

	if oncer := s.oncer.Load(); oncer != nil {
		oncer.Do(func() {
			ch := s.getChannelFromAtomicPointer()

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

// drainActiveChannel to prevent infinite write blocks
func (s *channelManagement[T]) drainChannel(ch chan T) {
	const doubleDrain = 2

	for range s.chBuffer * doubleDrain {
		select {
		case <-ch:
		default:
		}
	}
}

// open closed with Dispatch method Subscriber
func (s *channelManagement[T]) open() {
	ch := make(chan T, s.chBuffer)

	s.oncer.Store(new(sync.Once))
	s.ch.Store(ch)
}
