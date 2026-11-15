package fp

// Source: rel_context.cpp:133-227. A restart restores the query's original
// rules/predicates while keeping the partial original relations and completed
// strata saved by the termination program. This path covers finite compiled
// saturation; explanation and other relational backends have separate drivers.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type datalogRestartRound struct {
	timeout     uint32
	restarting  bool
	interrupted bool
	statistics  statistics.Statistics
	// totals is the engine's m_ectx counter set, which also persists
	// across queries.
	totals *rel.ExecutionStats
}

// Compilation is outside the restart timer. Termination must run with that
// timer disabled so it can publish the partial relation registers. Source
// execution_context::reset preserves its counters across these rounds.
func performDatalogRestartRound(execution, termination *rel.InstructionBlock, context *rel.ExecutionContext, round *datalogRestartRound) (complete, terminated bool, resultErr error) {
	if round != nil && round.timeout != 0 {
		if err := context.SetTimeLimit(round.timeout); err != nil {
			return false, false, err
		}
	}
	complete, err := execution.Perform(context)
	err = errors.Join(err, context.ResetTimeLimit())
	if err != nil {
		return complete, false, err
	}
	terminated, err = termination.Perform(context)
	if err != nil {
		return complete, terminated, err
	}
	if err = execution.ProcessAllCosts(); err != nil {
		return complete, terminated, err
	}
	if round != nil && round.totals != nil {
		executed, statsErr := context.Statistics()
		if statsErr != nil {
			return complete, terminated, statsErr
		}
		round.totals.Add(executed)
		round.statistics.Reset()
		round.totals.Collect(&round.statistics)
	} else if round != nil {
		err = context.CollectStatistics(&round.statistics)
	}
	return complete, terminated, err
}

func (e *PropositionalEngine) datalogInitialRestartTimeout() (timeout uint32, resultErr error) {
	if e.params == nil || e.globals == nil {
		return 0, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	return view.UInt("datalog.initial_restart_timeout", 0)
}

func nextDatalogRestartTimeout(current, initial uint32) uint32 {
	next := uint64(current) * uint64(initial)
	if next > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(next)
}

// SMTModuleTimeout reads smt.timeout (default UINT_MAX) as smt_params does.
func SMTModuleTimeout(context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (timeout uint32, resultErr error) {
	if context == nil || globals == nil {
		return 0, nil
	}
	module, err := globals.SourceGetModuleName("smt")
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&module)) }()
	name := "timeout"
	return context.GetUIntName(&module, &name, math.MaxUint32)
}

func (e *PropositionalEngine) queryFiniteDatalogWithRestarts(declarations []ast.ASTHandle) (lbool.LBool, error) {
	e.datalogTimedOut = false
	initial, err := e.datalogInitialRestartTimeout()
	if err != nil {
		return lbool.Undef, err
	}
	// rel_context::saturate: remaining_time_limit = soft_timeout(),
	// restart_time = initial_restart_timeout(), time_limit = remaining != 0.
	remaining := e.softTimeout
	timeLimit := remaining != 0
	restart := initial
	if e.datalogExecTotals == nil {
		e.datalogExecTotals = &rel.ExecutionStats{}
	}
	round := datalogRestartRound{totals: e.datalogExecTotals}
	conversionBase := len(e.datalogConversions)
	for {
		timeoutAfterThisRound := timeLimit && (restart == 0 || remaining <= restart)
		round.timeout = 0
		if timeLimit || restart != 0 {
			switch {
			case timeLimit && restart != 0:
				round.timeout = min(remaining, restart)
			case timeLimit:
				round.timeout = remaining
			default:
				round.timeout = restart
			}
		}
		status, err := e.queryFiniteDatalogRound(declarations, &round)
		if err != nil || !round.interrupted {
			return status, err
		}
		// Per-round deferred owners have restored the source rule/predicate
		// scope. Its model conversions must also retire before rerunning the
		// transforms; a refused release stays attached to the engine for retry.
		if err = closeHornModelConversions(e.datalogConversions[conversionBase:]); err != nil {
			return lbool.Undef, err
		}
		e.datalogConversions = e.datalogConversions[:conversionBase]
		if timeoutAfterThisRound {
			// m_context.set_status(TIMEOUT); result = l_undef.
			e.datalogTimedOut = true
			return lbool.Undef, nil
		}
		if timeLimit {
			remaining -= restart
		}
		restart = nextDatalogRestartTimeout(restart, initial)
		round.restarting = true
		round.interrupted = false
	}
}
