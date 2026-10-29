package shell

import (
	"errors"
	"io"
	"time"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrOPBFrontend = errors.New("shell: invalid OPB frontend state")

type OPBFrontendOptions struct{ DisplayStatistics bool }

type OPBFrontend struct {
	context  *cmd_context.Context
	output   io.Writer
	options  OPBFrontendOptions
	status   lbool.LBool
	document opt.OPBDocument
	summary  cmd_context.OPBLoadSummary
	started  time.Time
	elapsed  time.Duration
	finished bool
}

func NewOPBFrontend(context *cmd_context.Context, output io.Writer, options OPBFrontendOptions) (*OPBFrontend, error) {
	if context == nil || output == nil {
		return nil, ErrOPBFrontend
	}
	// OPB terms are integer-valued ITEs over Boolean literals. Select the
	// retained mixed Boolean/LIA strategy before the manager is initialized.
	if err := context.SetLogic("QF_LIA"); err != nil {
		return nil, err
	}
	if _, err := context.Manager(); err != nil {
		return nil, err
	}
	return &OPBFrontend{context: context, output: output, options: options, status: lbool.Undef}, nil
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
