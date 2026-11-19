package shell

import (
	"os"
	"os/signal"

	"github.com/glycerine/g3/pkg/util/control"
)

func newRunSignals(timer *TimeoutService) (*control.RunSignals, func()) {
	interrupt := make(chan os.Signal, 2)
	signal.Notify(interrupt, os.Interrupt)
	return &control.RunSignals{Done: timer.DoneChan(), Interrupt: interrupt}, func() { signal.Stop(interrupt) }
}
