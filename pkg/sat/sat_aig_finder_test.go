package sat

import "testing"

func TestSATAIGFinderRecognizesANDFromBinaryImplications(t *testing.T) {
	h, a, b := MakeLiteral(2, false), MakeLiteral(0, false), MakeLiteral(1, false)
	clauses := []satLUTClause{{Literals: []Literal{h, a.Negated(), b.Negated()}}}
	bins := []satLUTBinary{{First: h.Negated(), Second: a}, {First: h.Negated(), Second: b}}
	calls := 0
	remaining, e := newSATAIGFinder(func(head Literal, args []Literal) error {
		calls++
		if head != h || len(args) != 2 || args[0] != a || args[1] != b {
			t.Fatalf("head=%v args=%v", head, args)
		}
		return nil
	}, nil).Run(clauses, bins)
	if e != nil || calls != 1 || len(remaining) != 0 {
		t.Fatal(e, calls, len(remaining))
	}
}

func TestSATAIGFinderRecognizesITEFromFourClauses(t *testing.T) {
	r, c, y, n := MakeLiteral(3, false), MakeLiteral(0, false), MakeLiteral(1, false), MakeLiteral(2, false)
	clauses := []satLUTClause{{Literals: []Literal{r.Negated(), c.Negated(), y}}, {Literals: []Literal{r.Negated(), c, n}}, {Literals: []Literal{r, c.Negated(), y.Negated()}}, {Literals: []Literal{r, c, n.Negated()}}}
	calls := 0
	remaining, e := newSATAIGFinder(nil, func(head, cond, yes, no Literal) error {
		calls++
		if head != r || cond != c || yes != y || no != n {
			t.Fatalf("ite %v=%v?%v:%v", head, cond, yes, no)
		}
		return nil
	}).Run(clauses, nil)
	if e != nil || calls != 1 || len(remaining) != 0 {
		t.Fatal(e, calls, len(remaining))
	}
}

func TestSATAIGFinderLeavesIncompletePattern(t *testing.T) {
	clauses := []satLUTClause{{Literals: []Literal{0, 2, 4}}}
	calls := 0
	remaining, e := newSATAIGFinder(func(Literal, []Literal) error { calls++; return nil }, nil).Run(clauses, nil)
	if e != nil || calls != 0 || len(remaining) != 1 {
		t.Fatal(e, calls, len(remaining))
	}
}
