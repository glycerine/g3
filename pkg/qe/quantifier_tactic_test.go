package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifierTacticQuantifierFreeGoalAndRefusal(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	port, err := NewQuantifierTactic(m, nil, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, port.Close()) }()
	p := f.constant("qe-free-p", f.boolean)
	goal := f.goal(false, false)
	fmMust(t, goal.Assert(p))
	out := f.result()
	m.ResourceLimit().Cancel()
	err = port.Apply(goal, out)
	m.ResourceLimit().ResetCancel()
	if err == nil {
		t.Fatal("canceled goal succeeded")
	}
	fmMust(t, port.Apply(goal, out))
	depth, err := goal.Depth()
	fmMust(t, err)
	if depth != 1 {
		t.Fatal("goal depth", depth)
	}
	size, err := goal.Size()
	fmMust(t, err)
	if size != 1 {
		t.Fatal("quantifier-free goal changed")
	}
	h, err := goal.Form(0)
	fmMust(t, err)
	if h != p {
		t.Fatal("formula changed")
	}
	v := f.own(m.MkVar(0, f.boolean))
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: v}))
	quantified := f.goal(false, false)
	fmMust(t, quantified.Assert(q))
	second := f.result()
	if err = port.Apply(quantified, second); err == nil {
		t.Fatal("unfinished quantifier elimination accepted")
	}
	depth, err = quantified.Depth()
	fmMust(t, err)
	if depth != 0 {
		t.Fatal("refused goal was modified")
	}
}
