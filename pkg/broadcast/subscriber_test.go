package broadcast

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

var _ = runtime.NumCPU
var _ = time.Now

func newBenchSubscriber(b *testing.B, buffer int) (*channelManagement[int], func()) {
	b.Helper()

	pool := &sync.Pool{}
	pool.New = func() any {
		return newSubscriber[int](pool, func(*channelManagement[int]) {})
	}

	s := pool.Get().(*channelManagement[int])
	s.open(buffer)

	return s, func() { s.Dispatch() }
}

func BenchmarkChannelManagement_Write_Blocking(b *testing.B) {
	for _, buf := range []int{0, 1, 16, 256} {
		b.Run(sizeName(buf), func(b *testing.B) {
			s, cleanup := newBenchSubscriber(b, buf)
			defer cleanup()

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

			wg := new(sync.WaitGroup)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				s.write(ctx, wg, i)
			}

			b.StopTimer()
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkChannelManagement_Write_CtxCanceled(b *testing.B) {
	for _, buf := range []int{0, 1, 16, 256} {
		b.Run(sizeName(buf), func(b *testing.B) {
			s, cleanup := newBenchSubscriber(b, buf)
			defer cleanup()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			wg := new(sync.WaitGroup)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				s.write(ctx, wg, i)
			}
		})
	}
}

func BenchmarkChannelManagement_Write_Dispatched(b *testing.B) {
	for _, buf := range []int{0, 1, 16, 256} {
		b.Run(sizeName(buf), func(b *testing.B) {
			s, cleanup := newBenchSubscriber(b, buf)
			defer cleanup()

			s.Dispatch()

			ctx := context.Background()
			wg := new(sync.WaitGroup)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				s.write(ctx, wg, i)
			}
		})
	}
}

func BenchmarkChannelManagement_WriteNonBlock_Success(b *testing.B) {
	for _, buf := range []int{1, 16, 256, 4096} {
		b.Run(sizeName(buf), func(b *testing.B) {
			s, cleanup := newBenchSubscriber(b, buf)
			defer cleanup()

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
				s.writeNonBlock(i)
			}

			b.StopTimer()
			cancel()
			readerWg.Wait()
		})
	}
}

func BenchmarkChannelManagement_WriteNonBlock_Full(b *testing.B) {
	for _, buf := range []int{0, 1, 16, 256} {
		b.Run(sizeName(buf), func(b *testing.B) {
			s, cleanup := newBenchSubscriber(b, buf)
			defer cleanup()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				s.writeNonBlock(i)
			}
		})
	}
}

func BenchmarkChannelManagement_WriteNonBlock_Dispatched(b *testing.B) {
	for _, buf := range []int{0, 1, 16, 256} {
		b.Run(sizeName(buf), func(b *testing.B) {
			s, cleanup := newBenchSubscriber(b, buf)
			defer cleanup()

			s.Dispatch()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				s.writeNonBlock(i)
			}
		})
	}
}

func BenchmarkChannelManagement_Write_RaceWithDispatch(b *testing.B) {
	ctx := context.Background()
	for _, buf := range []int{0, 1, 16, 256} {
		b.Run(sizeName(buf), func(b *testing.B) {
			subscribersPool := make([]*channelManagement[int], 0, b.N)

			for i := 0; i < b.N; i++ {
				pool := &sync.Pool{}
				pool.New = func() any {
					return newSubscriber[int](pool, func(*channelManagement[int]) {})
				}

				s := pool.Get().(*channelManagement[int])
				s.open(buf)

				subscribersPool = append(subscribersPool, s)
			}

			wg := new(sync.WaitGroup)

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				s := subscribersPool[i]

				wg.Add(2)

				go func() {
					s.write(ctx, wg, i)
				}()

				go func() {
					defer wg.Done()
					s.Dispatch()
				}()

				wg.Wait()
			}
		})
	}
}

func BenchmarkChannelManagement_RLockOnly(b *testing.B) {
	s, cleanup := newBenchSubscriber(b, 0)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.mu.RLock()
		s.mu.RUnlock()
	}
}

func sizeName(n int) string {
	switch n {
	case 0:
		return "unbuffered"
	case 1:
		return "buf_1"
	case 16:
		return "buf_16"
	case 256:
		return "buf_256"
	case 4096:
		return "buf_4096"
	default:
		return "buf_" + itoa(n)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
