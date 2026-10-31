package qe

// Source: src/qe/qe_tactic.cpp, the concrete qe_nonlinear parameter path.

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type quantifierTacticOptions struct{ nonlinear bool }

// update reads a new parameter snapshot; the source uses false when this
// parameter is absent, even when an earlier update enabled it.
func (o *quantifierTacticOptions) update(context *uparams.ParamsContext, p *uparams.ParamsRef) error {
	if o == nil || context == nil || p == nil {
		return ast.ErrASTHandle
	}
	key := "qe_nonlinear"
	value, err := context.GetBoolName(p, &key, false)
	if err != nil {
		return err
	}
	o.nonlinear = value
	return nil
}

func collectQuantifierTacticOptions(descriptions *uparams.ParamDescrs) error {
	if descriptions == nil {
		return ast.ErrASTHandle
	}
	text := "(default: false) enable virtual term substitution."
	return descriptions.InsertName("qe_nonlinear", uparams.ParamBool, &text, nil, nil)
}
