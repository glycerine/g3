package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp
// lemma_bool_inductive_generalizer's owning cube and extra_lits vectors.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestSpacerInductiveGeneralizerOwnsExpandedArithmeticBounds(t *testing.T) {
	for _, positive := range []bool{true, false} {
		name := "positive-initial-state"
		if !positive {
			name = "negative-initial-state"
		}
		t.Run(name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			zero := own(m.ArithmeticIntegerNumeralText("0"))
			integer, err := m.GetSort(zero)
			depthMust(t, err)
			x := own(m.MkFreshConst("inductive-bound", integer))
			var constraint, expected ast.ASTHandle
			if positive {
				constraint = own(m.MkArithmeticGT(x, zero))
			} else {
				constraint = own(m.MkArithmeticLT(x, zero))
			}
			head, err := f.rule.Head()
			depthMust(t, err)
			rule, err := f.manager.NewRule(head, []ast.ASTHandle{constraint}, nil, "initial-arithmetic-state")
			depthMust(t, err)
			defer func() { depthMust(t, rule.Close()) }()
			set, err := base.NewRuleSet(f.manager)
			depthMust(t, err)
			defer func() { depthMust(t, set.Close()) }()
			depthMust(t, set.Add(rule))
			depthMust(t, c.UpdateRules(set))
			p := c.transformers[f.query]
			equality := own(m.MkEq(x, zero))
			obligation, err := p.NewProofObligation(nil, 1, 0, equality, nil)
			depthMust(t, err)
			original, err := NewLemmaFromProofObligation(obligation, []ast.ASTHandle{equality}, 1)
			depthMust(t, err)
			defer func() { depthMust(t, original.Close()) }()
			generalized, err := p.generalizeInductiveLemma(obligation, original)
			depthMust(t, err)
			if generalized == original {
				t.Fatal("equality was not generalized to a bound")
			}
			defer func() { depthMust(t, generalized.Close()) }()
			cube, err := generalized.Cube()
			depthMust(t, err)
			// Construct the expected bound after generalization so this test
			// does not keep speculative bounds alive on behalf of the generalizer.
			if positive {
				expected = own(m.MkArithmeticLE(x, zero))
			} else {
				expected = own(m.MkArithmeticGE(x, zero))
			}
			if len(cube) != 1 || cube[0] != expected {
				t.Fatal("wrong generalized bound", cube, expected)
			}
			before := m.Stats().LiveReferences
			core, _, inductive, err := p.checkInductiveCube(cube, 1)
			depthMust(t, err)
			if !inductive || len(core) != 1 {
				t.Fatal("expected owning singleton inductive core", inductive, len(core))
			}
			// The check has already closed its temporary result. Its core
			// reference must remain live across another solver invocation.
			for attempt := 0; attempt < 3; attempt++ {
				next, _, valid, err := p.checkInductiveCube(cube, 1)
				depthMust(t, err)
				if !valid {
					t.Fatal("repeated core check changed inductiveness")
				}
				depthMust(t, releaseNormalizeRefs(m, next))
				h, err := m.RefGet(core[0])
				depthMust(t, err)
				if h != expected {
					t.Fatal("owning core changed across repeated check")
				}
			}
			depthMust(t, releaseNormalizeRefs(m, core))
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("core cleanup references = %d, want %d", got, before)
			}
			_, err = m.GetDeclKind(cube[0])
			depthMust(t, err)
		})
	}
}
