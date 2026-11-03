package fp

// Source: dl_compiler.cpp compile_rule_evaluation emits one variant for each
// recursive tail in source order, replacing only that occurrence by its delta.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func addBoundIntervalWideningTailRule(t *testing.T, f *boundIntervalPublicFixture, head ast.ASTHandle, tails []ast.ASTHandle, name string) {
	t.Helper()
	// These compiler oracles require an exact stored occurrence order;
	// formula normalization may reorder an input conjunction's operands.
	rule, err := f.e.rules.NewRule(head, tails, nil, name)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	fragmentMust(t, f.e.set.Add(rule))
}

func requireBoundIntervalSelfJoinTailOrder(t *testing.T, f *boundIntervalPublicFixture, reverse bool) {
	t.Helper()
	rule, err := f.e.set.Rule(0)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, rule.Close()) }()
	head, err := rule.Head()
	fragmentMust(t, err)
	left, negated, err := rule.Tail(0)
	fragmentMust(t, err)
	if negated {
		t.Fatal("self-join fixture has a negative first tail")
	}
	right, negated, err := rule.Tail(1)
	fragmentMust(t, err)
	if negated {
		t.Fatal("self-join fixture has a negative second tail")
	}
	arg := func(app ast.ASTHandle, index uint32) ast.ASTHandle {
		value, cause := f.m.GetArg(app, index)
		fragmentMust(t, cause)
		return value
	}
	// Match head-variable occurrences rather than assuming the normalizer's
	// variable numbering. The expected head is H(y,z) in either body order.
	if !reverse && (arg(left, 1) != arg(head, 0) || arg(right, 0) != arg(head, 1)) ||
		reverse && (arg(left, 0) != arg(head, 1) || arg(right, 1) != arg(head, 0)) {
		t.Fatal("stored self-join rule did not preserve the requested tail order")
	}
}

func TestDatalogBoundIntervalWideningTwoRecursiveTailVariants(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, reverse := range []bool{false, true} {
			for _, allOrNothing := range []bool{false, true} {
				name := kinds[0] + "/forward"
				if reverse {
					name = kinds[0] + "/reverse"
				}
				if allOrNothing {
					name += "/all-or-nothing"
				} else {
					name += "/tail-deltas"
				}
				t.Run(name, func(t *testing.T) {
					f := newBoundIntervalPublicFixture(t, kinds)
					h := f.declare("two-recursive-H", 2)
					f.fact(h, 0, 10)
					vars := make([]ast.ASTHandle, 4)
					for i := range vars {
						vars[i] = f.hold(f.m.MkVar(uint32(i), f.integer))
					}
					x, y, z, w := vars[0], vars[1], vars[2], vars[3]
					left, right := f.app(h, x, y), f.app(h, z, w)
					if reverse {
						left, right = right, left
					}
					addBoundIntervalWideningTailRule(t, f, f.app(h, y, z), []ast.ASTHandle{left, right}, "two-recursive-tails")
					requireBoundIntervalSelfJoinTailOrder(t, f, reverse)
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
					if allOrNothing {
						fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
					}
					var initial ast.ASTRef
					for run := 0; run < 2; run++ {
						answer := boundIntervalWideningAnswer(t, f, h)
						initial = answer
						// Each default variant sees the target widened by earlier
						// variants at every full-relation occurrence. All-or-nothing
						// executes one full/full variant and keeps both endpoints.
						f.checkFormula(answer, []int{100, 5}, true)
						f.checkFormula(answer, []int{-1, 5}, !reverse && !allOrNothing)
						f.checkFormula(answer, []int{0, 11}, reverse && !allOrNothing)
					}
					if kinds[0] == "bound_relation" && !reverse && !allOrNothing {
						f.fact(h, 5, 20)
						if _, err := f.e.Answer(h); !errors.Is(err, ErrPropositionalEngine) {
							t.Fatal("seed update exposed a stale two-tail answer", err)
						}
						updated := boundIntervalWideningAnswer(t, f, h)
						f.checkFormula(updated, []int{-100, 20}, true)
						f.checkFormula(updated, []int{0, 21}, false)
						f.checkFormula(initial, []int{0, 11}, false)
						f.m.Limit.Cancel()
						_, err := f.e.QueryPredicate(h)
						f.m.Limit.ResetCancel()
						if err == nil {
							t.Fatal("two-tail widening ignored cancellation")
						}
						if _, err := f.e.Answer(h); !errors.Is(err, ErrPropositionalEngine) {
							t.Fatal("canceled two-tail query exposed an answer", err)
						}
						f.checkFormula(boundIntervalWideningAnswer(t, f, h), []int{-100, 20}, true)
					}
				})
			}
		}
	}
}

func TestDatalogBoundIntervalWideningRecursiveAndLowerTail(t *testing.T) {
	for _, lowerFirst := range []bool{false, true} {
		for _, allOrNothing := range []bool{false, true} {
			name := "recursive-first"
			if lowerFirst {
				name = "lower-first"
			}
			if allOrNothing {
				name += "/all-or-nothing"
			} else {
				name += "/tail-delta"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, []string{"interval_relation", "bound_relation"})
				h, lower, seed := f.declare("lower-tail-H", 2), f.declare("lower-tail-L", 2), f.declare("lower-tail-E", 2)
				f.fact(h, 0, 10)
				f.fact(seed, 100, 200)
				vars := make([]ast.ASTHandle, 4)
				for i := range vars {
					vars[i] = f.hold(f.m.MkVar(uint32(i), f.integer))
				}
				x, y, z, w := vars[0], vars[1], vars[2], vars[3]
				left, right := f.app(h, x, y), f.app(lower, z, w)
				if lowerFirst {
					left, right = right, left
				}
				addBoundIntervalWideningTailRule(t, f, f.app(h, y, z), []ast.ASTHandle{left, right}, "recursive-and-lower")
				rule, err := f.e.set.Rule(0)
				fragmentMust(t, err)
				first, err := rule.TailDeclaration(0)
				fragmentMust(t, err)
				fragmentMust(t, rule.Close())
				if !lowerFirst && first != h || lowerFirst && first != lower {
					t.Fatal("stored rule changed the lower-tail position")
				}
				// The lower relation has no direct seed: its full relation must
				// be computed first and supplied alongside the recursive delta.
				addBoundIntervalWideningTailRule(t, f, f.app(lower, z, w), []ast.ASTHandle{f.app(seed, z, w)}, "populate-lower")
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				if allOrNothing {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
				}
				answer := boundIntervalWideningAnswer(t, f, h)
				f.checkFormula(answer, []int{100, 100}, true)
				f.checkFormula(answer, []int{-1, 100}, false)
				f.checkFormula(answer, []int{0, 9}, false)
			})
		}
	}
}
