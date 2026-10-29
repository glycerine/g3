package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestRootObjectPurifyProofAndModelFilter(t *testing.T) {
	r, m, _ := rootObjectFixture(t, true, true)
	root := rootObjectNumeral(t, m)
	x := fixDLConst(t, m, false, "root-proof-x")
	formula, e := m.MkEq(root, x)
	equationMust(t, e)
	fixDLPin(t, m, formula)
	g := rootObjectGoal(t, m, true, true, formula)
	out := rootObjectApply(t, r, g)
	if out.ModelConverter == nil {
		t.Fatal("missing root model filter")
	}
	rewritten, e := g.Form(0)
	equationMust(t, e)
	proof, e := g.Proof(0)
	equationMust(t, e)
	fact, e := m.GetProofFact(proof)
	equationMust(t, e)
	if fact != rewritten {
		t.Fatal("root proof conclusion")
	}
	fresh, e := m.GetArg(rewritten, 0)
	equationMust(t, e)
	decl, e := m.GetDecl(fresh)
	equationMust(t, e)
	md, e := model.NewModel(m)
	equationMust(t, e)
	zero := fixDLNumber(t, m, false, "0")
	equationMust(t, md.CoreField.RegisterConstant(decl, zero))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	equationMust(t, out.ModelConverter.ApplyModel(&ref, 0))
	converted, e := ref.Get()
	equationMust(t, e)
	present, e := converted.HasInterpretation(decl)
	equationMust(t, e)
	if present {
		t.Fatal("root witness leaked")
	}
}
func TestRootObjectPurifyLifecycleAndTranslation(t *testing.T) {
	r, m, _ := rootObjectFixture(t, false, true)
	translated, e := r.Translate(m)
	equationMust(t, e)
	equationMust(t, translated.(*RootObjectPurifyTactic).Close())
	equationMust(t, r.IncRef())
	equationMust(t, r.DecRef())
	if e = r.open(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("retired root tactic", e)
	}
}
