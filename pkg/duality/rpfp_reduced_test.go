package duality

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRPFPReducedVariablesAndChildRelation(t *testing.T) {
	for _, horn := range []bool{false, true} {
		t.Run(fmt.Sprint(horn), func(t *testing.T) {
			ctx, _, m := dualityContextFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			x, err := ctx.BoolConst("reduced-x")
			must(err)
			closeExpr(t, x)
			sort, err := x.Sort()
			must(err)
			defer sort.Close()
			name, err := ctx.StringSymbol("reduced-R")
			must(err)
			decl, err := ctx.Function(name, []*Sort{sort}, sort)
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
			edge, err := g.CreateEdge(node, node.Annotation, []*Node{node})
			must(err)
			b, vars, err := g.RedVars(node)
			must(err)
			if len(vars) != 1 {
				t.Fatal("indexed parameters", len(vars))
			}
			h, err := b.Raw()
			must(err)
			if horn {
				if !m.IsTrue(h) {
					t.Fatal("Horn activation is not true")
				}
			} else {
				d, err := b.Decl()
				must(err)
				s, err := d.Name()
				must(err)
				text, err := s.String()
				must(err)
				must(d.Close())
				if text != "@b_1" {
					t.Fatal("activation name", text)
				}
			}
			expected, err := SuffixVariable(x, 1)
			must(err)
			v, err := vars[0].Raw()
			must(err)
			want, err := expected.Raw()
			must(err)
			if v != want {
				t.Fatal("indexed variable")
			}
			must(vars[0].Close())
			must(b.Close())
			result, err := g.RedDualRela(edge, []*Expr{x}, 0)
			must(err)
			root, err := result.Raw()
			must(err)
			op, err := m.GetDeclKind(root)
			must(err)
			if op != ast.ASTOpAnd {
				t.Fatal("missing activation conjunction", op)
			}
			conjunction, err := m.GetArg(root, 1)
			must(err)
			equality, err := m.GetArg(conjunction, 0)
			must(err)
			op, err = m.GetDeclKind(equality)
			must(err)
			if op != ast.ASTOpIff {
				t.Fatal("Boolean argument was not iff", op)
			}
			rhs, err := m.GetArg(equality, 1)
			must(err)
			if rhs != want {
				t.Fatal("wrong indexed equality")
			}
			must(result.Close())
			must(expected.Close())
			nullary, err := g.RedDualRela(edge, nil, 0)
			must(err)
			must(nullary.Close())
			if _, err = g.RedDualRela(edge, []*Expr{x, x}, 0); err == nil {
				t.Fatal("oversized argument vector accepted")
			}
			must(g.Close())
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("reduced relation leaked", before, after)
			}
		})
	}
}
