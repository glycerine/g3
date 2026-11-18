package opt

// Source: src/opt/maxres.cpp process_unsat(exprs const&).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) processCore(core []ast.ASTHandle) (err error) {
	if err := e.coreStartVerbose(core); err != nil {
		return err
	}
	// rational w = split_core(core)
	weight, err := e.splitCore(core)
	defer func() { err = errors.Join(err, e.q().Del(&weight)) }()
	if err != nil {
		return err
	}
	if err = e.coreDisplayVerbose(core); err != nil {
		return err
	}
	if err = e.maxResolve(core, &weight); err != nil {
		return err
	}
	if err = e.assertCoreBlock(core); err != nil {
		return err
	}
	return e.q().Add(&e.lower, &weight, &e.lower)
}
