package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_project_selects_util.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyArrayProjectionAckermannizesEqualModelIndices(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-ack-array", arraySort)
	i := f.constant("legacy-ack-i", f.integer)
	j := f.constant("legacy-ack-j", f.integer)
	selectI := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
	selectJ := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, j}))
	formula := f.own(m.MkEq(selectI, selectJ))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	base := f.own(m.MkConstArray(arraySort, seven))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{i: one, j: one, a: base} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.projectSelects(md.Core(), []ast.ASTHandle{a}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 1 {
		t.Fatal("select equivalence class count", len(result.variables))
	}
	constant, err := m.RefGet(result.variables[0])
	fmMust(t, err)
	decl, err := m.GetDecl(constant)
	fmMust(t, err)
	value, err := md.Core().ConstantInterpretation(decl)
	fmMust(t, err)
	text, _, numeral, err := m.ArithmeticNumeralText(value)
	fmMust(t, err)
	if !numeral || text != "7" {
		t.Fatal("select value constant model interpretation", text, numeral)
	}
	handle, err := m.RefGet(result.formula)
	fmMust(t, err)
	guard, err := m.GetArg(handle, 0)
	fmMust(t, err)
	expectedGuard := f.own(m.MkEq(i, j))
	if guard != expectedGuard {
		t.Fatal("source collects right select first and uses j as representative", guard, expectedGuard)
	}
	contains, err := ast.NewContainsApp(m, a)
	fmMust(t, err)
	defer func() { fmMust(t, contains.Close()) }()
	hasArray, err := contains.Check(handle)
	fmMust(t, err)
	if hasArray {
		t.Fatal("ackermannized formula retains projected array")
	}
}

func TestLegacyArrayAckermannSharedSelectsAndSortedIndexClasses(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("ack-stack-a", sort)
	i := f.constant("ack-stack-i", f.integer)
	j := f.constant("ack-stack-j", f.integer)
	k := f.constant("ack-stack-k", f.integer)
	nums := make([]ast.ASTHandle, 3)
	data := make([]ast.ASTHandle, 3)
	for n, text := range []string{"0", "1", "2"} {
		nums[n] = f.own(m.ArithmeticIntegerNumeralText(text))
	}
	for n, text := range []string{"10", "20", "30"} {
		data[n] = f.own(m.ArithmeticIntegerNumeralText(text))
	}
	array := f.own(m.MkConstArray(sort, nums[0]))
	for n := range nums {
		array = f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, nums[n], data[n]}))
	}
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: array, i: nums[2], j: nums[0], k: nums[1]} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	var forms []ast.ASTHandle
	for n, index := range []ast.ASTHandle{i, j, k} {
		selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index}))
		forms = append(forms, f.own(m.MkEq(selected, []ast.ASTHandle{data[2], data[0], data[1]}[n])))
	}
	formula := f.own(m.MkAndArray([]ast.ASTHandle{forms[0], forms[1], forms[2], forms[1]}))
	result, err := p.projectSelects(md.Core(), []ast.ASTHandle{a}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 3 {
		t.Fatal("shared select must reuse its value constant", len(result.variables))
	}
	// Source collects j, k, i; sorting the index representatives must not
	// change the already-created value-constant order or their interpretations.
	for n, ref := range result.variables {
		constant, err := m.RefGet(ref)
		fmMust(t, err)
		decl, err := m.GetDecl(constant)
		fmMust(t, err)
		value, err := md.Core().ConstantInterpretation(decl)
		fmMust(t, err)
		if value != data[n] {
			t.Fatal("fresh constant order or interpretation changed", n, value, data[n])
		}
	}
	root, err := m.RefGet(result.formula)
	fmMust(t, err)
	guards := []ast.ASTHandle{f.own(m.MkArithmeticLT(j, k)), f.own(m.MkArithmeticLT(k, i))}
	for n, want := range guards {
		actual, err := m.GetArg(root, uint32(n))
		fmMust(t, err)
		if actual != want {
			t.Fatal("sorted index-class strict chain", n, actual, want)
		}
	}
}

func TestLegacyArrayAckermannBooleanIndexClasses(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	boolean := f.own(m.MkBoolSort())
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), f.integer))
	a := f.constant("boolean-index-array", arraySort)
	i := f.constant("boolean-index-i", boolean)
	j := f.constant("boolean-index-j", boolean)
	selectI := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, i}))
	selectJ := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, j}))
	formula := f.own(m.MkEq(selectI, selectJ))
	truth := f.own(m.MkTrue())
	falsity := f.own(m.MkFalse())
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	array := f.own(m.MkConstArray(arraySort, zero))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: array, i: truth, j: falsity} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.projectSelects(md.Core(), []ast.ASTHandle{a}, formula)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if len(result.variables) != 2 {
		t.Fatal("distinct Boolean indices must create two select values", len(result.variables))
	}
	root, err := m.RefGet(result.formula)
	fmMust(t, err)
	guard, err := m.GetArg(root, 0)
	fmMust(t, err)
	equality := f.own(m.MkEq(i, j))
	want := f.own(m.MkNot(equality))
	if guard != want {
		t.Fatal("distinct Boolean index classes require a disequality")
	}
}
