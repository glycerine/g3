package smtcore

// Source: src/smt/theory_dense_diff_logic.h:32-42 and
// src/smt/theory_dense_diff_logic_def.h:263,565-576,705-709.

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSMTDenseDifferenceStatisticsSourceOperationsAndCollection(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTDenseDiffTheory(family, smtDenseMI)
	astTestMust(t, err)
	astTestMust(t, theory.ConfigureArithmetic(SMTArithState{Reflect: true}))
	astTestMust(t, state.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "dense-statistics-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "dense-statistics-y"), sort)
	astTestMust(t, err)
	fixture := smtIntegerDiffFixture{manager: m, family: family, state: state, sort: sort, x: x, y: y}
	strong := fixture.bound(t, smtArithOpLE, x, y, 2)
	weak := fixture.bound(t, smtArithOpLE, x, y, 3)
	for _, bound := range []ast.ASTHandle{strong, weak} {
		astTestMust(t, state.Internalize(bound, true))
	}
	strongLiteral, err := state.Literal(strong)
	astTestMust(t, err)
	weakLiteral, err := state.Literal(weak)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(strongLiteral, NewSMTBooleanAxiom()))
	astTestMust(t, state.PropagateTheoryAssignments())
	if assignment, err := state.Assignment(weakLiteral); err != nil || assignment != lbool.True {
		t.Fatal("dense weaker bound was not propagated", assignment, err)
	}
	theory.adapter.numAxioms = 3

	out := statistics.New()
	astTestMust(t, theory.CollectStatistics(out))
	want := []struct {
		key   statistics.Key
		value uint32
	}{
		{"dd assertions", 1},
		{"dd propagations", 1},
		{"arith eq adapter", 3},
	}
	if out.Size() != len(want) {
		t.Fatal("dense statistic count", out.Size())
	}
	for i, expected := range want {
		if out.Key(i) != expected.key {
			t.Fatal("dense statistic order", i, out.Key(i), expected.key)
		}
		if !out.IsUint(i) || out.UintValue(i) != expected.value {
			t.Fatal("dense statistic width/value", out.Key(i), out.UintValue(i), expected.value)
		}
	}
	if err = theory.CollectStatistics(nil); err != ast.ErrASTHandle {
		t.Fatal("nil statistics refusal", err)
	}

	theory.statistics = smtDenseDiffStatistics{assertions: math.MaxUint32, propagations: math.MaxUint32}
	astTestMust(t, theory.Assign(strongLiteral.Var(), true))
	if theory.statistics.assertions != 0 {
		t.Fatal("source uint32 assertion counter did not wrap", theory.statistics.assertions)
	}
}
