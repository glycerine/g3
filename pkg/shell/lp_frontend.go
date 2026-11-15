package shell

// Source: src/shell/lp_frontend.cpp:56-107; lp_frontend.h.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/util/control"
	lp "github.com/glycerine/g3/pkg/util/lp"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// LPFrontendConfig: a nil Input is an MPS file that could not be opened;
// mps_reader::read then reports it (set_m_ok_to_false). Diagnostics is
// std::cerr, which receives "cannot process" (os.Stderr when nil).
type LPFrontendConfig struct {
	Globals     *uparams.GlobalParamsService
	Input       io.Reader
	Output      io.Writer
	Diagnostics io.Writer
	Name        string
}

type LPFrontend struct {
	globals *uparams.GlobalParamsService
	params  *lp.LPParams
	limit   *control.Reslimit
	timer   *control.ScopedTimer
	reader  *lp.FloatMPSReader
	solver  *lp.FloatPrimalSimplex
	input   io.Reader
	output  io.Writer
	stderr  io.Writer
	name    string
	min     bool
	running bool
	closed  bool
}

func NewLPFrontend(c LPFrontendConfig) (f *LPFrontend, err error) {
	if c.Globals == nil || c.Output == nil {
		return nil, ErrShellOptionService
	}
	diagnostics := c.Diagnostics
	if diagnostics == nil {
		diagnostics = os.Stderr
	}
	f = &LPFrontend{globals: c.Globals, input: c.Input, output: c.Output, stderr: diagnostics, name: c.Name}
	defer func() {
		if err != nil {
			err = errors.Join(err, f.Close())
			f = nil
		}
	}()
	f.params, err = lp.NewLPParams(c.Globals.Context, nil, c.Globals, c.Globals.Context.Budget())
	if err != nil {
		return nil, err
	}
	f.limit, err = control.NewReslimit(c.Globals.Context.Budget())
	return f, err
}

func (f *LPFrontend) Run() (err error) {
	if f == nil || f.closed || f.running {
		return ErrShellOptionService
	}
	f.running = true
	defer func() { f.running = false }()
	if err = f.startLimits(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.stopLimits()) }()
	f.reader = lp.NewFloatMPSReader(f.input)
	if err = f.reader.SetMessageStream(f.output); err != nil {
		return err
	}
	if err = f.reader.Read(); err != nil {
		return f.cannotProcess(err)
	}
	if !f.reader.IsOK() {
		return f.cannotProcess(nil)
	}
	f.solver, err = f.reader.CreatePrimalSolver()
	if err != nil {
		return err
	}
	if err = f.configureSolver(); err != nil {
		return err
	}
	if err = f.solver.FindMaximalSolution(); err != nil {
		return err
	}
	return f.writeLPResult()
}
