package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func forwardTrigFixture(t *testing.T, proofs bool) (*ForwardTrigPurifyTactic, *ast.ASTManager, *FixDLVarTactic) {
	t.Helper()
	base, m := fixDLFixture(t, proofs)
	r, e := NewForwardTrigPurifyTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r, m, base
}
func forwardTrigGoal(t *testing.T, m *ast.ASTManager, proofs, models bool, forms ...ast.ASTHandle) *tactic.Goal {
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
func forwardTrigApply(t *testing.T, r *ForwardTrigPurifyTactic, g *tactic.Goal) *tactic.TacticResult {
	t.Helper()
	out := &tactic.TacticResult{}
	t.Cleanup(func() { equationMust(t, out.Close()) })
	equationMust(t, r.Apply(g, out))
	return out
}
func forwardTrigEq(t *testing.T, m *ast.ASTManager, term ast.ASTHandle, text string) ast.ASTHandle {
	t.Helper()
	h, e := m.MkEq(term, fixDLNumber(t, m, false, text))
	equationMust(t, e)
	fixDLPin(t, m, h)
	return h
}
