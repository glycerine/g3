package fp

// Source: src/muz/base/dl_engine_base.h:52-70 owns every relation-query
// argument and the completed expression until the backend call returns.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestRelationQueryColdConstructionFailuresAndRetry(t *testing.T) {
	for _, width := range []int{0, 512} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			failed, completed := false, false
			successfulAllowance := int64(-1)
			for allowance := int64(0); allowance < 512 && !completed; allowance++ {
				t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					var budget *arena.Budget
					t.Cleanup(func() {
						if budget != nil && budget.Used() != 0 {
							t.Fatal("relation query fixture leaked bytes", budget.Used())
						}
					})
					m, g := newPDRLifecycleServices(t)
					budget = m.Budget()
					e, err := NewPropositionalEngineWithServices(m, g.Context, g)
					fragmentMust(t, err)
					t.Cleanup(func() { fragmentMust(t, e.Close()) })
					boolean, err := m.MkBoolSort()
					fragmentMust(t, err)
					domain := make([]ast.ASTHandle, width)
					for i := range domain {
						domain[i] = boolean
					}
					name, err := m.Symbols.Intern("cold-relation-wrapper")
					fragmentMust(t, err)
					declaration, err := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
					fragmentMust(t, err)
					fragmentMust(t, e.RegisterPredicate(declaration))
					before := m.Stats()
					budget.FailAfter(allowance)
					query, failure := e.makeRelationQueryExpression(declaration)
					budget.FailAfter(-1)
					if failure != nil {
						failed = true
						if query != (ast.ASTRef{}) || !errors.Is(failure, arena.ErrInjectedFailure) {
							t.Fatal("failed query publication", query, failure)
						}
						var cleanup memory.ConstructionFailure
						if errors.As(failure, &cleanup) {
							fragmentMust(t, cleanup.Cleanup())
						}
					} else {
						completed = true
						successfulAllowance = allowance
						fragmentMust(t, m.DelRef(&query))
					}
					checkBalance := func() {
						t.Helper()
						after := m.Stats()
						if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
							t.Fatal("query construction leaked AST owners", allowance, before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences)
						}
					}
					checkBalance()
					query, err = e.makeRelationQueryExpression(declaration)
					fragmentMust(t, err)
					h, err := m.RefGet(query)
					fragmentMust(t, err)
					actual, err := m.GetDecl(h)
					fragmentMust(t, err)
					if actual != declaration {
						t.Fatal("retry changed the queried relation")
					}
					for i := uint32(0); i < uint32(width); i++ {
						arg, err := m.GetArg(h, i)
						fragmentMust(t, err)
						index, err := m.GetVarIndex(arg)
						fragmentMust(t, err)
						if index != i {
							t.Fatal("relation arguments lost source variable order", i, index)
						}
					}
					fragmentMust(t, m.DelRef(&query))
					checkBalance()
				})
				if t.Failed() {
					break
				}
			}
			// A nullary application can fit the existing AST arena pages without
			// another physical reservation; the wide case must interrupt allocation.
			if !completed || (width != 0 && !failed) {
				t.Fatal("missing cold failure/completion frontier", failed, completed)
			}
			t.Log("first successful allowance", successfulAllowance)
		})
	}
}

func TestRelationQueryOwnedExpressionAcrossLiveBackends(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc", "clp", "duality"} {
		for _, mixed := range []bool{false, true} {
			name := backend + "/nullary"
			if mixed {
				name = backend + "/mixed"
			}
			t.Run(name, func(t *testing.T) {
				m, g := newPDRLifecycleServices(t)
				e, err := NewPropositionalEngineWithServices(m, g.Context, g)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, e.Close()) }()
				fragmentMust(t, e.SelectEngine(backend))
				boolean, err := m.MkBoolSort()
				fragmentMust(t, err)
				var domain, values []ast.ASTHandle
				if mixed {
					zero, err := m.ArithmeticIntegerNumeralText("0")
					fragmentMust(t, err)
					integer, err := m.GetSort(zero)
					fragmentMust(t, err)
					truth, err := m.MkTrue()
					fragmentMust(t, err)
					domain, values = []ast.ASTHandle{boolean, integer}, []ast.ASTHandle{truth, zero}
				}
				symbol, err := m.Symbols.Intern("owned-relation-wrapper")
				fragmentMust(t, err)
				declaration, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, err)
				fragmentMust(t, e.RegisterPredicate(declaration))
				fact, err := m.MkApp(declaration, values)
				fragmentMust(t, err)
				fragmentMust(t, e.addRuleEager(fact, "wrapper-fact"))
				for repeat := 0; repeat < 2; repeat++ {
					status, err := e.QueryPredicate(declaration)
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("relation query lost its fact", status)
					}
					if backend != "duality" && mixed {
						for i, want := range []ast.ASTHandle{domain[1], domain[0]} {
							got, err := m.GetDomain(e.queryClonePredicate, uint32(i))
							fragmentMust(t, err)
							if got != want {
								t.Fatal("mixed query head lost source reversed domain", i)
							}
						}
					}
				}
			})
		}
	}
}
