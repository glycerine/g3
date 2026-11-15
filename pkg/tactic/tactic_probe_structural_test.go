package tactic

// Source: src/tactic/probe.cpp:211-527.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func probeEvalKind(t *testing.T, g *Goal, kind ProbeKind) float64 {
	t.Helper()
	p, err := NewProbe(g.manager.Budget(), kind, 0)
	astTestMust(t, err)
	value, err := p.Eval(g)
	astTestMust(t, err)
	astTestMust(t, p.Close())
	return value
}

func TestProbeConstantCountsQuantifiersPatternsAndFragments(t *testing.T) {
	m := astTestManager(t)
	err := m.RegisterDeclarationPlugins()
	astTestMust(t, err)

	plain := goalTestNew(t, m, false, false)
	atom := goalTestAtom(t, m, "probe-atom")
	astTestMust(t, plain.Assert(atom))
	if got := probeEvalKind(t, plain, ProbeNumBoolConsts); got != 1 {
		t.Fatal("Boolean constants", got)
	}
	for _, kind := range []ProbeKind{ProbeIsPropositional, ProbeIsQFBitVector, ProbeIsQFArrayUFBitVector, ProbeIsQFUFBitVector} {
		if got := probeEvalKind(t, plain, kind); got != 1 {
			t.Fatalf("Boolean goal fragment %d: %v", kind, got)
		}
	}

	userSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "ProbeU"), nil)
	astTestMust(t, err)
	astTestPin(t, m, userSort)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "probe-u"), nil, userSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	constant, err := m.MkApp(decl, nil)
	astTestMust(t, err)
	astTestPin(t, m, constant)
	equality, err := m.MkEq(constant, constant)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	nonBoolean := goalTestNew(t, m, false, false)
	astTestMust(t, nonBoolean.Assert(equality))
	if got := probeEvalKind(t, nonBoolean, ProbeNumConsts); got != 1 {
		t.Fatal("shared non-Boolean constant count", got)
	}
	for _, kind := range []ProbeKind{ProbeIsPropositional, ProbeIsQFBitVector, ProbeIsQFArrayUFBitVector, ProbeIsQFUFBitVector} {
		if got := probeEvalKind(t, nonBoolean, kind); got != 0 {
			t.Fatalf("unsupported-sort fragment %d: %v", kind, got)
		}
	}

	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	astTestPin(t, m, zero)
	integerSort, err := m.GetSort(zero)
	astTestMust(t, err)
	integerConstant, err := m.MkConst(astTestSymbol(t, m, "probe-i"), integerSort)
	astTestMust(t, err)
	astTestPin(t, m, integerConstant)
	integerEquality, err := m.MkEq(integerConstant, zero)
	astTestMust(t, err)
	astTestPin(t, m, integerEquality)
	arithmeticGoal := goalTestNew(t, m, false, false)
	astTestMust(t, arithmeticGoal.Assert(integerEquality))
	if got := probeEvalKind(t, arithmeticGoal, ProbeNumArithmeticConsts); got != 1 {
		t.Fatal("arithmetic constants", got)
	}

	bvSort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	astTestPin(t, m, bvSort)
	bvConstant, err := m.MkConst(astTestSymbol(t, m, "probe-bv"), bvSort)
	astTestMust(t, err)
	astTestPin(t, m, bvConstant)
	bvEquality, err := m.MkEq(bvConstant, bvConstant)
	astTestMust(t, err)
	astTestPin(t, m, bvEquality)
	bvGoal := goalTestNew(t, m, false, false)
	astTestMust(t, bvGoal.Assert(bvEquality))
	if got := probeEvalKind(t, bvGoal, ProbeNumBitVectorConsts); got != 1 {
		t.Fatal("bit-vector constants", got)
	}

	pattern, err := m.MkPattern([]ast.ASTHandle{atom})
	astTestMust(t, err)
	astTestPin(t, m, pattern)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")},
		Body: atom, Patterns: []ast.ASTHandle{pattern},
	})
	astTestMust(t, err)
	astTestPin(t, m, quantifier)
	quantified := goalTestNew(t, m, false, false)
	astTestMust(t, quantified.Assert(quantifier))
	if got := probeEvalKind(t, quantified, ProbeHasQuantifier); got != 1 {
		t.Fatal("quantifier probe", got)
	}
	if got := probeEvalKind(t, quantified, ProbeHasPattern); got != 1 {
		t.Fatal("pattern probe", got)
	}
	for _, kind := range []ProbeKind{ProbeIsPropositional, ProbeIsQFBitVector, ProbeIsQFArrayUFBitVector, ProbeIsQFUFBitVector} {
		if got := probeEvalKind(t, quantified, kind); got != 0 {
			t.Fatalf("quantified fragment %d: %v", kind, got)
		}
	}

	// tactic_top_01-F3: num_consts_probe ignores patterns, so a constant that
	// occurs only in a pattern is not counted.
	patternOnly := goalTestAtom(t, m, "probe-pattern-only")
	onlyPattern, err := m.MkPattern([]ast.ASTHandle{patternOnly})
	astTestMust(t, err)
	astTestPin(t, m, onlyPattern)
	patternQuantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")},
		Body: atom, Patterns: []ast.ASTHandle{onlyPattern},
	})
	astTestMust(t, err)
	astTestPin(t, m, patternQuantifier)
	patternGoal := goalTestNew(t, m, false, false)
	astTestMust(t, patternGoal.Assert(patternQuantifier))
	if got := probeEvalKind(t, patternGoal, ProbeNumBoolConsts); got != 1 {
		t.Fatal("pattern-only constant counted", got)
	}
	if got := probeEvalKind(t, patternGoal, ProbeHasPattern); got != 1 {
		t.Fatal("pattern probe with pattern-only constant", got)
	}
}
