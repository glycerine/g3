package arith

import "testing"

func TestFMExpressionTableSavesLiveModelBounds(t *testing.T) {
	converter, m := fmModelFixture(t)
	v := NewFMVariableTable()
	x := v.Add(false, false)
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, table.Close()) })
	equationMust(t, table.BindVariable(x, fixDLConst(t, m, false, "fm_saved_x")))
	lower := fmSubsumptionConstraint(nil, []uint32{x}, []int64{-1}, 2, false)
	upper := fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 7, false)
	dead := fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 9, false)
	dead.Dead = true
	v.Index.Lower[x] = []*FMConstraint{lower}
	v.Index.Upper[x] = []*FMConstraint{upper, dead}
	equationMust(t, table.SaveModelBounds(x, v, converter))
	if len(converter.variables) != 1 || len(converter.clauses) != 1 || len(converter.clauses[0]) != 2 {
		t.Fatalf("saved model rows = %d/%v", len(converter.variables), converter.clauses)
	}
}
