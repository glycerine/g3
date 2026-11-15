package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp
// lemma_array_eq_generalizer pairwise contradiction strengthening.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestArrayEqualityGeneralizerStrengthensContradictoryLiteral(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	owner, err := NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := owner.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	predicateName, err := manager.Symbols.Intern("array-generalizer-predicate")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(predicateName, nil, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	transformer, err := NewRulePredicateTransformer(owner, predicate, PropSolverConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := transformer.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	if err != nil {
		t.Fatal(err)
	}
	values := make([]ast.ASTHandle, 2)
	for index, text := range []string{"array-generalizer-a", "array-generalizer-b"} {
		name, nameErr := manager.Symbols.Intern(text)
		if nameErr != nil {
			t.Fatal(nameErr)
		}
		values[index], err = manager.MkConst(name, arraySort)
		if err != nil {
			t.Fatal(err)
		}
	}
	equality, err := manager.MkEq(values[0], values[1])
	if err != nil {
		t.Fatal(err)
	}
	disequality, err := manager.MkNot(equality)
	if err != nil {
		t.Fatal(err)
	}
	constants, err := transformer.collectArrayConstants(disequality)
	if err != nil {
		t.Fatal(err)
	}
	if len(constants) != 2 {
		t.Fatal("array constant collection", constants)
	}
	secondBefore, thirdBefore := owner.contexts[1].created, owner.contexts[2].created
	strengthened, checker, applicable, err := transformer.strengthenArrayEqualities([]ast.ASTHandle{disequality})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if checker != nil {
			if err := checker.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	defer func() {
		if err := releaseNormalizeRefs(manager, strengthened); err != nil {
			t.Error(err)
		}
	}()
	if !applicable {
		t.Fatal("array equality strengthening was not applicable")
	}
	// smt::kernel solver(m, fparams2()) is a standalone kernel: no pooled
	// virtual solver is created for the check.
	if owner.contexts[1].created != secondBefore || owner.contexts[2].created != thirdBefore {
		t.Fatal("array equality checker used a pooled solver", owner.contexts[1].created-secondBefore, owner.contexts[2].created-thirdBefore)
	}
	if len(strengthened) != 1 {
		t.Fatal("array equality strengthening size", len(strengthened))
	}
	strengthenedHandle, err := manager.RefGet(strengthened[0])
	if err != nil || strengthenedHandle != disequality {
		t.Fatal("array equality strengthening", strengthened, disequality)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	unchanged, unchangedChecker, applicable, err := transformer.strengthenArrayEqualities([]ast.ASTHandle{truth})
	if err != nil {
		t.Fatal(err)
	}
	if unchangedChecker != nil {
		defer func() {
			if err := unchangedChecker.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	defer func() {
		if err := releaseNormalizeRefs(manager, unchanged); err != nil {
			t.Error(err)
		}
	}()
	if len(unchanged) != 1 {
		t.Fatal("unchanged cube size", len(unchanged))
	}
	unchangedHandle, err := manager.RefGet(unchanged[0])
	if applicable || err != nil || unchangedHandle != truth {
		t.Fatal("array equality generalizer applied without array constants", applicable, unchanged)
	}
}

func TestArrayEqualityGeneralizerKeepsCandidateSortUniform(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	must(err)
	defer func() { must(owner.Close()) }()
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("mixed-array-predicate")
	must(err)
	decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	pt, err := NewRulePredicateTransformer(owner, decl, PropSolverConfig{})
	must(err)
	defer func() { must(pt.Close()) }()
	first, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	must(err)
	second, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), first)
	must(err)
	var cube []ast.ASTHandle
	for i, sort := range []ast.ASTHandle{first, second} {
		var terms []ast.ASTHandle
		for j := 0; j < 2; j++ {
			name, err := m.Symbols.Intern(string(rune('a' + i*2 + j)))
			must(err)
			value, err := m.MkConst(name, sort)
			must(err)
			terms = append(terms, value)
		}
		eq, err := m.MkEq(terms[0], terms[1])
		must(err)
		ne, err := m.MkNot(eq)
		must(err)
		cube = append(cube, ne)
	}
	formula, err := m.MkAndArray(cube)
	must(err)
	declarations, err := pt.collectArrayConstants(formula)
	must(err)
	if len(declarations) != 2 {
		t.Fatalf("collected %d mixed-sort constants, want first sort's pair", len(declarations))
	}
	for _, decl := range declarations {
		value, err := m.MkApp(decl, nil)
		must(err)
		sort, err := m.GetSort(value)
		must(err)
		if sort != first {
			t.Fatal("mixed array sort retained")
		}
	}
}

// Source: spacer_generalizers.cpp collect_array_proc and for_each_expr.h.
func TestArrayGeneralizerCollectsPostorderAndQuantifierChildren(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	boolean, err := m.MkBoolSort()
	must(err)
	integer, err := m.GetSort(normalizeNumeral(t, m, "0"))
	must(err)
	inner, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	must(err)
	outer, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), boolean)
	must(err)
	name, err := m.Symbols.Intern("array-collector-child")
	must(err)
	child, err := m.MkConst(name, inner)
	must(err)
	name, err = m.Symbols.Intern("array-collector-parent")
	must(err)
	parentDecl, err := m.MkFuncDecl(name, []ast.ASTHandle{inner}, outer, ast.ASTFuncDeclInfo{})
	must(err)
	parent, err := m.MkApp(parentDecl, []ast.ASTHandle{child})
	must(err)
	pt := &PredicateTransformer{manager: m}
	declarations, err := pt.collectArrayConstants(parent)
	must(err)
	childDecl, err := m.GetDecl(child)
	must(err)
	if len(declarations) != 1 || declarations[0] != childDecl {
		t.Fatal("first sort must come from child before parent", declarations, childDecl)
	}
	name, err = m.Symbols.Intern("array-collector-second")
	must(err)
	second, err := m.MkConst(name, inner)
	must(err)
	body, err := m.MkEq(child, second)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	quantified, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: truth, NoPatterns: []ast.ASTHandle{body}})
	must(err)
	declarations, err = pt.collectArrayConstants(quantified)
	must(err)
	secondDecl, err := m.GetDecl(second)
	must(err)
	// AUDIT.md muz_spacer_01-F2: the post-order visit fills a func_decl_set,
	// which is then read in hash-table order.
	if want := funcDeclSetOrder(t, m, childDecl, secondDecl); len(declarations) != 2 || declarations[0] != want[0] || declarations[1] != want[1] {
		t.Fatal("quantifier annotation traversal", declarations)
	}
	quantified, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: body})
	must(err)
	declarations, err = pt.collectArrayConstants(quantified)
	must(err)
	if len(declarations) != 2 {
		t.Fatal("quantifier body traversal", declarations)
	}
}

func funcDeclSetOrder(t *testing.T, m *ast.ASTManager, decls ...ast.ASTHandle) []ast.ASTHandle {
	t.Helper()
	set := ast.NewFuncDeclSet(m)
	for _, decl := range decls {
		if err := set.Insert(decl); err != nil {
			t.Fatal(err)
		}
	}
	return set.Keys()
}

// AUDIT.md muz_spacer_01-F2: lemma_array_eq_generalizer copies its
// func_decl_set of array constants in hash-table order, which orients the
// candidate equalities.
func TestArrayGeneralizerUsesFuncDeclSetOrder(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	integer, err := m.GetSort(normalizeNumeral(t, m, "0"))
	must(err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer)
	must(err)
	var constants, decls []ast.ASTHandle
	for _, text := range []string{"array-order-a", "array-order-b", "array-order-c", "array-order-d", "array-order-e"} {
		name, err := m.Symbols.Intern(text)
		must(err)
		constant, err := m.MkConst(name, arraySort)
		must(err)
		decl, err := m.GetDecl(constant)
		must(err)
		constants = append(constants, constant)
		decls = append(decls, decl)
	}
	// The equalities are visited in creation order.
	var equalities []ast.ASTHandle
	for i := 0; i+1 < len(constants); i++ {
		eq, err := m.MkEq(constants[i], constants[i+1])
		must(err)
		equalities = append(equalities, eq)
	}
	root, err := m.MkAndArray(equalities)
	must(err)
	pt := &PredicateTransformer{manager: m}
	declarations, err := pt.collectArrayConstants(root)
	must(err)
	want := funcDeclSetOrder(t, m, decls...)
	if len(declarations) != len(want) {
		t.Fatal("collected arrays", declarations, want)
	}
	for i := range want {
		if declarations[i] != want[i] {
			t.Fatal("array constants not in func_decl_set order", declarations, want)
		}
	}
}
