package duality

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDualityUnhoistLoopRewritesInvariantAndIncomingRelations(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		name := "constant-definition"
		if fresh {
			name = "new-parameter"
		}
		t.Run(name, func(t *testing.T) {
			ctx, m, _ := dualityNativeSolverFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			wrap := func(h ast.ASTHandle, err error) *Expr {
				t.Helper()
				must(err)
				e, err := NewExpr(ctx, h)
				must(err)
				closeExpr(t, e)
				return e
			}
			raw := func(e *Expr) ast.ASTHandle { t.Helper(); h, err := e.Raw(); must(err); return h }
			one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
			zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
			sort, err := m.GetSort(raw(one))
			must(err)
			constant := func(text string) *Expr { n, err := m.Symbols.Intern(text); must(err); return wrap(m.MkConst(n, sort)) }
			x, a, b, z := constant("loop-x"), constant("init-a"), constant("init-b"), constant("query-z")
			boolean, err := m.MkBoolSort()
			must(err)
			decl := func(text string) *FuncDecl {
				n, err := m.Symbols.Intern(text)
				must(err)
				h, err := m.MkFuncDecl(n, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				d, err := NewFuncDecl(ctx, h)
				must(err)
				closeDecl(t, d)
				return d
			}
			r, p, q := decl("loop-R"), decl("loop-pre"), decl("query-pre")
			q2 := decl("query-pre-second")
			apply := func(d *FuncDecl, e *Expr) *Expr { v, err := d.Call(e); must(err); closeExpr(t, v); return v }
			g, err := NewRPFP(ctx)
			must(err)
			defer g.Close()
			node, err := g.CreateNode(apply(r, x))
			must(err)
			queryTerm, err := ctx.BoolConst("loop-query")
			must(err)
			closeExpr(t, queryTerm)
			query, err := g.CreateNode(queryTerm)
			must(err)
			value := one
			if fresh {
				value = wrap(m.MkArithmeticAdd(raw(b), raw(one)))
			}
			initFormula := wrap(m.MkEq(raw(a), raw(value)))
			initial, err := NewTransformer(ctx, nil, []*Expr{a}, initFormula)
			must(err)
			defer initial.Close()
			init, err := g.CreateEdge(node, initial, nil)
			must(err)
			pre := apply(p, x)
			condition := wrap(m.MkArithmeticGT(raw(x), raw(zero)))
			loopFormula := wrap(m.MkAnd(raw(pre), raw(condition)))
			loopTrans, err := NewTransformer(ctx, []*FuncDecl{p}, []*Expr{x}, loopFormula)
			must(err)
			defer loopTrans.Close()
			loop, err := g.CreateEdge(node, loopTrans, []*Node{node})
			must(err)
			incomingTrans, err := NewTransformer(ctx, []*FuncDecl{q, q2}, nil, wrap(m.MkAnd(raw(apply(q, z)), raw(apply(q2, a)))))
			must(err)
			defer incomingTrans.Close()
			incoming, err := g.CreateEdge(query, incomingTrans, []*Node{node, node})
			must(err)
			oldName, err := m.RefGet(node.Name)
			must(err)
			must(g.Unhoist())
			count := 1
			if fresh {
				count = 2
			}
			for _, relation := range []*Transformer{node.Annotation, node.Bound, node.Underapprox, loop.F, init.F} {
				if len(relation.individualParams) != count {
					t.Fatal("parameter extension mismatch")
				}
			}
			loopRaw, err := m.RefGet(loop.F.formula)
			must(err)
			guard, err := m.GetArg(loopRaw, 1)
			must(err)
			lhs, err := m.GetArg(guard, 0)
			must(err)
			expected := raw(one)
			if fresh {
				param, err := m.RefGet(loop.F.individualParams[1])
				must(err)
				term := wrap(m.MkArithmeticAdd(param, raw(one)))
				expected = raw(term)
			}
			if lhs != expected {
				t.Fatal("hoisted expression not substituted into loop condition")
			}
			relation, err := g.GetRel(loop, 0)
			must(err)
			defer relation.Close()
			first, err := m.GetArg(raw(relation), 0)
			must(err)
			if first != raw(x) {
				t.Fatal("substitution entered protected precondition")
			}
			nextName, err := m.RefGet(node.Name)
			must(err)
			if fresh {
				if nextName == oldName {
					t.Fatal("extended node name not fresh")
				}
				app, err := g.GetRel(incoming, 0)
				must(err)
				defer app.Close()
				n, err := m.GetNumArgs(raw(app))
				must(err)
				if n != 2 {
					t.Fatal("incoming relation arity not extended")
				}
				arg, err := m.GetArg(raw(app), 0)
				must(err)
				if arg != raw(z) {
					t.Fatal("incoming original argument changed")
				}
				param, err := m.GetArg(raw(app), 1)
				must(err)
				want, err := m.RefGet(loop.F.individualParams[1])
				must(err)
				if param != want {
					t.Fatal("incoming invariant parameter mismatch")
				}
				second, err := g.GetRel(incoming, 1)
				must(err)
				defer second.Close()
				n, err = m.GetNumArgs(raw(second))
				must(err)
				if n != 2 {
					t.Fatal("repeated child position extended more than once")
				}
				arg, err = m.GetArg(raw(second), 0)
				must(err)
				if arg != raw(a) {
					t.Fatal("second child original argument changed")
				}
			} else if nextName != oldName {
				t.Fatal("definition without new parameters changed node declaration")
			}
		})
	}
}
