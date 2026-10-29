package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func rootObjectFixture(t *testing.T, proofs, elim bool) (*RootObjectPurifyTactic, *ast.ASTManager, *FixDLVarTactic) {
	t.Helper()
	base, m := fixDLFixture(t, proofs)
	p, e := base.context.NewValue()
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, base.context.Del(&p)) })
	name := "elim_root_objects"
	equationMust(t, base.context.SetBoolName(&p, &name, elim))
	r, e := NewRootObjectPurifyTactic(m, base.context, base.globals, &p, base.services)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r, m, base
}
func rootObjectNumeral(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	am, e := m.ArithmeticAlgebraicManager()
	equationMust(t, e)
	twoInt := unumeral.NewInt(2)
	var two, root polynomial.Algebraic
	equationMust(t, am.SetInteger(&two, &twoInt))
	equationMust(t, am.Root(&two, 2, &root))
	equationMust(t, am.Del(&two))
	h, e := m.ArithmeticAlgebraicNumeral(&root)
	equationMust(t, e)
	equationMust(t, am.Del(&root))
	fixDLPin(t, m, h)
	return h
}
func rootObjectGoal(t *testing.T, m *ast.ASTManager, proofs, models bool, forms ...ast.ASTHandle) *tactic.Goal {
	t.Helper()
	g, e := tactic.NewGoal(m, models, proofs, false)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, g.Close()) })
	for _, f := range forms {
		if proofs {
			p, e := m.MkProofAsserted(f)
			equationMust(t, e)
			equationMust(t, g.AssertWithProof(f, p, ast.ASTDependency{}))
		} else {
			equationMust(t, g.Assert(f))
		}
	}
	return g
}
func rootObjectApply(t *testing.T, r *RootObjectPurifyTactic, g *tactic.Goal) *tactic.TacticResult {
	t.Helper()
	out := &tactic.TacticResult{}
	t.Cleanup(func() { equationMust(t, out.Close()) })
	equationMust(t, r.Apply(g, out))
	return out
}
