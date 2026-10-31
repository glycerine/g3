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

func TestDatatypeSolverPluginBranchAssignment(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x := f.constant("qe-recursive-list", d.list)
	truth := f.own(m.MkTrue())
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	p, err := NewDatatypeSolverPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	service := m.NumeralService()
	count, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.CleanupValue(&count)) }()
	ok, err := p.GetNumBranches(contains, truth, &count)
	fmMust(t, err)
	n, err := service.Unsigned(&count)
	fmMust(t, err)
	if !ok || n != 2 {
		t.Fatal("recursive constructor branch count", ok, n)
	}
	branch, err := service.NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, service.CleanupValue(&branch)) }()
	fmMust(t, service.SetUnsigned(&branch, 1))
	fmMust(t, p.Assign(contains, truth, &branch))
	constraints, err := p.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 1 || !constraints[0].UseVariable {
		t.Fatal("branch constraints", len(constraints))
	}
	condition, err := m.RefGet(constraints[0].Formula)
	fmMust(t, err)
	if condition != d.app(d.consTest, x) {
		t.Fatal("recursive constructor recognizer")
	}
	fmMust(t, m.DelRef(&constraints[0].Formula))
	recognized := d.app(d.consTest, x)
	ok, err = p.GetNumBranches(contains, recognized, &count)
	fmMust(t, err)
	if ok {
		t.Fatal("recursive recognized equality path is not yet implemented")
	}
	if err = p.Assign(contains, recognized, &branch); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unsupported recursive assignment", err)
	}
	head := d.app(d.listFields[0], x)
	headEq := f.own(m.MkEq(head, f.number("1")))
	withSelector := f.own(m.MkAnd(recognized, headEq))
	ok, err = p.GetNumBranches(contains, withSelector, &count)
	fmMust(t, err)
	n, err = service.Unsigned(&count)
	fmMust(t, err)
	if !ok || n != 1 {
		t.Fatal("recognized recursive selector", ok, n)
	}
	fmMust(t, service.SetUnsigned(&branch, 0))
	fmMust(t, p.Assign(contains, withSelector, &branch))
	constraints, err = p.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 0 {
		t.Fatal("recognized selector adds no constraint")
	}
	nilTest, err := m.DatatypeConstructorRecognizer(d.nilCtor)
	fmMust(t, err)
	otherRecognizer := d.app(nilTest, x)
	wrongSelector := f.own(m.MkAnd(otherRecognizer, headEq))
	ok, err = p.GetNumBranches(contains, wrongSelector, &count)
	fmMust(t, err)
	if ok {
		t.Fatal("selector from another constructor must not match")
	}
	color := f.constant("qe-enum-color", d.color)
	colorContains, err := ast.NewContainsApp(m, color)
	fmMust(t, err)
	defer func() { fmMust(t, colorContains.Close()) }()
	ok, err = p.GetNumBranches(colorContains, truth, &count)
	fmMust(t, err)
	n, err = service.Unsigned(&count)
	fmMust(t, err)
	if !ok || n != 2 {
		t.Fatal("enum branch count", ok, n)
	}
	fmMust(t, service.SetUnsigned(&branch, 1))
	fmMust(t, p.Assign(colorContains, truth, &branch))
	constraints, err = p.TakeConstraints()
	fmMust(t, err)
	if len(constraints) != 1 {
		t.Fatal("enum branch constraint count", len(constraints))
	}
	condition, err = m.RefGet(constraints[0].Formula)
	fmMust(t, err)
	blueTest, err := m.DatatypeConstructorRecognizer(d.blue)
	fmMust(t, err)
	if condition != d.app(blueTest, color) {
		t.Fatal("enum branch recognizer")
	}
	fmMust(t, m.DelRef(&constraints[0].Formula))
}

func TestDatatypeSolverPluginSubstitutesAndReusesConstructor(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x, y := f.constant("qe-subst-pair-x", d.pair), f.constant("qe-subst-pair-y", d.pair)
	original := f.own(m.MkEq(x, y))
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	p, err := NewDatatypeSolverPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&branch)) }()
	formula, err := m.NewRef(original)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&formula)) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&definition)) }()
	ok, err := p.SubstituteNonRecursive(contains, &branch, formula, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("non-recursive datatype not supported")
	}
	term, err := m.RefGet(definition)
	fmMust(t, err)
	decl, err := m.GetDecl(term)
	fmMust(t, err)
	if decl != d.pairCtor {
		t.Fatal("wrong replacement constructor")
	}
	variables, err := ctx.Variables()
	fmMust(t, err)
	if len(variables) != 3 {
		t.Fatal("fresh constructor fields", len(variables))
	}
	for i := uint32(0); i < 2; i++ {
		arg, e := m.GetArg(term, i)
		fmMust(t, e)
		if arg != variables[i+1] {
			t.Fatal("fresh field order", i)
		}
	}
	rewritten, err := m.RefGet(formula)
	fmMust(t, err)
	if rewritten != f.own(m.MkEq(term, y)) {
		t.Fatal("formula was not substituted")
	}
	fmMust(t, m.SetRef(formula, original))
	ok, err = p.SubstituteNonRecursive(contains, &branch, formula, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("cached substitution unavailable")
	}
	second, err := m.RefGet(definition)
	fmMust(t, err)
	variables, err = ctx.Variables()
	fmMust(t, err)
	if second != term || len(variables) != 5 || variables[3] != variables[1] || variables[4] != variables[2] {
		t.Fatal("cached fresh variable reuse", variables)
	}
}

func TestDatatypeSolverPluginSubstitutesRecognizedConstructor(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	m, f := d.f.r.manager, d.f
	x := f.constant("qe-subst-color", d.color)
	blueTest, err := m.DatatypeConstructorRecognizer(d.blue)
	fmMust(t, err)
	recognized := d.app(blueTest, x)
	contains, err := ast.NewContainsApp(m, x)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	ctx, err := NewSolverContext(m, []ast.ASTHandle{x})
	fmMust(t, err)
	p, err := NewDatatypeSolverPlugin(ctx)
	fmMust(t, err)
	fmMust(t, ctx.AddPlugin(p))
	defer func() { fmMust(t, ctx.Close()) }()
	branch, err := m.NumeralService().NewValue()
	fmMust(t, err)
	defer func() { fmMust(t, m.NumeralService().CleanupValue(&branch)) }()
	formula, err := m.NewRef(recognized)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&formula)) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&definition)) }()
	ok, err := p.SubstituteNonRecursive(contains, &branch, formula, &definition)
	fmMust(t, err)
	if !ok {
		t.Fatal("recognized constructor not supported")
	}
	term, err := m.RefGet(definition)
	fmMust(t, err)
	decl, err := m.GetDecl(term)
	fmMust(t, err)
	if decl != d.blue {
		t.Fatal("recognizer did not select its constructor")
	}
}
