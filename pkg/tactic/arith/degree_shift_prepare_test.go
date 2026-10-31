package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDegreeShiftPrepare(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(map[bool]string{false: "without_proofs", true: "with_proofs"}[proofs], func(t *testing.T) {
			_, m := fixDLFixture(t, proofs)
			x := fixDLConst(t, m, false, "x")
			y := fixDLConst(t, m, false, "y")
			power := func(base ast.ASTHandle, n string) ast.ASTHandle {
				return fixDLApp(t, m, 18, base, fixDLNumber(t, m, false, n))
			}
			plan, err := degreeShiftPrepare(m, []ast.ASTHandle{power(x, "6"), power(x, "10"), power(y, "3")}, proofs)
			equationMust(t, err)
			defer func() { equationMust(t, plan.Close()) }()
			if len(plan.entries) != 2 {
				t.Fatalf("got %d substitutions, want 2", len(plan.entries))
			}
			want := map[ast.ASTHandle]string{x: "2", y: "3"}
			freshVars := make(map[ast.ASTHandle]bool)
			for _, entry := range plan.entries {
				original, err := m.RefGet(entry.original)
				equationMust(t, err)
				fresh, err := m.RefGet(entry.fresh)
				equationMust(t, err)
				if entry.degree.String() != want[original] || fresh == original || freshVars[fresh] {
					t.Fatalf("bad substitution: original=%v fresh=%v degree=%s", original, fresh, entry.degree)
				}
				freshVars[fresh] = true
				originalSort, err := m.GetSort(original)
				equationMust(t, err)
				freshSort, err := m.GetSort(fresh)
				equationMust(t, err)
				if originalSort != freshSort {
					t.Fatal("fresh variable has the wrong sort")
				}
				if proofs {
					proof, err := m.RefGet(entry.proof)
					equationMust(t, err)
					fact, err := m.GetProofFact(proof)
					equationMust(t, err)
					left, err := m.GetArg(fact, 0)
					equationMust(t, err)
					right, err := m.GetArg(fact, 1)
					equationMust(t, err)
					if left != fresh || right != power(original, entry.degree.String()) {
						t.Fatal("definition proof has the wrong fact")
					}
				} else if entry.proof != (ast.ASTRef{}) {
					t.Fatal("unexpected definition proof")
				}
				delete(want, original)
			}
			if len(want) != 0 {
				t.Fatal("missing substitutions")
			}
		})
	}
}
