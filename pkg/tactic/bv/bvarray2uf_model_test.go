package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestBVArray2UFPublishesPopulatedModelConverter(t *testing.T) {
	f := newArrayFixture(t)
	selected := f.app(t, ast.ASTArrayOpSelect, f.a, f.i)
	equal, e := f.m.MkEq(selected, f.v)
	arrayMust(t, e)
	g := f.goal(t, false, true, false, equal)
	var out tactic.TacticResult
	t.Cleanup(func() { arrayMust(t, out.Close()) })
	arrayMust(t, f.tactic.Apply(g, &out))
	if out.ModelConverter == nil {
		t.Fatal("models-enabled rewrite omitted extension/filter converter")
	}
	if f.tactic.rewriter.extension != nil || f.tactic.rewriter.filter != nil {
		t.Fatal("published converters remained borrowed by the rewriter")
	}
}
