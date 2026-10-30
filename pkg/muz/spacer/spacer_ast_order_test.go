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

func TestSpacerFramePropagationUsesLevelThenSourceASTIDOrder(t *testing.T) {
	command, m, owner := lemmaOwnerFixture(t)
	defer func() { spacerMust(t, command.Close()) }()
	defer func() { spacerMust(t, owner.Close()) }()
	p := spacerBool(t, m, "frame-source-id-p")
	notP, err := m.MkNot(p)
	spacerMust(t, err)
	q := spacerBool(t, m, "frame-source-id-q")
	r := spacerBool(t, m, "frame-source-id-r")
	s := spacerBool(t, m, "frame-source-id-s")
	fixture := &frameTransformerFixture{levels: map[ast.ASTHandle]uint32{notP: 3, q: 3}}
	frames, err := spacer.NewFrames(m, fixture)
	spacerMust(t, err)
	defer func() { spacerMust(t, frames.Close()) }()
	fixture.frames = frames
	for _, item := range []struct {
		body  ast.ASTHandle
		level uint32
	}{
		{q, 1}, {s, 2}, {notP, 1}, {r, 0},
	} {
		lemma, err := spacer.NewLemma(owner, item.body, item.level)
		spacerMust(t, err)
		_, err = frames.AddLemma(lemma)
		spacerMust(t, err)
	}
	all, err := frames.PropagateToNextLevel(1)
	spacerMust(t, err)
	if !all {
		t.Fatal("expected both level-one lemmas to propagate")
	}
	if len(fixture.checked) != 2 || fixture.checked[0] != notP || fixture.checked[1] != q {
		t.Fatal("invariant queries must use level then source AST ID order", fixture.checked)
	}
	// Before checking q, the source has already moved the promoted notP
	// past every lower-level lemma, rather than sorting after all queries.
	expectedOrder := []ast.ASTHandle{r, q, s, notP}
	if len(fixture.queryOrder) != 2 || len(fixture.queryOrder[1]) != len(expectedOrder) {
		t.Fatal("missing intermediate frame order", fixture.queryOrder)
	}
	for i, expected := range expectedOrder {
		if fixture.queryOrder[1][i] != expected {
			t.Fatal("promotion did not percolate before the next invariant query", fixture.queryOrder[1])
		}
	}
	promoted, err := frames.FrameLemmas(3)
	spacerMust(t, err)
	if len(promoted) != 2 || promoted[0] != notP || promoted[1] != q {
		t.Fatal("promoted frame lost source AST ID order", promoted)
	}
	for level, expected := range map[uint32]ast.ASTHandle{0: r, 2: s} {
		lemmas, err := frames.FrameLemmas(level)
		spacerMust(t, err)
		if len(lemmas) != 1 || lemmas[0] != expected {
			t.Fatal("propagation changed another level", level, lemmas)
		}
	}
}
