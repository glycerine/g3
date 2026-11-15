package api

// Source: src/api/api_ast.cpp:671-697, static simplify: the context's
// timeout default, a bare th_rewriter (no solver, no rlimit scope), the
// cancel_eh and context::set_interruptable scope, scoped_ctrl_c and
// scoped_timer around the rewrite.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/control"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// simplifyTimer is the scoped_timer constructor; tests replace it to see
// the timeout simplify passes.
var simplifyTimer = control.NewScopedTimer

func (c *SMTContext) simplifyWithAPIInterrupt(expression ast.ASTHandle, parameters *uparams.ParamsRef) (result ast.ASTRef, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	globals, err := c.command.GlobalParameters()
	if err != nil {
		return result, err
	}
	params := globals.Context
	local := parameters
	empty := params.EmptyValue()
	if local == nil {
		local = &empty
	}
	defaultTimeout, err := c.contextTimeout()
	if err != nil {
		return result, err
	}
	timeoutKey, ctrlCKey := "timeout", "ctrl_c"
	timeout, err := params.GetUIntName(local, &timeoutKey, defaultTimeout)
	if err != nil {
		return result, err
	}
	ctrlC, err := params.GetBoolName(local, &ctrlCKey, false)
	if err != nil {
		return result, err
	}
	rw, err := rewriter.NewTheoryRewriter(manager, params, globals, parameters)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, rw.Close()) }()
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
		}
	}()
	handler := newSMTContextCancelHandler(manager.ResourceLimit())
	scope, err := newSMTContextInterruptScope(c, handler)
	if err != nil {
		handler.close()
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		handler.close()
	}()
	interrupt, err := control.NewScopedCtrlC(handler, false, ctrlC)
	if err != nil {
		return result, err
	}
	timer, err := simplifyTimer(timeout, handler)
	if err != nil {
		return result, errors.Join(err, interrupt.Close())
	}
	err = rw.Rewrite(expression, result)
	err = errors.Join(err, timer.Close(), interrupt.Close())
	if err != nil {
		return result, err
	}
	published = true
	return result, nil
}
