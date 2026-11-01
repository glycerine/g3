package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func generalPurifierFailureFormula(t *testing.T, m *ast.ASTManager, kind string) ast.ASTHandle {
	t.Helper()
	integer := kind == "integer"
	x := fixDLConst(t, m, integer, "failure-x")
	zero := fixDLNumber(t, m, integer, "0")
	var term ast.ASTHandle
	switch kind {
	case "forward":
		term = fixDLApp(t, m, forwardTrigSin, x)
	case "integer":
		y := fixDLConst(t, m, true, "failure-y")
		term = fixDLApp(t, m, qfnraPurifyIDiv, x, y)
	case "inverse":
		term = fixDLApp(t, m, qfnraPurifyASin, x)
	case "root":
		term = rootObjectNumeral(t, m)
	case "quantifier":
		sort, err := m.GetSort(x)
		equationMust(t, err)
		variable, err := m.MkVar(0, sort)
		equationMust(t, err)
		fixDLPin(t, m, variable)
		term = fixDLApp(t, m, qfnraPurifyToInt, variable)
		zero = fixDLNumber(t, m, true, "0")
	}
	formula, err := m.MkEq(term, zero)
	equationMust(t, err)
	fixDLPin(t, m, formula)
	if kind == "quantifier" {
		sort, e := m.GetSort(x)
		equationMust(t, e)
		formula, e = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: formula})
		equationMust(t, e)
		fixDLPin(t, m, formula)
	}
	return formula
}

func TestGeneralPurifierAllocationFrontiers(t *testing.T) {
	for _, tc := range []struct {
		name              string
		proofs, converter bool
		wantSize          uint32
	}{{"forward", false, true, 2}, {"forward", true, true, 2}, {"integer", true, true, 7}, {"inverse", true, true, 4}, {"root", true, true, 4}, {"quantifier", true, true, 1}} {
		success := false
		for stop := int64(0); stop < 256 && !success; stop++ {
			t.Run(fmt.Sprintf("%s/proofs=%v/%d", tc.name, tc.proofs, stop), func(t *testing.T) {
				runner, m := qfnraPurifier(t, tc.proofs, true)
				formula := generalPurifierFailureFormula(t, m, tc.name)
				goal := fixDLGoal(t, m, tc.proofs, true, false, formula)
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				check := func(g *tactic.Goal, out *tactic.TacticResult) {
					size, e := g.Size()
					equationMust(t, e)
					if size != tc.wantSize || (out.ModelConverter != nil) != tc.converter {
						t.Fatalf("unexpected result: %d formulas, converter %v", size, out.ModelConverter)
					}
					if tc.name == "quantifier" {
						q, e := g.Form(0)
						equationMust(t, e)
						inner, e := m.GetQuantifierBody(q)
						equationMust(t, e)
						kind, e := m.GetKind(inner)
						equationMust(t, e)
						if kind != ast.ASTQuantifier {
							t.Fatal("retry lost existential binder")
						}
					}
				}
				m.Budget().FailAfter(stop)
				err := runner.Apply(goal, &result)
				m.Budget().FailAfter(-1)
				if err == nil {
					check(goal, &result)
					success = true
					return
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout && m.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(err)
				}
				retry := fixDLGoal(t, m, tc.proofs, true, false, formula)
				var retried tactic.TacticResult
				t.Cleanup(func() { equationMust(t, retried.Close()) })
				equationMust(t, runner.Apply(retry, &retried))
				check(retry, &retried)
			})
		}
		if !success {
			t.Fatalf("no successful allocation frontier for %s proofs=%v", tc.name, tc.proofs)
		}
	}
}

func TestGeneralPurifierConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 256 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			base, m := fixDLFixture(t, false)
			m.Budget().FailAfter(stop)
			value, err := NewQFNRAPurifyTactic(m, base.context, base.globals, nil, base.services)
			m.Budget().FailAfter(-1)
			if value != nil {
				equationMust(t, value.Close())
			}
			if err == nil {
				success = true
				return
			}
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
		})
	}
	if !success {
		t.Fatal("no successful constructor allocation frontier")
	}
}
