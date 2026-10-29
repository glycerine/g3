package sat

import (
	"reflect"
	"testing"
)

func TestSATAIGNodeAndCutClauseEncoding(t *testing.T) {
	a := testSATAIG(t)
	defer a.Close()
	if e := a.AddVar(0); e != nil {
		t.Fatal(e)
	}
	if e := a.AddVar(1); e != nil {
		t.Fatal(e)
	}
	var clauses [][]Literal
	a.SetOnClauseAdd(func(c []Literal) { clauses = append(clauses, append([]Literal(nil), c...)) })
	if e := a.AddNode(MakeLiteral(2, false), satAIGAnd, []Literal{MakeLiteral(0, false), MakeLiteral(1, true)}); e != nil {
		t.Fatal(e)
	}
	want := [][]Literal{{MakeLiteral(2, true), MakeLiteral(0, false)}, {MakeLiteral(2, true), MakeLiteral(1, true)}, {MakeLiteral(0, true), MakeLiteral(1, false), MakeLiteral(2, false)}}
	if len(clauses) != 5 || !reflect.DeepEqual(clauses[:3], want) {
		t.Fatalf("AND clauses %v want %v", clauses, want)
	}
	clauses = nil
	c := testSATCut([]uint32{0, 1}, 6)
	if e := a.Cut2Def(func(x []Literal) { clauses = append(clauses, append([]Literal(nil), x...)) }, c, MakeLiteral(3, false)); e != nil {
		t.Fatal(e)
	}
	if len(clauses) != 4 {
		t.Fatal(clauses)
	}
	for assignment, cl := range clauses {
		if len(cl) != 3 || cl[0].sign() != (assignment&1 != 0) || cl[1].sign() != (assignment&2 != 0) {
			t.Fatalf("assignment %d clause %v", assignment, cl)
		}
		wantSign := c.Table()&(uint64(1)<<uint32(assignment)) == 0
		if cl[2].sign() != wantSign {
			t.Fatalf("result sign assignment %d: %v", assignment, cl)
		}
	}
}

func TestSATAIGLUTClauseEncodingIsIndependent(t *testing.T) {
	a := testSATAIG(t)
	defer a.Close()
	if e := a.AddVar(0); e != nil {
		t.Fatal(e)
	}
	if e := a.AddVar(1); e != nil {
		t.Fatal(e)
	}
	var got [][]Literal
	a.SetOnClauseAdd(func(c []Literal) { got = append(got, append([]Literal(nil), c...)) })
	if e := a.AddLUT(2, 9, []BoolVar{0, 1}); e != nil {
		t.Fatal(e)
	}
	if len(got) != 6 {
		t.Fatal(got)
	}
	got[0][0] = MakeLiteral(9, false)
	if got[1][0].variable() == 9 {
		t.Fatal("callback clauses alias")
	}
}
