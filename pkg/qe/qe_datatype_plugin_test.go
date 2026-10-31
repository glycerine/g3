package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestDatatypeSolverPluginNonRecursiveBranches(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x, other := f.constant("qe-datatype-x", d.color), f.constant("qe-datatype-other", d.color)
	truth := f.own(m.MkTrue())
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	p, err := NewDatatypeSolverPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(p))
	t.Cleanup(func() { fmMust(t, ctx.Close()) })
	if got, err := ctx.PluginFor(x); err != nil || got != p {
		t.Fatal("datatype plugin dispatch", got, err)
	}
	branches, supported, err := p.NonRecursiveBranches(x, truth)
	fmMust(t, err)
	if !supported || branches != 2 {
		t.Fatal("enum branches", branches, supported)
	}
	constraint, err := p.NonRecursiveBranchConstraint(x, truth, 1)
	fmMust(t, err)
	condition, err := m.RefGet(constraint)
	fmMust(t, err)
	blueTest, err := m.DatatypeConstructorRecognizer(d.blue)
	fmMust(t, err)
	want := d.app(blueTest, x)
	if condition != want {
		t.Fatal("branch recognizer", condition, want)
	}
	fmMust(t, m.DelRef(&constraint))
	redTest, err := m.DatatypeConstructorRecognizer(d.red)
	fmMust(t, err)
	recognized := d.app(redTest, x)
	branches, supported, err = p.NonRecursiveBranches(x, recognized)
	fmMust(t, err)
	if !supported || branches != 1 {
		t.Fatal("existing recognizer", branches, supported)
	}
	constraint, err = p.NonRecursiveBranchConstraint(x, recognized, 0)
	fmMust(t, err)
	if constraint != (ast.ASTRef{}) {
		t.Fatal("duplicate recognizer constraint")
	}
	wrongVariable := d.app(redTest, other)
	branches, _, err = p.NonRecursiveBranches(x, wrongVariable)
	fmMust(t, err)
	if branches != 2 {
		t.Fatal("recognizer for another variable")
	}
	negated := f.own(m.MkNot(recognized))
	branches, _, err = p.NonRecursiveBranches(x, negated)
	fmMust(t, err)
	if branches != 2 {
		t.Fatal("negated recognizer counted as positive")
	}
	if _, err = p.NonRecursiveBranchConstraint(x, truth, 2); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid branch", err)
	}
	pair := f.constant("qe-datatype-pair", d.pair)
	branches, supported, err = p.NonRecursiveBranches(pair, truth)
	fmMust(t, err)
	if !supported || branches != 1 {
		t.Fatal("single constructor", branches, supported)
	}
	list := f.constant("qe-datatype-list", d.list)
	_, supported, err = p.NonRecursiveBranches(list, truth)
	fmMust(t, err)
	if supported {
		t.Fatal("recursive path must not claim support")
	}
	fmMust(t, ctx.Close())
	if _, _, err = p.NonRecursiveBranches(x, truth); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed plugin", err)
	}
}
