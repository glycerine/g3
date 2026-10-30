package arith

import "testing"

func TestFMConstraintExpressionConstantBranches(t *testing.T) {
	_, m := fixDLFixture(t, false)
	v := NewFMVariableTable()
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, table.Close()) })
	truth, e := table.ConstraintExpression(&FMConstraint{Constant: fmCRat(1, 1)})
	equationMust(t, e)
	wantTrue, e := m.MkTrue()
	equationMust(t, e)
	if truth != wantTrue {
		t.Fatal("positive constant did not reconstruct true")
	}
	falsehood, e := table.ConstraintExpression(&FMConstraint{Constant: fmCRat(-1, 1)})
	equationMust(t, e)
	wantFalse, e := m.MkFalse()
	equationMust(t, e)
	if falsehood != wantFalse {
		t.Fatal("negative constant did not reconstruct false")
	}
}

func TestFMConstraintExpressionStrictUsesNotGE(t *testing.T) {
	_, m := fixDLFixture(t, false)
	v := NewFMVariableTable()
	x := v.Add(false, false)
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, table.Close()) })
	equationMust(t, table.BindVariable(x, fixDLConst(t, m, false, "fm_strict_x")))
	expression, e := table.ConstraintExpression(fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 3, true))
	equationMust(t, e)
	isNot, child, e := m.IsNot(expression)
	equationMust(t, e)
	op, e := m.GetDeclKind(child)
	equationMust(t, e)
	if !isNot || op != 3 {
		t.Fatalf("strict reconstruction is not (not (>= ...)): not=%v op=%d", isNot, op)
	}
}
