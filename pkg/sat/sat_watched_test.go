package sat

import (
	"errors"
	"math"
	"testing"
)

func TestSATBinaryEncodedEqualityAndWrapper(t *testing.T) {
	b := newSATBinClause(2, 4, true)
	if b.Literal1() != 2 || b.Literal2() != 4 || !b.Learned() {
		t.Fatal(b)
	}
	x := newSATBinClause(9, 1, false)
	y := newSATBinClause(2, 4, true)
	if !x.Equal(y) {
		t.Fatal("source encoded reverse comparison")
	}
	rev := newSATBinClause(4, 2, true)
	if b.Equal(rev) {
		t.Fatal("not conventional unorderedpair")
	}
	big := newSATBinClause(0, Literal(math.MaxUint32), false)
	if big.Literal2() != Literal(math.MaxInt32) {
		t.Fatal(big)
	}
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 3), true)
	w := newSATClauseWrapper(r)
	n, e := w.Size(a)
	if n != 2 || e != nil {
		t.Fatal(n, e)
	}
	has, e := w.ContainsVar(a, 1)
	if !has || e != nil {
		t.Fatal(has, e)
	}
	binary := newSATBinaryWrapper(0, 3)
	if !binary.Binary() {
		t.Fatal(binary)
	}
	if _, e = binary.ClauseRef(); !errors.Is(e, ErrSATKind) {
		t.Fatal(e)
	}
}
func TestSATWatchedSourceTagsWidthDefectAndErase(t *testing.T) {
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 2, 4), true)
	off, e := a.Offset(r)
	satOK(t, e)
	b, e := newSATBinaryWatch(2, true)
	satOK(t, e)
	if !b.Binary() || !b.LearnedUnchecked() || b.BinaryNonLearned() {
		t.Fatal(b)
	}
	satOK(t, b.MarkNotLearned())
	if !b.BinaryNonLearned() {
		t.Fatal(b)
	}
	ter, e := newSATTernaryWatch(8, 2)
	satOK(t, e)
	l1, e := ter.Literal1()
	satOK(t, e)
	l2, e := ter.Literal2()
	if l1 != 2 || l2 != 8 || e != nil {
		t.Fatal(l1, l2, e)
	}
	c, e := newSATClauseWatch(4, off)
	satOK(t, e)
	for _, idx := range []uint32{0, 3, math.MaxUint32} {
		ext, e := newSATExternalWatch(idx)
		if e != nil {
			t.Fatal(e)
		}
		got, e := ext.ExternalIndex()
		if got != idx || e != nil {
			t.Fatal("source external index", got, e)
		}
	}
	wide, e := newSATTernaryWatch(0, Literal(math.MaxUint32))
	if satStorageSourceDebug && !errors.Is(e, ErrSATAssertion) {
		t.Fatal(e)
	}
	if !satStorageSourceDebug && e != nil {
		t.Fatal(e)
	}
	if got, _ := wide.Literal2(); got != Literal(0x3fffffff) {
		t.Fatal(got)
	}
	satOK(t, c.SetBlockedLiteral(Literal(math.MaxUint32)))
	if got, _ := c.BlockedLiteral(); got != Literal(0x3fffffff) {
		t.Fatal("setter truncates all profiles", got)
	}
	if !satWatchedLess(b, ter) || !satWatchedLess(ter, c) || satWatchedLess(c, ter) {
		t.Fatal("source ordering")
	}

}
func TestSATPackedSourceGetterSetterDomains(t *testing.T) {
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 2), false)
	off, _ := a.Offset(r)
	w, e := newSATClauseWatch(4, off)
	satOK(t, e)
	satOK(t, w.SetClauseOffset(off))
	w.SetClause(8, off)
	if l, e := w.BlockedLiteral(); l != 8 || e != nil {
		t.Fatal(l, e)
	}
	if got, e := w.ClauseOffset(); got != off || e != nil {
		t.Fatal(got, e)
	}
	bin, _ := newSATBinaryWatch(2, true)
	satOK(t, bin.SetLiteral(8))
	if l, e := bin.Literal(); l != 8 || e != nil {
		t.Fatal(l, e)
	}
	if learned, e := bin.Learned(); !learned || e != nil {
		t.Fatal(learned, e)
	}
	for _, kind := range []uint32{0, 1, 2, 3} {
		var x satWatched
		switch kind {
		case 0:
			x, _ = newSATBinaryWatch(10, false)
		case 1:
			x, _ = newSATTernaryWatch(2, 10)
		case 2:
			x, _ = newSATClauseWatch(10, off)
		case 3:
			x, _ = newSATExternalWatch(3)
		}
		l, e := x.Literal()
		if satStorageSourceDebug && kind != 0 {
			if !errors.Is(e, ErrSATAssertion) {
				t.Fatal(kind, e)
			}
		} else if l != Literal(x.val1) || e != nil {
			t.Fatal(kind, l, e)
		}
		ext, e := x.ExternalIndex()
		if satStorageSourceDebug && kind != 3 {
			if !errors.Is(e, ErrSATAssertion) {
				t.Fatal(kind, e)
			}
		} else if ext != x.val1 || e != nil {
			t.Fatal(kind, ext, e)
		}
		if !x.Equal(x) || x.NotEqual(x) {
			t.Fatal(kind)
		}
		j := newSATExternalJustification(88)
		l, e = j.Literal()
		if satStorageSourceDebug {
			if !errors.Is(e, ErrSATAssertion) {
				t.Fatal(e)
			}
		} else if l != 88 || e != nil {
			t.Fatal(l, e)
		}
	}
	sentinel := newSATBinaryWrapper(0, DefaultLiteral())
	if _, e = sentinel.Size(a); !errors.Is(e, ErrSATKind) {
		t.Fatal(e)
	}
}
