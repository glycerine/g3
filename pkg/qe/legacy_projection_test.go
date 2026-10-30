package qe

// Sources: src/muz/spacer/spacer_legacy_mbp.cpp;
// src/muz/spacer/spacer_util.cpp qe_project legacy path.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyProjectionCombinesBooleanArrayAndArithmetic(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	b := f.constant("legacy-combined-bool", f.boolean)
	x := f.constant("legacy-combined-int", f.integer)
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-combined-array-a", arraySort)
	c := f.constant("legacy-combined-array-c", arraySort)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	three := f.own(m.ArithmeticIntegerNumeralText("3"))
	truth := f.own(m.MkTrue())
	base := f.own(m.MkConstArray(arraySort, zero))
	arrayEquality := f.own(m.MkEq(a, c))
	lower := f.own(m.MkArithmeticGE(x, zero))
	upper := f.own(m.MkArithmeticLE(x, three))
	formula := f.own(m.MkAndArray([]ast.ASTHandle{b, arrayEquality, lower, upper}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{b: truth, x: one, a: base, c: base} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.Project(md.Core(), []ast.ASTHandle{b, a, x}, formula, false, true)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	remaining, err := result.Variables()
	fmMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("combined legacy projection retained variables", len(remaining))
	}
	handle, err := result.Formula()
	fmMust(t, err)
	for _, variable := range []ast.ASTHandle{b, a, x} {
		contains, err := ast.NewContainsApp(m, variable)
		fmMust(t, err)
		found, err := contains.Check(handle)
		fmMust(t, err)
		fmMust(t, contains.Close())
		if found {
			t.Fatal("combined legacy projection retains input variable")
		}
	}
}

func TestLegacyProjectionRunsLiteBeforeTheoryDispatch(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sortName, err := m.Symbols.Intern("legacy-lite-value")
	fmMust(t, err)
	valueSort := f.own(m.MkUninterpretedSort(sortName, nil))
	x := f.constant("legacy-lite-x", valueSort)
	value := f.constant("legacy-lite-constant", valueSort)
	predicate := f.predicate("legacy-lite-predicate", x)
	formula := f.own(m.MkAndArray([]ast.ASTHandle{f.own(m.MkEq(x, value)), predicate}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	result, err := p.Project(md.Core(), []ast.ASTHandle{x}, formula, false, true)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	remaining, err := result.Variables()
	fmMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("qe_lite retained an equality-defined variable")
	}
	got, err := result.Formula()
	fmMust(t, err)
	want := f.predicate("legacy-lite-predicate", value)
	if got != want {
		t.Fatal("qe_lite preprocessing result", got, want)
	}
}

// Source: spacer_util.cpp qe_project repeats qe_lite on array auxiliaries.
func TestLegacyProjectionEliminatesBooleanArrayAuxiliarySymbolically(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.boolean))
	a := f.constant("round-array", sort)
	b := f.constant("round-bool", f.boolean)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	falsity := f.own(m.MkFalse())
	arrays, err := m.ArrayFamilyID()
	fmMust(t, err)
	selectTerm := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, zero}))
	formula := f.own(m.MkEq(selectTerm, b))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	base := f.own(m.MkConstArray(sort, falsity))
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, b: falsity} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.Project(md.Core(), []ast.ASTHandle{a}, formula, false, true)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	root, err := result.Formula()
	fmMust(t, err)
	truth := m.IsTrue(root)
	if !truth {
		t.Fatal("array auxiliary must be eliminated by equality before model substitution", root)
	}
	remaining, err := result.Variables()
	fmMust(t, err)
	if len(remaining) != 0 {
		t.Fatal("array auxiliary remained", remaining)
	}
}
