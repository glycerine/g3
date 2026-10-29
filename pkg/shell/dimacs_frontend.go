package shell

// Source: src/shell/dimacs_frontend.cpp:85-140.
import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"io"
	"time"
)

type DIMACSFrontendConfig struct {
	Globals           *uparams.GlobalParamsService
	Input             io.Reader
	Writer            io.Writer
	DisplayStatistics bool
}
type DIMACSFrontend struct {
	globals *uparams.GlobalParamsService
	params  uparams.ParamsRef
	runtime *sat.Runtime
	limit   *control.Reslimit
	solver  *sat.Solver
	input   io.Reader
	output  io.Writer
	stats   bool
	start   time.Time
	closed  bool
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
func (f *DIMACSFrontend) Run() (err error) {
	if f == nil || f.closed || f.solver == nil {
		return ErrShellOptionService
	}
	if err = sat.ParseDIMACS(f.input, f.solver); err != nil {
		return err
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
	status, err := active.Check(assumptions)
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
