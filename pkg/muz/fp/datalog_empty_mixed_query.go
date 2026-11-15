package fp

// Source: src/muz/rel/rel_context.cpp rel_context::query. An empty program
// obtains each requested relation independently, so a heterogeneous set of
// finite and infinite outputs in a rule-free context does not require a
// whole-program relation plugin merely to decide whether any output is nonempty.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (e *PropositionalEngine) queryEmptyMixedDatalog(declarations []ast.ASTHandle) (lbool.LBool, bool, error) {
	if e.udocDatalog != nil || e.externalIntervalDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil || e.boundIntervalDatalog != nil {
		return lbool.Undef, false, nil
	}
	count, err := e.set.Len()
	if err != nil || count != 0 {
		return lbool.Undef, false, err
	}
	hasFinite, hasInfinite, nonempty := false, false, false
	for _, declaration := range declarations {
		state := e.predicates[declaration]
		if state == nil || state.external != nil {
			return lbool.Undef, false, nil
		}
		if state.table == nil {
			hasInfinite = true
			continue
		}
		size, err := state.table.Len()
		if err != nil {
			return lbool.Undef, false, err
		}
		hasFinite = true
		nonempty = nonempty || size != 0
	}
	if len(declarations) != 0 && (!hasFinite || !hasInfinite) {
		return lbool.Undef, false, nil
	}
	if len(declarations) == 0 {
		return lbool.True, true, nil
	}
	status, err := e.queryEmptyDatalogProgram(declarations, lbool.FromBool(nonempty))
	return status, true, err
}

// queryEmptyDatalogProgram runs rel_context::query's saturation of a
// rule-free program whose outputs include relations without finite tables:
// the transformation pipeline and its cancellation checks run as for any
// query, compilation yields empty instruction blocks, and the answer is
// whether some output relation is nonempty.
func (e *PropositionalEngine) queryEmptyDatalogProgram(declarations []ast.ASTHandle, answer lbool.LBool) (lbool.LBool, error) {
	e.datalogEmptyAnswer = &answer
	defer func() { e.datalogEmptyAnswer = nil }()
	return e.queryFiniteDatalogWithRestarts(declarations)
}

// finishEmptyDatalogRound is the remainder of rel_context::saturate for an
// empty program after transform_rules: no instructions are executed, the
// cancellation check after execution still runs, and the round completes.
func (e *PropositionalEngine) finishEmptyDatalogRound(round *datalogRestartRound) (lbool.LBool, error) {
	if !round.restarting {
		if err := e.resetNegationDependentTables(); err != nil {
			return lbool.Undef, err
		}
	}
	e.compiled = false
	e.datalogStats.Reset()
	canceled, err := e.manager.Canceled()
	if err != nil {
		return lbool.Undef, err
	}
	if canceled {
		return lbool.Undef, z3exception.NewDefaultException(e.manager.ResourceLimit().CancelMessage())
	}
	if err = e.recordDatalogTransformedRules(); err != nil {
		return lbool.Undef, err
	}
	return *e.datalogEmptyAnswer, nil
}
