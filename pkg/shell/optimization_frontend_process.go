package shell

// Source: src/shell/opt_frontend.cpp timeout and SIGINT handling.

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func runOptimizationFrontendProcess(
	ctx context.Context,
	commands *cmd_context.Context,
	output io.Writer,
	solve func(func() bool) (lbool.LBool, error),
	writeResult func(lbool.LBool) error,
	writeProgress func() error,
) error {
	if commands == nil || solve == nil || writeResult == nil || writeProgress == nil {
		return ErrShellOptionService
	}
	signals, stopSignals := newRunSignals(ctx)
	defer stopSignals()
	previous := commands.SetRunSignals(signals)
	defer commands.SetRunSignals(previous)

	var status lbool.LBool
	var err error
	if !signals.RunStopped() {
		status, err = solve(signals.RunStopped)
	}
	if signals.RunStopped() {
		if reportDeadline(ctx, output) {
			return writeProgress()
		}
		return nil
	}
	if err != nil {
		return err
	}
	if signals.InterruptCount() > 1 {
		stopSignals()
		return errors.Join(writeProgress(), raiseOptimizationFrontendInterrupt())
	}
	if err = writeResult(status); err != nil {
		return err
	}
	if signals.InterruptCount() > 1 {
		stopSignals()
		return errors.Join(writeProgress(), raiseOptimizationFrontendInterrupt())
	}
	return nil
}

func raiseOptimizationFrontendInterrupt() error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return process.Signal(os.Interrupt)
}
