package smtcore

import (
	"errors"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTWatchListTypedRegionsAndRemoval(t *testing.T) {
	var watches SMTWatchList
	first, second, third := &SMTClause{}, &SMTClause{}, &SMTClause{}
	for _, clause := range []*SMTClause{first, second, third} {
		if err := watches.InsertClause(clause); err != nil {
			t.Fatal(err)
		}
	}
	one := NewSMTLiteral(1, false)
	two := NewSMTLiteral(2, true)
	three := NewSMTLiteral(3, false)
	for _, literal := range []SMTLiteral{one, two, three} {
		if err := watches.InsertLiteral(literal); err != nil {
			t.Fatal(err)
		}
	}
	if watches.Size() != 6 || watches.FindClause(second) != 1 || watches.FindLiteral(two) != 1 {
		t.Fatal("watch lookup", watches.Size(), watches.FindClause(second), watches.FindLiteral(two))
	}
	if got := watches.Literals(); len(got) != 3 || got[0] != three || got[1] != two || got[2] != one {
		t.Fatal("source reverse literal insertion order", got)
	}
	if !watches.RemoveClause(second) || watches.RemoveClause(second) {
		t.Fatal("clause removal")
	}
	if got := watches.Clauses(); len(got) != 2 || got[0] != first || got[1] != third {
		t.Fatal("clause order after removal", got)
	}
	if !watches.RemoveLiteral(two) || watches.RemoveLiteral(two) {
		t.Fatal("literal removal")
	}
	if got := watches.Literals(); len(got) != 2 || got[0] != three || got[1] != one {
		t.Fatal("literal order after removal", got)
	}
}

func TestSMTWatchListCompactionResetAndGuards(t *testing.T) {
	var watches SMTWatchList
	a, b, c := &SMTClause{}, &SMTClause{}, &SMTClause{}
	for _, clause := range []*SMTClause{a, b, c} {
		if err := watches.InsertClause(clause); err != nil {
			t.Fatal(err)
		}
	}
	clauses := watches.Clauses()
	clauses[1] = clauses[2]
	if err := watches.SetClauseCount(2); err != nil {
		t.Fatal(err)
	}
	if got := watches.Clauses(); len(got) != 2 || got[0] != a || got[1] != c {
		t.Fatal("in-place compaction", got)
	}
	if err := watches.SetClauseCount(3); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("wide clause end", err)
	}
	if err := (*SMTWatchList)(nil).InsertClause(a); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil insertion", err)
	}
	if err := watches.InsertLiteral(SMTFalseLiteral); err != nil {
		t.Fatal(err)
	}
	watches.Reset()
	if watches.Size() != 0 || watches.Clauses() == nil || watches.Literals() == nil {
		t.Fatal("reset should retain allocated slices")
	}
	if err := watches.InsertLiteral(SMTTrueLiteral); err != nil {
		t.Fatal(err)
	}
	watches.ResetAndReleaseMemory()
	if watches.Size() != 0 || watches.Clauses() != nil || watches.Literals() != nil {
		t.Fatal("release reset")
	}
}

// AUDIT.md smtcore_14-F2: insert_literal writes in front of the live
// literals and remove_literal shifts only the literals in front of the
// removed one, so wide binary-clause fan-out is not quadratic.
func TestSMTWatchListLiteralInsertAndNewestFirstRemovalAreAmortizedConstant(t *testing.T) {
	const n = 300000
	start := time.Now()
	var watches SMTWatchList
	for i := int32(0); i < n; i++ {
		if err := watches.InsertLiteral(NewSMTLiteral(i, false)); err != nil {
			t.Fatal(err)
		}
	}
	if got := watches.Literals(); len(got) != n || got[0] != NewSMTLiteral(n-1, false) || got[n-1] != NewSMTLiteral(0, false) {
		t.Fatal("newest-first literal order")
	}
	for i := int32(n - 1); i >= 0; i-- {
		if !watches.RemoveLiteral(NewSMTLiteral(i, false)) {
			t.Fatal("remove", i)
		}
	}
	if watches.Size() != 0 || len(watches.Literals()) != 0 {
		t.Fatal("watch list not empty", watches.Size())
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatal("literal insert/remove is not amortized constant", elapsed)
	}

	// Middle removal after growth keeps the relative order of the rest.
	watches.Reset()
	for i := int32(0); i < 10; i++ {
		if err := watches.InsertLiteral(NewSMTLiteral(i, false)); err != nil {
			t.Fatal(err)
		}
	}
	if !watches.RemoveLiteral(NewSMTLiteral(6, false)) { // the 4th newest
		t.Fatal("middle removal")
	}
	want := []int32{9, 8, 7, 5, 4, 3, 2, 1, 0}
	got := watches.Literals()
	if len(got) != len(want) || watches.Size() != len(want) {
		t.Fatal("middle removal size", got)
	}
	for i, v := range want {
		if got[i] != NewSMTLiteral(v, false) {
			t.Fatal("middle removal order", got)
		}
	}
	if err := watches.InsertLiteral(NewSMTLiteral(42, false)); err != nil || watches.Literals()[0] != NewSMTLiteral(42, false) || watches.FindLiteral(NewSMTLiteral(0, false)) != len(want) {
		t.Fatal("insert after removal", err, watches.Literals())
	}
	watches.ResetAndReleaseMemory()
	if watches.Size() != 0 || len(watches.Literals()) != 0 {
		t.Fatal("release")
	}
}

func TestSMTWatchListRecycleDropsRetiredClauses(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		var w SMTWatchList
		for range count {
			if err := w.InsertClause(&SMTClause{}); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.InsertLiteral(SMTTrueLiteral); err != nil {
			t.Fatal(err)
		}
		backing := w.Clauses()
		w.recycle()
		if w.Size() != 0 || cap(w.clauses) != cap(backing) {
			t.Fatalf("recycle of %d clauses lost the buffer or left a watch", count)
		}
		for _, c := range backing {
			if c != nil {
				t.Fatalf("recycle of %d clauses retained a pointer", count)
			}
		}
	}
}
