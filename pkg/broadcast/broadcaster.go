package broadcast

import (
	"context"
	"sync"
	"sync/atomic"
)

type (
	// Broadcaster allows one goroutine to write same T value to multiple goroutines sequentially.
	Broadcaster[T any] interface {
		// Subscribe gets Subscriber from Broadcasters pool and appends Subscriber's active channel to
		// Broadcaster's active channel slice.
		Subscribe() Subscriber[T]

		// DetachAndWrite is a shorthand for "go WriteCtx()". Possible resource leak if
		// context.Background() or any other non-cancellable context is used. Short
		DetachAndWrite(ctx context.Context, data T)
		// WriteNonBlock uses select-default statement inside so the written data can be missed by some routines.
		// To prevent it use WriteCtx.
		WriteNonBlock(T)
		// WriteBlock uses select-case<-context.Background().Done() statement inside.
		WriteBlock(T)
		// WriteCtx uses select-case<-context.Context().Done() statement inside.
		WriteCtx(context.Context, T)
	}

	subscribersManagement[T any] struct {
		channels          *atomic.Pointer[sliceStorage[T]]
		sliceStoragesPool *sync.Pool
		subscribersPool   *sync.Pool
	}
)

// New creates go channel like Broadcaster. Buffer length may be passed as an argument. Args[1:] will be ignored.
func New[T any](
	buffer ...int,
) Broadcaster[T] {
	var buf int
	if len(buffer) > 0 {
		buf = buffer[0]
	}

	c := &subscribersManagement[T]{
		sliceStoragesPool: &sync.Pool{
			New: func() any { return newSliceStorage[T]() },
		},
		channels: new(atomic.Pointer[sliceStorage[T]]),
	}
	subscribersPool := new(sync.Pool)
	subscribersPool.New = func() any {
		return newSubscriber[T](
			subscribersPool,
			c.removeChannel,
			buf)
	}
	c.subscribersPool = subscribersPool

	c.channels.Store(c.getSliceStorageFromPool()) // never nil

	return c
}

// Subscribe gets Subscriber from Broadcasters pool and appends Subscriber's active channel to
// Broadcaster's active channel slice.
func (b *subscribersManagement[T]) Subscribe() Subscriber[T] {
	s := b.subscribersPool.Get().(*channelManagement[T])
	s.open()
	b.appendChannel(s.activeChan)

	return s
}

// DetachAndWrite is a shorthand for "go WriteCtx()". Possible resource leak if
// context.Background() or any other non-cancellable context is used.
func (b *subscribersManagement[T]) DetachAndWrite(ctx context.Context, data T) {
	go b.WriteCtx(ctx, data)
}

// WriteNonBlock uses select-default statement inside so the written data can be missed by some routines.
// To prevent it use WriteCtx.
func (b *subscribersManagement[T]) WriteNonBlock(data T) {
	b.write(context.Background(), data, false)
}

// WriteBlock uses select-case<-context.Background().Done() statement inside.
func (b *subscribersManagement[T]) WriteBlock(data T) { //nolint: unused
	b.write(context.Background(), data, true)
}

// WriteCtx uses select-case<-context.Context().Done() statement inside.
func (b *subscribersManagement[T]) WriteCtx(ctx context.Context, data T) {
	b.write(ctx, data, true)
}

// getSliceStorageFromPool is just a handy shortcut
func (b *subscribersManagement[T]) getSliceStorageFromPool() *sliceStorage[T] {
	return b.sliceStoragesPool.Get().(*sliceStorage[T])
}

// putSliceStorageToPool is just a handy shortcut
func (b *subscribersManagement[T]) putSliceStorageToPool(s *sliceStorage[T]) {
	b.sliceStoragesPool.Put(s)
}

// getCurrentAndCopiedSliceStorages atomically loads current slice storage and copies its contents to one got
// from pool.
func (b *subscribersManagement[T]) getCurrentAndCopiedSliceStorages() (
	currentStorage *sliceStorage[T],
	copiedStorage *sliceStorage[T],
) {
	currentStorage = b.channels.Load()
	copiedStorage = b.getSliceStorageFromPool()

	currentStorage.copySliceTo(copiedStorage)

	return
}

// compareAndSwapCurrentAndCopiedSliceStorages compares and swaps two slice storages and puts old one to
// slice storages pool if success and new one if not.
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

func (b *subscribersManagement[T]) write(ctx context.Context, data T, block bool) {
	writeFunc := getWriteFunc[T](ctx, block)

	for _, channel := range b.channels.Load().slice {
		writeFunc(channel, data)
	}
}

func getWriteFunc[T any](ctx context.Context, block bool) func(ch chan T, v T) {
	if block {
		return func(ch chan T, v T) {
			select {
			case ch <- v:
			case <-ctx.Done():
			}
		}
	}

	return func(ch chan T, v T) {
		select {
		case ch <- v:
		default:
		}
	}
}
