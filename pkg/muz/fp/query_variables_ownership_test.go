package fp

// Source dl_rule.cpp:285-301 owns every compaction binding in expr_ref_vector,
// including the Boolean dummy used for an absent free-variable index.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func newQueryVariablesOwnershipFixture(t *testing.T) *fpRuleNameFixture {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("query variable fixture leaked bytes", budget.Used())
		}
	})
	f := newFPRuleNameFixture(t)
	budget = f.m.Budget()
	return f
}

func sparseIntegerQuery(t *testing.T, f *fpRuleNameFixture, index uint32) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	zero := f.own(f.m.ArithmeticIntegerNumeralText("0"))
	integer, err := f.m.GetSort(zero)
	fragmentMust(t, err)
	x := f.own(f.m.MkVar(index, integer))
	_, query := f.predicate("owned-sparse-query-P", x)
	// Do not construct Bool #0 here: an existing root would conceal the leak
	// of the unused dummy binding that this fixture must detect.
	return query, integer
}

func checkCompactedUnaryQuery(t *testing.T, m *ast.ASTManager, result ast.ASTRef, source, sort ast.ASTHandle) {
	t.Helper()
	query, err := m.RefGet(result)
	fragmentMust(t, err)
	wantDecl, err := m.GetDecl(source)
	fragmentMust(t, err)
	decl, err := m.GetDecl(query)
	fragmentMust(t, err)
	arg, err := m.GetArg(query, 0)
	fragmentMust(t, err)
	index, err := m.GetVarIndex(arg)
	fragmentMust(t, err)
	actualSort, err := m.GetSort(arg)
	fragmentMust(t, err)
	if decl != wantDecl || index != 0 || actualSort != sort {
		t.Fatal("compaction changed the predicate, variable order or sort", decl, index, actualSort)
	}
}

func TestCompactQueryVariablesReleasesUnusedBooleanBindings(t *testing.T) {
	f := newQueryVariablesOwnershipFixture(t)
	query, integer := sparseIntegerQuery(t, f, 2)
	before := contextDisplayLiveCounts(f.m.Stats())
	result, err := f.e.compactQueryVariables(query)
	fragmentMust(t, err)
	checkCompactedUnaryQuery(t, f.m, result, query, integer)
	fragmentMust(t, f.m.DelRef(&result))
	if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
		t.Fatal("successful compaction leaked temporary AST ownership", before, after)
	}
}

func TestCompactQueryVariablesAllocationFailureAndRetry(t *testing.T) {
	for _, index := range []uint32{2, 129} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			failures, completed := 0, false
			for allowance := int64(0); allowance < 1024 && !completed; allowance++ {
				t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					f := newQueryVariablesOwnershipFixture(t)
					query, integer := sparseIntegerQuery(t, f, index)
					before := contextDisplayLiveCounts(f.m.Stats())
					budget := f.m.Budget()
					budget.FailAfter(allowance)
					result, cause := f.e.compactQueryVariables(query)
					budget.FailAfter(-1)
					if cause != nil {
						failures++
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("compaction lost allocation failure", cause)
						}
						if result != (ast.ASTRef{}) {
							t.Fatal("failed compaction published an output")
						}
						var cleanup memory.ConstructionFailure
						if errors.As(cause, &cleanup) {
							fragmentMust(t, cleanup.Cleanup())
						}
					} else {
						completed = true
						checkCompactedUnaryQuery(t, f.m, result, query, integer)
						fragmentMust(t, f.m.DelRef(&result))
					}
					if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
						t.Fatal("compaction attempt leaked temporary AST ownership", allowance, before, after)
					}
					result, err := f.e.compactQueryVariables(query)
					fragmentMust(t, err)
					checkCompactedUnaryQuery(t, f.m, result, query, integer)
					fragmentMust(t, f.m.DelRef(&result))
					if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
						t.Fatal("compaction retry leaked temporary AST ownership", before, after)
					}
					arg, err := f.m.GetArg(query, 0)
					fragmentMust(t, err)
					originalIndex, err := f.m.GetVarIndex(arg)
					fragmentMust(t, err)
					if originalIndex != index {
						t.Fatal("compaction mutated the caller's query", originalIndex, index)
					}
				})
				if t.Failed() {
					break
				}
			}
			if failures == 0 || !completed {
				t.Fatal("missing compaction failure/completion frontier", failures, completed)
			}
		})
	}
}

func TestQueryExpressionSparseVariableKeepsUnaryAnswer(t *testing.T) {
	f := newQueryVariablesOwnershipFixture(t)
	fragmentMust(t, f.e.SelectEngine("datalog"))
	bits := f.own(f.m.BitVectorSort(2))
	x := f.own(f.m.MkVar(2, bits))
	predicate, query := f.predicate("public-sparse-query-P", x)
	two := f.own(f.m.BitVectorNumeralUint64(2, 2))
	fragmentMust(t, f.e.AddFact(predicate, []ast.ASTHandle{two}))
	status, err := f.e.QueryExpression(query)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("sparse query did not reach the stored tuple", status)
	}
	arity, err := f.m.GetArity(f.e.queryClonePredicate)
	fragmentMust(t, err)
	if arity != 1 {
		t.Fatal("unused query-variable indices became head columns", arity)
	}
	answer, err := f.e.ExpressionAnswer()
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, f.m.DelRef(&answer)) })
	compact := f.own(f.m.MkVar(0, bits))
	want := f.own(f.m.MkEq(compact, two))
	actual, err := f.m.RefGet(answer)
	fragmentMust(t, err)
	if actual != want {
		t.Fatal("sparse query answer did not describe exactly value two", actual, want)
	}
	three := f.own(f.m.BitVectorNumeralUint64(3, 2))
	absent := f.own(f.m.MkApp(predicate, []ast.ASTHandle{three}))
	status, err = f.e.QueryExpression(absent)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("sparse query introduced a spurious source fact", status)
	}
	fragmentMust(t, f.e.Close())
	if actual, err := f.m.RefGet(answer); err != nil || actual != want {
		t.Fatal("owned sparse query answer changed after clone and engine disposal", actual, err)
	}
}
