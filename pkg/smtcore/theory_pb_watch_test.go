package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func smtPBWatchFixture(t *testing.T, weights []int64, bound int64, equality bool) (*ast.ASTManager, *SMTInternalizerState, *SMTPseudoBooleanTheory, SMTLiteral, []SMTLiteral) {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	_, _, s := smtInternalizerFixture(t, m, nil)
	pb, err := NewSMTPseudoBooleanTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(pb))
	var args []ast.ASTHandle
	var coefficients []numeral.OwnedRational
	service := m.NumeralService()
	for i, w := range weights {
		args = append(args, smtRelevancyBool(t, m, fmt.Sprintf("native-pb-%d", i)))
		q, err := service.NewValue()
		astTestMust(t, err)
		astTestMust(t, service.SetInt64(&q, w))
		coefficients = append(coefficients, q)
	}
	k, err := service.NewValue()
	astTestMust(t, err)
	astTestMust(t, service.SetInt64(&k, bound))
	var expression ast.ASTHandle
	if equality {
		expression, err = m.MkPseudoBooleanEQ(coefficients, args, &k)
	} else {
		expression, err = m.MkPseudoBooleanGE(coefficients, args, &k)
	}
	astTestMust(t, err)
	for i := range coefficients {
		astTestMust(t, service.Del(&coefficients[i]))
	}
	astTestMust(t, service.Del(&k))
	astTestMust(t, s.Internalize(expression, true))
	atom, err := s.Literal(expression)
	astTestMust(t, err)
	var literals []SMTLiteral
	for _, arg := range args {
		astTestMust(t, s.Internalize(arg, true))
		l, err := s.Literal(arg)
		astTestMust(t, err)
		literals = append(literals, l)
	}
	return m, s, pb, atom, literals
}

func smtPBWatchAssign(t *testing.T, s *SMTInternalizerState, literal SMTLiteral) {
	t.Helper()
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err := s.PropagateCore()
	astTestMust(t, err)
}

func TestSMTPBWeightedWatchReplacementJustificationAndRollback(t *testing.T) {
	_, s, pb, atom, ls := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
	if len(s.testClauses()) != 0 || len(pb.constraints) != 1 {
		t.Fatal("weighted atom was encoded into SAT", len(s.testClauses()))
	}
	c := pb.constraints[0]
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, s.PushScope())
		smtPBWatchAssign(t, s, atom)
		// Source watch permutations persist across pop; only the first
		// activation has the original literal ordering.
		if !c.active || attempt == 0 && (c.watchSize != 4 || c.watchSum.RatString() != "11") || c.watchSum.Cmp(big.NewRat(10, 1)) < 0 || c.maxWatch.RatString() != "4" {
			t.Fatal("initial weighted watch prefix", c.watchSize, &c.watchSum, &c.maxWatch, &c.current().bound)
		}
		smtPBWatchAssign(t, s, ls[0].Negated())
		value, err := s.Assignment(ls[1])
		astTestMust(t, err)
		if value != lbool.True || c.watchSize != 4 || c.watchSum.RatString() != "8" || c.maxWatch.RatString() != "3" {
			t.Fatal("failed watch replacement/weighted propagation", value, c.watchSize, &c.watchSum, &c.maxWatch)
		}
		data, err := s.BooleanData(ls[1].Var())
		astTestMust(t, err)
		owner, _ := data.Justification().Justification()
		j, ok := owner.(*smtPBJustification)
		if !ok || j.constraint != c || len(j.Literals) != 2 || j.Literals[0] != ls[0].Negated() || j.Literals[1] != atom || j.Consequent != ls[1] {
			t.Fatal("lost ordered PB premises", owner)
		}
		smtPBWatchAssign(t, s, ls[2].Negated())
		for _, l := range []SMTLiteral{ls[3], ls[4]} {
			value, err := s.Assignment(l)
			astTestMust(t, err)
			if value != lbool.True {
				t.Fatal("weighted deficit did not force remaining inputs", l, value)
			}
		}
		if len(s.testClauses()) != 0 || c.watchSum.RatString() != "6" {
			t.Fatal("propagation used encoding clauses or incorrect watch sum")
		}
		astTestMust(t, s.PopScopes(1))
		if c.active || c.negative || c.watchSize != 0 || c.fixed != 0 || c.maxSum.RatString() != "12" || c.minSum.Sign() != 0 {
			t.Fatal("PB scope trail did not restore variable-watch mode")
		}
		for _, l := range ls {
			if len(pb.info(l.Var()).variables) != 1 {
				t.Fatal("variable watches were lost or duplicated on pop")
			}
		}
	}
	astTestMust(t, s.PushScope())
	smtPBWatchAssign(t, s, atom.Negated())
	if !c.negative || c.current().bound.RatString() != "7" {
		t.Fatal("negative atom did not select normalized complement")
	}
	astTestMust(t, s.PopScopes(1))
}

func TestSMTPBUnassignedAtomBoundsAndEqualityInference(t *testing.T) {
	for _, positive := range []bool{false, true} {
		_, s, pb, atom, ls := smtPBWatchFixture(t, []int64{4, 3, 2, 2, 1}, 6, false)
		astTestMust(t, s.PushScope())
		for _, l := range ls[:2] {
			if !positive {
				l = l.Negated()
			}
			smtPBWatchAssign(t, s, l)
		}
		value, err := s.Assignment(atom)
		astTestMust(t, err)
		want := lbool.False
		if positive {
			want = lbool.True
		}
		if value != want {
			t.Fatal("variable-watch bounds did not infer atom", positive, value)
		}
		if pb.statistics.propagations == 0 || len(s.testClauses()) != 0 {
			t.Fatal("atom inference bypassed native watches")
		}
		astTestMust(t, s.PopScopes(1))
	}
	for _, holds := range []bool{false, true} {
		_, s, pb, atom, ls := smtPBWatchFixture(t, []int64{2, 3, 4}, 5, true)
		astTestMust(t, s.PushScope())
		smtPBWatchAssign(t, s, ls[0])
		smtPBWatchAssign(t, s, ls[1])
		last := ls[2].Negated()
		if !holds {
			last = ls[2]
		}
		smtPBWatchAssign(t, s, last)
		value, err := s.Assignment(atom)
		astTestMust(t, err)
		want := lbool.False
		if holds {
			want = lbool.True
		}
		if value != want || pb.constraints[0].fixed != 3 {
			t.Fatal("source equality variable-watch inference", value, pb.constraints[0].fixed)
		}
		astTestMust(t, s.PopScopes(1))
	}
}

func TestSMTPBLargeArityUsesLinearWatchStorage(t *testing.T) {
	weights := make([]int64, 64)
	for i := range weights {
		weights[i] = 1
	}
	_, s, pb, atom, ls := smtPBWatchFixture(t, weights, 32, false)
	if len(s.testClauses()) != 0 || len(s.boolExpr) != 66 || len(pb.constraints) != 1 {
		t.Fatal("large PB predicate expanded to an encoding")
	}
	astTestMust(t, s.PushScope())
	smtPBWatchAssign(t, s, atom)
	for _, l := range ls[:33] {
		astTestMust(t, s.AssignLiteral(l.Negated(), NewSMTBooleanAxiom()))
	}
	_, err := s.PropagateCore()
	astTestMust(t, err)
	if !s.inconsistent || pb.statistics.conflicts == 0 {
		t.Fatal("weighted watch exhaustion failed to conflict")
	}
	astTestMust(t, s.PopScopes(1))
	if s.inconsistent {
		t.Fatal("PB conflict survived rollback")
	}
}

func TestSMTPBConstantEqualityDoesNotWaitForAbsentCallbacks(t *testing.T) {
	for _, bound := range []int64{-1, 0, 1} {
		_, s, pb, atom, _ := smtPBWatchFixture(t, nil, bound, true)
		value, err := s.Assignment(atom)
		astTestMust(t, err)
		want := lbool.False
		if bound == 0 {
			want = lbool.True
		}
		if value != want || len(pb.constraints) != 0 {
			t.Fatal("constant equality left an inert native constraint", bound, value)
		}
	}
}
