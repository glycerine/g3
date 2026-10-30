package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// array_select_reducer.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyArrayProjectionReducesModelSelectedStore(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("legacy-reduce-array", arraySort)
	i := f.constant("legacy-reduce-i", f.integer)
	j := f.constant("legacy-reduce-j", f.integer)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	stored := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, seven}))
	selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{stored, j}))
	formula := f.own(m.MkEq(selected, seven))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	for _, variable := range []ast.ASTHandle{i, j} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, one))
	}
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	base := f.own(m.MkConstArray(arraySort, zero))
	aDecl, err := m.GetDecl(a)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(aDecl, base))
	result, err := p.reduceSelects(md.Core(), []ast.ASTHandle{a}, formula, false)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	handle, err := m.RefGet(result.formula)
	fmMust(t, err)
	seen := make(map[ast.ASTHandle]bool)
	work := []ast.ASTHandle{handle}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		_, storeTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpStore)
		fmMust(t, err)
		_, selectTerm, err := p.syntax.operation(current, p.syntax.family, ast.ASTArrayOpSelect)
		fmMust(t, err)
		if storeTerm || selectTerm {
			t.Fatal("selected store chain remains after reduction")
		}
		kind, err := m.GetKind(current)
		fmMust(t, err)
		if kind == ast.ASTApp {
			count, err := m.GetNumArgs(current)
			fmMust(t, err)
			for index := uint32(0); index < count; index++ {
				child, err := m.GetArg(current, index)
				fmMust(t, err)
				work = append(work, child)
			}
		}
	}
}

// Source: array_select_reducer::has_stores reads marks on replacement nodes.
func TestLegacyArrayReducerKeepsStoreMarksOnSelectedArrayValue(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	innerSort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	outerSort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), innerSort))
	a := f.constant("nested-reducer-a", outerSort)
	b := f.constant("nested-reducer-b", innerSort)
	i := f.constant("nested-reducer-i", f.integer)
	k := f.constant("nested-reducer-k", f.integer)
	j := f.constant("nested-reducer-j", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	innerStore := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{b, k, seven}))
	outerStore := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, innerStore}))
	innerSelect := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{outerStore, i}))
	selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{innerSelect, j}))
	formula := f.own(m.MkEq(selected, seven))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	base := f.own(m.MkConstArray(innerSort, zero))
	outerBase := f.own(m.MkConstArray(outerSort, base))
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{a: outerBase, b: base, i: zero, k: one, j: one} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	result, err := p.reduceSelects(md.Core(), []ast.ASTHandle{a, b}, formula, false)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	root, err := m.RefGet(result.formula)
	fmMust(t, err)
	occurs, err := m.OccursExpr(innerStore, root)
	fmMust(t, err)
	if occurs {
		t.Fatal("selected array value lost its existing store mark", root)
	}
	for _, v := range []ast.ASTHandle{a, b} {
		occurs, err := m.OccursExpr(v, root)
		fmMust(t, err)
		if occurs {
			t.Fatal("nested array remained", v, root)
		}
	}
}

func TestLegacyArrayReducerDoesNotUseStoreMarksFromSelectIndex(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("index-mark-a", sort)
	b := f.constant("index-mark-b", sort)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	markedStore := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{b, zero, seven}))
	name, err := m.Symbols.Intern("index-mark-function")
	fmMust(t, err)
	decl := f.own(m.MkFuncDecl(name, []ast.ASTHandle{sort}, f.integer, ast.ASTFuncDeclInfo{}))
	index := f.own(m.MkApp(decl, []ast.ASTHandle{markedStore}))
	unmarkedStore := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, zero, seven}))
	selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{unmarkedStore, index}))
	formula := f.own(m.MkEq(selected, seven))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	result, err := p.reduceSelects(md.Core(), []ast.ASTHandle{b}, formula, false)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	root, err := m.RefGet(result.formula)
	fmMust(t, err)
	occurs, err := m.OccursExpr(unmarkedStore, root)
	fmMust(t, err)
	if !occurs {
		t.Fatal("stores in the index must not enable reduction of an unmarked array", root)
	}
}

func TestLegacyArrayReducerProcessesRightSiblingBeforeLeft(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := newLegacyArrayProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("stack-order-a", sort)
	one := f.own(m.ArithmeticIntegerNumeralText("1"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	var forms, guards []ast.ASTHandle
	for _, prefix := range []string{"stack-order-left", "stack-order-right"} {
		i := f.constant(prefix+"-i", f.integer)
		j := f.constant(prefix+"-j", f.integer)
		for _, v := range []ast.ASTHandle{i, j} {
			decl, err := m.GetDecl(v)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, one))
		}
		stored := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, i, seven}))
		selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{stored, j}))
		forms = append(forms, f.own(m.MkEq(selected, seven)))
		guards = append(guards, f.own(m.MkEq(i, j)))
	}
	want := f.own(m.MkAndArray([]ast.ASTHandle{guards[1], guards[0]}))
	expected, err := p.rewrite(want)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&expected)) }()
	expectedRoot, err := m.RefGet(expected)
	fmMust(t, err)
	for _, shared := range []bool{false, true} {
		inputs := append([]ast.ASTHandle(nil), forms...)
		if shared {
			inputs = append(inputs, forms[1])
		}
		formula := f.own(m.MkAndArray(inputs))
		result, err := p.reduceSelects(md.Core(), []ast.ASTHandle{a}, formula, false)
		fmMust(t, err)
		root, err := m.RefGet(result.formula)
		fmMust(t, err)
		if root != expectedRoot {
			t.Fatal("source stack must emit the right sibling's guard first", shared, root, expectedRoot)
		}
		fmMust(t, result.Close())
	}
}
