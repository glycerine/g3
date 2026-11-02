package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRExpressionPrequeryOwnerFailureAndRetry(t *testing.T) {
	failures, preparationFailures, solveFailures, success := 0, 0, 0, false
	for stop := int64(0); stop <= 32768 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			var budget *arena.Budget
			t.Cleanup(func() {
				if budget != nil && budget.Used() != 0 {
					t.Fatal("expression owner fixture leaked bytes", budget.Used())
				}
			})
			f := newPDRLifecycleFixture(t)
			e, m := f.engine, f.manager
			budget = m.Budget()
			defer budget.FailAfter(-1)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			parameters, err := e.params.NewValue()
			must(err)
			defer func() { must(e.params.Del(&parameters)) }()
			for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
				must(e.params.SetBoolName(&parameters, &name, false))
			}
			must(e.SetQueryParameters(&parameters))
			arithmetic, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(arithmetic, 1, nil)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("failed-expression-integer")
			must(err)
			declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(declaration))
			zero, err := m.ArithmeticIntegerNumeralText("0")
			must(err)
			fact, err := m.MkApp(declaration, []ast.ASTHandle{zero})
			must(err)
			must(e.AddRule(fact, "failed-expression-zero"))
			variable, err := m.MkVar(0, integer)
			must(err)
			property, err := m.MkEq(variable, zero)
			must(err)
			propertyRef, err := m.NewRef(property)
			must(err)
			defer func() { must(m.DelRef(&propertyRef)) }()
			query, err := m.MkApp(declaration, []ast.ASTHandle{variable})
			must(err)
			queryRef, err := m.NewRef(query)
			must(err)
			defer func() { must(m.DelRef(&queryRef)) }()
			must(e.AddCover(2, declaration, property))
			owner := e.pdr
			budget.FailAfter(stop)
			status, err := e.QueryExpression(query)
			budget.FailAfter(-1)
			if err != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					must(cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected query failure", err)
				}
				if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published a result", err)
				}
				if _, err := e.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
					t.Fatal("failed query published an answer", err)
				}
				if e.queryClone != nil {
					solveFailures++
					if e.queryClone.pdr != owner {
						t.Fatal("failed solve lost its original context owner")
					}
					initialized, err := owner.Initialized()
					must(err)
					if !initialized {
						t.Fatal("failed-solve fixture did not install rules")
					}
				} else {
					preparationFailures++
					if e.pdr != owner {
						t.Fatal("failed preparation replaced its original owner")
					}
					initialized, err := owner.Initialized()
					must(err)
					if initialized {
						t.Fatal("failed preparation published a program")
					}
				}
				cover, err := e.CoverDelta(2, declaration)
				must(err)
				defer func() { must(m.DelRef(&cover)) }()
				handle, err := m.RefGet(cover)
				must(err)
				if handle != property {
					t.Fatal("failed query lost the pre-query cover", handle, property)
				}
				if e.queryClone != nil {
					must(e.AddRule(fact, "duplicate-after-failed-query"))
					if _, err := e.CoverDelta(2, declaration); !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal("rule mutation exposed stale failed-query frames", err)
					}
				}
				status, err = e.QueryExpression(query)
				must(err)
			} else {
				success = true
			}
			if status != lbool.True {
				t.Fatal("query/retry did not reach zero", status)
			}
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || preparationFailures == 0 || solveFailures == 0 || !success {
		t.Fatal("missing preparation/solve failure and success frontiers", failures, preparationFailures, solveFailures, success)
	}
}
