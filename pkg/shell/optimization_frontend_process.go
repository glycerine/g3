package shell

// Source: src/shell/opt_frontend.cpp timeout and SIGINT handling.

import (
	"errors"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func runOptimizationFrontendProcess(
	context *cmd_context.Context,
	timer *TimeoutService,
	solve func(func() bool) (lbool.LBool, error),
	writeResult func(lbool.LBool) error,
	writeProgress func() error,
) error {
	if context == nil || solve == nil || writeResult == nil || writeProgress == nil {
		return ErrShellOptionService
	}
	signals, stopSignals := newRunSignals(timer)
	defer stopSignals()
	previous := context.SetRunSignals(signals)
	defer context.SetRunSignals(previous)

	var status lbool.LBool
	var err error
	if !signals.RunStopped() {
		status, err = solve(signals.RunStopped)
	}
	if signals.RunStopped() {
		if timer.Caller() == control.EventCallerTimeout {
			timer.ReportTimeout()
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
