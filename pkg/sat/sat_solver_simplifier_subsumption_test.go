package sat

import (
	"testing"
)

func satKernelMaskLiterals(mask uint32) []Literal {
	var r []Literal
	for l := uint32(0); l < 6; l++ {
		if mask&(1<<l) != 0 {
			r = append(r, Literal(l))
		}
	}
	return r
}

// Source clause construction requires size>1 in debug; shrinking a real
// initialized two-slot clause preserves the defined raw helper input domain.
func satKernelRawClause(t *testing.T, a *ClauseAllocator, lits []Literal) StoredClauseRef {
	t.Helper()
	if len(lits) > 1 {
		return satMk(t, a, lits, false)
	}
	padded := []Literal{0, 2}
	if len(lits) == 1 {
		padded[0] = lits[0]
	}
	r := satMk(t, a, padded, false)
	satOK(t, a.Shrink(r, uint32(len(lits))))
	satOK(t, a.UpdateApprox(r))
	return r
}

func TestSATSimplifierSubsumptionIndependentFiniteSets(t *testing.T) {
	k, a, _, _ := satSimplifierKernelFixture(t, 3)
	for left := uint32(1); left < 64; left++ {
		for right := uint32(1); right < 64; right++ {
			c1 := satKernelRawClause(t, a, satKernelMaskLiterals(left))
			c2 := satKernelRawClause(t, a, satKernelMaskLiterals(right))
			want0 := left & ^right == 0
			got0, e := k.subsumes0(c1, c2)
			satOK(t, e)
			if got0 != want0 {
				t.Fatal(left, right, got0, want0)
			}
			missing, complement := 0, DefaultLiteral()
			for l := uint32(0); l < 6; l++ {
				if left&(1<<l) != 0 && right&(1<<l) == 0 {
					missing++
					if right&(1<<(l^1)) != 0 {
						complement = Literal(l ^ 1)
					} else {
						complement = DefaultLiteral()
						missing = 7
						break
					}
				}
			}
			want1 := missing <= 1
			out := Literal(999)
			got1, e := k.subsumes1(c1, c2, &out)
			satOK(t, e)
			wantLit := DefaultLiteral()
			if want1 && missing == 1 {
				wantLit = complement
			}
			if got1 != want1 || out != wantLit {
				t.Fatal(left, right, got1, out, want1, wantLit)
			}
			for l := Literal(0); l < 6; l++ {
				marked, e := k.isMarked(l)
				if e != nil || marked {
					t.Fatal("marks not cleared", l, marked, e)
				}
			}
			satOK(t, a.Delete(c1))
			satOK(t, a.Delete(c2))
		}
	}
}

func TestSATSimplifierOccurrenceChoiceCollectorsAndCompression(t *testing.T) {
	k, a, u, b := satSimplifierKernelFixture(t, 3)
	c := satMk(t, a, satLs(0, 2), false)
	direct := satMk(t, a, satLs(0, 2, 4), false)
	self := satMk(t, a, satLs(1, 2, 5), false)
	dead := satMk(t, a, satLs(0, 2, 5), false)
	for _, r := range []StoredClauseRef{c, dead, direct, self} {
		satOK(t, u.Insert(r))
	}
	satOK(t, a.SetRemoved(dead, true))
	satOK(t, u.Erase(dead))
	v, e := k.minOccurrenceVar(c)
	satOK(t, e)
	if v != 0 {
		t.Fatal(v)
	}
	l, e := k.minOccurrenceLiteral(c)
	satOK(t, e)
	if l != 0 {
		t.Fatal(l)
	}
	out, e := NewSATClauseVector(a, b)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, out.Close()) })
	ls, e := newSATSolverLiteralStore(b)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, ls.Close()) })
	ref, e := ls.NewList()
	satOK(t, e)
	k.subCounter = 100
	satOK(t, k.collectSubsumed1(c, out, ls, ref))
	if out.Size() != 2 {
		t.Fatal(out.Size())
	}
	for i, r := range []StoredClauseRef{direct, self} {
		got, e := out.Get(uint32(i))
		satOK(t, e)
		if got != r {
			t.Fatal(i, got, r)
		}
	}
	for i, want := range []Literal{DefaultLiteral(), 1} {
		got, e := ls.Read(ref, uint32(i))
		satOK(t, e)
		if got != want {
			t.Fatal(i, got, want)
		}
	}
	if k.subCounter != 90 {
		t.Fatal(k.subCounter)
	}
	v0, e := u.Get(0)
	satOK(t, e)
	record, e := v0.record()
	satOK(t, e)
	if len(record.payload) != 2 {
		t.Fatal("lexical iterator compaction", len(record.payload))
	}
	satOK(t, out.Shrink(0))
	satOK(t, k.collectSubsumed0(c, out))
	if out.Size() != 1 {
		t.Fatal(out.Size())
	}
}

func TestSATSimplifierNativeCollectorPairedPublicationAndUnsignedCost(t *testing.T) {
	k, a, u, b := satSimplifierKernelFixture(t, 2)
	c := satMk(t, a, satLs(0, 2), false)
	d := satMk(t, a, satLs(0, 2, 3), false)
	satOK(t, u.Insert(c))
	satOK(t, u.Insert(d))
	out, e := NewSATClauseVector(a, b)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, out.Close()) })
	satOK(t, out.Push(c))
	satOK(t, out.Shrink(0))
	ls, e := newSATSolverLiteralStore(b)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, ls.Close()) })
	ref, e := ls.NewList()
	satOK(t, e)
	k.subCounter = -2147483647
	b.FailAfter(0)
	e = k.collectSubsumed1Core(c, out, ls, ref, 0)
	satOK(t, e)
	if out.Size() != 1 {
		t.Fatal("native clause/literal pair published", out.Size())
	}
	n, e2 := ls.Size(ref)
	satOK(t, e2)
	if n != 1 {
		t.Fatal(n)
	}
	if k.subCounter != 2147483644 {
		t.Fatal("source unsigned subtraction", k.subCounter)
	}
	b.FailAfter(-1)
}

func TestSATSimplifierCollectorFinishesCompactionAfterOutputRefusal(t *testing.T) {
	k, a, u, b := satSimplifierKernelFixture(t, 3)
	source := satMk(t, a, satLs(0, 2), false)
	first := satMk(t, a, satLs(0, 2, 4), false)
	second := satMk(t, a, satLs(0, 2, 5), false)
	deadBefore := satMk(t, a, satLs(0, 3, 4), false)
	deadAfter := satMk(t, a, satLs(0, 3, 5), false)
	for _, c := range []StoredClauseRef{deadBefore, first, deadAfter, second} {
		satOK(t, u.Insert(c))
	}
	for _, c := range []StoredClauseRef{deadBefore, deadAfter} {
		satOK(t, a.SetRemoved(c, true))
		satOK(t, u.Erase(c))
	}
	out, err := NewSATClauseVector(a, b)
	satOK(t, err)
	t.Cleanup(func() { satOK(t, out.Close()) })
	lits, ref := satKernelOutput(t, b)
	k.subCounter = 100
	if err := k.collectSubsumed1Core(source, out, lits, satSolverLiteralListRef{}, 0); err != ErrSATOwner {
		t.Fatalf("refused literal output returned %v", err)
	}
	got, err := out.Get(0)
	satOK(t, err)
	n, err := lits.Size(ref)
	satOK(t, err)
	if got != first || out.Size() != 1 || n != 0 || k.subCounter != 95 {
		t.Fatal("the refusal did not preserve the first candidate's output prefix")
	}
	view, err := u.Get(0)
	satOK(t, err)
	record, err := view.record()
	satOK(t, err)
	if len(record.payload) != 2 || record.payload[0] != first || record.payload[1] != second || record.liveSize != 2 {
		t.Fatal("cleanup did not compact all retired occurrences in order", record.payload)
	}
}

func TestSATSimplifierApproximateFilterAndFirstTie(t *testing.T) {
	k, a, u, b := satSimplifierKernelFixture(t, 35)
	c := satMk(t, a, satLs(0, 2), false)
	collision := satMk(t, a, satLs(0, 66, 4), false)
	satOK(t, u.Insert(c))
	satOK(t, u.Insert(collision))
	// Variable1 wins: its positive+negative count1 is below variable0 count2.
	v, e := k.minOccurrenceVar(c)
	satOK(t, e)
	if v != 1 {
		t.Fatal(v)
	}
	out, e := NewSATClauseVector(a, b)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, out.Close()) })
	k.subCounter = 20
	satOK(t, k.collectSubsumed0Core(c, out, 0))
	if out.Size() != 0 || k.subCounter != 15 {
		t.Fatal(out.Size(), k.subCounter)
	}
	tie := satMk(t, a, satLs(6, 8), false)
	v, e = k.minOccurrenceVar(tie)
	satOK(t, e)
	if v != 3 {
		t.Fatal(v)
	}
}

func TestSATSimplifierSubsumptionActualReadFailurePrefixes(t *testing.T) {
	k, a, _, _ := satSimplifierKernelFixture(t, 3)
	c1 := satMk(t, a, satLs(0, 2), false)
	c2 := satMk(t, a, satLs(0, 40, 4), false)
	out := Literal(999)
	if _, e := k.subsumes1(c1, c2, &out); e == nil {
		t.Fatal("second mark must guard the actual read")
	}
	if out != 999 {
		t.Fatal("output initialization follows all c2 marks", out)
	}
	yes, e := k.isMarked(0)
	satOK(t, e)
	if !yes {
		t.Fatal("completed first mark")
	}
	satOK(t, k.InitVisited(3))
	c2 = satMk(t, a, satLs(0, 2, 4), false)
	if _, e = k.subsumes1(StoredClauseRef{}, c2, &out); e == nil {
		t.Fatal("actual c1 size read")
	}
	if out != DefaultLiteral() {
		t.Fatal("completed output publication", out)
	}
	for _, l := range []Literal{0, 2, 4} {
		yes, e = k.isMarked(l)
		satOK(t, e)
		if !yes {
			t.Fatal("no source exception unmark", l)
		}
	}
	// The zero-output adapter must not publish source marks.
	satOK(t, k.InitVisited(3))
	if _, e = k.subsumes1(c1, c2, nil); e != ErrSATOwner {
		t.Fatal(e)
	}
}

func TestSATSimplifierCollectorsWarmPhysicalPlateauAndWrappedOccurrence(t *testing.T) {
	k, a, u, b := satSimplifierKernelFixture(t, 3)
	c := satMk(t, a, satLs(0, 2), false)
	d := satMk(t, a, satLs(0, 2, 4), false)
	satOK(t, u.Insert(c))
	satOK(t, u.Insert(d))
	out, e := NewSATClauseVector(a, b)
	satOK(t, e)
	t.Cleanup(func() { satOK(t, out.Close()) })
	ls, ref := satKernelOutput(t, b)
	satOK(t, k.collectSubsumed1(c, out, ls, ref))
	before := b.Used()
	b.FailAfter(0)
	var failure error
	allocs := testing.AllocsPerRun(50, func() {
		if failure != nil {
			return
		}
		failure = out.Shrink(0)
		if failure == nil {
			failure = ls.Shrink(ref, 0)
		}
		if failure == nil {
			failure = k.collectSubsumed1(c, out, ls, ref)
		}
	})
	if failure != nil || allocs != 0 || b.Used() != before {
		t.Fatal(failure, allocs, b.Used(), before)
	}
	b.FailAfter(-1)
	// Exact source unsigned addition is tested without constructing billions of
	// entries; the real inline occurrence counters are the only modified fields.
	v, e := u.Get(0)
	satOK(t, e)
	r, e := v.record()
	satOK(t, e)
	old := r.liveSize
	r.liveSize = ^uint32(0)
	v1, e := u.Get(1)
	satOK(t, e)
	r1, e := v1.record()
	satOK(t, e)
	r1.liveSize = 2
	chosen, e := k.minOccurrenceVar(c)
	satOK(t, e)
	if chosen != 0 {
		t.Fatal("wrapped count1 should beat count2", chosen)
	}
	r.liveSize = old
	r1.liveSize = 0
}
