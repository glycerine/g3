package shell

import (
	"errors"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
	"time"
)

var ErrWCNFFrontend = errors.New("shell: invalid WCNF frontend state")

type WCNFFrontendOptions struct{ DisplayStatistics bool }
type WCNFFrontend struct {
	context  *cmd_context.Context
	output   io.Writer
	options  WCNFFrontendOptions
	status   lbool.LBool
	document opt.WCNFDocument
	summary  cmd_context.WCNFLoadSummary
	started  time.Time
	elapsed  time.Duration
	finished bool
}

func NewWCNFFrontend(c *cmd_context.Context, output io.Writer, options WCNFFrontendOptions) (*WCNFFrontend, error) {
	if c == nil || output == nil {
		return nil, ErrWCNFFrontend
	}
	if _, err := c.Manager(); err != nil {
		return nil, err
	}
	return &WCNFFrontend{context: c, output: output, options: options, status: lbool.Undef}, nil
}
func (f *WCNFFrontend) Status() (lbool.LBool, error) {
	if f == nil || !f.finished {
		return lbool.Undef, ErrWCNFFrontend
	}
	return f.status, nil
}
func (f *WCNFFrontend) Document() (opt.WCNFDocument, error) {
	if f == nil || !f.finished {
		return opt.WCNFDocument{}, ErrWCNFFrontend
	}
	return f.document.Copy(), nil
}
func (f *WCNFFrontend) Summary() (cmd_context.WCNFLoadSummary, error) {
	if f == nil || !f.finished {
		return cmd_context.WCNFLoadSummary{}, ErrWCNFFrontend
	}
	return f.summary, nil
}
