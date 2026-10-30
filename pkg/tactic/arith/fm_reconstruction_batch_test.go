package arith

import "testing"

func TestFMRemainingDeduplicatesAndMarksConstraints(t *testing.T) {
	_, m := fixDLFixture(t, false)
	v := NewFMVariableTable()
	x, y := v.Add(false, false), v.Add(false, false)
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, table.Close()) })
	equationMust(t, table.BindVariable(x, fixDLConst(t, m, false, "fm_remaining_x")))
	equationMust(t, table.BindVariable(y, fixDLConst(t, m, false, "fm_remaining_y")))
	c := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{1, 1}, 5, false)
	c.ID = 1
	v.Register(c)
	result, e := table.Remaining(v)
	equationMust(t, e)
	if len(result) != 1 || !c.Dead || result[0].Expression.IsNull() {
		t.Fatalf("remaining result = %#v dead=%v", result, c.Dead)
	}
}
