package smtcore

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtDiseqLiteral(t testing.TB, m *ast.ASTManager, s *SMTInternalizerState, a, b ast.ASTHandle) SMTLiteral {
	t.Helper()
	equality, err := m.MkEq(a, b)
	astTestMust(t, err)
	if variable, found := s.BoolVar(equality); found {
		return NewSMTLiteral(variable, true)
	}
	variable, err := s.MkBoolVar(equality)
	astTestMust(t, err)
	return NewSMTLiteral(variable, true)
}

func TestDisequalitySurvivesMergesOfEitherSide(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "diseq-merge", 4)
	s := f.s
	node := func(i int) *SMTEnode { return s.context.enodes.get(f.base[i]) }
	astTestMust(t, s.addDisequality(node(0), node(1), smtDiseqLiteral(t, f.m, s, f.base[0], f.base[1])))
	astTestMust(t, s.addEquality(node(0), node(2), NewSMTEqualityAxiom()))
	astTestMust(t, s.addEquality(node(1), node(3), NewSMTEqualityAxiom()))
	for _, pair := range [][2]int{{0, 1}, {2, 3}, {0, 3}, {2, 1}, {1, 0}, {3, 2}} {
		if !s.context.isDisequal(node(pair[0]), node(pair[1])) {
			t.Fatalf("lost disequality for %v", pair)
		}
	}
	if s.context.isDisequal(node(0), node(2)) {
		t.Fatal("a class is disequal from itself")
	}
}

func TestDisequalityIsRestoredByScopePop(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "diseq-scope", 5)
	s := f.s
	node := func(i int) *SMTEnode { return s.context.enodes.get(f.base[i]) }
	astTestMust(t, s.addDisequality(node(0), node(1), smtDiseqLiteral(t, f.m, s, f.base[0], f.base[1])))

	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(node(1), node(2), NewSMTEqualityAxiom()))
	astTestMust(t, s.addDisequality(node(2), node(3), smtDiseqLiteral(t, f.m, s, f.base[2], f.base[3])))
	if !s.context.isDisequal(node(0), node(2)) || !s.context.isDisequal(node(1), node(3)) {
		t.Fatal("scoped disequalities missing")
	}
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(node(3), node(4), NewSMTEqualityAxiom()))
	if !s.context.isDisequal(node(4), node(1)) {
		t.Fatal("disequality did not follow a nested merge")
	}
	astTestMust(t, s.PopScopes(2))

	if !s.context.isDisequal(node(0), node(1)) {
		t.Fatal("base disequality lost")
	}
	if s.context.isDisequal(node(0), node(2)) || s.context.isDisequal(node(1), node(3)) || s.context.isDisequal(node(2), node(3)) {
		t.Fatal("scoped disequality survived pop")
	}
	for i := 2; i < 5; i++ {
		if len(node(i).disequalities) != 0 {
			t.Fatalf("node %d kept %d entries", i, len(node(i).disequalities))
		}
	}
	if s.inconsistent {
		t.Fatal("unexpected conflict")
	}
	// The popped merge can happen again without a false conflict.
	astTestMust(t, s.addEquality(node(1), node(2), NewSMTEqualityAxiom()))
	if s.inconsistent {
		t.Fatal("retired disequality caused a conflict")
	}
}

// Disequality recorded before a same-scope merge into the same root: trails can
// reach the root in either order.
func TestDisequalityUndoToleratesAnyTrailOrder(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "diseq-order", 4)
	s := f.s
	node := func(i int) *SMTEnode { return s.context.enodes.get(f.base[i]) }
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addDisequality(node(0), node(1), smtDiseqLiteral(t, f.m, s, f.base[0], f.base[1])))
	astTestMust(t, s.addEquality(node(2), node(0), NewSMTEqualityAxiom()))
	astTestMust(t, s.addDisequality(node(0), node(3), smtDiseqLiteral(t, f.m, s, f.base[0], f.base[3])))
	astTestMust(t, s.PopScopes(1))
	for i := range f.base {
		if len(node(i).disequalities) != 0 {
			t.Fatalf("node %d kept %d entries", i, len(node(i).disequalities))
		}
	}
}

func TestSetDisequalToggleOnContext(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "diseq-toggle", 3)
	context := f.s.context
	a, b, c := context.enodes.get(f.base[0]), context.enodes.get(f.base[1]), context.enodes.get(f.base[2])
	astTestMust(t, context.SetDisequal(a, b, true))
	astTestMust(t, context.SetDisequal(a, c, true))
	if !context.isDisequal(a, b) || !context.isDisequal(c, a) {
		t.Fatal("SetDisequal did not record")
	}
	astTestMust(t, context.SetDisequal(b, a, false))
	if context.isDisequal(a, b) || !context.isDisequal(a, c) {
		t.Fatal("SetDisequal removed the wrong entry")
	}
	if err := context.SetDisequal(a, a, true); err == nil {
		t.Fatal("a term cannot be disequal from itself")
	}
}

// A decision and a merge must not copy or scan every recorded disequality.
func TestDisequalityPushAndMergeDoNotScaleWithCount(t *testing.T) {
	measure := func(count int) float64 {
		f := newSMTEqualityUndoFixture(t, fmt.Sprintf("diseq-scale-%d", count), count+3)
		s := f.s
		for i := 0; i < count; i++ {
			astTestMust(t, s.addDisequality(s.context.enodes.get(f.base[i+3]), s.context.enodes.get(f.base[i+2]), smtDiseqLiteral(t, f.m, s, f.base[i+3], f.base[i+2])))
		}
		left, right := s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1])
		return testing.AllocsPerRun(50, func() {
			astTestMust(t, s.PushScope())
			astTestMust(t, s.addEquality(left, right, NewSMTEqualityAxiom()))
			astTestMust(t, s.PopScopes(1))
		})
	}
	few, many := measure(5), measure(500)
	if many > few+2 {
		t.Fatalf("allocations grew from %v to %v with disequality count", few, many)
	}
}

func smtScanRelevantFalseEquality(t *testing.T, s *SMTInternalizerState, first, second *SMTEnode) bool {
	t.Helper()
	for variable := SMTFirstBoolVar; int(variable) < len(s.boolExpr); variable++ {
		expression := s.boolExpr[variable]
		left, right, isEquality, err := s.equalityAtomTerms(expression)
		astTestMust(t, err)
		if !isEquality || s.relevancy != nil && !s.relevancy.IsRelevant(expression) {
			continue
		}
		if s.assignment[NewSMTLiteral(variable, false).Index()] != lbool.False {
			continue
		}
		a, b := s.context.enodes.get(left), s.context.enodes.get(right)
		if a == nil || b == nil {
			continue
		}
		if a.root == first.root && b.root == second.root || a.root == second.root && b.root == first.root {
			return true
		}
	}
	return false
}

// The atom index must agree with scanning every variable, through scopes that
// create, assign, merge, and retire atoms.
func TestRelevantFalseEqualityMatchesFullScan(t *testing.T) {
	for seed := int64(0); seed < 30; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			f := newSMTEqualityUndoFixture(t, fmt.Sprintf("atom-index-%d", seed), 7)
			s := f.s
			depth := 0
			for step := 0; step < 150; step++ {
				pick := func() ast.ASTHandle { return f.base[r.Intn(len(f.base))] }
				switch action := r.Intn(12); {
				case action < 2:
					astTestMust(t, s.PushScope())
					depth++
				case action < 3 && depth > 0:
					count := 1 + r.Intn(depth)
					astTestMust(t, s.PopScopes(uint32(count)))
					depth -= count
				case action < 7:
					left, right := pick(), pick()
					if left == right {
						continue
					}
					equality, err := f.m.MkEq(left, right)
					astTestMust(t, err)
					astTestMust(t, s.Internalize(equality, true))
					astTestMust(t, s.markRelevant(equality))
					literal, err := s.Literal(equality)
					astTestMust(t, err)
					if r.Intn(2) == 0 && literal.Var() >= SMTFirstBoolVar && s.assignment[literal.Index()] == lbool.Undef {
						astTestMust(t, s.AssignLiteral(literal.Negated(), NewSMTBooleanAxiom()))
					}
				case action < 9:
					astTestMust(t, s.addEquality(s.context.enodes.get(pick()), s.context.enodes.get(pick()), NewSMTEqualityAxiom()))
				}
				for i := range f.base {
					for j := i + 1; j < len(f.base); j++ {
						first, second := s.context.enodes.get(f.base[i]), s.context.enodes.get(f.base[j])
						got, err := s.hasRelevantFalseEquality(first, second)
						astTestMust(t, err)
						want := first.root != second.root && smtScanRelevantFalseEquality(t, s, first, second)
						if got != want {
							t.Fatalf("step %d terms %d,%d: index %v, scan %v", step, i, j, got, want)
						}
					}
				}
				if s.inconsistent {
					return
				}
			}
		})
	}
}

// A disequality set directly on the context belongs to the enclosing scope too.
func TestContextSetDisequalIsRestoredByScopePop(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "diseq-direct", 2)
	s := f.s
	a, b := s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1])
	astTestMust(t, s.PushScope())
	astTestMust(t, s.context.SetDisequal(a, b, true))
	if !s.context.isDisequal(a, b) {
		t.Fatal("SetDisequal did not record")
	}
	astTestMust(t, s.PopScopes(1))
	if s.context.isDisequal(a, b) || len(a.disequalities) != 0 || len(b.disequalities) != 0 || len(s.context.disequalityTrail) != 0 {
		t.Fatal("direct disequality survived its scope")
	}
}
