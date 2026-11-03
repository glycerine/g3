package fp

// Source dl_context.cpp:861-887 flushes pending rules and constructs the
// backend before dl_interface::query reaches rule_manager::mk_query. The
// PDR/Spacer interface constructors allocate their actual contexts; a query
// preparation failure therefore cannot restore the pre-engine Pop phase.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func newExpressionScopePhaseFixture(t *testing.T, backend, preparation string) (*fpRuleNameFixture, ast.ASTHandle) {
	t.Helper()
	f := newQueryVariablesOwnershipFixture(t)
	fragmentMust(t, f.e.SelectEngine(backend))
	boolean := f.own(f.m.MkBoolSort())
	variable := f.own(f.m.MkVar(129, boolean))
	declaration, query := f.predicate("expression-phase-P", variable)
	truth := f.own(f.m.MkTrue())
	fact := f.own(f.m.MkApp(declaration, []ast.ASTHandle{truth}))
	fragmentMust(t, f.e.AddRule(fact, "expression-phase-fact"))
	if preparation == "hoist" {
		const count = 130
		sorts := make([]ast.ASTHandle, count)
		names := make([]symbol.Symbol, count)
		for i := range sorts {
			sorts[i] = boolean
			names[i] = symbol.Numerical(uint32(i))
		}
		query = f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: sorts, Names: names, Body: query}))
		_, gate := f.predicate("expression-phase-gate")
		fragmentMust(t, f.e.AddRule(gate, "expression-phase-gate-fact"))
		query = f.own(f.m.MkAnd(query, gate))
	}
	return f, query
}

func expressionScopePhaseCleanup(t *testing.T, cause error) {
	t.Helper()
	var cleanup memory.ConstructionFailure
	if errors.As(cause, &cleanup) {
		fragmentMust(t, cleanup.Cleanup())
	}
}

func expressionScopePhasePopRejected(t *testing.T, e *PropositionalEngine) {
	t.Helper()
	if err := e.Pop(); err == nil || !strings.Contains(err.Error(), "pop operation is only supported by duality engine") {
		t.Fatal("instantiated backend allowed Pop", err)
	}
	if len(e.scopes) != 1 {
		t.Fatal("rejected Pop consumed its saved scope", len(e.scopes))
	}
}

func TestExpressionQueryPreparationFailureKeepsInstantiatedScope(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		for _, preparation := range []string{"hoist", "compact"} {
			t.Run(backend+"/"+preparation, func(t *testing.T) {
				constructorFailed, preparationFailed := false, false
				for allowance := int64(0); allowance < 256 && !preparationFailed; allowance++ {
					if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
						f, query := newExpressionScopePhaseFixture(t, backend, preparation)
						e := f.e
						fragmentMust(t, e.flushPendingRules())
						fragmentMust(t, e.Push())
						persistent, revision := e.set, e.revision
						f.m.Budget().FailAfter(allowance)
						status, cause := e.QueryExpression(query)
						f.m.Budget().FailAfter(-1)
						if cause == nil || status != lbool.Undef || !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("campaign missed the expected allocation failure", allowance, status, cause)
						}
						expressionScopePhaseCleanup(t, cause)
						if e.queryClone != nil || e.queried || e.set != persistent || e.revision != revision {
							t.Fatal("preparation failure crossed clone/query or persistent rule phases", allowance, cause)
						}
						if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
							t.Fatal("failed query published a result", err)
						}
						if !strings.Contains(cause.Error(), "muz/fp: "+preparation+" query") {
							if e.pdr != nil || e.spacer != nil || e.engineInstantiated {
								t.Fatal("constructor failure published an engine", allowance, cause)
							}
							constructorFailed = true
							fragmentMust(t, e.Pop())
							if len(e.scopes) != 0 {
								t.Fatal("failed construction prevented the pre-engine Pop")
							}
							if allowance != 0 {
								return
							}
							// A failed constructor permits Pop and a later fresh query.
							status, err := e.QueryExpression(query)
							fragmentMust(t, err)
							if status != lbool.True {
								t.Fatal("query did not recover after constructor failure", status)
							}
							return
						}
						preparationFailed = true
						if !e.engineInstantiated || (e.pdr == nil && e.spacer == nil) {
							t.Fatal("mk_query failed before the source engine was instantiated", allowance, cause)
						}
						originalPDR, originalSpacer := e.pdr, e.spacer
						expressionScopePhasePopRejected(t, e)
						status, err := e.QueryExpression(query)
						fragmentMust(t, err)
						if status != lbool.True || e.queryClone == nil || e.queryClone.pdr != originalPDR || e.queryClone.spacer != originalSpacer {
							t.Fatal("retry did not use the constructed source context", status)
						}
						expressionScopePhasePopRejected(t, e)
					}) {
						break
					}
				}
				if !constructorFailed || !preparationFailed {
					t.Fatal("missing distinct constructor/preparation failure frontiers", constructorFailed, preparationFailed)
				}
			})
		}
	}
}

func TestExpressionQueryFailedFlushAllowsPopBeforeEngineConstruction(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f, query := newExpressionScopePhaseFixture(t, backend, "compact")
			e := f.e
			fragmentMust(t, e.Push())
			if len(e.rawRules) != 1 || !e.rawRules[0].pending {
				t.Fatal("fixture has no pending source rule")
			}
			persistent, revision, raw := e.set, e.revision, e.rawRules[0]
			before := contextDisplayLiveCounts(f.m.Stats())
			f.m.Budget().FailAfter(0)
			status, cause := e.QueryExpression(query)
			f.m.Budget().FailAfter(-1)
			if status != lbool.Undef || !errors.Is(cause, arena.ErrInjectedFailure) {
				t.Fatal("pending flush did not fail", status, cause)
			}
			expressionScopePhaseCleanup(t, cause)
			if e.set != persistent || e.revision != revision || len(e.rawRules) != 1 || e.rawRules[0] != raw || e.engineInstantiated || e.queried || e.pdr != nil || e.spacer != nil || e.queryClone != nil {
				t.Fatal("failed flush changed the source program or instantiated a backend", cause)
			}
			if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
				t.Fatal("failed flush changed AST ownership", before, after)
			}
			originalFormula, err := f.m.RefGet(raw.formula)
			fragmentMust(t, err)
			fragmentMust(t, e.Pop())
			if len(e.scopes) != 0 || len(e.rawRules) != 1 {
				t.Fatal("Pop lost the saved pending input")
			}
			restored := e.rawRules[0]
			restoredFormula, err := f.m.RefGet(restored.formula)
			fragmentMust(t, err)
			if restoredFormula != originalFormula || restored.name != raw.name || restored.bound != raw.bound || !restored.pending || restored.first != raw.first || restored.end != raw.end {
				t.Fatal("Pop changed the saved pending formula/name/bound/phase")
			}
			status, err = e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.True || !e.engineInstantiated || e.rawRules[0].pending {
				t.Fatal("successful retry did not flush then instantiate", status)
			}
		})
	}
}
