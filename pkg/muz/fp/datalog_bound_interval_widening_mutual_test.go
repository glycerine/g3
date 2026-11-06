package fp

// Source: dl_compiler.cpp detect_chains, compile_preds_init, compile_loop,
// and make_inloop_delta_transition. Every head reads the previous delta bank.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogBoundIntervalWideningMutualIdentity(t *testing.T) {
	for _, widen := range []bool{false, true} {
		name := "ordinary"
		if widen {
			name = "widen"
		}
		t.Run(name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			h, q := f.declare("identity-mutual-H", 1), f.declare("identity-mutual-Q", 1)
			f.fact(h, 0)
			x := f.hold(f.m.MkVar(0, f.integer))
			fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(f.app(h, x), f.app(q, x))), "recursive-first"))
			fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(f.app(q, x), f.app(h, x))), "recursive-second"))
			if widen {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
			}
			answer := boundIntervalWideningAnswer(t, f, h)
			f.checkFormula(answer, []int{0}, true)
			f.checkFormula(answer, []int{20}, false)
			f.checkFormula(answer, []int{21}, false)
		})
	}
}

func TestDatalogBoundIntervalWideningMutualUsesPreviousDeltas(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, gFirst := range []bool{false, true} {
			name := kinds[0] + "/H-first"
			if gFirst {
				name = kinds[0] + "/G-first"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				var h, g ast.ASTHandle
				if gFirst {
					g, h = f.declare("mutual-G", 2), f.declare("mutual-H", 2)
				} else {
					h, g = f.declare("mutual-H", 2), f.declare("mutual-G", 2)
				}
				f.fact(h, 0, 10)
				f.fact(g, 20, 30)
				x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
				hxy, gxy := f.app(h, x, y), f.app(g, x, y)
				hyx, gyx := f.app(h, y, x), f.app(g, y, x)
				if gFirst {
					addBoundIntervalWideningTailRule(t, f, gyx, []ast.ASTHandle{hxy}, "G-from-H")
					addBoundIntervalWideningTailRule(t, f, hyx, []ast.ASTHandle{gxy}, "H-from-G")
				} else {
					addBoundIntervalWideningTailRule(t, f, hyx, []ast.ASTHandle{gxy}, "H-from-G")
					addBoundIntervalWideningTailRule(t, f, gyx, []ast.ASTHandle{hxy}, "G-from-H")
				}
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				var oldH ast.ASTRef
				for run := 0; run < 2; run++ {
					oldH = boundIntervalWideningAnswer(t, f, h)
					answerG := f.own(f.e.Answer(g))
					f.checkFormula(oldH, []int{30, 20}, true)
					f.checkFormula(oldH, []int{-1, 20}, false)
					f.checkFormula(oldH, []int{0, 9}, false)
					f.checkFormula(answerG, []int{10, 0}, true)
					f.checkFormula(answerG, []int{21, 0}, false)
					f.checkFormula(answerG, []int{20, 31}, false)
				}
				if kinds[0] == "bound_relation" && !gFirst {
					guard := f.hold(f.m.MkArithmeticLE(x, f.numeral(-1)))
					body := f.hold(f.m.MkAnd(gxy, guard))
					fragmentMust(t, f.e.UpdateRule(f.hold(f.m.MkImplies(body, hyx)), "H-from-G"))
					updated := boundIntervalWideningAnswer(t, f, h)
					f.checkFormula(updated, []int{0, 10}, true)
					// The C++ empty-source projection admits this prior point in
					// the updated relation.
					f.checkFormula(updated, []int{30, 20}, true)
					f.checkFormula(oldH, []int{30, 20}, true)
					f.m.Limit.Cancel()
					_, err := f.e.QueryPredicate(h)
					f.m.Limit.ResetCancel()
					if err == nil {
						t.Fatal("mutual widening ignored cancellation")
					}
					if _, err := f.e.Answer(h); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("canceled mutual query exposed a result", err)
					}
					f.checkFormula(boundIntervalWideningAnswer(t, f, h), []int{0, 10}, true)
				}
			})
		}
	}
}

func TestDatalogBoundIntervalWideningMutualCycleBreakerOrder(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, allOrNothing := range []bool{false, true} {
			name := kinds[0] + "/previous-deltas"
			if allOrNothing {
				name = kinds[0] + "/full-relations"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				h, g := f.declare("ordered-mutual-H", 2), f.declare("ordered-mutual-G", 2)
				f.fact(h, 0, 10)
				f.fact(g, 0, 10)
				x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
				hxy, gxy := f.app(h, x, y), f.app(g, x, y)
				addBoundIntervalWideningTailRule(t, f, hxy, []ast.ASTHandle{hxy}, "H-identity-first")
				addBoundIntervalWideningTailRule(t, f, f.app(h, y, x), []ast.ASTHandle{gxy}, "ordered-H-from-G")
				addBoundIntervalWideningTailRule(t, f, f.app(g, y, x), []ast.ASTHandle{hxy}, "ordered-G-from-H")
				valid, err := f.e.set.Finalize()
				fragmentMust(t, err)
				if !valid {
					t.Fatal("mutual source rule set did not finalize")
				}
				strata, err := f.e.set.Strata()
				fragmentMust(t, err)
				if len(strata) != 1 || len(strata[0]) != 2 {
					t.Fatal("fixture does not contain one mutual component", strata)
				}
				dependencies := make(map[ast.ASTHandle][]ast.ASTHandle)
				for _, declaration := range []ast.ASTHandle{h, g} {
					dependencies[declaration], err = f.e.set.Dependencies(declaration)
					fragmentMust(t, err)
				}
				order, err := rel.SourceRecursivePredicateOrder(strata[0], dependencies)
				fragmentMust(t, err)
				if len(order) != 2 || order[0] != g || order[1] != h {
					t.Fatal("source cycle breaker must evaluate G before H", order)
				}
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				if allOrNothing {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
				}
				answerH := boundIntervalWideningAnswer(t, f, h)
				answerG := f.own(f.e.Answer(g))
				f.checkFormula(answerG, []int{100, 5}, true)
				f.checkFormula(answerG, []int{-1, 5}, false)
				f.checkFormula(answerG, []int{0, 11}, false)
				f.checkFormula(answerH, []int{100, 5}, true)
				// Only the full-relation mode lets later H consume G's newly
				// widened relation; previous deltas keep both original limits.
				f.checkFormula(answerH, []int{-1, 5}, allOrNothing)
				f.checkFormula(answerH, []int{0, 11}, allOrNothing)
			})
		}
	}
}

func TestDatalogBoundIntervalWideningMutualLowerInitializationAndArities(t *testing.T) {
	for _, mode := range []struct {
		name                string
		widen, allOrNothing bool
	}{{name: "ordinary"}, {name: "previous-deltas", widen: true}, {name: "full-relations", widen: true, allOrNothing: true}} {
		t.Run(mode.name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			h, g, seed := f.declare("arity-mutual-H", 2), f.declare("arity-mutual-G", 1), f.declare("arity-mutual-E", 2)
			f.fact(seed, 0, 10)
			x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
			hxy, gx := f.app(h, x, y), f.app(g, x)
			addBoundIntervalWideningTailRule(t, f, gx, []ast.ASTHandle{hxy}, "project-H-to-G")
			addBoundIntervalWideningTailRule(t, f, f.app(h, x, x), []ast.ASTHandle{gx}, "repeat-G-to-H")
			// Both recursive heads start empty; the later-listed lower rule
			// must initialize H before the mutual loop takes its delta copies.
			addBoundIntervalWideningTailRule(t, f, hxy, []ast.ASTHandle{f.app(seed, x, y)}, "lower-initialization")
			if mode.widen {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
			}
			if mode.allOrNothing {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
			}
			answerH := boundIntervalWideningAnswer(t, f, h)
			answerG := f.own(f.e.Answer(g))
			f.checkFormula(answerG, []int{0}, true)
			// Ordinary initialization projects an empty intermediate and
			// therefore exposes the reset payload; widening keeps its bounds.
			f.checkFormula(answerG, []int{1}, !mode.widen)
			f.checkFormula(answerH, []int{0, 5}, true)
			f.checkFormula(answerH, []int{0, -100}, true)
			f.checkFormula(answerH, []int{0, 11}, !mode.widen)
			f.checkFormula(answerH, []int{1, 5}, !mode.widen)
		})
	}
}
