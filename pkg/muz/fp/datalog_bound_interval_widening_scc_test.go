package fp

// Source: dl_compiler.cpp detect_chains and compile_dependent_rules. All SCC
// predicates have global input/output deltas, including non-cycle-breakers.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func requireBoundIntervalThreeHeadOrder(t *testing.T, f *boundIntervalPublicFixture, a, b, c ast.ASTHandle) {
	t.Helper()
	valid, err := f.e.set.Finalize()
	fragmentMust(t, err)
	if !valid {
		t.Fatal("three-head rule set did not finalize")
	}
	strata, err := f.e.set.Strata()
	fragmentMust(t, err)
	if len(strata) != 1 || len(strata[0]) != 3 {
		t.Fatal("expected one three-head component", strata)
	}
	dependencies := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, head := range strata[0] {
		dependencies[head], err = f.e.set.Dependencies(head)
		fragmentMust(t, err)
	}
	order, err := rel.SourceRecursivePredicateOrder(strata[0], dependencies)
	fragmentMust(t, err)
	if len(order) != 3 || order[0] != b || order[1] != a || order[2] != c {
		t.Fatal("cycle breaker must remove C and order B,A,C", order)
	}
}

func TestDatalogBoundIntervalWideningThreeHeadAsymmetricCycle(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, allOrNothing := range []bool{false, true} {
			name := kinds[0] + "/previous-deltas"
			if allOrNothing {
				name = kinds[0] + "/full-relations"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				a, b, c := f.declare("three-A", 2), f.declare("three-B", 2), f.declare("three-C", 2)
				f.fact(a, 0, 10)
				f.fact(b, 20, 30)
				f.fact(c, -20, -10)
				x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
				addBoundIntervalWideningTailRule(t, f, f.app(a, y, x), []ast.ASTHandle{f.app(b, x, y)}, "A-from-B")
				addBoundIntervalWideningTailRule(t, f, f.app(b, y, x), []ast.ASTHandle{f.app(c, x, y)}, "B-from-C")
				addBoundIntervalWideningTailRule(t, f, f.app(c, y, x), []ast.ASTHandle{f.app(a, x, y)}, "C-from-A")
				requireBoundIntervalThreeHeadOrder(t, f, a, b, c)
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				if allOrNothing {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
				}
				answerA := boundIntervalWideningAnswer(t, f, a)
				answerB, answerC := f.own(f.e.Answer(b)), f.own(f.e.Answer(c))
				f.checkFormula(answerA, []int{30, 20}, true)
				f.checkFormula(answerA, []int{-1, 20}, allOrNothing)
				f.checkFormula(answerA, []int{0, 9}, allOrNothing)
				f.checkFormula(answerB, []int{-10, -20}, true)
				f.checkFormula(answerB, []int{21, 0}, false)
				f.checkFormula(answerB, []int{20, 31}, false)
				f.checkFormula(answerC, []int{10, 0}, true)
				f.checkFormula(answerC, []int{-21, 0}, allOrNothing)
				f.checkFormula(answerC, []int{0, -11}, allOrNothing)
			})
		}
	}
}

func TestDatalogBoundIntervalWideningThreeHeadEmptyDeltaWaves(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, allOrNothing := range []bool{false, true} {
			name := kinds[0] + "/previous-deltas"
			if allOrNothing {
				name = kinds[0] + "/full-relations"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				a, b, c := f.declare("wave-A", 1), f.declare("wave-B", 1), f.declare("wave-C", 1)
				f.fact(a, 7)
				x := f.hold(f.m.MkVar(0, f.integer))
				addBoundIntervalWideningTailRule(t, f, f.app(a, x), []ast.ASTHandle{f.app(b, x)}, "wave-A-from-B")
				addBoundIntervalWideningTailRule(t, f, f.app(b, x), []ast.ASTHandle{f.app(c, x)}, "wave-B-from-C")
				addBoundIntervalWideningTailRule(t, f, f.app(c, x), []ast.ASTHandle{f.app(a, x)}, "wave-C-from-A")
				requireBoundIntervalThreeHeadOrder(t, f, a, b, c)
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				if allOrNothing {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
				}
				// The cycle-breaker C begins empty; A alone must start the
				// loop, then C and B keep later iterations active in turn.
				boundIntervalWideningAnswer(t, f, a)
				for _, head := range []ast.ASTHandle{a, b, c} {
					answer := f.own(f.e.Answer(head))
					f.checkFormula(answer, []int{7}, true)
					f.checkFormula(answer, []int{6}, false)
					f.checkFormula(answer, []int{8}, false)
				}
			})
		}
	}
}

func TestDatalogBoundIntervalWideningRefusesUnsupportedRecursiveComponents(t *testing.T) {
	for _, shape := range []string{"three-positive-tails", "negative-tail"} {
		for _, widen := range []bool{false, true} {
			name := shape + "/ordinary"
			if widen {
				name = shape + "/widen"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
				h, p, q := f.declare("guard-H", 1), f.declare("guard-P", 1), f.declare("guard-Q", 1)
				f.fact(p, 0)
				x := f.hold(f.m.MkVar(0, f.integer))
				tails := []ast.ASTHandle{f.app(p, x), f.app(q, x)}
				negated := []bool{false, true}
				if shape == "three-positive-tails" {
					tails = append(tails, f.app(h, x))
					negated = nil
				}
				rule, err := f.e.rules.NewRule(f.app(h, x), tails, negated, "unsupported-tail-shape")
				fragmentMust(t, err)
				fragmentMust(t, f.e.set.Add(rule))
				fragmentMust(t, rule.Close())
				if widen {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				}
				if _, err := f.e.QueryPredicate(h); !errors.Is(err, ErrBoundIntervalDatalogFragment) {
					t.Fatal("unsupported tail shape accepted", err)
				}
				if _, err := f.e.Answer(h); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("refused tail shape published an answer", err)
				}
			})
		}
	}
}
