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
				if ratText(m, &entry.degree) != want[original] || fresh == original || freshVars[fresh] {
					t.Fatalf("bad substitution: original=%v fresh=%v degree=%s", original, fresh, ratText(m, &entry.degree))
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
					if left != fresh || right != power(original, ratText(m, &entry.degree)) {
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

// AUDIT.md tactic_arith_01-F1: save_degree re-inserts an existing key into
// m_var2degree, and core_hashtable::insert expands before the lookup, so a
// second power of a known constant can grow the table and reorder it.
func TestDegreeShiftPrepareReinsertExpandsTable(t *testing.T) {
	_, m := fixDLFixture(t, false)
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	vars := make([]ast.ASTHandle, len(names))
	powers := make([]ast.ASTHandle, len(names))
	for i, name := range names {
		vars[i] = fixDLConst(t, m, false, name)
		powers[i] = fixDLApp(t, m, 18, vars[i], fixDLNumber(t, m, false, "2"))
	}
	sum := fixDLApp(t, m, 6, powers...)
	second := fixDLApp(t, m, 18, vars[0], fixDLNumber(t, m, false, "4"))
	_, order, err := degreeShiftCandidatesOrdered(m, []ast.ASTHandle{sum, second}, nil)
	equationMust(t, err)
	// The arguments of the sum are popped last-first, so the keys enter
	// g..a; the second root then re-inserts a.
	expanded, plain := ast.NewObjMap(m), ast.NewObjMap(m)
	for i := len(vars) - 1; i >= 0; i-- {
		equationMust(t, expanded.Insert(vars[i], [2]ast.ASTHandle{}))
		equationMust(t, plain.Insert(vars[i], [2]ast.ASTHandle{}))
	}
	equationMust(t, expanded.Insert(vars[0], [2]ast.ASTHandle{}))
	keys := func(o *ast.ObjMap) []ast.ASTHandle {
		var out []ast.ASTHandle
		for _, entry := range o.Entries() {
			out = append(out, entry.Key)
		}
		return out
	}
	want, without := keys(expanded), keys(plain)
	if len(order) != len(want) {
		t.Fatal(len(order), len(want))
	}
	same := true
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("candidate %d differs from the expanded obj_map order", i)
		}
		same = same && want[i] == without[i]
	}
	if same {
		t.Fatal("fixture does not distinguish the expanded table")
	}
}
