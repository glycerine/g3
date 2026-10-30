package sat

import "testing"

func TestSATXORFinderExtractsParityDefinition(t *testing.T) {
	clauses := satLUTDefinitionClauses(2, func(v uint32) bool { return (v&1)^((v>>1)&1) != 0 })
	var got []Literal
	remaining, e := newSATXORFinder(func(xs []Literal) error { got = append([]Literal(nil), xs...); return nil }).Run(clauses, nil)
	if e != nil || len(remaining) != 0 || len(got) != 3 {
		t.Fatal(e, len(remaining), got)
	}
	for assignment := uint32(0); assignment < 8; assignment++ {
		want := assignment&4 != 0 == ((assignment&1 != 0) != (assignment&2 != 0))
		if satXORLiteralsValue(got, assignment) != want {
			t.Fatalf("assignment %d literals %v", assignment, got)
		}
	}
}
func TestSATXORFinderIncompleteAndParityMismatch(t *testing.T) {
	clauses := satLUTDefinitionClauses(2, func(v uint32) bool { return v&3 == 3 })
	calls := 0
	remaining, e := newSATXORFinder(func([]Literal) error { calls++; return nil }).Run(clauses, nil)
	if e != nil || calls != 0 || len(remaining) != len(clauses) {
		t.Fatal(e, calls, len(remaining))
	}
	x := newSATXORFinder(nil)
	if !x.Parity(3, 7) || x.Parity(3, 3) {
		t.Fatal("parity table")
	}
}
func TestSATXORFinderUsesBinarySupport(t *testing.T) {
	all := satLUTDefinitionClauses(2, func(v uint32) bool { return (v&1)^((v>>1)&1) != 0 })
	binary := satLUTBinary{First: all[0].Literals[1], Second: all[0].Literals[2], Learned: true}
	clauses := []satLUTClause{all[1], all[2], all[3]}
	calls := 0
	remaining, e := newSATXORFinder(func([]Literal) error { calls++; return nil }).Run(clauses, []satLUTBinary{binary})
	if e != nil || calls != 1 || len(remaining) != 0 {
		t.Fatal(e, calls, len(remaining))
	}
}
