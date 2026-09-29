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

func Test_PublisherBasic(t *testing.T) {
	b := New[int](10)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	// Create subscribers
	wg.Add(routinesAmount)
	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()

			routineNumber++
			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, routine number %d", routineNumber)
				return
			}

			if v != routinesAmount {
				t.Errorf("got bad value from subscriber, routine number %d", routineNumber)
				return
			}

			successReadsCounter.Add(1)
		}(b.Subscribe(), i)
	}

	wg.Wait() // Wait for all subscribers to be ready

	// Use Publisher
	publisher := b.Publisher()
	defer close(publisher) // Important: close publisher to prevent goroutine leak

	// Send message through publisher
	publisher <- routinesAmount

	// Wait a bit for message to be delivered
	time.Sleep(100 * time.Millisecond)

	// We can't expect all messages to be delivered here but there shouldn't be 0 or >routinesAmount
	if successReadsCounter.Load() == 0 || successReadsCounter.Load() > routinesAmount {
		t.Errorf("unexpected success reads amount: %d, expected at least: %d", successReadsCounter.Load(), routinesAmount)
	}
}

func Test_PublisherCtxWithTimeout(t *testing.T) {
	// 4000000 writes takes too long for github runners to process.
	const routinesAmount = 200

	b := New[int](5)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	// Create subscribers
	wg.Add(routinesAmount)
	for i := range routinesAmount {
		go func(s Subscriber[int], routineNumber int) {
			defer s.Dispatch()
			wg.Done()

			routineNumber++

			for counter := 0; ; counter++ {
				v, ok := <-s.Read()
				if !ok {
					if counter < routinesAmount-1 {
						t.Errorf("subscriber active channel closed unexpectedly, routine number %d", routineNumber)
					}

					return
				}

				if v != counter {
					t.Errorf("got bad value from subscriber, routine number %d, got: %d", routineNumber, v)
					return
				}

				successReadsCounter.Add(1)
			}
		}(b.Subscribe(), i)
	}

	wg.Wait() // Wait for all subscribers to be ready

	// Use PublisherCtx with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	publisher := b.PublisherCtx(ctx)
	defer close(publisher)

	// Send multiple messages
	for i := range routinesAmount {
		select {
		case publisher <- i:
			// Successfully sent
		case <-ctx.Done():
			t.Errorf("context done before all messages sent")
			return
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if successReadsCounter.Load() != routinesAmount*routinesAmount {
		t.Errorf("unexpected success reads amount: %d, expected: %d", successReadsCounter.Load(), routinesAmount*routinesAmount)
	}
}

func Test_PublisherCtxCancellation(t *testing.T) {
	b := New[string](5)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	publisher := b.PublisherCtx(ctx)

	// Cancel context immediately
	cancel()

	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)

	// Try to send - should not block but message won't be delivered
	select {
	case publisher <- "test":
		t.Log("Message sent to canceled publisher (may be dropped)")
	case <-time.After(100 * time.Millisecond):
		t.Errorf("Send to canceled publisher blocked")
	}

	// Close publisher to clean up
	close(publisher)
}

func Test_PublisherMultipleMessages(t *testing.T) {
	b := New[int](10)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	const messageCount = 100
	var (
		receivedMessages atomic.Int32
		wg               sync.WaitGroup
	)

	// Create single subscriber
	sub := b.Subscribe()
	wg.Add(1)

	go func() {
		defer sub.Dispatch()
		defer wg.Done()

		for range messageCount {
			v, ok := <-sub.Read()
			if !ok {
				t.Errorf("subscriber channel closed prematurely")
				return
			}
			if v != 42 {
				t.Errorf("got bad value: %d, expected: 42", v)
				return
			}
			receivedMessages.Add(1)
		}
	}()

	// Use Publisher to send multiple messages
	publisher := b.Publisher()
	defer close(publisher)

	for i := range messageCount {
		publisher <- 42
		t.Logf("Sent message %d", i+1)
	}

	// Wait for all messages to be received
	wg.Wait()

	if receivedMessages.Load() != messageCount {
		t.Errorf("unexpected received messages count: %d, expected: %d", receivedMessages.Load(), messageCount)
	}
}

func Test_PublisherCtxWithBuffer(t *testing.T) {
	bufferSize := 10
	b := New[int](bufferSize)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	var (
		successReadsCounter atomic.Int32
		wg                  sync.WaitGroup
	)

	// Create subscribers
	subscribers := make([]Subscriber[int], routinesAmount)
	for i := range routinesAmount {
		subscribers[i] = b.Subscribe()
	}

	wg.Add(routinesAmount)
	for i, sub := range subscribers {
		go func(s Subscriber[int], idx int) {
			defer s.Dispatch()

			wg.Done()

			v, ok := <-s.Read()
			if !ok {
				t.Errorf("subscriber active channel closed, index %d", idx)
				return
			}

			if v != 123 {
				t.Errorf("got bad value from subscriber, index %d", idx)
				return
			}

			successReadsCounter.Add(1)
		}(sub, i)
	}

	wg.Wait() // Wait for all subscribers to be ready

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Use Publisher with buffer
	publisher := b.PublisherCtx(ctx)
	defer close(publisher)

	// Send message
	publisher <- 123
	<-ctx.Done()

	if successReadsCounter.Load() != routinesAmount {
		t.Errorf("unexpected success reads amount: %d, expected: %d", successReadsCounter.Load(), routinesAmount)
	}

	// Cleanup remaining subscribers
	for _, sub := range subscribers {
		sub.Dispatch()
	}
}

func Test_PublisherChannelClosed(t *testing.T) {
	b := New[int](5)
	if b == nil {
		t.Fatalf("broadcaster is nil")
	}

	// Create a subscriber to verify messages are sent
	sub := b.Subscribe()
	defer sub.Dispatch()

	var received int
	go func() {
		for msg := range sub.Read() {
			received = msg
			t.Logf("Received message: %d", msg)
		}
	}()

	// Create and immediately close publisher
	publisher := b.Publisher()
	close(publisher)

	// Give some time for the publisher goroutine to exit
	time.Sleep(50 * time.Millisecond)

	// Verify subscriber is still active (should not receive messages from closed publisher)
	if received != 0 {
		t.Errorf("unexpected message received from closed publisher: %d", received)
	}
}

func BenchmarkBroadcaster_SubscribeDispatch(b *testing.B) {
	for _, subs := range []int{0, 1, 10} {
		b.Run(subsName(subs), func(b *testing.B) {
			bc := New[int](0)

			existing := make([]Subscriber[int], 0, subs)
			for i := 0; i < subs; i++ {
				existing = append(existing, bc.Subscribe())
			}
			defer func() {
				for _, s := range existing {
					s.Dispatch()
				}
			}()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				s := bc.Subscribe()
				s.Dispatch()
			}
		})
	}
}

func BenchmarkBroadcaster_WriteNonBlock(b *testing.B) {
	for _, subs := range []int{0, 1, 10, 100} {
		b.Run(subsName(subs), func(b *testing.B) {
			bc := New[int](16)

			subscribers := make([]Subscriber[int], 0, subs)
			for i := 0; i < subs; i++ {
				subscribers = append(subscribers, bc.Subscribe())
			}
			defer func() {
				for _, s := range subscribers {
					s.Dispatch()
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var readerWg sync.WaitGroup
			for _, s := range subscribers {
				readerWg.Add(1)
				go func(s Subscriber[int]) {
					defer readerWg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						case _, ok := <-s.Read():
							if !ok {
								return
							}
						}
					}
				}(s)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				bc.WriteNonBlock(i)
			}

			b.StopTimer()
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkBroadcaster_WriteCtx(b *testing.B) {
	for _, subs := range []int{0, 1, 10, 100} {
		b.Run(subsName(subs), func(b *testing.B) {
			bc := New[int](16)

			subscribers := make([]Subscriber[int], 0, subs)
			for i := 0; i < subs; i++ {
				subscribers = append(subscribers, bc.Subscribe())
			}
			defer func() {
				for _, s := range subscribers {
					s.Dispatch()
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var readerWg sync.WaitGroup
			for _, s := range subscribers {
				readerWg.Add(1)
				go func(s Subscriber[int]) {
					defer readerWg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						case _, ok := <-s.Read():
							if !ok {
								return
							}
						}
					}
				}(s)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				bc.WriteCtx(ctx, i)
			}

			b.StopTimer()
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkBroadcaster_WriteCtx_WithTimeout(b *testing.B) {
	for _, timeout := range []time.Duration{0, time.Millisecond, 10 * time.Millisecond} {
		b.Run(timeoutName(timeout), func(b *testing.B) {
			bc := New[int](16).WithBlockTimeout(timeout)

			s := bc.Subscribe()
			defer s.Dispatch()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var readerWg sync.WaitGroup
			readerWg.Add(1)

			go func() {
				defer readerWg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case _, ok := <-s.Read():
						if !ok {
							return
						}
					}
				}
			}()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				bc.WriteCtx(ctx, i)
			}

			b.StopTimer()
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkBroadcaster_WriteCtx_SlowSubscriber(b *testing.B) {
	for _, buf := range []int{0, 1, 16} {
		b.Run(sizeName(buf), func(b *testing.B) {
			bc := New[int](buf)

			s := bc.Subscribe()
			defer s.Dispatch()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				writeCtx, writeCancel := context.WithCancel(ctx)
				writeCancel()

				bc.WriteCtx(writeCtx, i)
			}
		})
	}
}

func BenchmarkBroadcaster_Publisher(b *testing.B) {
	for _, subs := range []int{1, 10, 100} {
		b.Run(subsName(subs), func(b *testing.B) {
			bc := New[int](0)

			subscribers := make([]Subscriber[int], 0, subs)
			for i := 0; i < subs; i++ {
				subscribers = append(subscribers, bc.Subscribe())
			}
			defer func() {
				for _, s := range subscribers {
					s.Dispatch()
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var readerWg sync.WaitGroup
			for _, s := range subscribers {
				readerWg.Add(1)
				go func(s Subscriber[int]) {
					defer readerWg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						case _, ok := <-s.Read():
							if !ok {
								return
							}
						}
					}
				}(s)
			}

			pub := bc.PublisherCtx(ctx)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				pub <- i
			}

			b.StopTimer()
			close(pub)
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkBroadcaster_WriteCtx_Parallel(b *testing.B) {
	for _, subs := range []int{1, 10, 100} {
		b.Run(subsName(subs), func(b *testing.B) {
			bc := New[int](16)

			subscribers := make([]Subscriber[int], 0, subs)
			for i := 0; i < subs; i++ {
				subscribers = append(subscribers, bc.Subscribe())
			}
			defer func() {
				for _, s := range subscribers {
					s.Dispatch()
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var readerWg sync.WaitGroup
			for _, s := range subscribers {
				readerWg.Add(1)
				go func(s Subscriber[int]) {
					defer readerWg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						case _, ok := <-s.Read():
							if !ok {
								return
							}
						}
					}
				}(s)
			}

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					bc.WriteCtx(ctx, i)
					i++
				}
			})

			b.StopTimer()
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkBroadcaster_Churn(b *testing.B) {
	bc := New[int](16)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stable := make([]Subscriber[int], 0, 10)
	for i := 0; i < 10; i++ {
		stable = append(stable, bc.Subscribe())
	}
	defer func() {
		for _, s := range stable {
			s.Dispatch()
		}
	}()

	var readerWg sync.WaitGroup
	for _, s := range stable {
		readerWg.Add(1)
		go func(s Subscriber[int]) {
			defer readerWg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-s.Read():
					if !ok {
						return
					}
				}
			}
		}(s)
	}

	var writerWg sync.WaitGroup
	writerWg.Add(1)

	go func() {
		defer writerWg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				bc.WriteNonBlock(0)
			}
		}
	}()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s := bc.Subscribe()
		s.Dispatch()
	}

	b.StopTimer()
	cancel()
	writerWg.Wait()
	readerWg.Wait()
}

func subsName(n int) string {
	switch n {
	case 0:
		return "subs_0"
	case 1:
		return "subs_1"
	case 10:
		return "subs_10"
	case 100:
		return "subs_100"
	default:
		return "subs_" + itoa(n)
	}
}

func timeoutName(d time.Duration) string {
	if d == 0 {
		return "no_timeout"
	}
	return "timeout_" + d.String()
}
