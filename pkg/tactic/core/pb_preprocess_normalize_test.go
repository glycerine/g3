package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPBPreprocessNormalizesNegatedGE(t *testing.T) {
	f := newPBPreprocessFixture(t)
	ge := f.ge(f.vars[:2], []string{"2", "3"}, "4")
	negated, e := f.m.MkNot(ge)
	simplifyMust(t, e)
	g := f.goal(negated)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal("normalized one-sided constraint survived", n, e)
	}
}

// tactic_core_02-F1: C++ normalize reads the pb app directly, so a negated
// pbge with non-literal arguments must be rewritten, not crash.
func TestPBPreprocessNormalizesNegatedGEWithCompoundArgs(t *testing.T) {
	f := newPBPreprocessFixture(t)
	and1, e := f.m.MkAnd(f.vars[0], f.vars[1])
	simplifyMust(t, e)
	ge := f.ge([]ast.ASTHandle{and1, f.vars[2]}, []string{"1", "1"}, "2")
	negated, e := f.m.MkNot(ge)
	simplifyMust(t, e)
	g := f.goal(negated)
	f.apply(g)
	notAnd, e := f.m.MkNot(and1)
	simplifyMust(t, e)
	notZ, e := f.m.MkNot(f.vars[2])
	simplifyMust(t, e)
	want := f.ge([]ast.ASTHandle{notAnd, notZ}, []string{"1", "1"}, "1")
	n, e := g.Size()
	simplifyMust(t, e)
	if n != 1 {
		t.Fatal("expected one formula, got", n)
	}
	got, e := g.Form(0)
	simplifyMust(t, e)
	if got != want {
		t.Fatal("normalized constraint differs from (pbge 1 1 1) (not (and x y)) (not z)")
	}
}
