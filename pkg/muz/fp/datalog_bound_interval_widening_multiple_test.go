package fp

// Source: dl_compiler.cpp compile_preds/compile_loop and
// dl_product_relation.cpp union_fn. Each recursive rule contributes its own
// product delta while reading the same incoming delta register.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestDatalogBoundIntervalWideningMultiplePerRuleDelta(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, allOrNothing := range []bool{false, true} {
			name := kinds[0] + "/component-deltas"
			if allOrNothing {
				name = kinds[0] + "/all-or-nothing-deltas"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				e, h := f.declare("multiple-widen-E", 4), f.declare("multiple-widen-H", 4)
				f.fact(e, 0, 10, 0, 10)
				f.fact(e, 10, 0, 0, 10)
				vars := make([]ast.ASTHandle, 4)
				for i := range vars {
					vars[i] = f.hold(f.m.MkVar(uint32(i), f.integer))
				}
				x, y, z, w := vars[0], vars[1], vars[2], vars[3]
				head, tail := f.app(h, vars...), f.app(e, vars...)
				ordered := f.hold(f.m.MkArithmeticLE(x, y))
				body := f.hold(f.m.MkAnd(tail, ordered))
				fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(body, head)), "initialize-weak-bound"))
				intervalHead := f.app(h, x, y, w, z)
				boundHead := f.app(h, y, x, z, w)
				fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(head, intervalHead)), "swap-interval-columns"))
				fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(head, boundHead)), "swap-bound-columns"))
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				if allOrNothing {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
				}
				var original ast.ASTRef
				for run := 0; run < 2; run++ {
					answer := boundIntervalWideningAnswer(t, f, h)
					original = answer
					// Rule one changes only intervals; rule two changes only the
					// weak bound. Each resulting product delta is independently
					// empty, so their partial components must not create a loop.
					f.checkFormula(answer, []int{10, 0, 100, 5}, true)
					f.checkFormula(answer, []int{0, 0, -1, 5}, false)
					f.checkFormula(answer, []int{0, 0, 0, 11}, false)
					f.checkFormula(answer, []int{-1, 0, 0, 5}, false)
					f.checkFormula(answer, []int{11, 0, 0, 5}, false)
				}
				if kinds[0] == "bound_relation" && !allOrNothing {
					guard := f.hold(f.m.MkArithmeticLE(x, f.numeral(-1)))
					guarded := f.hold(f.m.MkAnd(head, guard))
					fragmentMust(t, f.e.UpdateRule(f.hold(f.m.MkImplies(guarded, boundHead)), "swap-bound-columns"))
					answer := boundIntervalWideningAnswer(t, f, h)
					f.checkFormula(answer, []int{0, 10, 100, 5}, true)
					// The empty guarded intermediate is projected by C++ with
					// its reset payload, admitting this previously derived point.
					f.checkFormula(answer, []int{10, 0, 100, 5}, true)
					f.checkFormula(original, []int{10, 0, 100, 5}, true)
				}
			})
		}
	}
}

func TestDatalogBoundIntervalWideningMultipleShareInputSnapshot(t *testing.T) {
	for _, guardedFirst := range []bool{false, true} {
		name := "unguarded-first"
		if guardedFirst {
			name = "guarded-first"
		}
		t.Run(name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			h := f.declare("multiple-snapshot-H", 2)
			f.fact(h, 0, 10)
			x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
			tail, head := f.app(h, x, y), f.app(h, y, x)
			guard := f.hold(f.m.MkArithmeticLE(y, f.numeral(100)))
			guarded := f.hold(f.m.MkAnd(tail, guard))
			bodies := []ast.ASTHandle{tail, guarded}
			if guardedFirst {
				bodies[0], bodies[1] = bodies[1], bodies[0]
			}
			for i, body := range bodies {
				fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(body, head)), []string{"first-swap", "second-swap"}[i]))
			}
			fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
			answer := boundIntervalWideningAnswer(t, f, h)
			// Both rules consume the original (0,10) input delta. Reading
			// the first rule's widened target in the second would also lose
			// the lower x and upper y endpoints checked here.
			f.checkFormula(answer, []int{100, 5}, true)
			f.checkFormula(answer, []int{-1, 5}, false)
			f.checkFormula(answer, []int{0, 11}, false)
		})
	}
}

func TestDatalogBoundIntervalWideningMultipleAllOrNothingReplaysInitialization(t *testing.T) {
	for _, allOrNothing := range []bool{false, true} {
		name := "input-delta"
		if allOrNothing {
			name = "all-or-nothing"
		}
		t.Run(name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"interval_relation", "bound_relation"})
			family, err := f.m.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			f.integer = f.hold(f.m.MkFamilySort(family, 0, nil)) // Real signature for open endpoints.
			e, h := f.declare("real-widen-E", 1), f.declare("real-widen-H", 1)
			real := func(text string) ast.ASTHandle { return f.hold(f.m.ArithmeticRealNumeralText(text)) }
			zero, ten := real("0"), real("10")
			fragmentMust(t, f.e.AddFact(e, []ast.ASTHandle{zero}))
			fragmentMust(t, f.e.AddFact(e, []ast.ASTHandle{ten}))
			x := f.hold(f.m.MkVar(0, f.integer))
			ex, hx := f.app(e, x), f.app(h, x)
			strict := f.hold(f.m.MkAnd(ex, f.hold(f.m.MkArithmeticLT(x, ten))))
			fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(strict, hx)), "strict-init-first"))
			fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(ex, hx)), "closed-init-second"))
			fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(hx, hx)), "recursive-real-identity"))
			fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
			if allOrNothing {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
			}
			answer := boundIntervalWideningAnswer(t, f, h)
			formula, err := f.m.RefGet(answer)
			fragmentMust(t, err)
			substitution, err := rewriter.NewVarSubstitution(f.m, false)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, substitution.Close()) }()
			// Ordinary init unions produce [0,10]. All-or-nothing replays
			// the strict init rule using widening: source [0,10) against
			// closed target [0,10] drops the equal upper endpoint.
			for _, test := range []struct {
				value string
				want  bool
			}{{"10", true}, {"100", allOrNothing}, {"-1", false}} {
				ground := f.own(f.m.NewRef(ast.ASTHandle{}))
				fragmentMust(t, substitution.Substitute(formula, []ast.ASTHandle{real(test.value)}, ground))
				root, err := f.m.RefGet(ground)
				fragmentMust(t, err)
				rewritten := f.own(f.m.NewRef(ast.ASTHandle{}))
				fragmentMust(t, f.e.guard.RewriteWithBindings(root, nil, rewritten))
				result, err := f.m.RefGet(rewritten)
				fragmentMust(t, err)
				if f.m.IsTrue(result) != test.want || !f.m.IsTrue(result) && !f.m.IsFalse(result) {
					t.Fatal("real initialization replay formula", test.value, test.want)
				}
			}
		})
	}
}

func TestDatalogBoundIntervalWideningMultipleReplaysGroundRule(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	h := f.declare("ground-replay-H", 1)
	fragmentMust(t, f.e.addRuleEager(f.app(h, f.numeral(0)), "ground-seed-rule"))
	x := f.hold(f.m.MkVar(0, f.integer))
	tail := f.app(h, x)
	for i, value := range []int{10, 20} {
		fragmentMust(t, f.e.addRuleEager(f.hold(f.m.MkImplies(tail, f.app(h, f.numeral(value)))), []string{"first-constant", "second-constant"}[i]))
	}
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
	answer := boundIntervalWideningAnswer(t, f, h)
	f.checkFormula(answer, []int{100}, true)
	f.checkFormula(answer, []int{-1}, false)
}
