package control

import "os"

// RunSignals is polled by one worker on its owning goroutine. Workers can
// share the same close-only run channel. A nil channel disables that
// signal. The interrupt count belongs to the worker consuming the OS signals.
type RunSignals struct {
	Done       <-chan struct{}
	Interrupt  <-chan os.Signal
	interrupts int
}

func (s *RunSignals) RunStopped() bool {
	select {
	case <-s.Done:
		return true
	default:
		return false
	}
}

func (s *RunSignals) InterruptCount() int {
	for s.interrupts < 2 {
		select {
		case <-s.Interrupt:
			s.interrupts++
		default:
			return s.interrupts
		}
	}
	return s.interrupts
}

func (s *RunSignals) Canceled() bool {
	return (s.Done != nil && s.RunStopped()) || (s.Interrupt != nil && s.InterruptCount() != 0)
}
