package cmd_context

// Source: src/cmd_context/cmd_context.cpp check_sat (the opt branch's
// catch); src/shell/opt_frontend.cpp run_opt_problem; src/api/api_opt.cpp
// Z3_optimize_check.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (c *Context) handleOptimizationFailure(cause error) (lbool.LBool, error) {
	var fatal z3exception.Z3Error
	if errors.As(cause, &fatal) {
		return lbool.Undef, cause
	}
	canceled, err := c.manager.Canceled()
	if err != nil {
		return lbool.Undef, errors.Join(cause, err)
	}
	message := ""
	var source z3exception.Z3Exception
	if canceled {
		message = c.manager.ResourceLimit().CancelMessage()
	} else if errors.As(cause, &source) {
		message = source.Message()
		c.optimizationException = message
	} else {
		return lbool.Undef, cause
	}
	status, err := c.publishOptimizationUnknown(message)
	if err != nil || c.optimizationFrontend {
		// opt_frontend catches the z3_exception and reports it on stderr.
		return status, err
	}
	// cmd_context::check_sat: catch (z3_exception & ex) { throw
	// cmd_exception(ex.msg()); } so the command prints (error "...")
	// instead of a check-sat result.
	return lbool.Undef, z3exception.NewDefaultException(message)
}
