package arith

import "testing"

func TestFMConstraintExpressionMixedArithmeticCoercesInteger(t *testing.T) {
	_, m := fixDLFixture(t, false)
	v := NewFMVariableTable()
	x, y := v.Add(true, false), v.Add(false, false)
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, table.Close()) })
	equationMust(t, table.BindVariable(x, fixDLConst(t, m, true, "fm_mixed_x")))
	equationMust(t, table.BindVariable(y, fixDLConst(t, m, false, "fm_mixed_y")))
	expression, e := table.ConstraintExpression(fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{1, 1}, 4, false))
	equationMust(t, e)
	lhs, e := m.GetArg(expression, 0)
	equationMust(t, e)
	coerced, e := m.GetArg(lhs, 0)
	equationMust(t, e)
	op, e := m.GetDeclKind(coerced)
	equationMust(t, e)
	if op != 14 {
		t.Fatalf("mixed integer term op = %d, want to_real", op)
	}
}

func TestFMConstraintExpressionRebuildsLiteralClause(t *testing.T) {
	_, m := fixDLFixture(t, false)
	v := NewFMVariableTable()
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, table.Close()) })
	ph := fmBoolConst(t, m, "fm_clause_p")
	qh := fmBoolConst(t, m, "fm_clause_q")
	p, e := table.AddBoolean(ph)
	equationMust(t, e)
	q, e := table.AddBoolean(qh)
	equationMust(t, e)
	expression, e := table.ConstraintExpression(&FMConstraint{Literals: []int32{int32(p), -int32(q)}, Constant: fmCRat(-1, 1)})
	equationMust(t, e)
	notQ, e := m.MkNot(qh)
	equationMust(t, e)
	want, e := m.MkOr(ph, notQ)
	equationMust(t, e)
	if expression != want {
		t.Fatal("literal clause was not rebuilt with source signs")
	}
}
