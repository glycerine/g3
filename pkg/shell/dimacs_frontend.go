package shell

// Source: src/shell/dimacs_frontend.cpp:85-140.
import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type DIMACSFrontendConfig struct {
	Globals *uparams.GlobalParamsService

	Input             io.Reader
	Writer            io.Writer
	DisplayStatistics bool
}
type DIMACSFrontend struct {
	globals *uparams.GlobalParamsService
	params  uparams.ParamsRef
	runtime *sat.Runtime
	limit   *control.Reslimit

	solver *sat.Solver
	input  io.Reader
	output io.Writer
	stats  bool
	start  time.Time
	closed bool
}

func NewDIMACSFrontend(c DIMACSFrontendConfig) (f *DIMACSFrontend, err error) {
	if c.Globals == nil || c.Input == nil || c.Writer == nil {
		return nil, ErrShellOptionService
	}
	f = &DIMACSFrontend{globals: c.Globals, input: c.Input, output: c.Writer, stats: c.DisplayStatistics, start: time.Now()}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
			f = nil
		}
	}()
	f.params, err = c.Globals.SourceGetModuleName("sat")
	if err != nil {
		return nil, err
	}
	key := "produce_models"
	if err = c.Globals.Context.SetBoolName(&f.params, &key, true); err != nil {
		return nil, err
	}
	f.runtime, err = sat.NewRuntime(sat.RuntimeConfig{Budget: c.Globals.Context.Budget(), ParamsContext: c.Globals.Context, Globals: c.Globals})
	if err != nil {
		return nil, err
	}
	f.limit, err = control.NewReslimit(c.Globals.Context.Budget())
	if err != nil {
		return nil, err
	}
	f.solver, err = sat.NewSATSolver(sat.SolverConfig{ParamsContext: c.Globals.Context, Params: f.params, RLimit: f.limit, Budget: c.Globals.Context.Budget(), Runtime: f.runtime})
	return f, err
}
func (f *DIMACSFrontend) Run() error { return f.RunContext(context.Background()) }

func (f *DIMACSFrontend) RunContext(ctx context.Context) error {
	return f.runContext(ctx, false)
}

// RunFileContext checks a one-shot input opened at the start of a file by the shell.
func (f *DIMACSFrontend) RunFileContext(ctx context.Context) error {
	return f.runContext(ctx, true)
}

func (f *DIMACSFrontend) runContext(ctx context.Context, oneShotFile bool) (err error) {
	if f == nil || f.closed || f.solver == nil {
		return ErrShellOptionService
	}
	signals, stopSignals := newRunSignals(ctx)
	defer stopSignals()
	f.limit.RunSignals = signals
	defer func() { f.limit.RunSignals = nil }()
	finishInterrupt := func(s *sat.Solver) error {
		if signals.InterruptCount() == 0 {
			return nil
		}
		if f.stats {
			if e := f.writeStatistics(s); e != nil {
				return e
			}
		}
		stopSignals()
		process, e := os.FindProcess(os.Getpid())
		if e != nil {
			return e
		}
		return process.Signal(os.Interrupt)
	}
	finishTimeout := func(s *sat.Solver) error {
		if !reportDeadline(ctx, f.output) {
			return nil
		}
		if f.stats {
			return f.writeStatistics(s)
		}
		return nil
	}
	if signals.RunStopped() {
		return finishTimeout(f.solver)
	}
	if oneShotFile && !diagnostic.TraceCompiled() {
		count, value := f.probeUniformModel()
		if signals.InterruptCount() != 0 {
			return finishInterrupt(f.solver)
		}
		if signals.RunStopped() {
			return finishTimeout(f.solver)
		}
		if value != lbool.Undef {
			if err := writeShellText(f.output, "sat\n"); err != nil {
				return err
			}
			model := make(sat.Model, int(count)+1)
			for i := range model {
				model[i] = value
			}
			if err := writeDIMACSModel(f.output, model); err != nil {
				return err
			}
			if f.stats {
				return f.writeStatistics(f.solver)
			}
			return nil
		}
	}
	if err = sat.ParseDIMACS(f.input, f.solver); err != nil {
		return err
	}
	if signals.InterruptCount() != 0 {
		return finishInterrupt(f.solver)
	}
	if signals.RunStopped() {
		return finishTimeout(f.solver)
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(20)
	if err != nil {
		return err
	}
	if verbose {
		out, e := diagnostic.BorrowProcessVerboseStream()
		if e != nil {
			return e
		}
		if e = f.solver.DisplayStatus(out); e != nil {
			return e
		}
	}
	core, err := f.coreEnabled()
	if err != nil {
		return err
	}
	active := f.solver
	var tracking [][]sat.Literal
	var assumptions []sat.Literal
	if core {
		active, tracking, assumptions, err = f.trackedSolver()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, active.Close()) }()
	}
	status := lbool.Undef
	if !core && !diagnostic.TraceCompiled() {
		var contradiction bool
		contradiction, err = active.ProbeClauseCapacity()
		if contradiction {
			status = lbool.False
		}
	}
	if err == nil && status == lbool.Undef {
		status, err = active.Check(assumptions)
	}
	if signals.InterruptCount() != 0 {
		return finishInterrupt(active)
	}
	if signals.RunStopped() {
		return finishTimeout(active)
	}
	if err != nil {
		return err
	}
	if err = f.writeResult(active, status, tracking); err != nil {
		return err
	}
	if f.stats {
		return f.writeStatistics(active)
	}
	return nil
}
func (f *DIMACSFrontend) Close() (err error) {
	if f == nil || f.closed {
		return nil
	}
	f.closed = true
	if f.solver != nil {
		err = errors.Join(err, f.solver.Close())
		f.solver = nil
	}
	if f.limit != nil {
		f.limit.Close()
		f.limit = nil
	}
	if f.runtime != nil {
		err = errors.Join(err, f.runtime.Close())
		f.runtime = nil
	}
	if f.globals != nil {
		err = errors.Join(err, f.globals.Context.Del(&f.params))
	}
	return err
}
