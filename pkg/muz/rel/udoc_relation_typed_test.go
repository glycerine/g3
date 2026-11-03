package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func typedUDocFixture(t *testing.T) (*UDocRelationFactory, *ast.ASTManager, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m := docASTManager(t)
	factory, err := NewUDocRelationFactory(m, arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: arena.NewBudget(1 << 20)}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(factory.Close)
	bv, err := m.BitVectorSort(3)
	if err != nil {
		t.Fatal(err)
	}
	finite, err := m.MkDatalogFiniteSort(astTestSymbol(t, m, "typed-udoc-finite"), 5)
	if err != nil {
		t.Fatal(err)
	}
	return factory, m, bv, finite
}

func TestTypedUDocFactsCloneAndColumnFormula(t *testing.T) {
	factory, m, bv, finite := typedUDocFixture(t)
	relation, err := factory.NewEmpty([]ast.ASTHandle{bv, finite})
	if err != nil {
		t.Fatal(err)
	}
	defer relation.Close()
	bvValue, err := m.BitVectorNumeralUint64(5, 3)
	if err != nil {
		t.Fatal(err)
	}
	finiteValue, err := m.MkDatalogNumeral(4, finite)
	if err != nil {
		t.Fatal(err)
	}
	inserted, err := relation.AddFact([]ast.ASTHandle{bvValue, finiteValue})
	if err != nil || !inserted {
		t.Fatal("add", inserted, err)
	}
	found, err := relation.ContainsFact([]ast.ASTHandle{bvValue, finiteValue})
	if err != nil || !found {
		t.Fatal("contains", found, err)
	}
	clone, err := relation.Clone()
	if err != nil {
		t.Fatal(err)
	}
	defer clone.Close()
	if err = relation.Reset(); err != nil {
		t.Fatal("reset", err)
	}
	if empty, err := relation.FastEmpty(); err != nil || !empty {
		t.Fatal("reset emptiness", empty, err)
	}
	found, err = clone.ContainsFact([]ast.ASTHandle{bvValue, finiteValue})
	if err != nil || !found {
		t.Fatal("clone independence", found, err)
	}
	formula, err := clone.ToFormula()
	if err != nil {
		t.Fatal(err)
	}
	defer m.DelRef(&formula)
	formulaHandle, err := m.RefGet(formula)
	if err != nil {
		t.Fatal(err)
	}
	v0, _ := m.MkVar(0, bv)
	v1, _ := m.MkVar(1, finite)
	eq0, _ := m.MkEq(v0, bvValue)
	eq1, _ := m.MkEq(v1, finiteValue)
	want, _ := m.MkAndArray([]ast.ASTHandle{eq0, eq1})
	if formulaHandle != want {
		t.Fatal("formula did not preserve original column sorts", formulaHandle, want)
	}
}

func TestTypedUDocJoinProjectAndNullaryFormula(t *testing.T) {
	factory, m, bv, _ := typedUDocFixture(t)
	left, _ := factory.NewEmpty([]ast.ASTHandle{bv, bv})
	right, _ := factory.NewEmpty([]ast.ASTHandle{bv})
	defer left.Close()
	defer right.Close()
	one, _ := m.BitVectorNumeralUint64(1, 3)
	two, _ := m.BitVectorNumeralUint64(2, 3)
	three, _ := m.BitVectorNumeralUint64(3, 3)
	_, _ = left.AddFact([]ast.ASTHandle{one, two})
	_, _ = left.AddFact([]ast.ASTHandle{three, one})
	_, _ = right.AddFact([]ast.ASTHandle{two})
	projected, err := left.JoinProject(right, []uint32{1}, []uint32{0}, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	defer projected.Close()
	if got := projected.Sorts(); len(got) != 1 || got[0] != bv {
		t.Fatal("projected signature", got)
	}
	found, err := projected.ContainsFact([]ast.ASTHandle{one})
	if err != nil || !found {
		t.Fatal("joined projected tuple", found, err)
	}
	found, err = projected.ContainsFact([]ast.ASTHandle{three})
	if err != nil || found {
		t.Fatal("spurious joined tuple", found, err)
	}

	full, _ := factory.NewFull(nil)
	empty, _ := factory.NewEmpty(nil)
	defer full.Close()
	defer empty.Close()
	fullFormula, err := full.ToFormula()
	if err != nil {
		t.Fatal(err)
	}
	defer m.DelRef(&fullFormula)
	emptyFormula, err := empty.ToFormula()
	if err != nil {
		t.Fatal(err)
	}
	defer m.DelRef(&emptyFormula)
	fullHandle, err := m.RefGet(fullFormula)
	if err != nil {
		t.Fatal(err)
	}
	emptyHandle, err := m.RefGet(emptyFormula)
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsTrue(fullHandle) || !m.IsFalse(emptyHandle) {
		t.Fatal("nullary formulas", fullHandle, emptyHandle)
	}
}

func TestTypedUDocUnionFiltersRenameAndDelta(t *testing.T) {
	factory, m, bv, _ := typedUDocFixture(t)
	target, err := factory.NewEmpty([]ast.ASTHandle{bv, bv})
	if err != nil {
		t.Fatal(err)
	}
	source, _ := factory.NewEmpty([]ast.ASTHandle{bv, bv})
	delta, _ := factory.NewEmpty([]ast.ASTHandle{bv, bv})
	defer target.Close()
	defer source.Close()
	defer delta.Close()
	zero, _ := m.BitVectorNumeralUint64(0, 3)
	one, _ := m.BitVectorNumeralUint64(1, 3)
	two, _ := m.BitVectorNumeralUint64(2, 3)
	add := func(r *TypedUDocRelation, values ...ast.ASTHandle) {
		t.Helper()
		if inserted, e := r.AddFact(values); e != nil || !inserted {
			t.Fatal("add", inserted, e)
		}
	}
	add(target, zero, zero)
	add(source, one, two)
	add(source, two, two)
	if err = target.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	for _, values := range [][]ast.ASTHandle{{one, two}, {two, two}} {
		found, e := delta.ContainsFact(values)
		if e != nil || !found {
			t.Fatal("delta", found, e)
		}
	}
	renamed, err := source.Rename([]uint32{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	defer renamed.Close()
	found, err := renamed.ContainsFact([]ast.ASTHandle{two, one})
	if err != nil || !found {
		t.Fatal("rename", found, err)
	}
	if err = source.FilterEqual(1, two); err != nil {
		t.Fatal(err)
	}
	if err = source.FilterIdentical([]uint32{0, 1}); err != nil {
		t.Fatal(err)
	}
	found, err = source.ContainsFact([]ast.ASTHandle{two, two})
	if err != nil || !found {
		t.Fatal("identical kept", found, err)
	}
	found, err = source.ContainsFact([]ast.ASTHandle{one, two})
	if err != nil || found {
		t.Fatal("identical removed", found, err)
	}
}
