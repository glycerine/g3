package api

import (
	"strings"
	"testing"
)

func TestASTVectorStringUsesSourceEnvelopeAndKinds(t *testing.T) {
	f, a, _ := astVectorFixture(t)
	boolean := f.pin(f.m.MkBoolSort())
	declaration, err := f.m.GetDecl(a)
	astTestMust(t, err)
	v, err := f.c.NewASTVectorValues(a, boolean, declaration)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, v.Close()) })
	text, err := v.String()
	astTestMust(t, err)
	if !strings.HasPrefix(text, "(ast-vector\n  ") || !strings.HasSuffix(text, ")") || strings.Count(text, "\n  ") != 3 {
		t.Fatalf("vector display: %q", text)
	}
}
