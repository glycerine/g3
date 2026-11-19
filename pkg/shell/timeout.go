package shell

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// TimeoutService belongs to one shell run. The deadline and Close both
// broadcast to every worker through the same channel.
type TimeoutService struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	timer  *control.ScopedTimer
	output io.Writer
	report sync.Once
}

// NewTimeoutService fixes the timeout at construction. Zero and UINT_MAX
// disable the deadline. Close still wakes workers in either case.
func NewTimeoutService(milliseconds uint32, output io.Writer) (*TimeoutService, error) {
	return newTimeoutService(milliseconds, output, nil)
}

func newTimeoutService(milliseconds uint32, output io.Writer, trace *diagnostic.TraceTags) (*TimeoutService, error) {
	if output == nil {
		output = os.Stdout
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	timer, err := control.NewScopedTimerWithMemoryTrace(milliseconds, control.EventHandlerFunc(func(control.EventCaller) {
		cancel(context.DeadlineExceeded)
	}), newTimeoutMemoryTrace(trace))
	if err != nil {
		cancel(nil)
		return nil, err
	}
	return &TimeoutService{ctx: ctx, cancel: cancel, timer: timer, output: output}, nil
}

// DoneChan returns nil when no run timer was supplied to a frontend.
func (s *TimeoutService) DoneChan() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.ctx.Done()
}

func (s *TimeoutService) Caller() control.EventCaller {
	if s != nil && context.Cause(s.ctx) == context.DeadlineExceeded {
		return control.EventCallerTimeout
	}
	return control.EventCallerUnset
}

// ReportTimeout prints once if the deadline expired before Close. The shell
// calls it on the goroutine that owns the output writer.
func (s *TimeoutService) ReportTimeout() {
	if s.Caller() == control.EventCallerTimeout {
		s.report.Do(func() { _, _ = io.WriteString(s.output, "timeout\n") })
	}
}

func (s *TimeoutService) Close() error {
	if s == nil {
		return nil
	}
	s.cancel(nil)
	return s.timer.Close()
}
