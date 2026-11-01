package shell

// Source: src/shell/smtlib_frontend.cpp:26-89; smtlib_frontend.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/util/stopwatch"
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

	if err := f.context.SetShellDiagnosticOutput(f.diagnostics); err != nil {
		return err
	}
	if err := f.watch.Start(); err != nil {
		return err
	}
	parseErr := smt2.ParseShellCommands(f.context, f.input, f.output)
	stopErr := f.watch.Stop()
	seconds, secondsErr := f.watch.Seconds()
	f.result.DurationSeconds = seconds
	f.result.ParseError = parseErr

	var statisticsErr error
	if f.stats && stopErr == nil && secondsErr == nil {
		statisticsErr = f.writeFinalStatistics(seconds)
	}
	f.result.StatisticsError = statisticsErr
	return errors.Join(parseErr, stopErr, secondsErr, statisticsErr)
}

func (f *SMTLIBFrontend) begin() error {
	if f == nil || f.closed || f.context == nil || f.input == nil || f.output == nil || f.running || f.ran {
		return ErrShellOptionService
	}
	f.running = true
	return nil
}
