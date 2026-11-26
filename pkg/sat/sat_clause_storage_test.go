package sat

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func satStore(t *testing.T, b *arena.Budget) *ClauseAllocator {
	t.Helper()
	a, e := NewSATClauseAllocator(satClauseAllocatorConfig{Storage: arena.SmallBytesConfig{Descriptors: arena.Config{Budget: b}}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func satLs(v ...uint32) []Literal {
	r := make([]Literal, len(v))
	for i, x := range v {
		r[i] = LiteralFromUint(x)
	}
	return r
}
func satMk(t *testing.T, a *ClauseAllocator, lits []Literal, learned bool) StoredClauseRef {
	t.Helper()
	r, e := a.MkClause(lits, learned)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func satOK(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func TestSATClauseSourceLifecycleFlagsAndApprox(t *testing.T) {
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 3, 64), true)
	if id, e := a.ID(r); id != 0 || e != nil {
		t.Fatal(id, e)
	}
	if n, e := a.Size(r); n != 3 || e != nil {
		t.Fatal(n, e)
	}
	if n, e := a.Capacity(r); n != 3 || e != nil {
		t.Fatal(n, e)
	}
	if mask, e := a.Approx(r); mask != 3 || e != nil {
		t.Fatal(mask, e)
	}
	for _, get := range []func(StoredClauseRef) (bool, error){a.Learned, a.Strengthened} {
		v, e := get(r)
		if !v || e != nil {
			t.Fatal(v, e)
		}
	}
	for _, get := range []func(StoredClauseRef) (bool, error){a.Removed, a.Used, a.Frozen, a.OnReinitStack} {
		v, e := get(r)
		if v || e != nil {
			t.Fatal(v, e)
		}
	}
	satOK(t, a.UnmarkStrengthened(r))
	satOK(t, a.SetLiteral(r, 0, LiteralFromUint(10)))
	if mask, _ := a.Approx(r); mask != 3 {
		t.Fatal("literal assignment must retain stale mask")
	}
	_, e := a.CheckApprox(r)
	if satStorageSourceDebug && !errors.Is(e, ErrSATAssertion) {
		t.Fatal(e)
	}
	if !satStorageSourceDebug && e != nil {
		t.Fatal(e)
	}
	if mask, _ := a.Approx(r); mask != 35 {
		t.Fatal("check approx must mutate", mask)
	}
	satOK(t, a.Freeze(r))
	if frozen, _ := a.Frozen(r); !frozen {
		t.Fatal("freeze")
	}
	satOK(t, a.Unfreeze(r))
	satOK(t, a.SetRemoved(r, true))
	satOK(t, a.MarkUsed(r))
	satOK(t, a.SetReinitStack(r, true))
	for i := 0; i < 256; i++ {
		satOK(t, a.IncInactivity(r))
	}
	if v, _ := a.Inactivity(r); v != 0 {
		t.Fatal(v)
	}
	satOK(t, a.SetGlue(r, math.MaxUint32))
	satOK(t, a.SetPSM(r, 256))
	if v, _ := a.Glue(r); v != 255 {
		t.Fatal(v)
	}
	if v, _ := a.PSM(r); v != 255 {
		t.Fatal(v)
	}
	satOK(t, a.ResetInactivity(r))
	satOK(t, a.UnmarkUsed(r))
	satOK(t, a.UnsetLearned(r))
	satOK(t, a.Shrink(r, 2))
	if n, _ := a.Capacity(r); n != 3 {
		t.Fatal(n)
	}
	satOK(t, a.Elim(r, LiteralFromUint(10)))
	if n, _ := a.Size(r); n != 1 {
		t.Fatal(n)
	}
	if l, _ := a.Literal(r, 0); l != LiteralFromUint(3) {
		t.Fatal(l)
	}
	if l, e := a.Literal(r, 1); satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal(l, e)
		}
	} else if e != nil || l != LiteralFromUint(3) {
		t.Fatal(l, e)
	}
}
func TestSATClauseReleaseDebugBodiesAndModels(t *testing.T) {
	a := satStore(t, nil)
	for _, n := range []int{0, 1} {
		r, e := a.MkClause(make([]Literal, n), false)
		if satStorageSourceDebug {
			if !errors.Is(e, ErrSATAssertion) || !a.IsLive(r) {
				t.Fatal("ctor actual body prefix", r, e)
			}
		} else if e != nil {
			t.Fatal(e)
		}
		if v, _ := a.Size(r); v != uint32(n) {
			t.Fatal(v)
		}
		satOK(t, a.Delete(r))
	}
	r := satMk(t, a, satLs(0, 3, 4), false)
	e := a.Elim(r, LiteralFromUint(99))
	n, _ := a.Size(r)
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) || n != 3 {
			t.Fatal(n, e)
		}
	} else if e != nil || n != 2 {
		t.Fatal("release absent drops last", n, e)
	}
	e = a.Shrink(r, 100)
	if satStorageSourceDebug && !errors.Is(e, ErrSATAssertion) {
		t.Fatal(e)
	}
	if !satStorageSourceDebug && e != nil {
		t.Fatal(e)
	}
	e = a.Freeze(r)
	if satStorageSourceDebug && !errors.Is(e, ErrSATAssertion) {
		t.Fatal(e)
	}
	if !satStorageSourceDebug {
		v, _ := a.Frozen(r)
		if !v || e != nil {
			t.Fatal(v, e)
		}
	}
	for state := 0; state < 27; state++ {
		m := make(Model, 3)
		x := state
		for i := range m {
			m[i] = lbool.LBool(x%3 - 1)
			x /= 3
		}
		want := false
		for i := uint32(0); i < n; i++ {
			l, _ := a.Literal(r, i)
			v := m[l.variable()]
			want = want || (!l.sign() && v == lbool.True) || (l.sign() && v == lbool.False)
		}
		got, e := a.SatisfiedBy(r, m)
		if got != want || e != nil {
			t.Fatal(state, got, want, e)
		}
	}
	satOK(t, a.Shrink(r, 0))
	if e = a.Elim(r, 0); !errors.Is(e, arena.ErrBounds) && !errors.Is(e, ErrSATAssertion) {
		t.Fatal(e)
	}
}
func TestSATTmpClauseSourceReuseAndDestroyBeforeGrow(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	tmp, e := NewSATTmpClause(b)
	satOK(t, e)
	defer tmp.Close()
	satOK(t, tmp.Set(satLs(0, 2, 4), true))
	r := tmp.Get()
	if id, _ := tmp.owner.ID(r); id != math.MaxUint32 {
		t.Fatal(id)
	}
	satOK(t, tmp.owner.SetRemoved(r, true))
	satOK(t, tmp.owner.SetGlue(r, 9))
	before, _ := tmp.owner.Approx(r)
	satOK(t, tmp.SetBinary(10, 12, false))
	if tmp.Get() != r {
		t.Fatal("reuse changed handle")
	}
	if mask, _ := tmp.owner.Approx(r); mask != before {
		t.Fatal("reuse refreshed mask")
	}
	if v, _ := tmp.owner.Removed(r); !v {
		t.Fatal("reuse reset flags")
	}
	if v, _ := tmp.owner.Glue(r); v != 9 {
		t.Fatal(v)
	}
	base := b.Used()
	b.FailAfter(0)
	if e = tmp.Set(satLs(0, 2, 4, 6), false); !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal(e)
	}
	if tmp.Get() != (StoredClauseRef{}) || tmp.owner.IsLive(r) || b.Used() >= base {
		t.Fatal("old must be destroyed", tmp.Get(), b.Used(), base)
	}
	b.FailAfter(-1)
	satOK(t, tmp.SetBinClause(newSATBinClause(2, 4, true)))
	tmp.Close()
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
}
func TestSATClauseSourceIDsLocatorsAndDeleteFailure(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	a := satStore(t, b)
	r := satMk(t, a, satLs(0, 2), false)
	off, e := a.Offset(r)
	satOK(t, e)
	back, e := a.GetClause(off)
	if back != r || e != nil {
		t.Fatal(back, e)
	}
	b.FailAfter(0)
	satOK(t, a.Delete(r))
	b.FailAfter(-1)
	next := satMk(t, a, satLs(2, 4), true)
	if id, _ := a.ID(next); id != 0 {
		t.Fatal(id)
	}
	if next == r || a.IsLive(r) {
		t.Fatal("generation")
	}
	if _, e = a.GetClause(off); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("reused clause ID revived a stale offset", e)
	}
	if e = a.Delete(r); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("reused clause ID let a stale reference delete the new clause", e)
	}
	if !a.IsLive(next) {
		t.Fatal("stale deletion changed the new clause")
	}
	foreign := satStore(t, nil)
	if _, e = foreign.GetClause(off); !errors.Is(e, ErrSATOwner) {
		t.Fatal(e)
	}
	a.Close()
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
	if _, e = a.GetClause(off); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}
func TestSATClauseWideExtentAndBudgetRefusal(t *testing.T) {
	for _, n := range []uint32{0, 1, (math.MaxUint32 - 20) / 4, (math.MaxUint32-20)/4 + 1, math.MaxUint32} {
		bytes, e := satClauseObjectBytes(n)
		if bytes != 20+uint64(n)*4 || e != nil {
			t.Fatal(n, bytes, e)
		}
	}
	if bytes, _ := satClauseObjectBytes(math.MaxUint32); bytes != 17179869200 {
		t.Fatal(bytes)
	}
	if satClauseWideExtent(4294967295) || !satClauseWideExtent(4294967296) {
		t.Fatal("exact extent branch")
	}
	if e := satClauseHostExtent(17179869200, math.MaxInt32); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	b := arena.NewBudget(1 << 20)
	a := satStore(t, b)
	base := b.Used()
	if _, e := a.MkClauseN(math.MaxUint32, nil, false); !errors.Is(e, arena.ErrBudget) || b.Used() != base || a.ids.Range() != 0 {
		t.Fatal("wide refusal before ID/copy", e, b.Used(), base)
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(StoredClauseRef{}), reflect.TypeOf(satClauseOffset{}), reflect.TypeOf(satClauseHotHeader{})} {
		satOK(t, arena.ValidateRecordType(typ))
	}
}

func TestSATClauseDirectArenaBoundary(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	a, err := NewSATClauseAllocator(satClauseAllocatorConfig{
		Storage:        arena.SmallBytesConfig{Descriptors: arena.Config{Budget: budget}},
		PooledOverride: true,
	})
	satOK(t, err)
	t.Cleanup(func() { satOK(t, a.Close()) })
	// The 20-byte clause header places these widths on opposite sides of 248 bytes.
	var literals [57]Literal
	for i := range literals {
		literals[i] = Literal(2 * i)
	}
	pooled := satMk(t, a, literals[:56], false)
	direct := satMk(t, a, literals[:], false)
	if pooled.storageKind != 0 || direct.storageKind != 1 || a.storage.Stats().Live != 1 {
		t.Fatal("clauses were routed to the wrong stores", pooled, direct)
	}
	offset, err := a.Offset(direct)
	satOK(t, err)
	satOK(t, a.Delete(direct))
	if a.largeBytes != 0 || !a.IsLive(pooled) {
		t.Fatal("direct release did not return its payload budget")
	}
	reused := satMk(t, a, literals[:], false)
	if reused.slot != direct.slot || reused.generation <= direct.generation {
		t.Fatal("direct storage did not advance the recycled slot", direct, reused)
	}
	if _, err := a.GetClause(offset); err != arena.ErrInvalidSlot {
		t.Fatal("recycled direct clause revived a stale watch offset", err)
	}
	satOK(t, a.Close())
	if budget.Used() != 0 {
		t.Fatal("closing the clause allocator left a budget charge", budget.Used())
	}
}
func TestSATClauseElimUnsignedWrapBoundary(t *testing.T) {
	for _, c := range []struct {
		size, index uint32
		bad         bool
	}{{0, 0, false}, {^uint32(0), ^uint32(0), true}, {^uint32(0), ^uint32(0) - 1, false}, {5, 5, false}} {
		e := satClauseElimCopyStart(c.size, c.index)
		if errors.Is(e, arena.ErrBounds) != c.bad {
			t.Fatal(c, e)
		}
	}
}
func TestSATClauseReleaseRetainedLiteralAccess(t *testing.T) {
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 2, 4), false)
	satOK(t, a.Shrink(r, 2))
	l, e := a.Literal(r, 2)
	if satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal(l, e)
		}
	} else {
		if l != 4 || e != nil {
			t.Fatal("release body reads still allocated literal", l, e)
		}
		satOK(t, a.SetLiteral(r, 2, 8))
		l, e = a.Literal(r, 2)
		if l != 8 || e != nil {
			t.Fatal(l, e)
		}
	}
	if _, e = a.Literal(r, 3); satStorageSourceDebug {
		if !errors.Is(e, ErrSATAssertion) {
			t.Fatal(e)
		}
	} else if !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
}
func TestSATClauseCheckedIterationAndApproxCount(t *testing.T) {
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 3, 64), false)
	it, e := a.LiteralIter(r)
	satOK(t, e)
	for _, want := range []Literal{0, 3, 64} {
		got, ok, e := it.Next()
		if got != want || !ok || e != nil {
			t.Fatal(got, ok, e)
		}
	}
	if _, ok, e := it.Next(); ok || e != nil {
		t.Fatal(ok, e)
	}
	m, e := SATApproxLiteralsN(3, satLs(0, 3, 64))
	if m != 3 || e != nil {
		t.Fatal(m, e)
	}
	if _, e = SATApproxLiteralsN(2, satLs(0)); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	it, e = a.LiteralIter(r)
	satOK(t, e)
	satOK(t, a.Delete(r))
	if _, _, e = it.Next(); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal(e)
	}
}
