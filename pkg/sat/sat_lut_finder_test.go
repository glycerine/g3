package sat

import "testing"

func satLUTDefinitionClauses(inputs uint32, fn func(uint32) bool) []satLUTClause {
	var out []satLUTClause
	for assignment := uint32(0); assignment < uint32(1)<<(inputs+1); assignment++ {
		want, actual := fn(assignment&((1<<inputs)-1)), assignment&(1<<inputs) != 0
		if want == actual {
			continue
		}
		cl := make([]Literal, inputs+1)
		for i := uint32(0); i <= inputs; i++ {
			cl[i] = MakeLiteral(BoolVar(i), assignment&(1<<i) != 0)
		}
		out = append(out, satLUTClause{Literals: cl})
	}
	return out
}

func TestSATLUTFinderExtractsTruthTables(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func(uint32) bool
		want uint64
	}{
		{"xor", func(x uint32) bool { return (x&1)^((x>>1)&1) != 0 }, 6},
		{"and", func(x uint32) bool { return x&3 == 3 }, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clauses := satLUTDefinitionClauses(2, tc.fn)
			var table uint64
			var args []BoolVar
			var result BoolVar
			remaining, e := newSATLUTFinder(func(x uint64, a []BoolVar, r BoolVar) { table = x; args = append([]BoolVar(nil), a...); result = r }).Run(clauses, nil)
			if e != nil || len(remaining) != 0 || table != tc.want || len(args) != 2 || args[0] != 0 || args[1] != 1 || result != 2 {
				t.Fatalf("error=%v remaining=%d table=%x args=%v result=%d", e, len(remaining), table, args, result)
			}
			for x := uint32(0); x < 4; x++ {
				if satLUTValue(table, []bool{x&1 != 0, x&2 != 0}) != tc.fn(x) {
					t.Fatalf("assignment %d", x)
				}
			}
		})
	}
}

func TestSATLUTFinderRejectsIncompleteDefinition(t *testing.T) {
	clauses := satLUTDefinitionClauses(2, func(x uint32) bool { return x&3 == 3 })[:3]
	calls := 0
	remaining, e := newSATLUTFinder(func(uint64, []BoolVar, BoolVar) { calls++ }).Run(clauses, nil)
	if e != nil || calls != 0 || len(remaining) != 3 {
		t.Fatal(e, calls, len(remaining))
	}
	if satLUTMaskString(6, 4) != "0110" {
		t.Fatal(satLUTMaskString(6, 4))
	}
}

func TestSATLUTFinderExpandsBinarySupport(t *testing.T) {
	all := satLUTDefinitionClauses(2, func(x uint32) bool { return x&3 == 3 })
	// The first two clauses differ only in x0. Their shared x1/y literals form
	// the binary clause that the source reads from the watch lists.
	binary := satLUTBinary{First: all[0].Literals[1], Second: all[0].Literals[2], Learned: true}
	clauses := []satLUTClause{all[2], all[3]}
	var table uint64
	remaining, e := newSATLUTFinder(func(x uint64, _ []BoolVar, _ BoolVar) { table = x }).Run(clauses, []satLUTBinary{binary})
	if e != nil || len(remaining) != 0 || table != 8 {
		t.Fatal(e, len(remaining), table)
	}
}
