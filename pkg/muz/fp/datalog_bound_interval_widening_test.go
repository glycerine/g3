package fp

// Source: src/muz/rel/dl_compiler.cpp, compile_preds_init and compile_loop;
// product_relation union/widen constructs a delta from its component deltas.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func boundIntervalWideningAnswer(t *testing.T, f *boundIntervalPublicFixture, declaration ast.ASTHandle) ast.ASTRef {
	t.Helper()
	status, err := f.e.QueryPredicate(declaration)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty widened product must report approximation", status)
	}
	return f.own(f.e.Answer(declaration))
}

func TestDatalogBoundIntervalWideningStopsOnEmptyProductDelta(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		for _, allOrNothing := range []bool{false, true} {
			name := kinds[0] + "/component-deltas"
			if allOrNothing {
				name = kinds[0] + "/all-or-nothing-deltas"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, kinds)
				h := f.declare("widen-swap-H", 2)
				f.fact(h, 0, 10)
				x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
				rule := f.hold(f.m.MkImplies(f.app(h, x, y), f.app(h, y, x)))
				fragmentMust(t, f.e.AddRule(rule, "swap-product"))
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
				if allOrNothing {
					fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.all_or_nothing_deltas", "true"))
				}
				var initial ast.ASTRef
				for run := 0; run < 2; run++ {
					answer := boundIntervalWideningAnswer(t, f, h)
					if run == 0 {
						initial = answer
					}
					// The first recursive step widens to [0,+inf] x [-inf,10].
					// Its unchanged bound component makes the product delta empty,
					// so another full-relation step must not widen both axes again.
					f.checkFormula(answer, []int{100, 5}, true)
					f.checkFormula(answer, []int{0, -100}, true)
					f.checkFormula(answer, []int{-1, 5}, false)
					f.checkFormula(answer, []int{0, 11}, false)
				}
				if kinds[0] == "bound_relation" && !allOrNothing {
					f.fact(h, 5, 20)
					updated := boundIntervalWideningAnswer(t, f, h)
					f.checkFormula(updated, []int{100, 20}, true)
					f.checkFormula(updated, []int{0, 21}, false)
					f.checkFormula(initial, []int{0, 11}, false)
					f.m.Limit.Cancel()
					_, err := f.e.QueryPredicate(h)
					f.m.Limit.ResetCancel()
					if err == nil {
						t.Fatal("widening ignored query cancellation")
					}
					if _, err := f.e.Answer(h); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("canceled widening query published an answer", err)
					}
					f.checkFormula(boundIntervalWideningAnswer(t, f, h), []int{100, 20}, true)
					guard := f.hold(f.m.MkArithmeticLE(x, f.numeral(-1)))
					body := f.hold(f.m.MkAnd(f.app(h, x, y), guard))
					replacement := f.hold(f.m.MkImplies(body, f.app(h, y, x)))
					fragmentMust(t, f.e.UpdateRule(replacement, "swap-product"))
					strengthened := boundIntervalWideningAnswer(t, f, h)
					f.checkFormula(strengthened, []int{0, 10}, true)
					f.checkFormula(strengthened, []int{5, 20}, true)
					// Projection of the empty guarded delta uses the source
					// reset payload in the C++ vector relation.
					f.checkFormula(strengthened, []int{100, 5}, true)
					f.checkFormula(updated, []int{100, 5}, true)
				}
			})
		}
	}
}

func TestDatalogBoundIntervalWideningInitializesWithOrdinaryUnions(t *testing.T) {
	f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
	// Declare consumers first and interleave their rules with dependencies.
	h, nonrecursive := f.declare("widen-init-H", 1), f.declare("widen-init-K", 1)
	p, q := f.declare("widen-init-P", 1), f.declare("widen-init-Q", 1)
	f.fact(p, 0)
	f.fact(q, 10)
	x := f.hold(f.m.MkVar(0, f.integer))
	for _, edge := range []struct {
		tail ast.ASTHandle
		head ast.ASTHandle
		name string
	}{{h, h, "recursive-identity"}, {q, h, "upper-init"}, {p, nonrecursive, "lower-acyclic"}, {p, h, "lower-init"}, {q, nonrecursive, "upper-acyclic"}} {
		fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(f.app(edge.tail, x), f.app(edge.head, x))), edge.name))
	}
	fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
	for _, declaration := range []ast.ASTHandle{h, nonrecursive} {
		answer := boundIntervalWideningAnswer(t, f, declaration)
		f.checkFormula(answer, []int{0}, true)
		f.checkFormula(answer, []int{5}, true)
		f.checkFormula(answer, []int{10}, true)
		f.checkFormula(answer, []int{-1}, false)
		f.checkFormula(answer, []int{11}, false)
	}
}

func TestDatalogBoundIntervalWideningParameterPrecedence(t *testing.T) {
	for _, test := range []struct {
		name, module, local string
		widen               bool
	}{
		{name: "default"},
		{name: "module-disabled", module: "false"},
		{name: "module-enabled", module: "true", widen: true},
		{name: "local-disabled", module: "true", local: "false"},
		{name: "local-enabled", module: "false", local: "true", widen: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"interval_relation", "bound_relation"})
			h := f.declare("widen-parameter-H", 1)
			f.fact(h, 0)
			x := f.hold(f.m.MkVar(0, f.integer))
			fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(f.app(h, x), f.app(h, f.numeral(10)))), "increase-upper"))
			if test.module != "" {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", test.module))
			}
			if test.local != "" {
				local := f.e.params.EmptyValue()
				key := "datalog.compile_with_widening"
				fragmentMust(t, f.e.params.SetBoolName(&local, &key, test.local == "true"))
				fragmentMust(t, f.e.SetQueryParameters(&local))
				fragmentMust(t, f.e.params.Del(&local))
			}
			answer := boundIntervalWideningAnswer(t, f, h)
			f.checkFormula(answer, []int{0}, true)
			f.checkFormula(answer, []int{10}, true)
			f.checkFormula(answer, []int{100}, test.widen)
			f.checkFormula(answer, []int{-1}, false)
		})
	}
}

func TestDatalogBoundIntervalWideningMultiHeadIdentityComponents(t *testing.T) {
	for _, shape := range []string{"three-heads", "mutual-two-tails"} {
		for _, widen := range []bool{false, true} {
			name := shape + "/ordinary"
			if widen {
				name = shape + "/widen"
			}
			t.Run(name, func(t *testing.T) {
				f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
				h, q := f.declare("widen-boundary-H", 1), f.declare("widen-boundary-Q", 1)
				f.fact(h, 0)
				x := f.hold(f.m.MkVar(0, f.integer))
				body := f.app(h, x)
				if shape == "mutual-two-tails" {
					body = f.hold(f.m.MkAnd(body, f.app(q, x)))
					f.fact(q, 0)
				}
				fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(body, f.app(q, x))), "recursive-first"))
				tail := f.app(q, x)
				if shape == "three-heads" {
					r := f.declare("widen-boundary-R", 1)
					fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(tail, f.app(r, x))), "recursive-middle"))
					tail = f.app(r, x)
				}
				fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(tail, f.app(h, x))), "recursive-last"))
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
}

func TestDatalogBoundIntervalWideningRecursiveTailsWithConstantHeads(t *testing.T) {
	for _, widen := range []bool{false, true} {
		name := "ordinary"
		if widen {
			name = "widen"
		}
		t.Run(name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			h := f.declare("widen-constant-head-H", 1)
			f.fact(h, 0)
			x, y := f.hold(f.m.MkVar(0, f.integer)), f.hold(f.m.MkVar(1, f.integer))
			body := f.hold(f.m.MkAnd(f.app(h, x), f.app(h, y)))
			fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(body, f.app(h, f.numeral(10)))), "recursive-first"))
			fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(f.app(h, x), f.app(h, f.numeral(20)))), "recursive-second"))
			if widen {
				fragmentMust(t, f.e.globals.SourceSetName("fixedpoint.datalog.compile_with_widening", "true"))
			}
			answer := boundIntervalWideningAnswer(t, f, h)
			f.checkFormula(answer, []int{0}, true)
			f.checkFormula(answer, []int{20}, true)
			f.checkFormula(answer, []int{21}, widen)
			f.checkFormula(answer, []int{-1}, false)
		})
	}
}
