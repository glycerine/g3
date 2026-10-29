package arith

import (
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestForwardTrigPurifyProofAndModelReconstruction(t *testing.T) {
	r, m, _ := forwardTrigFixture(t, true)
	theta := fixDLConst(t, m, false, "model-theta")
	sin := fixDLApp(t, m, forwardTrigSin, theta)
	g := forwardTrigGoal(t, m, true, true, forwardTrigEq(t, m, sin, "0"))
	out := forwardTrigApply(t, r, g)
	if out.ModelConverter == nil {
		t.Fatal("missing basis model converter")
	}
	formula, e := g.Form(0)
	equationMust(t, e)
	proof, e := g.Proof(0)
	equationMust(t, e)
	fact, e := m.GetProofFact(proof)
	equationMust(t, e)
	if fact != formula {
		t.Fatal("basis proof conclusion")
	}
	fresh, e := m.GetArg(formula, 0)
	equationMust(t, e)
	freshDecl, e := m.GetDecl(fresh)
	equationMust(t, e)
	thetaDecl, e := m.GetDecl(theta)
	equationMust(t, e)
	md, e := model.NewModel(m)
	equationMust(t, e)
	zero := fixDLNumber(t, m, false, "0")
	equationMust(t, md.CoreField.RegisterConstant(freshDecl, zero))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	equationMust(t, out.ModelConverter.ApplyModel(&ref, 0))
	converted, e := ref.Get()
	equationMust(t, e)
	present, e := converted.HasInterpretation(thetaDecl)
	equationMust(t, e)
	if !present {
		t.Fatal("theta not reconstructed")
	}
	present, e = converted.HasInterpretation(freshDecl)
	equationMust(t, e)
	if present {
		t.Fatal("basis witness not filtered")
	}
}
