package smtcore

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSMTPBStatisticsUnsignedWrapThroughRetainedOperations(t *testing.T) {
	m, s, pb, _, _ := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	pb.statistics.predicates = math.MaxUint32
	aux, err := m.MkFreshPseudoBooleanAux()
	astTestMust(t, err)
	astTestMust(t, s.Internalize(aux, true))
	if pb.statistics.predicates != 0 {
		t.Fatal("source predicate statistic did not wrap")
	}
	pb.statistics.compiled = math.MaxUint32
	pb.statistics.compiledVariables = math.MaxUint32
	pb.statistics.compiledClauses = math.MaxUint32
	astTestMust(t, pb.compileInequality(pb.constraints[0]))
	// ge(full,2,[x,y]) dualizes to le(full,0,[~x,~y]): one
	// comparator maximum with three network clauses, excluding link clauses.
	if pb.statistics.compiled != 0 || pb.statistics.compiledVariables != 0 || pb.statistics.compiledClauses != 2 {
		t.Fatal("source compiler-statistic wrap", pb.statistics)
	}
	_, forcing, fpb, atom, inputs := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	fpb.statistics.propagations = math.MaxUint32
	astTestMust(t, forcing.AssignLiteral(atom, NewSMTBooleanAxiom()))
	_, err = forcing.PropagateCore()
	astTestMust(t, err)
	if fpb.statistics.propagations != 1 {
		t.Fatal("two native argument propagations did not wrap", fpb.statistics)
	}
	for _, l := range inputs {
		v, err := forcing.Assignment(l)
		astTestMust(t, err)
		if v != lbool.True {
			t.Fatal("statistic wrap changed native forcing")
		}
	}

	_, state, other, predicate, args := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	other.statistics.conflicts = math.MaxUint32
	for _, l := range []SMTLiteral{predicate, args[0].Negated(), args[1].Negated()} {
		astTestMust(t, state.AssignLiteral(l, NewSMTBooleanAxiom()))
	}
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if !state.Inconsistent() || other.statistics.conflicts != 0 {
		t.Fatal("native PB conflict did not wrap source statistic", other.statistics)
	}
}

func TestSMTPBStatisticsSourceCollectionOrder(t *testing.T) {
	_, _, pb, _, _ := smtPBWatchFixture(t, []int64{4, 3, 2}, 6, false)
	pb.statistics = smtPseudoBooleanStatistics{conflicts: 1, propagations: 2, predicates: 3, compiled: 4, compiledClauses: 5, compiledVariables: 6}
	out := statistics.New()
	astTestMust(t, pb.CollectStatistics(out))
	want := []string{"pb conflicts", "pb propagations", "pb predicates", "pb compilations", "pb compiled clauses", "pb compiled vars"}
	if out.Size() != len(want) {
		t.Fatal("source PB statistic count", out.Size())
	}
	for i, key := range want {
		if string(out.Key(i)) != key || !out.IsUint(i) || out.UintValue(i) != uint32(i+1) {
			t.Fatal("source contribution order/width", i, out.Key(i))
		}
	}
}

func TestSMTPBLiteralNetworkStatisticsWrapBeforeFactoryAndClause(t *testing.T) {
	_, s, pb, _, _ := smtPBWatchFixture(t, []int64{1, 1}, 2, false)
	before := len(s.boolExpr)
	network := smtPBSort{theory: pb, variables: math.MaxUint32, clauses: math.MaxUint32, clauseVariables: math.MaxUint32}
	l := network.fresh()
	astTestMust(t, network.err)
	if l == SMTNullLiteral || len(s.boolExpr) != before+1 || network.variables != 0 {
		t.Fatal("source network fresh-variable counter did not wrap")
	}
	network.add(SMTTrueLiteral)
	astTestMust(t, network.err)
	if network.clauses != 0 || network.clauseVariables != 0 || len(s.testClauses()) != 0 {
		t.Fatal("source counts attempted tautological clauses before context simplification")
	}
}
