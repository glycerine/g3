package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDegreeShiftRewrite(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_proofs", true: "with_proofs"}[proofs], func(t *testing.T) {
			_, m := fixDLFixture(t, proofs)
			x := fixDLConst(t, m, false, "rewrite-x")
			y := fixDLConst(t, m, false, "rewrite-y")
			power := func(base ast.ASTHandle, exponent string) ast.ASTHandle {
				return fixDLApp(t, m, 18, base, fixDLNumber(t, m, false, exponent))
			}
			root, err := m.MkEq(power(x, "6"), power(x, "10"))
			equationMust(t, err)
			fixDLPin(t, m, root)
			second, err := m.MkEq(power(y, "3"), fixDLNumber(t, m, false, "1"))
			equationMust(t, err)
			fixDLPin(t, m, second)
			plan, err := degreeShiftPrepare(m, []ast.ASTHandle{root, second}, proofs)
			equationMust(t, err)
			defer func() { equationMust(t, plan.Close()) }()
			fresh := make(map[ast.ASTHandle]ast.ASTHandle)
			for _, entry := range plan.entries {
				old, e := m.RefGet(entry.original)
				equationMust(t, e)
				fresh[old], e = m.RefGet(entry.fresh)
				equationMust(t, e)
			}
			walk, err := newDegreeShiftTraversal(plan, proofs)
			equationMust(t, err)
			defer func() { equationMust(t, walk.Close()) }()
			out, err := m.NewRef(ast.ASTHandle{})
			equationMust(t, err)
			defer func() { equationMust(t, m.DelRef(&out)) }()
			pr, err := m.NewRef(ast.ASTHandle{})
			equationMust(t, err)
			defer func() { equationMust(t, m.DelRef(&pr)) }()
			for _, tc := range []struct {
				input, left, right ast.ASTHandle
			}{
				{root, power(fresh[x], "3"), power(fresh[x], "5")},
				{second, fresh[y], fixDLNumber(t, m, false, "1")},
			} {
				if proofs {
					err = walk.RewriteWithProof(tc.input, out, pr)
				} else {
					err = walk.Rewrite(tc.input, out)
				}
				equationMust(t, err)
				result, e := m.RefGet(out)
				equationMust(t, e)
				left, e := m.GetArg(result, 0)
				equationMust(t, e)
				right, e := m.GetArg(result, 1)
				equationMust(t, e)
				if left != tc.left || right != tc.right {
					t.Fatal("incorrect reduced powers")
				}
				if proofs {
					p, e := m.RefGet(pr)
					equationMust(t, e)
					fact, e := m.GetProofFact(p)
					equationMust(t, e)
					from, e := m.GetArg(fact, 0)
					equationMust(t, e)
					to, e := m.GetArg(fact, 1)
					equationMust(t, e)
					if from != tc.input || to != result {
						t.Fatal("rewrite proof has the wrong fact")
					}
				}
			}
		})
	}
}
