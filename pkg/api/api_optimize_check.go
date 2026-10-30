package api

// Source: src/api/api_opt.cpp:139-181.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (o *APIOptimize) Check(assumptions ...ast.ASTHandle) (lbool.LBool, error) {
	if err := o.open(); err != nil {
		return lbool.Undef, err
	}
	return o.command.CheckSat(assumptions)
}

func (o *APIOptimize) ReasonUnknown() (string, error) {
	if err := o.open(); err != nil {
		return "", err
	}
	return o.command.ReasonUnknown()
}
