package spacer

// Source: spacer_context.cpp derivation::create_next_child strengthens the
// active premise using a selected fact and projects its new-state auxiliaries.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestDerivationNextChildProjectsSelectedFactAuxiliary(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	p, m := f.context.transformers[f.query], f.context.manager
	if len(p.reachFacts) != 0 {
		t.Fatal("recursive fixture unexpectedly has an initial fact")
	}
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	prefix, err := m.MkFreshConst("next-child-aux", boolean)
	depthMust(t, err)
	prefixDecl, err := m.GetDecl(prefix)
	depthMust(t, err)
	newDecl, err := p.owner.NewPredicate(prefixDecl)
	depthMust(t, err)
	aux, err := m.MkApp(newDecl, nil)
	depthMust(t, err)
	fact, err := NewReachFact(m, f.rule, aux, []ast.ASTHandle{aux}, false)
	depthMust(t, err)
	added, err := p.AddReachFact(fact)
	depthMust(t, err)
	depthMust(t, fact.Close())
	if !added {
		t.Fatal("missing must fact")
	}
	truth, err := m.MkTrue()
	depthMust(t, err)
	root, err := p.NewProofObligation(nil, 2, 0, truth, nil)
	depthMust(t, err)
	d, err := NewDerivation(root, f.rule, truth, nil)
	depthMust(t, err)
	defer func() { depthMust(t, d.Close()) }()
	depthMust(t, d.AddPremise(p, 2, truth, false, nil))
	depthMust(t, d.AddPremise(p, 3, truth, false, nil))
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	first, err := d.CreateFirstChild(md.Core())
	depthMust(t, err)
	if first == nil {
		t.Fatal("missing first child")
	}
	child, err := d.CreateNextChild()
	depthMust(t, err)
	if child == nil || d.active != 1 || !d.premises[0].Must() {
		t.Fatal("did not advance using selected fact")
	}
	oldDecl, err := p.owner.NewToOld(newDecl, 2)
	depthMust(t, err)
	originAux, err := m.MkApp(oldDecl, nil)
	depthMust(t, err)
	summary, err := d.premises[0].Summary()
	depthMust(t, err)
	variables, err := d.premises[0].Variables()
	depthMust(t, err)
	if summary != originAux || len(variables) != 1 || variables[0] != originAux {
		t.Fatalf("selected summary %v, variables %v, expected %v", summary, variables, originAux)
	}
	transition, err := d.Transition()
	depthMust(t, err)
	for _, a := range []ast.ASTHandle{aux, originAux} {
		occurs, err := m.OccursExpr(a, transition)
		depthMust(t, err)
		if occurs {
			t.Fatal("selected auxiliary survived projection")
		}
	}
	if !m.IsTrue(transition) {
		t.Fatal("expected true projected transition")
	}
}
