package fp

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type boundIntervalPublicFixture struct {
	t       *testing.T
	m       *ast.ASTManager
	e       *PropositionalEngine
	integer ast.ASTHandle
	boolean ast.ASTHandle
	kinds   []string
	roots   []ast.ASTRef
}

func newBoundIntervalPublicFixture(t *testing.T, kinds []string) *boundIntervalPublicFixture {
	t.Helper()
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	f := &boundIntervalPublicFixture{t: t, m: m, e: e, kinds: kinds}
	t.Cleanup(func() {
		for i := len(f.roots) - 1; i >= 0; i-- {
			fragmentMust(t, m.DelRef(&f.roots[i]))
		}
		fragmentMust(t, e.Close())
	})
	fragmentMust(t, e.SelectEngine("datalog"))
	for _, option := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.datalog.unbound_compressor"} {
		fragmentMust(t, globals.SourceSetName(option, "false"))
	}
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	f.integer = f.hold(m.MkFamilySort(family, 1, nil))
	f.boolean = f.hold(m.MkBoolSort())
	return f
}

func (f *boundIntervalPublicFixture) own(ref ast.ASTRef, err error) ast.ASTRef {
	f.t.Helper()
	fragmentMust(f.t, err)
	f.roots = append(f.roots, ref)
	return ref
}

func (f *boundIntervalPublicFixture) hold(value ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	fragmentMust(f.t, err)
	f.own(f.m.NewRef(value))
	return value
}

func (f *boundIntervalPublicFixture) declare(name string, arity int) ast.ASTHandle {
	f.t.Helper()
	symbol, err := f.m.Symbols.Intern("public-bound-interval-" + name)
	fragmentMust(f.t, err)
	domain := make([]ast.ASTHandle, arity)
	for i := range domain {
		domain[i] = f.integer
	}
	d := f.hold(f.m.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(f.t, f.e.RegisterPredicate(d))
	names := append([]string(nil), f.kinds...)
	fragmentMust(f.t, f.e.SetPredicateRepresentation(d, names))
	names[0] = "tr_sparse"
	return d
}

func (f *boundIntervalPublicFixture) numeral(value int) ast.ASTHandle {
	f.t.Helper()
	return f.hold(f.m.ArithmeticIntegerNumeralText(strconv.Itoa(value)))
}

func (f *boundIntervalPublicFixture) app(d ast.ASTHandle, values ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	return f.hold(f.m.MkApp(d, values))
}

func (f *boundIntervalPublicFixture) fact(d ast.ASTHandle, values ...int) {
	f.t.Helper()
	args := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		args[i] = f.numeral(value)
	}
	fragmentMust(f.t, f.e.AddFact(d, args))
}

func (f *boundIntervalPublicFixture) checkFormula(answer ast.ASTRef, values []int, want bool) {
	f.t.Helper()
	formula, err := f.m.RefGet(answer)
	fragmentMust(f.t, err)
	args := make([]ast.ASTHandle, len(values))
	for i, value := range values {
		args[i] = f.numeral(value)
	}
	substitution, err := rewriter.NewVarSubstitution(f.m, false)
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, substitution.Close()) }()
	ground := f.own(f.m.NewRef(ast.ASTHandle{}))
	fragmentMust(f.t, substitution.Substitute(formula, args, ground))
	root, err := f.m.RefGet(ground)
	fragmentMust(f.t, err)
	rewritten := f.own(f.m.NewRef(ast.ASTHandle{}))
	fragmentMust(f.t, f.e.guard.RewriteWithBindings(root, nil, rewritten))
	value, err := f.m.RefGet(rewritten)
	fragmentMust(f.t, err)
	if f.m.IsTrue(value) != want || !f.m.IsTrue(value) && !f.m.IsFalse(value) {
		f.t.Fatal("product answer disagrees with valuation", values, want)
	}
}

func TestDatalogBoundIntervalPublicQueriesAndOwnership(t *testing.T) {
	for _, kinds := range [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, kinds)
			m, e := f.m, f.e
			p, ground, copied := f.declare("P", 2), f.declare("Ground", 2), f.declare("Copy", 2)
			joined, empty := f.declare("Join", 1), f.declare("Empty", 1)
			unit := f.declare("Unit", 0)
			f.fact(unit)
			f.fact(p, 2, 10)
			fragmentMust(t, e.AddTableFact(p, []uint32{5, 20}))
			fragmentMust(t, e.AddRule(f.app(ground, f.numeral(10), f.numeral(100)), "ground-product"))
			x, y, z := f.hold(m.MkVar(0, f.integer)), f.hold(m.MkVar(1, f.integer)), f.hold(m.MkVar(2, f.integer))
			pxy := f.app(p, x, y)
			body := f.hold(m.MkAnd(pxy, f.hold(m.MkArithmeticLE(x, f.numeral(3)))))
			fragmentMust(t, e.AddRule(f.hold(m.MkImplies(body, f.app(copied, y, x))), "one-tail-product"))
			body = f.hold(m.MkAnd(pxy, f.app(ground, y, z)))
			fragmentMust(t, e.AddRule(f.hold(m.MkImplies(body, f.app(joined, z))), "two-tail-product"))

			for _, test := range []struct {
				outputs []ast.ASTHandle
				want    lbool.LBool
			}{{[]ast.ASTHandle{unit}, lbool.Undef}, {[]ast.ASTHandle{empty}, lbool.False}, {[]ast.ASTHandle{empty, joined}, lbool.Undef}, {[]ast.ASTHandle{ground, copied}, lbool.Undef}} {
				status, err := e.QueryPredicates(test.outputs)
				fragmentMust(t, err)
				if status != test.want {
					t.Fatal("public product relation query", status, test.want)
				}
			}
			if e.boundIntervalDatalog == nil || e.boundDatalog != nil || e.intervalDatalog != nil {
				t.Fatal("explicit pair did not activate the bound/interval product")
			}
			oldAnswer := f.own(e.Answer(copied))
			f.checkFormula(oldAnswer, []int{10, 2}, true)
			f.checkFormula(oldAnswer, []int{20, 3}, true)
			f.checkFormula(oldAnswer, []int{20, 4}, false)
			f.checkFormula(oldAnswer, []int{21, 3}, false)
			joinedAnswer := f.own(e.Answer(joined))
			f.checkFormula(joinedAnswer, []int{100}, true)
			f.checkFormula(joinedAnswer, []int{101}, false)
			f.checkFormula(f.own(e.Answer(unit)), nil, true)

			status, err := e.QueryExpression(f.app(copied, f.numeral(20), f.numeral(2)))
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("nonempty ground expression lost product imprecision", status)
			}
			f.checkFormula(f.own(e.ExpressionAnswer()), nil, true)
			query := f.app(copied, f.numeral(20), x)
			status, err = e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.Undef || e.queryClone == nil || e.queryClone == e || e.queryClone.boundIntervalDatalog == nil || e.queryClone.boundIntervalDatalog == e.boundIntervalDatalog ||
				e.queryClone.boundIntervalDatalog.entries[p].seed == e.boundIntervalDatalog.entries[p].seed {
				t.Fatal("public product expression query did not use its own engine", status)
			}
			if names := e.queryClone.predicates[p].requestedRepresentations; len(names) != 2 || names[0] != kinds[0] || names[1] != kinds[1] {
				t.Fatal("expression clone lost copied representation order", names)
			}
			expressionAnswer := f.own(e.ExpressionAnswer())
			f.checkFormula(expressionAnswer, []int{2}, true)
			f.checkFormula(expressionAnswer, []int{1}, false)

			f.fact(p, 1, 30)
			if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("fact update exposed a stale expression answer", err)
			}
			f.checkFormula(expressionAnswer, []int{1}, false)
			status, err = e.QueryPredicate(copied)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("fact update lost source product seeds", status)
			}
			updated := f.own(e.Answer(copied))
			f.checkFormula(updated, []int{30, 1}, true)
			f.checkFormula(oldAnswer, []int{30, 1}, false)
			status, err = e.QueryExpression(f.app(copied, f.numeral(31), f.numeral(1)))
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("empty product expression query was not false", status)
			}
			f.checkFormula(f.own(e.ExpressionAnswer()), nil, false)
			fragmentMust(t, e.Close())
			for _, answer := range []ast.ASTRef{oldAnswer, expressionAnswer, updated} {
				if _, err := m.RefGet(answer); err != nil {
					t.Fatal("owning answer lost with engine or expression clone", err)
				}
			}
		})
	}
}

func TestDatalogBoundIntervalPublicRejectsUnsupportedRules(t *testing.T) {
	for _, name := range []string{"three-positive-tails", "negative-tail"} {
		t.Run(name, func(t *testing.T) {
			f := newBoundIntervalPublicFixture(t, []string{"bound_relation", "interval_relation"})
			p, q, r, output := f.declare("P", 1), f.declare("Q", 1), f.declare("R", 1), f.declare("Output", 1)
			f.fact(p, 7)
			f.fact(q, 7)
			f.fact(r, 7)
			x := f.hold(f.m.MkVar(0, f.integer))
			body := f.hold(f.m.MkAnd3(f.app(p, x), f.app(q, x), f.app(r, x)))
			if name == "negative-tail" {
				body = f.hold(f.m.MkAnd(f.app(p, x), f.hold(f.m.MkNot(f.app(q, x)))))
			}
			fragmentMust(t, f.e.AddRule(f.hold(f.m.MkImplies(body, f.app(output, x))), name))
			if _, err := f.e.QueryPredicate(output); !errors.Is(err, ErrBoundIntervalDatalogFragment) {
				t.Fatal("unsupported product program did not report its fragment boundary", err)
			}
			if _, err := f.e.Answer(output); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("failed product query exposed an answer", err)
			}
		})
	}
}
