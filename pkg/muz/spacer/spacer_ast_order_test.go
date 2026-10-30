package spacer_test

// Source: spacer_context.cpp lemma::mk_cube and pob::set_post use ast_lt_proc.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"testing"
)

func TestSpacerCubeAndBindingsUseSourceASTIDOrder(t *testing.T) {
	command, m, owner := lemmaOwnerFixture(t)
	p := spacerBool(t, m, "source-id-p")
	notP, err := m.MkNot(p)
	spacerMust(t, err)
	q := spacerBool(t, m, "source-id-q")
	conjunction, err := m.MkAndArray([]ast.ASTHandle{q, notP})
	spacerMust(t, err)
	input, err := m.NewRef(conjunction)
	spacerMust(t, err)
	negated, err := m.PushNot(conjunction)
	spacerMust(t, err)
	lemma, err := spacer.NewLemma(owner, negated, 1)
	spacerMust(t, err)
	cube, err := lemma.Cube()
	spacerMust(t, err)
	if len(cube) != 2 || cube[0] != notP || cube[1] != q {
		t.Fatal("lemma cube source AST ID order", cube, notP, q)
	}
	truth, err := m.MkTrue()
	spacerMust(t, err)
	falsehood, err := m.MkFalse()
	spacerMust(t, err)
	transformer, err := spacer.NewPredicateTransformer(owner, "source-id-bindings", falsehood, truth, spacer.PropSolverConfig{})
	spacerMust(t, err)
	obligation, err := spacer.NewProofObligation(nil, transformer, 1, 0, conjunction, []ast.ASTHandle{q, notP})
	spacerMust(t, err)
	bindings, err := obligation.Bindings()
	spacerMust(t, err)
	if len(bindings) != 2 || bindings[0] != notP || bindings[1] != q {
		t.Fatal("proof obligation binding source AST ID order", bindings, notP, q)
	}
	spacerMust(t, obligation.Close())
	spacerMust(t, obligation.Release())
	spacerMust(t, transformer.Close())
	spacerMust(t, lemma.Close())
	spacerMust(t, m.DelRef(&input))
	spacerMust(t, owner.Close())
	spacerMust(t, command.Close())
}
