package pdr

// Source: pdr_util.cpp arith_normalizer's scoped intermediate expression owners.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticNormalizationDiscardsTemporariesUnderRefusedBudget(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	two, err := m.ArithmeticRealNumeralText("2")
	closureMust(t, err)
	// The normalized bound 53 is outside the owning small-numeral cache.
	bound, err := m.ArithmeticRealNumeralText("106")
	closureMust(t, err)
	sort, err := m.GetSort(two)
	closureMust(t, err)
	x, err := m.MkFreshConst("discard-normalization", sort)
	closureMust(t, err)
	product, err := m.MkArithmeticMul(two, x)
	closureMust(t, err)
	relation, err := m.MkArithmeticLT(product, bound)
	closureMust(t, err)
	formula, err := m.MkNot(relation)
	closureMust(t, err)
	input, err := m.NewRef(formula)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&input)) }()
	for attempt := 0; attempt < 4; attempt++ {
		before := m.Stats()
		n := arithmeticNormalization{manager: m}
		output, err := n.tree(formula, make(map[ast.ASTHandle]ast.ASTHandle))
		closureMust(t, err)
		if output == formula {
			t.Fatal("fixture did not create normalized temporaries")
		}
		if m.Stats().LiveNodes <= before.LiveNodes {
			t.Fatal("fixture did not own new AST nodes")
		}
		// This is the disposal path used after a later rewriting failure: no
		// completed root was published, and cleanup cannot reserve arena storage.
		m.Budget().FailAfter(0)
		err = n.close()
		m.Budget().FailAfter(-1)
		closureMust(t, err)
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatalf("attempt %d: cleanup nodes/refs = %d/%d, want %d/%d", attempt, after.LiveNodes, after.LiveReferences, before.LiveNodes, before.LiveReferences)
		}
		original, err := m.RefGet(input)
		closureMust(t, err)
		if original != formula {
			t.Fatal("discarding a rewrite changed its input")
		}
	}
	closureMust(t, NormalizeArithmeticRef(m, input))
	output, err := m.RefGet(input)
	closureMust(t, err)
	if output == formula {
		t.Fatal("discarded normalization could not be retried")
	}
}
