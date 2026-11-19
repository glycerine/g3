package shell

import (
	"context"
	"os"
	"os/signal"

	"github.com/glycerine/g3/pkg/util/control"
)

func newRunSignals(ctx context.Context) (*control.RunSignals, func()) {
	interrupt := make(chan os.Signal, 2)
	signal.Notify(interrupt, os.Interrupt)
	return &control.RunSignals{Done: ctx.Done(), Interrupt: interrupt}, func() { signal.Stop(interrupt) }
}
