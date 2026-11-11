package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The raw ast_manager::mk_and(1, &a) builds a unary and, which the source
// th_rewriter reduces to its argument (array_select_reducer::mk_result).
func TestTheoryRewriterReducesUnaryAnd(t *testing.T) {
	m, context, globals, local := boolRewriterConfigFixture(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	atom := goalTestAtom(t, m, "unary-and-atom")
	astTestPin(t, m, atom)
	unary, err := m.MkAndArgs([]ast.ASTHandle{atom})
	astTestMust(t, err)
	astTestPin(t, m, unary)
	theory, err := NewTheoryRewriter(m, context, globals, local)
	astTestMust(t, err)
	defer func() { astTestMust(t, theory.Close()) }()
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, theory.Rewrite(unary, out))
	actual, err := m.RefGet(out)
	astTestMust(t, err)
	if actual != atom {
		t.Fatal("unary and was not reduced to its argument")
	}
}
