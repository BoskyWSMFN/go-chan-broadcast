package broadcast

// Cloner is implemented by value types that need to be duplicated before
// being delivered to each subscriber.
//
// When the broadcaster's value type T implements Cloner[T], the write
// methods call Clone once per subscriber and deliver the returned value
// to that subscriber's channel. This allows each subscriber to receive
// an independent instance — for example, a buffer from a pool — rather
// than a shared value.
//
// If T does not implement Cloner[T], the same value is delivered to every
// subscriber. In that case, T should be safe for concurrent reads and
// should not be mutated by any subscriber.
//
// Typical use cases:
//
//   - T is a pointer to a pooled buffer (*[]byte, *bytes.Buffer). Clone
//     retrieves a new buffer from the pool and copies the payload into it.
//
//   - T is a wrapper around an immutable payload that must be counted or
//     reference-tracked per subscriber.
//
//   - T is a value type whose copy is expensive (large struct) and should
//     be shared between subscribers when possible. In that case, do not
//     implement Cloner and share the value instead.
//
// Ownership:
//
//   - If Clone returns a value that implements Releaser, the caller is
//     responsible for releasing it after handling (for example, via
//     Subscriber.Release).
//
//   - If a clone is not accepted by a subscriber (its buffer is full, or
//     the context is canceled before delivery), the broadcaster releases
//     the clone automatically by calling Release on it, provided the clone
//     implements Releaser.
//
//   - Clones delivered successfully are not released by the broadcaster.
//
// Clone may be called concurrently from multiple goroutines: each
// subscriber is served by its own goroutine during a broadcast. Clone
// must therefore be safe for concurrent use.
//
// Example:
//
//	type Packet struct {
//	    data []byte
//	}
//
//	func (p *Packet) Clone() *Packet {
//	    buf := pool.Get().([]byte)
//	    copy(buf, p.data)
//	    return &Packet{data: buf}
//	}
//
//	func (p *Packet) Release() {
//	    pool.Put(p.data)
//	}
//
// With this type, each subscriber receives its own *Packet with an
// independent backing buffer. The caller must release successfully
// delivered packets; the broadcaster releases packets that no subscriber
// accepted.
type Cloner[T any] interface {
	// Clone returns a new value to be delivered to a single subscriber.
	//
	// Clone is called once per subscriber per write. The returned value
	// must be independent of the receiver: mutations to the clone must
	// not affect the original, and vice versa.
	//
	// Clone must be safe for concurrent use.
	Clone() T
}

// Releaser is implemented by values that must be returned to their source
// after use — for example, to a sync.Pool.
//
// When a value produced by Cloner.Clone implements Releaser, its lifecycle
// is split between the broadcaster and the caller:
//
//   - The broadcaster calls Release on clones that were not accepted by
//     any subscriber (buffer full, context canceled, subscriber dispatched
//     before delivery).
//
//   - The caller calls Release on clones that were accepted by a subscriber,
//     after handling them. This is typically done by calling
//     Subscriber.Release, or by invoking Release directly:
//
//     for v := range sub.Read() {
//     handle(v)
//     if r, ok := any(v).(Releaser); ok {
//     r.Release()
//     }
//     }
//
// Failure to release accepted clones may exhaust the underlying pool and
// lead to unbounded allocation growth.
//
// Releaser is intentionally separate from Cloner: a type may implement
// Cloner without implementing Releaser (for example, when Clone simply
// copies a value and the garbage collector handles the clone). Conversely,
// a type may implement Releaser without implementing Cloner, although in
// that case the broadcaster will not call Clone and will not release
// anything automatically.
//
// Release may be called concurrently from multiple goroutines: the
// broadcaster may release clones from a background goroutine while the
// caller releases others. Release must therefore be safe for concurrent
// use.
//
// Releasing the same value more than once, or releasing a value that was
// not produced by Clone, is the caller's responsibility and may result in
// undefined behavior in the Releaser implementation.
type Releaser interface {
	// Release returns the value to its source, for example a sync.Pool.
	//
	// Release is called by the broadcaster for clones that no subscriber
	// accepted, and by the caller for clones that were delivered and
	// handled.
	//
	// Release must be safe for concurrent use.
	Release()
}
