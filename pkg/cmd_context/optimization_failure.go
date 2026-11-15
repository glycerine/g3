package cmd_context

// Source: src/cmd_context/cmd_context.cpp check_sat (the opt branch's
// catch); src/shell/opt_frontend.cpp run_opt_problem; src/api/api_opt.cpp
// Z3_optimize_check.

import (
	"errors"

	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (c *Context) handleOptimizationFailure(cause error) (lbool.LBool, error) {
	var fatal z3exception.Z3Error
	if errors.As(cause, &fatal) {
		return lbool.Undef, cause
	}
	var preprocess opt.SATPreprocessError
	if !c.optimizationFrontend && errors.As(cause, &preprocess) {
		return c.publishOptimizationSolverException(preprocess.Err)
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
	if err == nil {
		// m_check_sat_result is the opt context: its collect_statistics
		// reports m_solver, the solver this optimize() set up.
		c.optimizationStatsResult, c.optimizationStatsOptimizer, c.optimizationStatsPareto = c.tacticResult, c.optimizationRunOptimizer, nil
		if c.optimizationRunOptimizer == nil && c.optimizationParetoBackend != nil {
			c.optimizationStatsPareto = c.optimizationParetoBackend
		}
	}
	if err != nil || c.optimizationFrontend {
		// opt_frontend catches the z3_exception and reports it on stderr.
		return status, err
	}
	// cmd_context::check_sat: catch (z3_exception & ex) { throw
	// cmd_exception(ex.msg()); } so the command prints (error "...")
	// instead of a check-sat result.
	return lbool.Undef, z3exception.NewDefaultException(message)
}

// publishOptimizationSolverException is cmd_context::check_sat after an
// exception that escaped optimize() from the inc_sat_solver's preprocessing:
// m_check_sat_result is already the opt context, so a later reason-unknown is
// opt::context::reason_unknown() (its m.canceled() test, then m_solver's
// reason) and statistics are opt::context::collect_statistics; the command
// prints cmd_exception(ex.msg()).
func (c *Context) publishOptimizationSolverException(cause error) (status lbool.LBool, resultErr error) {
	result, err := solver.NewSimpleCheckSatResult(c.manager)
	if err != nil {
		return lbool.Undef, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.SetStatus(lbool.Undef); err != nil {
		return lbool.Undef, err
	}
	reason, err := c.optimizationSolverReasonUnknownCore(false)
	if err != nil {
		return lbool.Undef, err
	}
	if err = result.SetReasonUnknownAfterCancelCheck(reason); err != nil {
		return lbool.Undef, err
	}
	c.tacticResult = result
	c.status, c.checked = lbool.Undef, true
	c.optimizationStatsResult, c.optimizationStatsOptimizer, c.optimizationStatsPareto = result, c.optimizationRunOptimizer, nil
	published = true
	return lbool.Undef, z3exception.NewDefaultException(z3exception.SourceMessage(cause))
}
