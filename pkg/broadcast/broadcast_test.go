package broadcast

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	routinesAmount = 2000
)

func Test_BroadcastNonBlockNotBuffered(t *testing.T) {
	b := New[int]()
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	if _, ok := b.(*subscribersManagement[int]); !ok {
		t.Errorf("type assertion of %v failed", b)
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	wg.Add(routinesAmount) // routines init

	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			var failed bool

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)

				failed = true
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)

				failed = true
			}

			if !failed {
				successReadsCounter.Add(1)
			}

			wg.Done()
		}(b.Subscribe(), i)
	}

	wg.Wait()              // wait for routines to init
	wg.Add(routinesAmount) // read init

	b.WriteNonBlock(routinesAmount)

	wg.Wait() // wait for routines to read copied data

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d", successReadsCounter.Load())
	}
}

func Test_BroadcastBlockNotBuffered(t *testing.T) {
	b := New[int]()
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	if _, ok := b.(*subscribersManagement[int]); !ok {
		t.Errorf("type assertion of %v failed", b)
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	wg.Add(routinesAmount) // routines init

	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			var failed bool

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)

				failed = true
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)

				failed = true
			}

			if !failed {
				successReadsCounter.Add(1)
			}

			wg.Done()
		}(b.Subscribe(), i)
	}

	wg.Wait()              // wait for routines to init
	wg.Add(routinesAmount) // read init

	b.WriteBlock(routinesAmount)

	wg.Wait() // wait for routines to read copied data

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d", successReadsCounter.Load())
	}
}

func Test_BroadcastCtxNotBuffered(t *testing.T) {
	b := New[int]()
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	if _, ok := b.(*subscribersManagement[int]); !ok {
		t.Errorf("type assertion of %v failed", b)
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	wg.Add(routinesAmount) // routines init

	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			var failed bool

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)

				failed = true
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)

				failed = true
			}

			if !failed {
				successReadsCounter.Add(1)
			}

			wg.Done()
		}(b.Subscribe(), i)
	}

	wg.Wait()              // wait for routines to init
	wg.Add(routinesAmount) // read init

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()

	b.WriteCtx(ctx, routinesAmount)

	wg.Wait() // wait for routines to read copied data

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d", successReadsCounter.Load())
	}
}

func Test_BroadcastNonBlockBuffered(t *testing.T) {
	b := New[int](1)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	if _, ok := b.(*subscribersManagement[int]); !ok {
		t.Errorf("type assertion of %v failed", b)
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	wg.Add(routinesAmount) // routines init

	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			var failed bool

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)

				failed = true
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)

				failed = true
			}

			if !failed {
				successReadsCounter.Add(1)
			}

			wg.Done()
		}(b.Subscribe(), i)
	}

	wg.Wait()              // wait for routines to init
	wg.Add(routinesAmount) // read init

	b.WriteNonBlock(routinesAmount)

	wg.Wait() // wait for routines to read copied data

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d", successReadsCounter.Load())
	}
}

func Test_BroadcastBlockBuffered(t *testing.T) {
	b := New[int](1)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	if _, ok := b.(*subscribersManagement[int]); !ok {
		t.Errorf("type assertion of %v failed", b)
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	wg.Add(routinesAmount) // routines init

	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			var failed bool

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)

				failed = true
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)

				failed = true
			}

			if !failed {
				successReadsCounter.Add(1)
			}

			wg.Done()
		}(b.Subscribe(), i)
	}

	wg.Wait()              // wait for routines to init
	wg.Add(routinesAmount) // read init

	b.WriteBlock(routinesAmount)

	wg.Wait() // wait for routines to read copied data

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d", successReadsCounter.Load())
	}
}

func Test_BroadcastCtxBuffered(t *testing.T) {
	b := New[int](1)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	if _, ok := b.(*subscribersManagement[int]); !ok {
		t.Errorf("type assertion of %v failed", b)
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	wg.Add(routinesAmount) // routines init

	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			var failed bool

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)

				failed = true
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)

				failed = true
			}

			if !failed {
				successReadsCounter.Add(1)
			}

			wg.Done()
		}(b.Subscribe(), i)
	}

	wg.Wait()              // wait for routines to init
	wg.Add(routinesAmount) // read init

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	defer cancelFunc()

	b.WriteCtx(ctx, routinesAmount)

	wg.Wait() // wait for routines to read copied data

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d", successReadsCounter.Load())
	}
}
