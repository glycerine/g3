package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source quant_hoist.cpp extracts declarations in their original order and
// replaces each fresh constant by the next free de Bruijn index. Most of these
// 512 declarations are unused, so their fresh nodes cannot survive accidentally
// through the returned formula or a pre-existing caller variable.
func TestQueryHoistAllocationFailureAndRetryOwnsFreshNodes(t *testing.T) {
	failures, completed := 0, false
	for allowance := int64(0); allowance < 1024 && !completed; allowance++ {
		if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newQueryVariablesOwnershipFixture(t)
			boolean := f.own(f.m.MkBoolSort())
			bits := f.own(f.m.BitVectorSort(2))
			const width = 512
			sorts := make([]ast.ASTHandle, width)
			names := make([]symbol.Symbol, width)
			for i := range sorts {
				sorts[i] = boolean
				if i%2 != 0 {
					sorts[i] = bits
				}
				name, err := f.m.Symbols.Intern(fmt.Sprintf("query-hoist-%d", i))
				fragmentMust(t, err)
				names[i] = name
			}
			originalIndices := []uint32{width - 1, width/2 - 1, 0}
			args := make([]ast.ASTHandle, len(originalIndices))
			for i, index := range originalIndices {
				args[i] = f.own(f.m.MkVar(index, sorts[width-1-int(index)]))
			}
			predicate, body := f.predicate("owned-hoist-P", args...)
			free := f.own(f.m.MkVar(2, boolean))
			_, gate := f.predicate("owned-hoist-Q", free)
			exists := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: sorts, Names: names, Body: body}))
			query := f.own(f.m.MkAndArgs([]ast.ASTHandle{exists, gate}))
			checkInput := func() {
				t.Helper()
				first, err := f.m.GetArg(query, 0)
				fragmentMust(t, err)
				second, err := f.m.GetArg(query, 1)
				fragmentMust(t, err)
				originalBody, err := f.m.GetQuantifierBody(exists)
				fragmentMust(t, err)
				count, err := f.m.GetNumDeclarations(exists)
				fragmentMust(t, err)
				if first != exists || second != gate || originalBody != body || count != width {
					t.Fatal("query hoisting changed the caller's query", first, second, originalBody, count)
				}
				for i, argument := range args {
					actual, err := f.m.GetArg(body, uint32(i))
					fragmentMust(t, err)
					if actual != argument {
						t.Fatal("query hoisting changed an original bound variable", i, actual, argument)
					}
				}
				for i, name := range names {
					actualName, err := f.m.GetDeclarationName(exists, uint32(i))
					fragmentMust(t, err)
					actualSort, err := f.m.GetDeclarationSort(exists, uint32(i))
					fragmentMust(t, err)
					if actualName != name || actualSort != sorts[i] {
						t.Fatal("query hoisting changed a declaration", i, actualName, actualSort)
					}
				}
			}
			checkResult := func(result ast.ASTRef) {
				t.Helper()
				formula, err := f.m.RefGet(result)
				fragmentMust(t, err)
				hasQuantifiers, err := f.m.HasQuantifiers(formula)
				fragmentMust(t, err)
				bodyResult, err := f.m.GetArg(formula, 0)
				fragmentMust(t, err)
				gateResult, err := f.m.GetArg(formula, 1)
				fragmentMust(t, err)
				declaration, err := f.m.GetDecl(bodyResult)
				fragmentMust(t, err)
				if hasQuantifiers || declaration != predicate || gateResult != gate {
					t.Fatal("hoisted query lost its body or free-variable gate", hasQuantifiers, declaration, gateResult)
				}
				for i, originalIndex := range originalIndices {
					argument, err := f.m.GetArg(bodyResult, uint32(i))
					fragmentMust(t, err)
					index, err := f.m.GetVarIndex(argument)
					fragmentMust(t, err)
					sort, err := f.m.GetSort(argument)
					fragmentMust(t, err)
					// #2 is the original highest free variable; extracted declaration
					// zero therefore starts at #3, and bound #0 is declaration511.
					wantIndex := uint32(3+width-1) - originalIndex
					wantSort := sorts[width-1-int(originalIndex)]
					if index != wantIndex || sort != wantSort {
						t.Fatal("hoisting changed source variable order or sort", i, index, wantIndex, sort, wantSort)
					}
				}
			}
			before := contextDisplayLiveCounts(f.m.Stats())
			f.m.Budget().FailAfter(allowance)
			result, cause := f.e.hoistQueryExistentials(query)
			f.m.Budget().FailAfter(-1)
			if cause != nil {
				failures++
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("query hoisting lost allocation cause", cause)
				}
				if result != (ast.ASTRef{}) {
					t.Fatal("failed query hoisting published a partial result", result, cause)
				}
				var cleanup memory.ConstructionFailure
				if errors.As(cause, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
			} else {
				completed = true
				checkResult(result)
				fragmentMust(t, f.m.DelRef(&result))
			}
			if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
				t.Fatal("query hoisting lost AST ownership", allowance, cause, before, after)
			}
			checkInput()
			result, err := f.e.hoistQueryExistentials(query)
			fragmentMust(t, err)
			checkResult(result)
			fragmentMust(t, f.m.DelRef(&result))
			if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
				t.Fatal("successful query hoisting retry lost AST ownership", before, after)
			}
			checkInput()
		}) {
			break
		}
	}
	if failures == 0 || !completed {
		t.Fatal("query hoisting allocation campaign missed failure or completion", failures, completed)
	}
}

func TestQueryExpressionNestedExistentialPreservesWitnessColumns(t *testing.T) {
	f := newQueryVariablesOwnershipFixture(t)
	fragmentMust(t, f.e.SelectEngine("datalog"))
	boolean := f.own(f.m.MkBoolSort())
	bits2 := f.own(f.m.BitVectorSort(2))
	bits3 := f.own(f.m.BitVectorSort(3))
	b := f.own(f.m.MkVar(1, boolean))
	x := f.own(f.m.MkVar(0, bits2))
	innerFree := f.own(f.m.MkVar(2, bits3))
	outerFree := f.own(f.m.MkVar(0, bits3))
	r, body := f.predicate("nested-hoist-R", b, x, innerFree)
	s, guard := f.predicate("nested-hoist-S", outerFree)
	exists := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Sorts: []ast.ASTHandle{boolean, bits2},
		Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: body,
	}))
	query := f.own(f.m.MkAnd(exists, guard))
	truth, falsity := f.own(f.m.MkTrue()), f.own(f.m.MkFalse())
	var small, wide []ast.ASTHandle
	for i := uint64(0); i < 8; i++ {
		wide = append(wide, f.own(f.m.BitVectorNumeralUint64(i, 3)))
		if i < 4 {
			small = append(small, f.own(f.m.BitVectorNumeralUint64(i, 2)))
		}
	}
	fragmentMust(t, f.e.AddFact(r, []ast.ASTHandle{truth, small[2], wide[5]}))
	fragmentMust(t, f.e.AddFact(r, []ast.ASTHandle{falsity, small[1], wide[6]}))
	fragmentMust(t, f.e.AddFact(s, []ast.ASTHandle{wide[5]}))
	substitution, err := rewriter.NewVarSubstitution(f.m, false)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, substitution.Close()) })
	ground, err := f.m.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	reduced, err := f.m.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	f.roots = append(f.roots, ground, reduced)
	var held ast.ASTRef
	var heldFormula ast.ASTHandle
	for repeat := 0; repeat < 2; repeat++ {
		status, err := f.e.QueryExpression(query)
		fragmentMust(t, err)
		if status != lbool.True {
			t.Fatal("nested existential query lost its witness", repeat, status)
		}
		arity, err := f.m.GetArity(f.e.queryClonePredicate)
		fragmentMust(t, err)
		if arity != 3 {
			t.Fatal("query head lost a bound or free variable", arity)
		}
		// Fresh constants follow declaration order b,x; their indices start
		// after free #0. The source query head reverses [BV3,Bool,BV2].
		for i, want := range []ast.ASTHandle{bits2, boolean, bits3} {
			got, err := f.m.GetDomain(f.e.queryClonePredicate, uint32(i))
			fragmentMust(t, err)
			if got != want {
				t.Fatal("query head changed witness column order or sort", i, got, want)
			}
		}
		answer, err := f.e.ExpressionAnswer()
		fragmentMust(t, err)
		f.roots = append(f.roots, answer)
		formula, err := f.m.RefGet(answer)
		fragmentMust(t, err)
		for i, xv := range small {
			for j, bv := range []ast.ASTHandle{falsity, truth} {
				for k, fv := range wide {
					fragmentMust(t, substitution.Substitute(formula, []ast.ASTHandle{xv, bv, fv}, ground))
					term, err := f.m.RefGet(ground)
					fragmentMust(t, err)
					fragmentMust(t, f.e.guard.RewriteWithBindings(term, nil, reduced))
					value, err := f.m.RefGet(reduced)
					fragmentMust(t, err)
					want := i == 2 && j == 1 && k == 5
					if f.m.IsTrue(value) != want || !f.m.IsTrue(value) && !f.m.IsFalse(value) {
						t.Fatal("query answer changed witness coordinates", repeat, i, j, k, want)
					}
				}
			}
		}
		if repeat == 0 {
			held, heldFormula = answer, formula
		} else if got, err := f.m.RefGet(held); err != nil || got != heldFormula {
			t.Fatal("repeated query invalidated its caller-owned previous answer", got, err)
		}
	}
	fragmentMust(t, f.e.Close())
	if got, err := f.m.RefGet(held); err != nil || got != heldFormula {
		t.Fatal("engine disposal invalidated its caller-owned answer", got, err)
	}
}
