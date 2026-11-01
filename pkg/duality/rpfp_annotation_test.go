package duality

import (
	"fmt"
	"testing"
)

func TestRPFPAnnotationDecodeAndUpperBound(t *testing.T) {
	for _, horn := range []bool{false, true} {
		t.Run(fmt.Sprint(horn), func(t *testing.T) {
			ctx, _, m := dualityContextFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			x, err := ctx.BoolConst("annot-x")
			must(err)
			closeExpr(t, x)
			sort, err := x.Sort()
			must(err)
			defer sort.Close()
			symbol, err := ctx.StringSymbol("annot-R")
			must(err)
			decl, err := ctx.Function(symbol, []*Sort{sort}, sort)
			must(err)
			closeDecl(t, decl)
			term, err := decl.Call(x)
			must(err)
			closeExpr(t, term)
			before := m.Stats()
			g, err := NewRPFP(ctx)
			must(err)
			defer func() { must(g.Close()) }()
			g.HornClauses = horn
			node, err := g.CreateNode(term)
			must(err)
			original, err := g.GetAnnotation(node)
			must(err)
			h, err := original.Raw()
			must(err)
			if !m.IsTrue(h) {
				t.Fatal("full annotation shortcut")
			}
			must(original.Close())
			b, vars, err := g.RedVars(node)
			must(err)
			learned, err := b.And(vars[0])
			must(err)
			must(g.SetAnnotation(node, learned))
			truth, err := ctx.BoolVal(true)
			must(err)
			decoded, err := truth.And(x)
			must(err)
			got, err := node.Annotation.Formula()
			must(err)
			gh, err := got.Raw()
			must(err)
			want, err := decoded.Raw()
			must(err)
			if gh != want {
				t.Fatal("annotation decode")
			}
			negative, err := b.Not()
			must(err)
			indexed, err := truth.And(vars[0])
			must(err)
			expected, err := negative.Or(indexed)
			must(err)
			result, err := g.GetAnnotation(node)
			must(err)
			gh, err = result.Raw()
			must(err)
			want, err = expected.Raw()
			must(err)
			if gh != want {
				t.Fatal("annotation encoding")
			}
			// Bound formulas can use different parameter names; encoding substitutes
			// by position into the node annotation vocabulary.
			bound, err := NewTransformer(ctx, nil, []*Expr{x}, x)
			must(err)
			must(node.Bound.Close())
			node.Bound = bound
			notVar, err := vars[0].Not()
			must(err)
			upper, err := b.And(notVar)
			must(err)
			actual, err := g.GetUpperBound(node)
			must(err)
			gh, err = actual.Raw()
			must(err)
			want, err = upper.Raw()
			must(err)
			if gh != want {
				t.Fatal("upper bound encoding")
			}
			// Setting replaces the old annotation instead of strengthening it.
			must(g.SetAnnotation(node, truth))
			full, err := node.Annotation.IsFull()
			must(err)
			if !full {
				t.Fatal("annotation was strengthened instead of replaced")
			}
			for _, e := range []*Expr{actual, upper, notVar, result, expected, indexed, negative, got, decoded, truth, learned, vars[0], b} {
				must(e.Close())
			}
			must(g.Close())
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("annotation references leaked", before, after)
			}
		})
	}
}
