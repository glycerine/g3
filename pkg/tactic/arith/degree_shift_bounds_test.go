package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDegreeShiftEvenBounds(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_proofs", true: "with_proofs"}[proofs], func(t *testing.T) {
			_, m := fixDLFixture(t, proofs)
			x := fixDLConst(t, m, false, "even-x")
			y := fixDLConst(t, m, false, "odd-y")
			power := func(base ast.ASTHandle, n string) ast.ASTHandle {
				return fixDLApp(t, m, 18, base, fixDLNumber(t, m, false, n))
			}
			plan, err := degreeShiftPrepare(m, []ast.ASTHandle{power(x, "4"), power(y, "3")}, proofs)
			equationMust(t, err)
			defer func() { equationMust(t, plan.Close()) }()
			goal := fixDLGoal(t, m, proofs, false, false)
			equationMust(t, degreeShiftAssertBounds(plan, goal, proofs))
			size, err := goal.Size()
			equationMust(t, err)
			if size != 1 {
				t.Fatalf("got %d bounds, want one", size)
			}
			bound, err := goal.Form(0)
			equationMust(t, err)
			decl, err := m.GetDecl(bound)
			equationMust(t, err)
			kind, err := m.GetDeclKind(decl)
			equationMust(t, err)
			if kind != 3 {
				t.Fatal("bound is not a greater-than-or-equal comparison")
			}
			left, err := m.GetArg(bound, 0)
			equationMust(t, err)
			if left != mustFreshFor(t, m, plan, x) {
				t.Fatal("bound uses the wrong variable")
			}
			right, err := m.GetArg(bound, 1)
			equationMust(t, err)
			value, _, found, err := m.ArithmeticNumeralText(right)
			equationMust(t, err)
			if !found || value != "0" {
				t.Fatalf("bound is not against zero: %s", value)
			}
			if proofs {
				proof, e := goal.Proof(0)
				equationMust(t, e)
				fact, e := m.GetProofFact(proof)
				equationMust(t, e)
				if fact != bound {
					t.Fatal("bound proof has the wrong fact")
				}
			}
		})
	}
}

func mustFreshFor(t *testing.T, m *ast.ASTManager, plan *degreeShiftPlan, original ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	for _, entry := range plan.entries {
		old, err := m.RefGet(entry.original)
		equationMust(t, err)
		if old == original {
			fresh, err := m.RefGet(entry.fresh)
			equationMust(t, err)
			return fresh
		}
	}
	t.Fatal("missing substitution")
	return ast.ASTHandle{}
}
