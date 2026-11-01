package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDatatypeSolverPluginCollectsBothAtomPolarities(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x := f.constant("qe-both-polarities-x", d.list)
	y := f.constant("qe-both-polarities-y", d.list)
	recognized := d.app(d.consTest, x)
	equality := f.own(m.MkEq(x, y))
	negated := f.own(m.MkNot(equality))
	formula := f.own(m.MkAnd(recognized, f.own(m.MkOr(equality, negated))))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	context, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	defer func() { fmMust(t, context.Close()) }()
	plugin, err := NewDatatypeSolverPlugin(context)
	fmMust(t, err)
	fmMust(t, context.AddPlugin(plugin))
	ok, err := plugin.Prepare(contains, formula, []ast.ASTHandle{recognized, equality, equality}, []ast.ASTHandle{equality, equality})
	fmMust(t, err)
	if !ok {
		t.Fatal("both-polarity equality rejected")
	}
	entry := plugin.equalities[arithmeticPluginKey{variable: x, formula: formula}]
	if len(entry.positive) != 1 || len(entry.negative) != 1 || entry.negative[0].value != y {
		t.Fatal("equality atom lost a polarity or was duplicated")
	}
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&branch)) }()
	fmMust(t, m.NumeralService().SetUnsigned(&branch, 1))
	result, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&definition)) }()
	ok, err = plugin.SubstituteRecursive(contains, &branch, result, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("fallback branch rejected")
	}
	definitionValue, err := m.RefGet(definition)
	fmMust(t, err)
	count, err := m.GetNumArgs(definitionValue)
	fmMust(t, err)
	if count != 1 {
		t.Fatal("diagonal witness did not include the negative equality", count)
	}
	arg, err := m.GetArg(definitionValue, 0)
	fmMust(t, err)
	if arg != y {
		t.Fatal("diagonal witness has the wrong excluded term")
	}
	resultValue, err := m.RefGet(result)
	fmMust(t, err)
	stillContains, err := contains.Check(resultValue)
	fmMust(t, err)
	if stillContains {
		t.Fatal("fallback branch left the variable in the formula")
	}
}

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
	formulaRef, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&formulaRef)) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&definition)) }()
	fmMust(t, m.NumeralService().SetUnsigned(&branch, 0))
	ok, err = p.SubstituteRecursive(contains, &branch, formulaRef, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("prepared equality substitution unsupported")
	}
	selected, err := m.RefGet(definition)
	fmMust(t, err)
	if selected != y {
		t.Fatal("wrong equality definition")
	}
	got, err := m.RefGet(formulaRef)
	fmMust(t, err)
	truth := f.own(m.MkTrue())
	want := f.own(m.MkAnd(truth, f.own(m.MkAnd(f.own(m.MkEq(y, y)), f.own(m.MkNot(f.own(m.MkEq(z, y))))))))
	if got != want {
		t.Fatal("wrong equality substitution")
	}
	fmMust(t, m.SetRef(formulaRef, formula))
	fmMust(t, m.NumeralService().SetUnsigned(&branch, 1))
	ok, err = p.SubstituteRecursive(contains, &branch, formulaRef, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("prepared fallback substitution unsupported")
	}
	got, err = m.RefGet(formulaRef)
	fmMust(t, err)
	no := f.own(m.MkFalse())
	want = f.own(m.MkAnd(truth, f.own(m.MkAnd(no, f.own(m.MkNot(no))))))
	if got != want {
		t.Fatal("wrong fallback substitution")
	}
	diagonal, err := m.RefGet(definition)
	fmMust(t, err)
	if n, e := m.GetNumArgs(diagonal); e != nil || n != 1 {
		t.Fatal("diagonal definition arity", n, e)
	}
	arg, err := m.GetArg(diagonal, 0)
	fmMust(t, err)
	if arg != z {
		t.Fatal("diagonal disequality value")
	}
}

func TestDatatypeSolverPluginAtomOccursCheckAndNestedEquality(t *testing.T) {
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
	nestedAtoms, err := NewArithmeticAtomSet(m)
	fmMust(t, err)
	defer func() { fmMust(t, nestedAtoms.Close()) }()
	fmMust(t, nestedAtoms.AddPositive(nested))
	ok, err = p.PrepareAtoms(contains, nested, nestedAtoms)
	fmMust(t, err)
	if !ok {
		t.Fatal("constructor equality rejected")
	}
	entry = p.equalities[arithmeticPluginKey{variable: x, formula: nested}]
	if entry == nil || len(entry.positive) != 1 {
		t.Fatal("constructor equality not recorded")
	}
	selected := entry.positive[0]
	wantValue := d.app(d.listFields[1], y)
	if selected.value != wantValue {
		t.Fatal("wrong constructor accessor value")
	}
	truth := f.own(m.MkTrue())
	wantGuard := f.own(m.MkAnd(d.app(d.consTest, y), truth))
	if selected.condition != wantGuard {
		t.Fatal("wrong constructor recognizer guard")
	}
	reversed := f.own(m.MkEq(y, term))
	reversedAtoms, err := NewArithmeticAtomSet(m)
	fmMust(t, err)
	defer func() { fmMust(t, reversedAtoms.Close()) }()
	fmMust(t, reversedAtoms.AddPositive(reversed))
	ok, err = p.PrepareAtoms(contains, reversed, reversedAtoms)
	fmMust(t, err)
	if !ok {
		t.Fatal("reversed constructor equality rejected")
	}
	reversedEntry := p.equalities[arithmeticPluginKey{variable: x, formula: reversed}]
	if reversedEntry == nil || len(reversedEntry.positive) != 1 || reversedEntry.positive[0].value != wantValue {
		t.Fatal("reversed constructor equality changed solution")
	}
}

func TestDatatypeSolverPluginNestedConstructorEqualitySubstitution(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x, y := f.constant("qe-nested-x", d.list), f.constant("qe-nested-y", d.list)
	recognized := d.app(d.consTest, x)
	term := d.app(d.consCtor, f.number("1"), x)
	eq := f.own(m.MkEq(term, y))
	formula := f.own(m.MkAnd(recognized, eq))
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
	ok, err := p.PrepareAtoms(contains, formula, atoms)
	fmMust(t, err)
	if !ok {
		t.Fatal("nested constructor equality rejected")
	}
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&branch)) }()
	formulaRef, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&formulaRef)) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&definition)) }()
	ok, err = p.SubstituteRecursive(contains, &branch, formulaRef, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("nested equality substitution unsupported")
	}
	value, err := m.RefGet(definition)
	fmMust(t, err)
	if value != d.app(d.listFields[1], y) {
		t.Fatal("wrong selected accessor")
	}
	truth := f.own(m.MkTrue())
	guard := f.own(m.MkAnd(d.app(d.consTest, y), truth))
	rewritten := f.own(m.MkAnd(truth, f.own(m.MkEq(d.app(d.consCtor, f.number("1"), value), y))))
	want := f.own(m.MkAnd(guard, rewritten))
	got, err := m.RefGet(formulaRef)
	fmMust(t, err)
	if got != want {
		t.Fatal("nested equality guard or substitution differs")
	}
}
