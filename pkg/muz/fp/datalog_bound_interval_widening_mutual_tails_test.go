package fp

// Source: dl_compiler.cpp compile_rule_evaluation selects one delta occurrence
// per variant; other operands remain live full relations in source head order.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func requireBoundIntervalMutualTailHeadOrder(t *testing.T, f *boundIntervalPublicFixture, first, second ast.ASTHandle) {
	t.Helper()
	valid, err := f.e.set.Finalize()
	fragmentMust(t, err)
	if !valid {
		t.Fatal("mutual two-tail fixture did not finalize")
	}
	strata, err := f.e.set.Strata()
	fragmentMust(t, err)
	if len(strata) != 1 || len(strata[0]) != 2 {
		t.Fatal("expected one two-head recursive component", strata)
	}
	dependencies := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, declaration := range []ast.ASTHandle{first, second} {
		dependencies[declaration], err = f.e.set.Dependencies(declaration)
		fragmentMust(t, err)
	}
	order, err := rel.SourceRecursivePredicateOrder(strata[0], dependencies)
	fragmentMust(t, err)
	if len(order) != 2 || order[0] != first || order[1] != second {
		t.Fatal("unexpected source recursive head order", order)
	}
}

func TestDatalogBoundIntervalWideningMutualTailOccurrencesUseLiveSiblings(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, reverse := range []bool{false, true} {
			for _, allOrNothing := range []bool{false, true} {
				name := kinds[0] + "/H-G-tails"
				if reverse {
					name = kinds[0] + "/G-H-tails"
				}
				if allOrNothing {
					name += "/all-or-nothing"
				} else {
					name += "/occurrence-deltas"
				}
				t.Run(name, func(t *testing.T) {
					f := newBoundIntervalPublicFixture(t, kinds)
					h, g := f.declare("mutual-tail-H", 2), f.declare("mutual-tail-G", 2)
					f.fact(h, 0, 10)
					f.fact(g, 20, 30)
					vars := make([]ast.ASTHandle, 4)
					for i := range vars {
						vars[i] = f.hold(f.m.MkVar(uint32(i), f.integer))
					}
					x, y, z, w := vars[0], vars[1], vars[2], vars[3]
					addBoundIntervalWideningTailRule(t, f, f.app(h, y, x), []ast.ASTHandle{f.app(g, x, y)}, "H-swap-G")
					left, right := f.app(h, x, y), f.app(g, z, w)
					if reverse {
						left, right = right, left
					}
					addBoundIntervalWideningTailRule(t, f, f.app(g, y, z), []ast.ASTHandle{left, right}, "G-cross-product")
					stored, err := f.e.set.Rule(1)
					fragmentMust(t, err)
					for i, want := range []ast.ASTHandle{left, right} {
						tail, negated, err := stored.Tail(i)
						fragmentMust(t, err)
						if negated || tail != want {
							t.Fatal("two-tail source occurrence changed", i)
						}
					}
					fragmentMust(t, stored.Close())
					requireBoundIntervalMutualTailHeadOrder(t, f, h, g)
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
					if allOrNothing {
						fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
					}
					for run := 0; run < 2; run++ {
						answerH := boundIntervalWideningAnswer(t, f, h)
						answerG := f.own(f.e.Answer(g))
						f.checkFormula(answerH, []int{30, 20}, true)
						f.checkFormula(answerH, []int{-1, 20}, false)
						f.checkFormula(answerH, []int{0, 9}, false)
						// G's nonselected H operand must see H widened earlier
						// in this iteration; its selected occurrence still reads
						// the old input delta. Swapping raw occurrences changes
						// which full G relation the later variant observes.
						f.checkFormula(answerG, []int{-100, 20}, true)
						f.checkFormula(answerG, []int{100, 20}, true)
						f.checkFormula(answerG, []int{0, 31}, reverse && !allOrNothing)
					}
				})
			}
		}
	}
}

func TestDatalogBoundIntervalWideningMutualTailKeepsOtherHeadDelta(t *testing.T) {
	for _, allOrNothing := range []bool{false, true} {
		name := "previous-deltas"
		if allOrNothing {
			name = "full-relations"
		}
		t.Run(name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			h, g := f.declare("empty-mutual-tail-H", 2), f.declare("empty-mutual-tail-G", 2)
			f.fact(h, 0, 10)
			vars := make([]ast.ASTHandle, 4)
			for i := range vars {
				vars[i] = f.hold(f.m.MkVar(uint32(i), f.integer))
			}
			x, y, z, w := vars[0], vars[1], vars[2], vars[3]
			addBoundIntervalWideningTailRule(t, f, f.app(h, y, z), []ast.ASTHandle{f.app(h, x, y), f.app(g, z, w)}, "H-cross-product")
			addBoundIntervalWideningTailRule(t, f, f.app(g, y, x), []ast.ASTHandle{f.app(h, x, y)}, "G-initialize-from-H")
			requireBoundIntervalMutualTailHeadOrder(t, f, g, h)
			fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
			if allOrNothing {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
			}
			answerH := boundIntervalWideningAnswer(t, f, h)
			answerG := f.own(f.e.Answer(g))
			f.checkFormula(answerH, []int{100, 10}, true)
			f.checkFormula(answerH, []int{-1, 10}, false)
			f.checkFormula(answerH, []int{0, 9}, false)
			f.checkFormula(answerH, []int{0, 11}, false)
			f.checkFormula(answerG, []int{10, 0}, true)
			// G's first populated delta must survive H's empty output delta
			// until the next whole-iteration transition. In full mode this
			// second pass widens G from H's current full interval.
			f.checkFormula(answerG, []int{10, 100}, allOrNothing)
			f.checkFormula(answerG, []int{10, -1}, false)
			f.checkFormula(answerG, []int{11, 0}, false)
		})
	}
}
