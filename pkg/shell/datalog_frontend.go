package shell

// Source: src/shell/datalog_frontend.cpp retained file/directory dispatch,
// parsing, saturation timing, and frontend result lifecycle.

import (
	"errors"
	"io"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrDatalogFrontend = errors.New("shell: invalid Datalog frontend state")

type DatalogPredicateResult struct {
	Declaration ast.ASTHandle
	Name        string
	Status      lbool.LBool
}

type DatalogFrontend struct {
	context                             *cmd_context.Context
	output                              io.Writer
	path                                string
	loaded, saturated                   bool
	started, parseFinished, runFinished time.Time
	results                             []DatalogPredicateResult
}

func NewDatalogFrontend(context *cmd_context.Context, output io.Writer) (*DatalogFrontend, error) {
	if context == nil || output == nil {
		return nil, ErrDatalogFrontend
	}
	if _, err := context.Manager(); err != nil {
		return nil, err
	}
	return &DatalogFrontend{context: context, output: output}, nil
}

func (f *DatalogFrontend) Results() ([]DatalogPredicateResult, error) {
	if f == nil || !f.saturated {
		return nil, ErrDatalogFrontend
	}
	return append([]DatalogPredicateResult(nil), f.results...), nil
}
