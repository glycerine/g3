package shell

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestShellTimeoutBroadcast(t *testing.T) {
	for _, test := range []struct {
		name       string
		ms         uint32
		closeEarly bool
		want       string
	}{
		{"expires", 5, false, "timeout\n"},
		{"close-before-expiry", 60_000, true, ""},
		{"close-without-deadline", 0, true, ""},
		{"close-with-disabled-deadline", math.MaxUint32, true, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			timer, err := NewTimeoutService(test.ms, &out)
			if err != nil {
				t.Fatal(err)
			}
			defer timer.Close()
			done := timer.DoneChan()
			if done == nil || done != timer.DoneChan() {
				t.Fatal("unstable worker channel")
			}
			const workers = 16
			stopped := make(chan struct{}, workers)
			for i := 0; i < workers; i++ {
				go func() {
					<-done
					stopped <- struct{}{}
				}()
			}
			if test.closeEarly {
				if err := timer.Close(); err != nil {
					t.Fatal(err)
				}
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
			case <-timer.DoneChan():
			default:
				t.Fatal("late worker missed cancellation")
			}
			if err := timer.Close(); err != nil {
				t.Fatal(err)
			}
			if expired := timer.Caller() == control.EventCallerTimeout; expired == test.closeEarly {
				t.Fatalf("unexpected expiry %v", expired)
			}
			timer.ReportTimeout()
			timer.ReportTimeout()
			if out.String() != test.want {
				t.Fatalf("output %q, want %q", out.String(), test.want)
			}
		})
	}
}
