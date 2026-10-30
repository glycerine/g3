package api

import (
	"strings"
	"testing"
)

func TestAPIFixedpointSMT2IncludesOrderedQueries(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	truth := f.pin(f.m.MkTrue())
	astTestMust(t, f.fp.AddFact(f.p, nil))
	astTestMust(t, f.fp.AddRule(f.step, "api-display-step"))
	astTestMust(t, f.fp.Assert(truth))
	text, err := f.fp.SMT2(f.pAtom, f.qAtom)
	astTestMust(t, err)
	fragments := []string{
		"(declare-rel api-object-fp-p ())",
		"(assert true)",
		":named api-display-step",
		"(query api-object-fp-p)",
		"(query api-object-fp-q)",
	}
	position := -1
	for _, fragment := range fragments {
		next := strings.Index(text, fragment)
		if next < 0 {
			t.Fatalf("fixedpoint display omitted %q:\n%s", fragment, text)
		}
		if strings.HasPrefix(fragment, "(query") && next <= position {
			t.Fatalf("fixedpoint query order in:\n%s", text)
		}
		if strings.HasPrefix(fragment, "(query") {
			position = next
		}
	}
}
