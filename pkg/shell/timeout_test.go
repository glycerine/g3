package shell

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestShellTimeoutBroadcast(t *testing.T) {
	for _, test := range []struct {
		name       string
		ms         uint32
		closeEarly bool
		want       error
	}{
		{"expires", 5, false, context.DeadlineExceeded},
		{"close-before-expiry", 60_000, true, context.Canceled},
		{"close-without-deadline", 0, true, context.Canceled},
		{"close-with-disabled-deadline", math.MaxUint32, true, context.Canceled},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := CommandLine{TimeoutMilliseconds: test.ms}
			ctx, cancel := command.WithTimeout(context.Background())
			defer cancel()
			done := ctx.Done()
			const workers = 16
			stopped := make(chan struct{}, workers)
			for i := 0; i < workers; i++ {
				go func() {
					<-done
					stopped <- struct{}{}
				}()
			}
			if test.closeEarly {
				cancel()
			}
			guard := time.After(2 * time.Second)
			for i := 0; i < workers; i++ {
				select {
				case <-stopped:
				case <-guard:
					t.Fatal("worker missed cancellation")
				}
			}
			select {
			case <-ctx.Done():
			default:
				t.Fatal("late worker missed cancellation")
			}
			cancel()
			if ctx.Err() != test.want {
				t.Fatalf("run ended with %v, want %v", ctx.Err(), test.want)
			}
		})
	}
}
