package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util store-chain path.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestLegacyArrayProjectionSubstitutesStoreEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-store-a", arraySort)
	b := f.constant("legacy-store-b", arraySort)
	i := f.constant("legacy-store-i", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	storedA := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, seven}))
	storeEquality := f.own(m.MkEq(storedA, b))
	base := f.own(m.MkConstArray(arraySort, zero))
	storedBase := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, i, seven}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, b: storedBase, i: one} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, eliminated, err := p.projectStoreEquality(md.Core(), a, storeEquality)
	fmMust(t, err)
	if !eliminated {
		t.Fatal("store-chain array equality was not eliminated")
	}
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 1 {
		t.Fatal("store-chain diff constant count", len(result.variables))
	}
	handle, err := m.RefGet(result.formula)
	fmMust(t, err)
	contains, err := ast.NewContainsApp(m, a)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	remaining, err := contains.Check(handle)
	fmMust(t, err)
	if remaining {
		t.Fatal("store-chain substitution retains projected variable")
	}
}

// Source array_project_eqs_util::mk_result leaves model-false equalities in
// the formula after a store equality supplies the array substitution. The
// substituted equality can hold in another model even when q is false; the
// old eager false replacement reduced this projection to q.
func TestLegacyArrayStoreSubstitutionPreservesOtherEqualities(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-store-map-a", arraySort)
	b := f.constant("legacy-store-map-b", arraySort)
	c := f.constant("legacy-store-map-c", arraySort)
	i := f.constant("legacy-store-map-i", f.integer)
	q := f.constant("legacy-store-map-q", f.boolean)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	storedA := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, seven}))
	storeEquality := f.own(m.MkEq(storedA, b))
	otherEquality := f.own(m.MkEq(a, c))
	formula := f.own(m.MkAnd(storeEquality, f.own(m.MkOr(otherEquality, q))))
	base := f.own(m.MkConstArray(arraySort, zero))
	storedBase := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, one, seven}))
	different := f.own(m.MkConstArray(arraySort, one))
	truth := f.own(m.MkTrue())
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, b: storedBase, c: different, i: one, q: truth} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, eliminated, err := p.projectStoreEquality(md.Core(), a, formula)
	fmMust(t, err)
	if !eliminated {
		t.Fatal("store-chain array equality was not eliminated")
	}
	defer func() { fmMust(t, result.Close()) }()
	actual, err := m.RefGet(result.formula)
	fmMust(t, err)
	notQ := f.own(m.MkNot(q))
	witness := f.own(m.MkAnd(actual, notQ))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(witness))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("store substitution collapsed the other equality to its source-model value", status, actual)
	}
}

func TestLegacyArrayProjectionFactorsSelectInsidePartialEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-partial-select-a", arraySort)
	b := f.constant("legacy-partial-select-b", arraySort)
	k := f.constant("legacy-partial-select-k", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	index := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, k}))
	storedA := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, index, seven}))
	formula := f.own(m.MkEq(storedA, b))
	base := f.own(m.MkConstArray(arraySort, zero))
	storedBase := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, seven}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, b: storedBase, k: one} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, eliminated, err := p.projectStoreEquality(md.Core(), a, formula)
	fmMust(t, err)
	if !eliminated {
		t.Fatal("partial-equality select case was not eliminated")
	}
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 2 {
		t.Fatal("partial-equality select and diff constants", len(result.variables))
	}
	handle, err := m.RefGet(result.formula)
	fmMust(t, err)
	contains, err := ast.NewContainsApp(m, a)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	remaining, err := contains.Check(handle)
	fmMust(t, err)
	if remaining {
		t.Fatal("factored partial equality retains projected array")
	}
}
