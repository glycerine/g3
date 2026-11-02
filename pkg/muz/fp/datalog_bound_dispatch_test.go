package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogBoundDefaultAndExplicitLifecycle(t *testing.T) {
	for _, favourite := range []string{"pentagon", "bound_relation"} {
		t.Run(favourite, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", favourite))
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(name string) ast.ASTHandle {
				sym, err := m.Symbols.Intern(name)
				must(err)
				d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			p := declare("bound-public-P")
			one, err := m.ArithmeticIntegerNumeralText("1")
			must(err)
			ref, err := m.NewRef(one)
			must(err)
			defer func() { must(m.DelRef(&ref)) }()
			must(e.Push())
			must(e.AddTableFact(p, []uint32{1, 1}))
			if err = e.Pop(); err == nil {
				t.Fatal("bound fact did not instantiate engine")
			}
			q := declare("bound-public-late-Q")
			x, err := m.MkVar(0, integer)
			must(err)
			y, err := m.MkVar(1, integer)
			must(err)
			px, err := m.MkApp(p, []ast.ASTHandle{x, y})
			must(err)
			qy, err := m.MkApp(q, []ast.ASTHandle{y, x})
			must(err)
			less, err := m.MkArithmeticLT(x, y)
			must(err)
			body, err := m.MkAnd(px, less)
			must(err)
			rule, err := m.MkImplies(body, qy)
			must(err)
			must(e.AddRule(rule, "bound-order"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
			must(e.SetQueryParameters(nil))
			status, err := e.QueryPredicate(q)
			must(err)
			if status != lbool.Undef || e.boundDatalog == nil || e.intervalDatalog != nil {
				t.Fatal("source bound selection", status)
			}
			relation, err := e.boundDatalog.relation(q)
			must(err)
			strict, _, err := relation.Bounds(1)
			must(err)
			if len(strict) != 1 || strict[0] != 0 {
				t.Fatal("public bound head permutation lost strict order", strict)
			}
			answer, err := e.Answer(q)
			must(err)
			defer func() { must(m.DelRef(&answer)) }()
			// Check the public exported formula independently at satisfying,
			// reversed and equal valuations, including its variable order.
			formula, err := m.RefGet(answer)
			must(err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			must(err)
			twoRef, err := m.NewRef(two)
			must(err)
			defer func() { must(m.DelRef(&twoRef)) }()
			substitution, err := astrewriter.NewVarSubstitution(m, false)
			must(err)
			defer func() { must(substitution.Close()) }()
			for _, valuation := range []struct {
				values []ast.ASTHandle
				want   bool
			}{{[]ast.ASTHandle{two, one}, true}, {[]ast.ASTHandle{one, two}, false}, {[]ast.ASTHandle{one, one}, false}} {
				ground, err := m.NewRef(ast.ASTHandle{})
				must(err)
				must(substitution.Substitute(formula, valuation.values, ground))
				h, err := m.RefGet(ground)
				must(err)
				result, err := m.NewRef(ast.ASTHandle{})
				must(err)
				must(e.guard.RewriteWithBindings(h, nil, result))
				reduced, err := m.RefGet(result)
				must(err)
				if m.IsTrue(reduced) != valuation.want || (!m.IsTrue(reduced) && !m.IsFalse(reduced)) {
					t.Fatal("exported bound formula disagrees with source ordering", valuation.want)
				}
				must(m.DelRef(&result))
				must(m.DelRef(&ground))
			}

			query, err := m.MkApp(q, []ast.ASTHandle{x, y})
			must(err)
			queryRef, err := m.NewRef(query)
			must(err)
			defer func() { must(m.DelRef(&queryRef)) }()
			status, err = e.QueryExpression(query)
			must(err)
			if status != lbool.Undef {
				t.Fatal("expression did not inherit bound relation", status)
			}
			if e.queryClone.boundDatalog == nil || e.queryClone.boundDatalog == e.boundDatalog {
				t.Fatal("query seed owner not independent")
			}
			held, err := e.ExpressionAnswer()
			must(err)
			defer func() { must(m.DelRef(&held)) }()
			must(e.clearQueryResources())
			if _, err = m.RefGet(held); err != nil {
				t.Fatal("answer lost with clone", err)
			}
			m.Limit.Cancel()
			status, err = e.QueryPredicate(q)
			m.Limit.ResetCancel()
			if err == nil || status != lbool.Undef {
				t.Fatal("bound query ignored cancellation", status, err)
			}
			if _, err = e.Answer(q); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("cancellation exposed stale bound answer", err)
			}
			status, err = e.QueryPredicate(q)
			must(err)
			if status != lbool.Undef {
				t.Fatal("bound query failed after cancellation reset", status)
			}
		})
	}
}

func TestDatalogBoundFinitePrecedenceAndMixedGuard(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default-finite", true: "explicit-bound"}[explicit], func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			if explicit {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "bound_relation"))
			}
			if explicit { // Exercise insertion after an empty selected runtime exists.
				status, err := e.QueryPredicate(f.declarations["p"])
				must(err)
				if status != lbool.False {
					t.Fatal(status)
				}
			}
			must(e.AddTableFact(f.declarations["p"], nil))
			status, err := e.QueryExpression(f.applications["p"])
			must(err)
			want := lbool.True
			if explicit {
				want = lbool.Undef
			}
			if status != want {
				t.Fatal("nullary source representation precedence", status, want)
			}
		})
	}
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("mixed-original-arithmetic")
	must(err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	_, err = e.QueryPredicate(d)
	if !errors.Is(err, ErrBoundDatalogFragment) {
		t.Fatal("mixed original finite/arithmetic silently converted", err)
	}
	if e.boundDatalog != nil {
		t.Fatal("unsupported hybrid installed whole-program bound store")
	}
}
