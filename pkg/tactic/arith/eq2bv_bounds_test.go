package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestEq2BVBoundConstraintsPreserveDirectionAndDependencies(t *testing.T) {
	bounds, m := boundManagerFixture(t)
	x := fixDLConst(t, m, true, "bounds-x")
	number := func(s string) ast.ASTHandle { return fixDLNumber(t, m, true, s) }
	lower := fixDLApp(t, m, 3, x, number("2"))
	upper := fixDLApp(t, m, 2, x, number("7"))
	eq := collectorEq(t, m, x, number("3"))
	root, e := m.MkNot(eq)
	equationMust(t, e)
	fixDLPin(t, m, root)
	sort, e := m.MkBoolSort()
	equationMust(t, e)
	name, e := m.Symbols.Intern("bound-label")
	equationMust(t, e)
	label, e := m.MkConst(name, sort)
	equationMust(t, e)
	fixDLPin(t, m, label)
	dep, e := m.DependencyLeaf(label)
	equationMust(t, e)
	equationMust(t, m.DependencyInc(dep))
	defer func() { equationMust(t, m.DependencyDec(dep)) }()
	equationMust(t, bounds.Collect(lower, dep))
	equationMust(t, bounds.Collect(upper, ast.ASTDependency{}))
	equationMust(t, bounds.Collect(root, ast.ASTDependency{}))
	plan, e := eq2bvPrepare(m, bounds, []ast.ASTHandle{lower, upper, root})
	equationMust(t, e)
	defer func() { equationMust(t, plan.Close()) }()
	if len(plan.entries) != 1 {
		t.Fatalf("replacements %d", len(plan.entries))
	}
	fresh, e := m.RefGet(plan.entries[0].fresh)
	equationMust(t, e)
	constraints, e := eq2bvMakeBounds(plan, bounds)
	equationMust(t, e)
	if len(constraints.entries) != 2 {
		t.Fatalf("constraints %d", len(constraints.entries))
	}
	for i, want := range []string{"2", "7"} {
		entry := constraints.entries[i]
		if i == 0 && entry.dependency != dep {
			t.Fatal("lower dependency lost")
		}
		if i == 1 && entry.dependency != (ast.ASTDependency{}) {
			t.Fatal("upper dependency changed")
		}
		h, e := m.RefGet(entry.formula)
		equationMust(t, e)
		var left, right ast.ASTHandle
		ok, e := m.BitVectorULEOperands(h, &left, &right)
		equationMust(t, e)
		if !ok {
			t.Fatal("not an unsigned comparison")
		}
		value := right
		if i == 0 {
			value = left
			if right != fresh {
				t.Fatal("lower direction")
			}
		} else if left != fresh {
			t.Fatal("upper direction")
		}
		text, yes, e := m.BitVectorNumeralText(value)
		equationMust(t, e)
		if !yes || text != want {
			t.Fatalf("bound %q want %q", text, want)
		}
	}
	equationMust(t, constraints.Close())
}
