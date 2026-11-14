package tactic

// Source: src/sat/tactic/atom2bool_var.cpp:25-36;
// src/sat/tactic/goal2sat.cpp:691-779.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSATGoalInverseLiteralOwnership(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	sort, e := m.MkBoolSort()
	satOK(t, e)
	p, e := m.MkConst(astTestSymbol(t, m, "inverse-p"), sort)
	satOK(t, e)
	satOK(t, tr.Compile([]ast.ASTHandle{p}))
	v, e := tr.Variable(p)
	satOK(t, e)
	var lits satGoalLiteralExpressions
	t.Cleanup(func() { satOK(t, lits.close(m)) })
	satOK(t, lits.resize(m, 2*(uint32(v)+2)))
	satOK(t, tr.inverseAtoms(&lits))
	if lits.values[2*uint32(v)] != p {
		t.Fatal("lost positive atom")
	}
	neg := lits.values[2*uint32(v)+1]
	yes, child, e := m.IsNot(neg)
	satOK(t, e)
	if !yes || child != p {
		t.Fatal("incorrect negative atom")
	}
	if !lits.values[len(lits.values)-1].IsNull() {
		t.Fatal("unmapped literal populated")
	}
	satOK(t, tr.Close())
	_, e = m.GetKind(p)
	satOK(t, e)
	// INC incoming before DEC old preserves assignment to the same expression.
	satOK(t, lits.set(m, 2*uint32(v), p))
	satOK(t, lits.close(m))
	if m.IsLiveHandle(p) {
		t.Fatal("inverse vector leaked its AST roots")
	}
}

func TestSATGoalRecoveryIndependentBinaryTruthTables(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			m := astTestManager(t)
			s, _, _ := satSolverCheckFixture(t)
			tr, e := NewGoalTranslator(m, s)
			satOK(t, e)
			t.Cleanup(func() { satOK(t, tr.Close()) })
			p := goalTestAtom(t, m, "recover-p")
			q := goalTestAtom(t, m, "recover-q")
			// The initial OR registers both atoms and is included in the independent
			// reference CNF; every recovered assignment must also satisfy it.
			f, e := m.MkOr(p, q)
			satOK(t, e)
			satOK(t, tr.Compile([]ast.ASTHandle{f}))
			pv, e := tr.Variable(p)
			satOK(t, e)
			qv, e := tr.Variable(q)
			satOK(t, e)
			cnf := [][]sat.Literal{{sat.MakeLiteral(0, false), sat.MakeLiteral(1, false)}}
			for i := 0; i < 4; i++ {
				if mask&(1<<i) != 0 {
					satOK(t, s.MkClause([]sat.Literal{sat.MakeLiteral(pv, i&1 != 0), sat.MakeLiteral(qv, i&2 != 0)}))
					cnf = append(cnf, []sat.Literal{sat.MakeLiteral(0, i&1 != 0), sat.MakeLiteral(1, i&2 != 0)})
				}
			}
			g := goalTestNew(t, m, false, false)
			satOK(t, recoverSATGoalFormulas(tr, g, false, ^uint64(0)))
			forms, e := g.AppendFormulas(nil)
			satOK(t, e)
			for bits := 0; bits < 4; bits++ {
				check, _, _ := satSolverCheckFixture(t)
				back, e := NewGoalTranslator(m, check)
				satOK(t, e)
				t.Cleanup(func() { satOK(t, back.Close()) })
				input := append([]ast.ASTHandle(nil), forms...)
				for i, h := range []ast.ASTHandle{p, q} {
					if bits&(1<<i) == 0 {
						h, e = m.MkNot(h)
						satOK(t, e)
						astTestPin(t, m, h)
					}
					input = append(input, h)
				}
				satOK(t, back.Compile(input))
				got, e := check.Check(nil)
				satOK(t, e)
				want := lbool.True
				for _, cl := range cnf {
					yes := false
					for _, l := range cl {
						yes = yes || ((bits&(1<<uint32(l.Variable())) != 0) != l.Sign())
					}
					if !yes {
						want = lbool.False
					}
				}
				if got != want {
					t.Fatalf("mask %d assignment %d: got %v want %v", mask, bits, got, want)
				}
			}
		})
	}
}

func TestSATGoalRecoveryFreshUnitsLearnedAndCheckpoints(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	for i := 0; i < 4; i++ {
		_, e = s.MkVar(false, true)
		satOK(t, e)
	}
	satOK(t, s.MkClause([]sat.Literal{sat.MakeLiteral(0, true)}))
	satOK(t, s.MkClause([]sat.Literal{sat.MakeLiteral(1, false), sat.MakeLiteral(2, true), sat.MakeLiteral(3, false)}))
	_, e = s.Lifecycle.MkClauseCore(2, []sat.Literal{sat.MakeLiteral(1, true), sat.MakeLiteral(2, false)}, true)
	satOK(t, e)
	_, e = s.Lifecycle.MkClauseCore(3, []sat.Literal{sat.MakeLiteral(1, true), sat.MakeLiteral(2, false), sat.MakeLiteral(3, true)}, true)
	satOK(t, e)
	for _, learned := range []bool{false, true} {
		g := goalTestNew(t, m, false, false)
		satOK(t, recoverSATGoalFormulas(tr, g, learned, ^uint64(0)))
		n, e := g.Size()
		satOK(t, e)
		want := uint32(2)
		if learned {
			want = 4
		}
		if n != want {
			t.Fatal("source unit/binary/original/learned count", n, want)
		}
		first, e := g.Form(0)
		satOK(t, e)
		yes, atom, e := m.IsNot(first)
		satOK(t, e)
		if !yes {
			t.Fatal("negative unit sign lost")
		}
		ok, e := tr.isUninterpretedConstant(atom)
		satOK(t, e)
		if !ok {
			t.Fatal("missing fresh Boolean auxiliary")
		}
		// Source order is units, binaries, originals, then learned long clauses.
		for i, arity := range []uint32{3, 3} {
			index := uint32(i + 1)
			if learned {
				index = uint32(i + 2)
			} else if i == 1 {
				continue
			}
			f, e := g.Form(index)
			satOK(t, e)
			got, e := m.GetNumArgs(f)
			satOK(t, e)
			if got != arity {
				t.Fatal("long-clause order", index, got, arity)
			}
		}
	}
	g := goalTestNew(t, m, false, false)
	m.Limit.Cancel()
	if e = recoverSATGoalFormulas(tr, g, false, ^uint64(0)); e == nil || !strings.Contains(e.Error(), "canceled") {
		t.Fatal("missing source cancellation", e)
	}
	m.Limit.ResetCancel()
	// The checkpoint reads actual Runtime allocations, excluding native slices.
	block, e := s.RuntimeOwner().Allocate(1)
	satOK(t, e)
	defer func() { satOK(t, s.RuntimeOwner().Deallocate(&block)) }()
	if e = recoverSATGoalFormulas(tr, g, false, 0); e == nil || !strings.Contains(e.Error(), "memory") {
		t.Fatal("missing source memory checkpoint", e)
	}
	// Inconsistent shortcut precedes initialization/checkpoint in the source.
	satOK(t, s.MkClause([]sat.Literal{sat.MakeLiteral(0, false)}))
	m.Limit.Cancel()
	satOK(t, recoverSATGoalFormulas(tr, g, false, 0))
	decided := g.IsDecidedUnsat()
	if !decided {
		t.Fatal("inconsistent solver did not recover false")
	}
	m.Limit.ResetCancel()
}

func TestSATGoalRecoveryLongClauseBufferGrowth(t *testing.T) {
	m := astTestManager(t)
	s, _, _ := satSolverCheckFixture(t)
	tr, e := NewGoalTranslator(m, s)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, tr.Close()) })
	var clause []sat.Literal
	for i := 0; i < 40; i++ {
		v, e := s.MkVar(false, true)
		satOK(t, e)
		clause = append(clause, sat.MakeLiteral(v, i%2 != 0))
	}
	satOK(t, s.MkClause(clause))
	// Read actual stored order: source clause creation may reorder watches.
	stored, e := s.Clauses.AppendLiterals(s.Lifecycle.Original[0], nil)
	satOK(t, e)
	g := goalTestNew(t, m, false, false)
	satOK(t, recoverSATGoalFormulas(tr, g, false, ^uint64(0)))
	n, e := g.Size()
	satOK(t, e)
	if n != 1 {
		t.Fatal(n)
	}
	f, e := g.Form(0)
	satOK(t, e)
	n, e = m.GetNumArgs(f)
	satOK(t, e)
	if n != 40 {
		t.Fatal("native slice lost literals", n)
	}
	for i, l := range stored {
		a, e := m.GetArg(f, uint32(i))
		satOK(t, e)
		neg, _, e := m.IsNot(a)
		satOK(t, e)
		if neg != l.Sign() {
			t.Fatal("literal order/sign", i, l, neg)
		}
	}
	// Recovery's local references have retired; the actual goal still owns all
	// forty generated expressions after translator/solver destruction.
	satOK(t, tr.Close())
	satOK(t, s.Close())
	_, e = m.GetKind(f)
	satOK(t, e)
}
