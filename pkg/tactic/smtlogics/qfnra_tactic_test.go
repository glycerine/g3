package smtlogics_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md tactic_small_02-F3: mk_qfnra_sat_solver passes
// nla2bv_max_bv_size only as constructor params; nla2bv's updt_params
// replaces them, so after updt_params(empty) no bit-vector cap remains.
func TestQFNRABoundedRouteNla2bvParamsNotSticky(t *testing.T) {
	apply := func(update bool) error {
		c, err := cmd_context.NewStandaloneContext()
		qfaufliaMust(t, err)
		t.Cleanup(func() { qfaufliaMust(t, c.Close()) })
		m, err := c.Manager()
		qfaufliaMust(t, err)
		config := ackrConfig(t, c)
		params, err := config.Params.NewValue()
		qfaufliaMust(t, err)
		key := "nla2bv_max_bv_size"
		qfaufliaMust(t, config.Params.SetUIntName(&params, &key, 4))
		route, err := smtlogics.QFNRASATSolverRouteForTest(m, &params, config)
		qfaufliaMust(t, err)
		qfaufliaMust(t, config.Params.Del(&params))
		qfaufliaMust(t, route.IncRef())
		t.Cleanup(func() { qfaufliaMust(t, route.DecRef()) })
		if update {
			empty := config.Params.EmptyValue()
			qfaufliaMust(t, route.UpdateParams(&empty))
		}
		x, err := m.MkConst(qfnraSymbol(t, m, "bounded-x"), qfnraRealSort(t, m))
		qfaufliaMust(t, err)
		qfaufliaPin(t, m, x)
		term, err := m.MkArithmeticMul(x, x)
		qfaufliaMust(t, err)
		qfaufliaPin(t, m, term)
		two, err := m.ArithmeticRealNumeralText("2")
		qfaufliaMust(t, err)
		eq, err := m.MkEq(term, two)
		qfaufliaMust(t, err)
		qfaufliaPin(t, m, eq)
		goal, err := tactic.NewGoal(m, false, false, false)
		qfaufliaMust(t, err)
		t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
		qfaufliaMust(t, goal.Assert(eq))
		var result tactic.TacticResult
		t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
		return route.Apply(goal, &result)
	}
	// (= (* x x) 2.0): with the 4-bit cap the under-approximation is unsat
	// and fail-if-undecided fails; without the cap x = sqrt(2) is found.
	if capped := apply(false); capped == nil {
		t.Fatal("constructor nla2bv_max_bv_size 4 was not applied")
	}
	if updated := apply(true); updated != nil {
		t.Fatal("updt_params kept nla2bv_max_bv_size", updated)
	}
}

func qfnraSymbol(t *testing.T, m *ast.ASTManager, name string) usymbol.Symbol {
	t.Helper()
	s, err := m.Symbols.Intern(name)
	qfaufliaMust(t, err)
	return s
}

func qfnraRealSort(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	zero, err := m.ArithmeticRealNumeralText("0")
	qfaufliaMust(t, err)
	sort, err := m.GetSort(zero)
	qfaufliaMust(t, err)
	return sort
}
