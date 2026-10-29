package rewriter

import (
	"errors"
	"testing"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestQuantifierHoisterWithoutRewriteOwnsNoParameterizedRewriter(t *testing.T) {
	m, _, _, _ := boolRewriterConfigFixture(t)
	h, err := NewQuantifierHoisterWithoutRewrite(m)
	astTestMust(t, err)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	v, err := m.MkVar(0, sort)
	astTestMust(t, err)
	q := hoistQuantifier(t, m, true, "raw", v)
	result, err := h.PullQuantifierIndexed(true, q, true, false)
	astTestMust(t, err)
	if result.Index != 1 || len(result.Sorts) != 1 || len(result.Names) != 1 {
		t.Fatal("raw indexed pull", result.Index, len(result.Sorts), len(result.Names))
	}
	astTestMust(t, result.Close())
	if _, err = h.PullQuantifier(true, q, true, true); !errors.Is(err, uparams.ErrParamsOwner) {
		t.Fatal("parameterless hoister accepted Boolean rewriting", err)
	}
	astTestMust(t, h.Close())
	astTestMust(t, h.Close())
}
