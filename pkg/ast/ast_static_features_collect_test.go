package ast

import (
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestStaticFeaturesBooleanTraversalClausesSharingAndDepth(t *testing.T) {
	m := astTestManager(t)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "sf-a"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, a)
	b, err := m.MkConst(astTestSymbol(t, m, "sf-b"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, b)
	shared, err := m.MkAnd(a, b)
	astTestMust(t, err)
	astTestPin(t, m, shared)
	notA, err := m.MkNot(a)
	astTestMust(t, err)
	astTestPin(t, m, notA)
	clause, err := m.MkOr(shared, notA)
	astTestMust(t, err)
	astTestPin(t, m, clause)
	f, err := NewStaticFeatures(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	astTestMust(t, f.Collect(clause, shared))
	if f.NumRoots != 1 || f.NumClauses != 1 || f.NumBinClauses != 1 || f.SumClauseSize != 2 {
		t.Fatal("root clause accounting", f.NumRoots, f.NumClauses, f.NumBinClauses, f.SumClauseSize)
	}
	if f.NumSharing < 2 || f.MaxDepth != 3 || f.NumFormulaTrees != 1 || f.NumOrAndTrees != 1 {
		t.Fatal("tree accounting", f.NumSharing, f.MaxDepth, f.NumFormulaTrees, f.NumOrAndTrees)
	}
	if f.CNF || f.NumANDs != 1 || f.NumBoolExprs < 3 {
		t.Fatal("gate accounting", f.CNF, f.NumANDs, f.NumBoolExprs)
	}
}

func TestStaticFeaturesTermITEIsNotAFormulaTree(t *testing.T) {
	m := astTestManager(t)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	condition, err := m.MkConst(astTestSymbol(t, m, "sf-ite-condition"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, condition)
	termSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "sf-ite-sort"), nil)
	astTestMust(t, err)
	left, err := m.MkConst(astTestSymbol(t, m, "sf-ite-left"), termSort)
	astTestMust(t, err)
	astTestPin(t, m, left)
	right, err := m.MkConst(astTestSymbol(t, m, "sf-ite-right"), termSort)
	astTestMust(t, err)
	astTestPin(t, m, right)
	ite, err := m.MkIte(condition, left, right)
	astTestMust(t, err)
	astTestPin(t, m, ite)
	f, err := NewStaticFeatures(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	astTestMust(t, f.Collect(ite))
	if f.NumITETerms != 1 || f.NumITEFormulas != 0 || f.NumITETrees != 1 {
		t.Fatal("term ITE accounting", f.NumITETerms, f.NumITEFormulas, f.NumITETrees)
	}
	if f.NumFormulaTrees != 0 || f.MaxFormulaDepth != 0 {
		t.Fatal("term ITE counted as formula tree", f.NumFormulaTrees, f.MaxFormulaDepth)
	}
}

func TestStaticFeaturesTheoryFamiliesSortsEqualitiesAndAliens(t *testing.T) {
	m := astTestManager(t)
	f, err := NewStaticFeatures(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	for i, name := range []string{"arith", "bv", "array", "fpa", "seq"} {
		sym, e := m.Symbols.Intern(name)
		astTestMust(t, e)
		fid, e := m.families.GetID(sym)
		astTestMust(t, e)
		if fid != FamilyID(5+i) {
			t.Fatal("static feature utility family construction order", name, fid)
		}
	}
	arith, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integerSort := m.external(m.arithmeticPlugin.fields[1])
	x, err := m.MkConst(astTestSymbol(t, m, "sf-int-x"), integerSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	one, err := m.numerals.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, m.numerals.Del(&one)) }()
	astTestMust(t, m.numerals.SetInt64(&one, 1))
	oneExpr, err := m.ArithmeticNumeral(&one, true)
	astTestMust(t, err)
	astTestPin(t, m, oneExpr)
	eq, err := m.MkEq(x, oneExpr)
	astTestMust(t, err)
	astTestPin(t, m, eq)
	userSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "sf-theory-result"), nil)
	astTestMust(t, err)
	userA, err := m.MkConst(astTestSymbol(t, m, "sf-user-a"), userSort)
	astTestMust(t, err)
	astTestPin(t, m, userA)
	userB, err := m.MkConst(astTestSymbol(t, m, "sf-user-b"), userSort)
	astTestMust(t, err)
	astTestPin(t, m, userB)
	userEq, err := m.MkEq(userA, userB)
	astTestMust(t, err)
	astTestPin(t, m, userEq)
	decl, err := m.MkFuncDecl(usymbol.Numerical(8001), []ASTHandle{integerSort}, userSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	alien, err := m.MkApp(decl, []ASTHandle{oneExpr})
	astTestMust(t, err)
	astTestPin(t, m, alien)
	astTestMust(t, f.Collect(eq, alien, userEq))
	if !f.HasInt || f.NumMarkedTheories != 1 || f.NumEqualities != 2 {
		t.Fatal("sort and equality accounting", f.HasInt, f.NumMarkedTheories, f.NumEqualities)
	}
	if int(arith) >= len(f.NumTheoryEqualities) || f.NumTheoryEqualities[arith] != 1 {
		t.Fatal("per-theory equality accounting", arith, f.NumTheoryEqualities)
	}
	if int(arith) >= len(f.NumTheoryConstants) || f.NumTheoryConstants[arith] != 1 {
		t.Fatal("uninterpreted arithmetic constant accounting", arith, f.NumTheoryConstants)
	}
	if len(f.NumTheoryConstants) > 4 && f.NumTheoryConstants[4] != 0 || len(f.NumTheoryEqualities) > 4 && f.NumTheoryEqualities[4] != 0 {
		t.Fatal("user sorts counted as interpreted theories", f.NumTheoryConstants, f.NumTheoryEqualities)
	}
	if f.NumAliens != 1 || int(arith) >= len(f.NumAliensPerFamily) || f.NumAliensPerFamily[arith] != 1 {
		t.Fatal("alien accounting", f.NumAliens, f.NumAliensPerFamily)
	}
	if int(arith) >= len(f.NumTheoryTerms) || f.NumTheoryTerms[arith] != 1 {
		t.Fatal("arithmetic numeral theory term", arith, f.NumTheoryTerms)
	}
}

func TestStaticFeaturesArithmeticDifferenceAndNonlinearAccounting(t *testing.T) {
	m := astTestManager(t)
	arith, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integerSort := m.external(m.arithmeticPlugin.fields[1])
	x, err := m.MkConst(astTestSymbol(t, m, "sf-diff-x"), integerSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	y, err := m.MkConst(astTestSymbol(t, m, "sf-diff-y"), integerSort)
	astTestMust(t, err)
	astTestPin(t, m, y)
	numeral := func(text string, integer bool) ASTHandle {
		value, e := m.numerals.NewValue()
		astTestMust(t, e)
		astTestMust(t, m.numerals.SetString(&value, text))
		h, e := m.ArithmeticNumeral(&value, integer)
		astTestMust(t, e)
		astTestMust(t, m.numerals.Del(&value))
		astTestPin(t, m, h)
		return h
	}
	minusOne, two, three, half := numeral("-1", true), numeral("2", true), numeral("3", true), numeral("1/2", false)
	negY, err := m.MkFamilyApp(arith, 9, nil, []ASTHandle{minusOne, y})
	astTestMust(t, err)
	astTestPin(t, m, negY)
	difference, err := m.MkFamilyApp(arith, 6, nil, []ASTHandle{x, negY})
	astTestMust(t, err)
	astTestPin(t, m, difference)
	le, err := m.MkFamilyApp(arith, 2, nil, []ASTHandle{difference, three})
	astTestMust(t, err)
	astTestPin(t, m, le)
	eq, err := m.MkEq(difference, three)
	astTestMust(t, err)
	astTestPin(t, m, eq)
	nonlinear, err := m.MkFamilyApp(arith, 9, nil, []ASTHandle{x, y})
	astTestMust(t, err)
	astTestPin(t, m, nonlinear)
	shifted, err := m.MkFamilyApp(arith, 6, nil, []ASTHandle{two, x})
	astTestMust(t, err)
	astTestPin(t, m, shifted)
	userSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "sf-diff-result"), nil)
	astTestMust(t, err)
	decl, err := m.MkFuncDecl(usymbol.Numerical(8002), []ASTHandle{integerSort}, userSort, ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	alien, err := m.MkApp(decl, []ASTHandle{shifted})
	astTestMust(t, err)
	astTestPin(t, m, alien)
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	condition, err := m.MkConst(astTestSymbol(t, m, "sf-arith-ite-condition"), boolSort)
	astTestMust(t, err)
	astTestPin(t, m, condition)
	ite, err := m.MkIte(condition, shifted, three)
	astTestMust(t, err)
	astTestPin(t, m, ite)
	limit := m.ResourceLimit()
	astTestMust(t, limit.Push(1))
	defer limit.Pop()
	initialSteps := limit.Count()
	f, err := NewStaticFeatures(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	astTestMust(t, f.Collect(le, eq, nonlinear, alien, ite, half))
	if limit.Count() != initialSteps {
		t.Fatalf("static feature collection consumed %d search steps", limit.Count()-initialSteps)
	}
	if f.NumArithInequalities != 1 || f.NumDiffInequalities != 1 || f.NumSimpleInequalities != 0 {
		t.Fatal("arithmetic inequality accounting", f.NumArithInequalities, f.NumDiffInequalities, f.NumSimpleInequalities)
	}
	if f.NumArithEqualities != 1 || f.NumDiffEqualities != 1 || f.NumSimpleEqualities != 0 {
		t.Fatal("arithmetic equality accounting", f.NumArithEqualities, f.NumDiffEqualities, f.NumSimpleEqualities)
	}
	if f.NumNonLinear != 1 || f.NumArithTerms != 3 || f.NumDiffTerms != 3 || !f.HasRational {
		t.Fatal("term arithmetic accounting", f.NumNonLinear, f.NumArithTerms, f.NumDiffTerms, f.HasRational)
	}
	sum, err := m.numerals.String(&f.ArithKSum)
	astTestMust(t, err)
	if sum != "16" {
		t.Fatal("source absolute arithmetic coefficient sum", sum)
	}
	small, err := f.ArithKSumIsSmall()
	astTestMust(t, err)
	if !small {
		t.Fatal("small arithmetic coefficient sum rejected")
	}
}

func TestStaticFeaturesArrayParametersContributeSortFeatures(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterArrayPlugin()
	astTestMust(t, err)
	integerSort := m.external(m.arithmeticPlugin.fields[1])
	realSort := m.external(m.arithmeticPlugin.fields[0])
	arraySort, err := m.MkArraySort(SliceArguments([]ASTHandle{integerSort}), realSort)
	astTestMust(t, err)
	array, err := m.MkConst(astTestSymbol(t, m, "sf-array"), arraySort)
	astTestMust(t, err)
	astTestPin(t, m, array)
	f, err := NewStaticFeatures(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	astTestMust(t, f.Collect(array))
	if !f.HasArrays || !f.HasInt || !f.HasReal {
		t.Fatal("array parameter sort accounting", f.HasArrays, f.HasInt, f.HasReal)
	}
}

func TestStaticFeaturesBitVectorFloatingPointAndSequenceSortFlags(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	bvSort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	ebits, err := m.NewParameterInt(8)
	astTestMust(t, err)
	sbits, err := m.NewParameterInt(24)
	astTestMust(t, err)
	floatSort, err := m.MkFamilySort(m.floatingPointPlugin.family, 0, []ASTParameter{ebits, sbits})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&sbits))
	astTestMust(t, m.DelParameter(&ebits))
	stringSort := m.external(m.sequencePlugin.stringSort)
	userSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "sf-sequence-element"), nil)
	astTestMust(t, err)
	element, err := m.NewParameterAST(userSort)
	astTestMust(t, err)
	sequenceSort, err := m.MkFamilySort(m.sequencePlugin.family, 0, []ASTParameter{element})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&element))
	var roots []ASTHandle
	for _, row := range []struct {
		name string
		sort ASTHandle
	}{{"bv", bvSort}, {"float", floatSort}, {"string", stringSort}, {"sequence", sequenceSort}} {
		h, e := m.MkConst(astTestSymbol(t, m, "sf-"+row.name), row.sort)
		astTestMust(t, e)
		astTestPin(t, m, h)
		roots = append(roots, h)
	}
	f, err := NewStaticFeatures(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.Close()) })
	astTestMust(t, f.Collect(roots...))
	if !f.HasBitVector || !f.HasFloatingPoint || !f.HasString || !f.HasNonStringSequence {
		t.Fatal("builtin sort flags", f.HasBitVector, f.HasFloatingPoint, f.HasString, f.HasNonStringSequence)
	}
}
