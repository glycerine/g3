package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestLegacyModelProjectionCandidatesIncludeAnnotationsInSourceOrder(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("order-x", f.integer)
	bodyIndex := f.constant("order-body", f.integer)
	patternIndex := f.constant("order-pattern", f.integer)
	noPatternIndex := f.constant("order-no-pattern", f.integer)
	innerIndex := f.constant("order-inner", f.integer)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("order-array", sort)
	arrays, err := m.ArrayFamilyID()
	fmMust(t, err)
	read := func(index ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index}))
	}
	inner := read(innerIndex)
	nested := read(inner)
	bodyEq := f.own(m.MkEq(x, bodyIndex))
	patternRead := read(patternIndex)
	pattern := f.own(m.MkPattern([]ast.ASTHandle{patternRead}))
	noPatternRead := read(noPatternIndex)
	// The body shares a select with its no-pattern list. Source marking visits
	// that app once, while still visiting distinct apps with the same index.
	body := f.own(m.MkAndArray([]ast.ASTHandle{bodyEq, f.own(m.MkEq(nested, nested))}))
	innerQuantifier := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(0)},
		Body: body, NoPatterns: []ast.ASTHandle{noPatternRead, nested},
	}))
	// Source quantifiers cannot combine patterns and no-patterns. The outer
	// pattern is visited after the inner body's no-pattern annotations.
	formula := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []symbol.Symbol{symbol.Numerical(1)},
		Body: innerQuantifier, Patterns: []ast.ASTHandle{pattern},
	}))
	terms, err := p.modelProjectionTerms(formula, x)
	fmMust(t, err)
	want := []ast.ASTHandle{bodyIndex, innerIndex, inner, noPatternIndex, patternIndex}
	if len(terms) != len(want) {
		t.Fatal("candidate count", len(terms), len(want))
	}
	for i := range want {
		if terms[i] != want[i] {
			t.Fatal("candidate order", i, terms[i], want[i])
		}
	}
}

func TestLegacyModelTermProjectionUsesUpdatedFormulaAfterSwapRemoval(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("sequential-x", f.integer)
	y := f.constant("sequential-y", f.integer)
	z := f.constant("sequential-z", f.integer)
	unused := f.constant("sequential-unused", f.integer)
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	xy := f.own(m.MkEq(x, y))
	yz := f.own(m.MkEq(y, z))
	formula := f.own(m.MkAndArray([]ast.ASTHandle{xy, yz}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for _, variable := range []ast.ASTHandle{x, y, z, unused} {
		value := two
		if variable == unused {
			value = seven
		}
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	current, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&current)) }()
	remaining, err := p.projectModelTerms(md.Core(), current, []ast.ASTHandle{x, unused, y})
	fmMust(t, err)
	if len(remaining) != 1 || remaining[0] != unused {
		t.Fatal("sequential elimination/swap removal", remaining)
	}
	root, err := m.RefGet(current)
	fmMust(t, err)
	zz := f.own(m.MkEq(z, z))
	expected := f.own(m.MkAndArray([]ast.ASTHandle{zz, zz}))
	if root != expected {
		t.Fatal("later projection did not use the substituted formula", root, expected)
	}
	for _, variable := range []ast.ASTHandle{x, y} {
		occurs, err := m.OccursExpr(variable, root)
		fmMust(t, err)
		if occurs {
			t.Fatal("eliminated variable remains", variable)
		}
	}
}
