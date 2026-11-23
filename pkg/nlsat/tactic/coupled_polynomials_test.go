package tactic

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	basetactic "github.com/glycerine/g3/pkg/tactic"
)

func TestCoupledPolynomialComponentsRetainTupleCorrespondence(t *testing.T) {
	f := newGoalCompilerFixture(t)
	m := f.manager
	g, err := basetactic.NewGoal(m, false, true, true)
	compilerMust(t, err)
	defer func() { compilerMust(t, g.Close()) }()
	two, err := m.ArithmeticRealNumeralText("2")
	compilerMust(t, err)
	sort, err := m.GetSort(two)
	compilerMust(t, err)
	family, err := m.ArithmeticFamilyID()
	compilerMust(t, err)
	a := compilerConst(t, m, "parameter-a", sort)
	b := compilerConst(t, m, "parameter-b", sort)
	var forms []ast.ASTHandle
	var owners []int
	var systems [9][3]ast.ASTHandle
	add := func(form ast.ASTHandle, owner int) {
		t.Helper()
		compilerMust(t, g.Assert(form))
		forms = append(forms, form)
		owners = append(owners, owner)
	}
	for i := range systems {
		parameter := a
		if i == 4 || i == 6 {
			parameter = b
		}
		for j := range systems[i] {
			x := compilerConst(t, m, fmt.Sprintf("tuple-%d-%d", i, j), sort)
			systems[i][j] = x
			square, err := m.MkArithmeticMul(x, x)
			compilerMust(t, err)
			root, err := m.MkEq(square, parameter)
			compilerMust(t, err)
			add(root, i)
		}
		x, y, z := systems[i][0], systems[i][1], systems[i][2]
		sum, err := m.MkArithmeticAdd(x, y)
		compilerMust(t, err)
		link, err := m.MkArithmeticLE(sum, z)
		compilerMust(t, err)
		last := y
		if i == 3 || i == 7 {
			last = x
		}
		// This assertion shares a mixed child with link. All three slots
		// have identical unary constraints, so the child slot matters.
		check, err := m.MkArithmeticGT(sum, last)
		compilerMust(t, err)
		if i%2 != 0 {
			add(check, i)
			add(link, i)
			add(check, i)
		} else {
			add(link, i)
			add(check, i)
		}
	}
	// A variable buried in a Boolean formula connects the last two
	// participating systems. Their other rows cannot be omitted separately.
	nested, err := m.MkArithmeticGT(systems[5][0], systems[8][2])
	compilerMust(t, err)
	negated, err := m.MkNot(nested)
	compilerMust(t, err)
	add(negated, -1)
	p, err := scanPolynomialComponents(m, family, forms, map[ast.ASTHandle]bool{a: true, b: true})
	compilerMust(t, err)
	grouped, err := p.groupCoupledSystems(forms)
	compilerMust(t, err)
	if !grouped {
		t.Fatal("expected matching independent systems")
	}
	representative := map[int]int{1: 0, 2: 0, 6: 4, 7: 3}
	for i, system := range systems {
		for slot, x := range system {
			var want uint32
			if j, ok := representative[i]; ok {
				want = p.terms[systems[j][slot]].variable
			}
			if got := p.variables[p.terms[x].variable-1].representative; got != want {
				t.Fatalf("system %d slot %d uses variable %d, want %d", i, slot, got, want)
			}
		}
	}
	for i, form := range forms {
		if _, copied := representative[owners[i]]; copied != p.omitted[form] {
			t.Fatalf("system %d assertion %d omitted=%v, want %v", owners[i], i, p.omitted[form], copied)
		}
	}
}
