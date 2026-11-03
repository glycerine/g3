package ast

// Source: src/ast/factor_equivs.cpp; src/ast/factor_equivs.h.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTFactorEqualitiesArithmeticNormalizationAndRemainder(t *testing.T) {
	m := astTestManager(t)
	fid, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	intSort := m.external(m.arithmeticPlugin.fields[1])
	x, e := m.MkConst(astTestSymbol(t, m, "factor-x"), intSort)
	astTestMust(t, e)
	y, e := m.MkConst(astTestSymbol(t, m, "factor-y"), intSort)
	astTestMust(t, e)
	p, e := m.MkConst(astTestSymbol(t, m, "factor-p"), m.external(m.boolSort))
	astTestMust(t, e)
	for _, h := range []ASTHandle{x, y, p} {
		astTestPin(t, m, h)
	}
	q := m.numerals
	n, e := q.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, q.Del(&n)) }()
	numeral := func(text string) ASTHandle {
		astTestMust(t, q.SetString(&n, text))
		h, err := m.ArithmeticNumeral(&n, true)
		astTestMust(t, err)
		return h
	}
	zero, minusOne := numeral("0"), numeral("-1")
	negX, e := m.MkFamilyApp(fid, 9, nil, []ASTHandle{minusOne, x})
	astTestMust(t, e)
	difference, e := m.MkFamilyApp(fid, 6, nil, []ASTHandle{y, negX})
	astTestMust(t, e)
	eq, e := m.MkEq(difference, zero)
	astTestMust(t, e)
	conjunction, e := m.MkAnd(eq, p)
	astTestMust(t, e)
	astTestPin(t, m, conjunction)

	classes, e := NewExprEquivClass(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, classes.Close()) })
	rest, e := m.FactorEqualities([]ASTHandle{conjunction}, classes)
	astTestMust(t, e)
	if len(rest) != 1 || rest[0] != p {
		t.Fatal("factor remainder", rest)
	}
	same, e := classes.AreEquivalent(x, y)
	astTestMust(t, e)
	if !same {
		t.Fatal("normalized y + -1*x = 0 did not merge x and y")
	}
}

func TestASTExprEquivClassEmissionAndOwnership(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	a, e := m.MkConst(astTestSymbol(t, m, "equiv-a"), b)
	astTestMust(t, e)
	c, e := m.MkConst(astTestSymbol(t, m, "equiv-c"), b)
	astTestMust(t, e)
	d, e := m.MkConst(astTestSymbol(t, m, "equiv-d"), b)
	astTestMust(t, e)
	for _, h := range []ASTHandle{a, c, d} {
		astTestPin(t, m, h)
	}
	equiv, e := NewExprEquivClass(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, equiv.Close()) })
	astTestMust(t, equiv.Merge(a, c))
	astTestMust(t, equiv.Merge(c, d))
	compact, e := m.EquivToExpr(equiv)
	astTestMust(t, e)
	if len(compact) != 2 {
		t.Fatal("representative equalities", len(compact))
	}
	for i := range compact {
		astTestMust(t, m.DelRef(&compact[i]))
	}
	full, dirty, e := m.EquivToExprFull(equiv)
	astTestMust(t, e)
	if !dirty || len(full) != 3 {
		t.Fatal("full equality closure", dirty, len(full))
	}
	for i := range full {
		astTestMust(t, m.DelRef(&full[i]))
	}
	astTestMust(t, equiv.Reset())
	if len(equiv.objects) != 0 || len(equiv.ids) != 0 {
		t.Fatal("reset retained class state")
	}
}

func TestASTExprEquivClassCloseAdvancesAfterNativeRefusalConsumesSlot(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var manager *ASTManager
	var equiv *ExprEquivClass
	active, callbacks := false, 0
	var closeErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && callbacks == 0 {
			callbacks++
			closeErr = equiv.Close()
		}
		return source
	}})
	must(err)
	numerals, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err = NewASTManager(ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if equiv != nil {
			must(equiv.Close())
		}
		must(manager.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("equivalence cleanup fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	left, err := manager.MkFreshConst("equiv-close-left", boolean)
	must(err)
	right, err := manager.MkFreshConst("equiv-close-right", boolean)
	must(err)
	equiv, err = NewExprEquivClass(manager)
	must(err)
	must(equiv.Merge(left, right))

	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || callbacks != 1 || !errors.Is(closeErr, ErrASTPending) {
		t.Fatal("native refusal did not stop equivalence cleanup", body, callbacks, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if equiv.nextRef < len(equiv.refs) && equiv.refs[equiv.nextRef] != (ASTRef{}) {
		must(manager.DelRef(&equiv.refs[equiv.nextRef]))
	}
	must(equiv.Close())
	must(equiv.Close())
	if !equiv.closed || equiv.nextRef != 0 || len(equiv.refs) != 0 {
		t.Fatal("equivalence cleanup did not advance past consumed slot")
	}
}
