package cmd_context

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCommandContextQFLRACombinedSolverProofResult(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_LRA"))
	contextMust(t, c.SetBoolOption("produce-proofs", true))
	real, err := c.LookupSort("Real")
	contextMust(t, err)
	_, err = c.DeclareFunction("x", nil, real)
	contextMust(t, err)
	x, err := c.MakeApp("x", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	zero, err := c.manager.ArithmeticRealNumeralText("0")
	contextMust(t, err)
	one, err := c.manager.ArithmeticRealNumeralText("1")
	contextMust(t, err)
	eq, err := c.manager.MkEq(x, zero)
	contextMust(t, err)
	contextMust(t, c.Assert(eq, ""))
	lower, err := c.MakeApp(">=", []ast.ASTHandle{x, one}, nil, ast.ASTHandle{})
	contextMust(t, err)
	contextMust(t, c.Assert(lower, ""))
	if _, ok := c.solver.(*solver.CombinedSolver); !ok {
		t.Fatal("QF_LRA did not select the combined solver")
	}
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.False {
		t.Fatal("combined strategy failed", status)
	}
	proof, err := c.solver.Proof()
	contextMust(t, err)
	fact, err := c.manager.GetProofFact(proof)
	contextMust(t, err)
	if !c.manager.IsFalse(fact) {
		t.Fatal("missing contradiction proof")
	}
}

func TestCommandContextQFLRAFixedPivotSurvivesCallerUpdates(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_LRA"))
	m, err := c.Manager()
	contextMust(t, err)
	real, err := c.LookupSort("Real")
	contextMust(t, err)
	for _, name := range []string{"x", "y"} {
		_, err = c.DeclareFunction(name, nil, real)
		contextMust(t, err)
	}
	x, err := c.MakeApp("x", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	y, err := c.MakeApp("y", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	two, err := m.ArithmeticRealNumeralText("2")
	contextMust(t, err)
	twiceY, err := c.MakeApp("*", []ast.ASTHandle{two, y}, nil, ast.ASTHandle{})
	contextMust(t, err)
	for i, term := range []ast.ASTHandle{y, twiceY} {
		sum, e := c.MakeApp("+", []ast.ASTHandle{x, term}, nil, ast.ASTHandle{})
		contextMust(t, e)
		text := "1"
		if i == 1 {
			text = "3"
		}
		rhs, e := m.ArithmeticRealNumeralText(text)
		contextMust(t, e)
		bound, e := c.MakeApp(">=", []ast.ASTHandle{sum, rhs}, nil, ast.ASTHandle{})
		contextMust(t, e)
		contextMust(t, c.Assert(bound, ""))
	}
	p, err := c.config.Params.NewValue()
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, c.config.Params.Del(&p)) })
	for _, name := range []string{"auto_config", "arith.greatest_error_pivot"} {
		contextMust(t, c.config.Params.SetBoolName(&p, &name, false))
	}
	selected, err := smtlogics.NewQFLRATactic(m, &p, c.config)
	contextMust(t, err)
	contextMust(t, selected.IncRef())
	t.Cleanup(func() { contextMust(t, selected.DecRef()) })
	// Both the constructor's false override and later updates must lose to
	// the source factory's innermost fixed greatest-error setting.
	for i := 0; i < 2; i++ {
		contextMust(t, selected.ResetStatistics())
		contextMust(t, selected.UpdateParams(&p))
		status, e := c.CheckSatUsing(selected, &p)
		contextMust(t, e)
		if status != lbool.True {
			t.Fatal("QF_LRA result", status)
		}
		if pivots := statistics.Sums(selected.CollectStatistics)["arith pivots"]; pivots != 1 {
			t.Fatal("fixed pivot policy lost", pivots)
		}
	}
}
