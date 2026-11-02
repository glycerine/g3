package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_eqs_util direct-equality path.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestLegacyArrayProjectionSubstitutesDirectEquality(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-direct-a", arraySort)
	b := f.constant("legacy-direct-b", arraySort)
	i := f.constant("legacy-direct-i", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	equality := f.own(m.MkEq(a, b))
	selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
	valueEquality := f.own(m.MkEq(selected, seven))
	formula := f.own(m.MkAnd(equality, valueEquality))
	base := f.own(m.MkConstArray(arraySort, seven))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: base, b: base, i: one} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, eliminated, err := p.projectDirectEquality(md.Core(), a, formula)
	fmMust(t, err)
	if !eliminated {
		t.Fatal("direct array equality was not eliminated")
	}
	defer func() { fmMust(t, result.Close()) }()
	handle, err := m.RefGet(result.formula)
	fmMust(t, err)
	contains, err := ast.NewContainsApp(m, a)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	remaining, err := contains.Check(handle)
	fmMust(t, err)
	if remaining {
		t.Fatal("direct array substitution retains projected variable")
	}
}

// Source array_project_eqs_util::mk_result applies false-equality replacements
// only when no substitution was found. Once a = b supplies a substitution,
// another model-false equality a = c must become b = c, not false.
func TestLegacyArrayDirectSubstitutionPreservesOtherEqualities(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-direct-map-a", arraySort)
	b := f.constant("legacy-direct-map-b", arraySort)
	c := f.constant("legacy-direct-map-c", arraySort)
	q := f.constant("legacy-direct-map-q", f.boolean)
	ab := f.own(m.MkEq(a, b))
	ac := f.own(m.MkEq(a, c))
	formula := f.own(m.MkAnd(ab, f.own(m.MkOr(ac, q))))
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	zeroArray := f.own(m.MkConstArray(arraySort, zero))
	oneArray := f.own(m.MkConstArray(arraySort, one))
	truth := f.own(m.MkTrue())
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: zeroArray, b: zeroArray, c: oneArray, q: truth} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, eliminated, err := p.projectDirectEquality(md.Core(), a, formula)
	fmMust(t, err)
	if !eliminated {
		t.Fatal("direct array equality was not eliminated")
	}
	defer func() { fmMust(t, result.Close()) }()
	actual, err := m.RefGet(result.formula)
	fmMust(t, err)
	want := f.own(m.MkOr(f.own(m.MkEq(b, c)), q))
	difference := f.own(m.MkXor(actual, want))
	kernel := qsatKernelOwner(t, f)
	solver, err := kernel.solver()
	fmMust(t, err)
	fmMust(t, solver.AssertFormula(difference))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("direct substitution replaced a model-false equality instead of substituting it", status, actual, want)
	}
}
