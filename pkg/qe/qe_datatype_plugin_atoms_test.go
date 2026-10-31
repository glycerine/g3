package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDatatypeSolverPluginDirectEqualityBranches(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x, y, z := f.constant("qe-atoms-x", d.list), f.constant("qe-atoms-y", d.list), f.constant("qe-atoms-z", d.list)
	recognized := d.app(d.consTest, x)
	eq := f.own(m.MkEq(x, y))
	neq := f.own(m.MkEq(z, x))
	formula := f.own(m.MkAnd(recognized, f.own(m.MkAnd(eq, f.own(m.MkNot(neq))))))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	p, err := NewDatatypeSolverPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	atoms, err := NewArithmeticAtomSet(m)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	fmMust(t, atoms.AddPositive(recognized))
	fmMust(t, atoms.AddPositive(eq))
	fmMust(t, atoms.AddNegative(neq))
	ok, err := p.PrepareAtoms(contains, formula, atoms)
	fmMust(t, err)
	if !ok {
		t.Fatal("direct equalities rejected")
	}
	count, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&count)) }()
	ok, err = p.GetNumBranches(contains, formula, &count)
	fmMust(t, err)
	n, err := m.NumeralService().Unsigned(&count)
	fmMust(t, err)
	if !ok || n != 2 {
		t.Fatal("equality branch count", ok, n)
	}
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&branch)) }()
	fmMust(t, p.Assign(contains, formula, &branch))
	constraints, err := p.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 1 {
		t.Fatal("positive equality constraint", len(constraints))
	}
	h, err := m.RefGet(constraints[0].Formula)
	fmMust(t, err)
	if h != eq {
		t.Fatal("wrong positive equality")
	}
	fmMust(t, m.DelRef(&constraints[0].Formula))
	fmMust(t, m.NumeralService().SetUnsigned(&branch, 1))
	fmMust(t, p.Assign(contains, formula, &branch))
	constraints, err = p.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 1 {
		t.Fatal("fallback constraint count", len(constraints))
	}
	h, err = m.RefGet(constraints[0].Formula)
	fmMust(t, err)
	if h != f.own(m.MkNot(eq)) {
		t.Fatal("wrong fallback constraint")
	}
	fmMust(t, m.DelRef(&constraints[0].Formula))
}

func TestDatatypeSolverPluginAtomOccursCheckAndRefusal(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x, y := f.constant("qe-occurs-x", d.list), f.constant("qe-occurs-y", d.list)
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	p, err := NewDatatypeSolverPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	term := d.app(d.consCtor, f.number("1"), x)
	cyclic := f.own(m.MkEq(x, term))
	atoms, err := NewArithmeticAtomSet(m)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	fmMust(t, atoms.AddPositive(cyclic))
	ok, err := p.PrepareAtoms(contains, cyclic, atoms)
	fmMust(t, err)
	if !ok {
		t.Fatal("constructor occurs check was rejected")
	}
	entry := p.equalities[arithmeticPluginKey{variable: x, formula: cyclic}]
	if entry == nil || len(entry.unsat) != 1 || entry.unsat[0] != cyclic {
		t.Fatal("cyclic equality was not recorded")
	}
	nested := f.own(m.MkEq(term, y))
	unsupported, err := NewArithmeticAtomSet(m)
	fmMust(t, err)
	defer func() { fmMust(t, unsupported.Close()) }()
	fmMust(t, unsupported.AddPositive(nested))
	ok, err = p.PrepareAtoms(contains, nested, unsupported)
	fmMust(t, err)
	if ok {
		t.Fatal("unimplemented nested positive equality accepted")
	}
}
