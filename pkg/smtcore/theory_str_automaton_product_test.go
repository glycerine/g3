package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringCachedAutomatonProductUsesNestedSolver(t *testing.T) {
	m, _, theory := stringExpressionSolverFixture(t)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	character := pin(m.MkConst(astTestSymbol(t, m, "cached-product-byte"), sort))
	symbolic := pin(m.MkSequenceToRegex(pin(m.MkSequenceUnit(character))))
	complement := pin(m.MkRegexComplement(symbolic))
	for _, tc := range []struct {
		left  ast.ASTHandle
		empty bool
	}{{symbolic, true}, {stringRegexLiteral(t, m, "b"), false}} {
		astTestMust(t, theory.Setup()) // theory_str::init at setup
		left, err := rewriter.NewStringRegexAutomaton(m, tc.left, theory.regexExpressionSolver)
		astTestMust(t, err)
		right, err := rewriter.NewStringRegexAutomaton(m, complement, theory.regexExpressionSolver)
		astTestMust(t, err)
		for _, owner := range []*rewriter.StringRegexAutomaton{left, right} {
			graph, err := owner.Graph()
			astTestMust(t, err)
			graph.Compress()
		}
		before := theory.regexExpressionSolver.kernel.Statistics().NumChecks
		product, err := rewriter.NewStringRegexAutomatonProduct(left, right, theory.regexExpressionSolver)
		astTestMust(t, err)
		// sym_expr mk_and of c and (not c) folds to false in bool_rewriter, so
		// is_sat decides it without the solver; the literal case needs it.
		if checked := theory.regexExpressionSolver.kernel.Statistics().NumChecks > before; checked == tc.empty {
			t.Fatal("cached graph product solver use", checked, tc.empty)
		}
		astTestMust(t, left.Close())
		astTestMust(t, right.Close())
		graph, err := product.Graph()
		astTestMust(t, err)
		graph.Compress()
		if graph.IsEmpty() != tc.empty {
			t.Fatal("symbolic cached-automaton product changed language emptiness", graph.IsEmpty(), tc.empty)
		}
		if len(theory.regexExpressionSolver.kernel.scopes) != 0 || len(theory.regexExpressionSolver.kernel.assertions) != 0 {
			t.Fatal("cached product query leaked into the nested solver")
		}
		astTestMust(t, product.Close())
	}
}
