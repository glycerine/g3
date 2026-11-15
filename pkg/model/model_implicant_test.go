package model_test

// Source: src/model/model_implicant.cpp:46-466,617-917.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func modelImplicantFixture(t *testing.T) (*ast.ASTManager, *model.Model, *model.ModelImplicant) {
	t.Helper()
	m, md, _, _, _, config := evaluatorFixture(t)
	i, err := model.NewModelImplicant(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, i.Close()) })
	return m, md, i
}

func TestModelImplicantBooleanLiteralAndConeSelection(t *testing.T) {
	m, md, implicant := modelImplicantFixture(t)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	a := goalTestAtom(t, m, "implicant-a")
	b := goalTestAtom(t, m, "implicant-b")
	aDecl, err := m.GetDecl(a)
	astTestMust(t, err)
	bDecl, err := m.GetDecl(b)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(aDecl, yes))
	astTestMust(t, md.Core().RegisterConstant(bDecl, no))
	or, err := m.MkOr(b, a)
	astTestMust(t, err)
	formula, err := m.MkAnd(a, or)
	astTestMust(t, err)
	astTestPin(t, m, formula)

	literals, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
	astTestMust(t, err)
	defer func() { astTestMust(t, model.CloseImplicantResults(m, &literals)) }()
	if len(literals) != 1 {
		t.Fatal("source OR witness and duplicate pruning", len(literals))
	}
	got, err := m.RefGet(literals[0])
	astTestMust(t, err)
	if got != a {
		t.Fatal("selected literal", got, a)
	}

	cone, err := implicant.MinimizeModel([]ast.ASTHandle{formula}, md)
	astTestMust(t, err)
	defer func() { astTestMust(t, model.CloseImplicantResults(m, &cone)) }()
	if len(cone) != 1 {
		t.Fatal("cone should retain only a", len(cone))
	}
	eq, err := m.RefGet(cone[0])
	astTestMust(t, err)
	left, err := m.GetArg(eq, 0)
	astTestMust(t, err)
	right, err := m.GetArg(eq, 1)
	astTestMust(t, err)
	if left != a || right != yes {
		t.Fatal("cone equality", left, right)
	}
}

func TestModelImplicantArithmeticDisequalitySplit(t *testing.T) {
	m, md, implicant := modelImplicantFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	intSort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "implicant-x"), intSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "implicant-y"), intSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	astTestPin(t, m, y)
	xDecl, err := m.GetDecl(x)
	astTestMust(t, err)
	yDecl, err := m.GetDecl(y)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(xDecl, one))
	astTestMust(t, md.Core().RegisterConstant(yDecl, two))
	eq, err := m.MkEq(x, y)
	astTestMust(t, err)
	formula, err := m.MkNot(eq)
	astTestMust(t, err)
	astTestPin(t, m, formula)

	literals, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
	astTestMust(t, err)
	defer func() { astTestMust(t, model.CloseImplicantResults(m, &literals)) }()
	if len(literals) != 1 {
		t.Fatal(len(literals))
	}
	lt, err := m.RefGet(literals[0])
	astTestMust(t, err)
	family, err := m.GetFamilyID(lt)
	astTestMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	op, err := m.GetDeclKind(lt)
	astTestMust(t, err)
	left, err := m.GetArg(lt, 0)
	astTestMust(t, err)
	right, err := m.GetArg(lt, 1)
	astTestMust(t, err)
	if family != arithmetic || op != 4 || left != x || right != y {
		t.Fatal("arithmetic disequality split", family, op, left, right)
	}
}

func TestModelImplicantEvalExpressionUsesCompletion(t *testing.T) {
	m, md, implicant := modelImplicantFixture(t)
	a := goalTestAtom(t, m, "implicant-eval")
	result, err := implicant.EvalExpr(md, a)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&result)) }()
	got, err := m.RefGet(result)
	astTestMust(t, err)
	if !m.IsFalse(got) {
		t.Fatal("model completion should choose false for missing Boolean", got)
	}
}

func TestModelImplicantExtensionalArrayEquality(t *testing.T) {
	m, md, implicant := modelImplicantFixture(t)
	arrayFamily, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	indexSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "implicant-index"), nil)
	astTestMust(t, err)
	boolSort := mustBoolSort(t, m)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolSort)
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	allNo, err := m.MkConstArray(arraySort, no)
	astTestMust(t, err)
	allYes, err := m.MkConstArray(arraySort, yes)
	astTestMust(t, err)
	left, err := m.MkConst(astTestSymbol(t, m, "implicant-left-array"), arraySort)
	astTestMust(t, err)
	right, err := m.MkConst(astTestSymbol(t, m, "implicant-right-array"), arraySort)
	astTestMust(t, err)
	astTestPin(t, m, left)
	astTestPin(t, m, right)
	leftDecl, err := m.GetDecl(left)
	astTestMust(t, err)
	rightDecl, err := m.GetDecl(right)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(leftDecl, allNo))
	astTestMust(t, md.Core().RegisterConstant(rightDecl, allYes))
	eq, err := m.MkEq(left, right)
	astTestMust(t, err)
	formula, err := m.MkNot(eq)
	astTestMust(t, err)
	astTestPin(t, m, formula)

	literals, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
	astTestMust(t, err)
	defer func() { astTestMust(t, model.CloseImplicantResults(m, &literals)) }()
	if len(literals) != 1 {
		t.Fatal(len(literals))
	}
	literal, err := m.RefGet(literals[0])
	astTestMust(t, err)
	op, err := m.GetDeclKind(literal)
	astTestMust(t, err)
	arg, err := m.GetArg(literal, 0)
	astTestMust(t, err)
	if op != ast.ASTOpNot || arg != eq {
		t.Fatal("array disequality literal", arrayFamily, op, arg, eq)
	}
	astTestMust(t, model.CloseImplicantResults(m, &literals))

	index, err := m.GetSomeValue(indexSort)
	astTestMust(t, err)
	stored, err := m.MkFamilyApp(arrayFamily, ast.ASTArrayOpStore, nil, []ast.ASTHandle{allNo, index, yes})
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(rightDecl, stored))
	literals, err = implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
	astTestMust(t, err)
	if len(literals) != 1 {
		t.Fatal("store-row disequality", len(literals))
	}
}

// AUDIT.md model_01-F5 (z3 4.7.1 bug kept fixed in g3): at index 1 the
// nested values (store K0 0 1) and (store (store K0 0 1) 3 0) are different
// terms for equal arrays, so the nested comparison marks the equality true;
// at index 2 the values (store K0 0 5) and (store K0 0 6) differ. z3 4.7.1
// keeps the true mark (set_false cannot clear it) and finds the satisfied
// disequality false; g3 re-checks every index, so (not (= a b)) is the
// implicant literal (C++ red test tst_z3_4_7_1_implicant_nested_array_eq).
func TestModelImplicantNestedArrayTrueIsNotSticky(t *testing.T) {
	m, md, implicant := modelImplicantFixture(t)
	arrayFamily, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	num := func(k int64) ast.ASTHandle {
		h, err := m.ArithmeticNumeralRat(big.NewRat(k, 1), true)
		astTestMust(t, err)
		astTestPin(t, m, h)
		return h
	}
	n0, n1, n2, n3, n5, n6 := num(0), num(1), num(2), num(3), num(5), num(6)
	intSort, err := m.GetSort(n0)
	astTestMust(t, err)
	inner, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{intSort}), intSort)
	astTestMust(t, err)
	outer, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{intSort}), inner)
	astTestMust(t, err)
	k0, err := m.MkConstArray(inner, n0)
	astTestMust(t, err)
	astTestPin(t, m, k0)
	kk, err := m.MkConstArray(outer, k0)
	astTestMust(t, err)
	astTestPin(t, m, kk)
	store := func(base, index, value ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkFamilyApp(arrayFamily, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, value})
		astTestMust(t, err)
		astTestPin(t, m, h)
		return h
	}
	x1 := store(k0, n0, n1)
	x2 := store(x1, n3, n0)
	aValue := store(store(kk, n2, store(k0, n0, n5)), n1, x1)
	bValue := store(store(kk, n2, store(k0, n0, n6)), n1, x2)
	a, err := m.MkConst(astTestSymbol(t, m, "implicant-nested-a"), outer)
	astTestMust(t, err)
	astTestPin(t, m, a)
	b, err := m.MkConst(astTestSymbol(t, m, "implicant-nested-b"), outer)
	astTestMust(t, err)
	astTestPin(t, m, b)
	aDecl, err := m.GetDecl(a)
	astTestMust(t, err)
	bDecl, err := m.GetDecl(b)
	astTestMust(t, err)
	astTestMust(t, md.Core().RegisterConstant(aDecl, aValue))
	astTestMust(t, md.Core().RegisterConstant(bDecl, bValue))
	eq, err := m.MkEq(a, b)
	astTestMust(t, err)
	astTestPin(t, m, eq)
	formula, err := m.MkNot(eq)
	astTestMust(t, err)
	astTestPin(t, m, formula)
	literals, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
	astTestMust(t, err)
	defer func() { astTestMust(t, model.CloseImplicantResults(m, &literals)) }()
	if len(literals) != 1 {
		t.Fatal("nested array implicant literals", len(literals))
	}
	literal, err := m.RefGet(literals[0])
	astTestMust(t, err)
	if literal != formula {
		t.Fatal("nested array disequality was not kept")
	}
}
