package shell

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrOPBFrontend = errors.New("shell: invalid OPB frontend state")

type OPBFrontendOptions struct {
	DisplayStatistics bool
	Timer             *TimeoutService
	Diagnostics       io.Writer
}

type OPBFrontend struct {
	context     *cmd_context.Context
	output      io.Writer
	options     OPBFrontendOptions
	timer       *TimeoutService
	diagnostics io.Writer
	initialized bool
	status      lbool.LBool
	document    opt.OPBDocument
	summary     cmd_context.OPBLoadSummary
	started     time.Time
	elapsed     time.Duration
	finished    bool
}

func NewOPBFrontend(context *cmd_context.Context, output io.Writer, options OPBFrontendOptions) (*OPBFrontend, error) {
	if context == nil || output == nil {
		return nil, ErrOPBFrontend
	}
	// opt_frontend.cpp parse_opt: a bare ast_manager with reg_decl_plugins
	// and a plain opt::context; no logic is set and no cmd_context solver
	// exists.
	if !context.UseDatalogFrontendManager() {
		return nil, ErrOPBFrontend
	}
	timer := options.Timer
	diagnostics := options.Diagnostics
	if diagnostics == nil {
		diagnostics = os.Stderr
	}
	return &OPBFrontend{context: context, output: output, options: options, timer: timer, diagnostics: diagnostics, status: lbool.Undef}, nil
}

func (f *OPBFrontend) Status() (lbool.LBool, error) {
	if f == nil || !f.finished {
		return lbool.Undef, ErrOPBFrontend
	}
	return f.status, nil
}

func (f *OPBFrontend) Document() (opt.OPBDocument, error) {
	if f == nil || !f.finished {
		return opt.OPBDocument{}, ErrOPBFrontend
	}
	return f.document.Copy(), nil
}

func (f *OPBFrontend) Summary() (cmd_context.OPBLoadSummary, error) {
	if f == nil || !f.finished {
		return cmd_context.OPBLoadSummary{}, ErrOPBFrontend
	}
	return f.summary, nil
}
