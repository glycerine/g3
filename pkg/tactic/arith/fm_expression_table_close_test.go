package arith

import "testing"

func TestFMExpressionTableCloseReleasesReferences(t *testing.T) {
	_, m := fixDLFixture(t, false)
	v := NewFMVariableTable()
	x := v.Add(true, false)
	term := fixDLConst(t, m, true, "fm_close_x")
	before, e := m.GetRefCount(term)
	equationMust(t, e)
	table, e := NewFMExpressionTable(m, v)
	equationMust(t, e)
	equationMust(t, table.BindVariable(x, term))
	after, e := m.GetRefCount(term)
	equationMust(t, e)
	if after != before+1 {
		t.Fatalf("binding refcount = %d, want %d", after, before+1)
	}
	equationMust(t, table.Close())
	after, e = m.GetRefCount(term)
	equationMust(t, e)
	if after != before {
		t.Fatalf("close refcount = %d, want %d", after, before)
	}
	equationMust(t, table.Close())
}
