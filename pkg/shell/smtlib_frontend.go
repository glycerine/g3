package shell

// Source: src/shell/smtlib_frontend.cpp:26-89; smtlib_frontend.h.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// SMTLIBFrontendConfig describes one SMT-LIB command stream. Context is
// borrowed when supplied. When it is nil, the frontend constructs and owns a
// standalone command context, as the source shell does for each invocation.
type SMTLIBFrontendConfig struct {
	Context           *cmd_context.Context
	Input             io.Reader
	Output            io.Writer
	Diagnostics       io.Writer
	DisplayStatistics bool
	Timer             *TimeoutService
	Clock             func() (uint64, error)
}

// SMTLIBFrontend owns the shell-level lifetime around the SMT2 parser. The
// parser owns command-local references; Context owns declarations, assertions,
// solver state and results until Close.
type SMTLIBFrontend struct {
	context     *cmd_context.Context
	input       io.Reader
	output      io.Writer
	diagnostics io.Writer
	inputClose  io.Closer
	watch       stopwatch.Stopwatch
	timer       *TimeoutService
	stats       bool
	owned       bool
	running     bool
	ran         bool
	closed      bool
	result      SMTLIBFrontendResult
}

func NewSMTLIBFrontend(c SMTLIBFrontendConfig) (f *SMTLIBFrontend, err error) {
	if c.Input == nil || c.Output == nil {
		return nil, ErrShellOptionService
	}
	f = &SMTLIBFrontend{
		context:     c.Context,
		input:       c.Input,
		output:      c.Output,
		diagnostics: c.Diagnostics,
		stats:       c.DisplayStatistics,
		watch:       stopwatch.New(c.Clock),
	}
	if f.context == nil {
		f.context, err = cmd_context.NewStandaloneContext()
		if err != nil {
			return nil, err
		}
		f.owned = true
	}
	// read_smtlib2_commands: ctx.set_interpolating_solver_factory(mk_smt_solver_factory()).
	f.context.SetInterpolatingSMTSolverFactory()
	// cmd_context construction installs dl_context, whose smt_params snapshot
	// smt.timeout for the Datalog soft timeout.
	if err = f.context.CaptureFixedpointSoftTimeout(); err != nil {
		if f.owned {
			err = errors.Join(err, f.context.Close())
		}
		return nil, err
	}
	f.timer = c.Timer
	return f, nil
}

// Run executes exactly one command stream. Re-running a consumed stream is an
// ownership error rather than silently replaying commands into mutated state.
func (f *SMTLIBFrontend) Run() (result error) {
	if err := f.begin(); err != nil {
		return err
	}
	defer func() {
		f.running = false
		f.ran = true
		f.result.Completed = true
	}()

	// The source timeout handler exits the process: nothing the parser
	// writes after the timeout fired reaches the output.
	signals, stopSignals := newRunSignals(f.timer)
	defer stopSignals()
	// The context may retain its output transports after this run returns.
	defer func() { signals.Done = nil }()
	previous := f.context.SetRunSignals(signals)
	defer f.context.SetRunSignals(previous)
	if err := f.context.SetShellDiagnosticOutput(timeoutGateWriter(f.diagnostics, signals)); err != nil {
		return err
	}
	if err := f.watch.Start(); err != nil {
		return err
	}
	parseErr := smt2.ParseShellCommandsWithStop(f.context, f.input, timeoutGateWriter(f.output, signals), signals.Canceled)
	stopErr := f.watch.Stop()
	seconds, secondsErr := f.watch.Seconds()
	f.result.DurationSeconds = seconds
	f.result.ParseError = parseErr
	stopped := signals.RunStopped()
	if stopped && f.timer.Caller() != control.EventCallerTimeout {
		return errors.Join(stopErr, secondsErr)
	}
	if stopped {
		f.timer.ReportTimeout()
	}

	var statisticsErr error
	// A source exit(status) inside a command skips the final statistics.
	var processExit *z3exception.ProcessExit
	exited := errors.As(parseErr, &processExit)
	if f.stats && stopErr == nil && secondsErr == nil && !exited {
		statisticsErr = f.writeFinalStatistics(seconds)
	}
	f.result.StatisticsError = statisticsErr
	if signals.InterruptCount() != 0 {
		stopSignals()
		process, signalErr := os.FindProcess(os.Getpid())
		if signalErr == nil {
			signalErr = process.Signal(os.Interrupt)
		}
		return errors.Join(parseErr, stopErr, secondsErr, statisticsErr, signalErr)
	}
	if stopped {
		// The C++ timeout callback prints the requested statistics and exits
		// successfully from the process; parser cancellation is only the Go-side
		// mechanism for unwinding the active command.
		return errors.Join(stopErr, secondsErr, statisticsErr)
	}
	return errors.Join(parseErr, stopErr, secondsErr, statisticsErr)
}

func (f *SMTLIBFrontend) begin() error {
	if f == nil || f.closed || f.context == nil || f.input == nil || f.output == nil || f.running || f.ran {
		return ErrShellOptionService
	}
	f.running = true
	return nil
}

// gatedWriter drops every write once the timeout fired.
type gatedWriter struct {
	w       io.Writer
	signals *control.RunSignals
}

func timeoutGateWriter(w io.Writer, signals *control.RunSignals) io.Writer {
	if w == nil || signals.Done == nil {
		return w
	}
	return &gatedWriter{w: w, signals: signals}
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	if g.signals.RunStopped() {
		return len(p), nil
	}
	return g.w.Write(p)
}
