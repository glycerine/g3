package numeral

// Source: src/util/scoped_numeral.h, src/util/scoped_numeral_vector.h.

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestScopedNumeralSourceTstScoped(t *testing.T) {
	m, budget := integerManager(t)
	baseline := m.Cells.Stats().Live
	a, e := NewScopedInt(m)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewScopedInt(m)
	if e != nil {
		t.Fatal(e)
	}
	x, _ := a.Get()
	y, _ := b.Get()
	if e = m.SetInt64(x, 1000231); e != nil {
		t.Fatal(e)
	}
	if e = m.SetString(y, "102928187172727273"); e != nil {
		t.Fatal(e)
	}
	first, e := a.Display()
	if e != nil {
		t.Fatal(e)
	}
	second, e := b.Display()
	if e != nil {
		t.Fatal(e)
	}
	if e = b.MulAssign(x); e != nil {
		t.Fatal(e)
	}
	third, e := b.Display()
	if e != nil {
		t.Fatal(e)
	}
	if "a: "+first+"\nb: "+second+"\nb: "+third+"\n" != "a: 1000231\nb: 102928187172727273\nb: 102951963583964173000063\n" {
		t.Fatal(first, second, third)
	}
	a.Close()
	b.Close()
	if m.Cells.Stats().Live != baseline {
		t.Fatal("source destructors did not balance")
	}
	m.Close()
	if budget.Used() != 0 {
		t.Fatal(budget.Used())
	}
}

func TestScopedNumeralResizeUsesGoSliceStorage(t *testing.T) {
	m, b := integerManager(t)
	base := b.Used()
	b.FailAfter(0)
	v, e := NewScopedIntVector(m)
	if e != nil {
		t.Fatal(e)
	}
	if e = v.Resize(5); e != nil || v.Size() != 5 || v.Capacity() < 5 {
		t.Fatal("native slice resize", e, v.Size(), v.Capacity())
	}
	if b.Used() != base {
		t.Fatal("Go wrapper or slice charged to numeric arena", b.Used(), base)
	}
	b.FailAfter(-1)
	if e = v.Close(); e != nil || b.Used() != base {
		t.Fatal("native slice close", e, b.Used(), base)
	}
}

func TestScopedNumeralMoveAndAscendingCleanup(t *testing.T) {
	m, _ := integerManager(t)
	v, _ := NewScopedIntVector(m)
	defer v.Close()
	var x Int
	m.SetUint64(&x, 1<<63)
	v.Push(&x)
	v.Push(&x)
	first := v.data[0].Cell
	second := v.data[1].Cell
	live := m.Cells.Stats().Live
	v.Push(&x)
	if v.data[0].Cell != first || v.data[1].Cell != second || m.Cells.Stats().Live != live+1 {
		t.Fatal("slice growth changed element ownership")
	}
	third := v.data[2].Cell
	v.Shrink(0)
	if m.Cells.Stats().Live != live-2 {
		t.Fatal("truncate immediate deletion")
	}
	// Source deletes ascending index. The allocator's LIFO recycle exposes the
	// last-deleted cell first; each freed payload was released exactly once.
	var a, b, c Int
	m.SetUint64(&a, 1<<63)
	m.SetUint64(&b, 1<<63)
	m.SetUint64(&c, 1<<63)
	if NumericSlot(a.Cell) != NumericSlot(third) || NumericSlot(b.Cell) != NumericSlot(second) || NumericSlot(c.Cell) != NumericSlot(first) {
		t.Fatal("ascending vector deletion order")
	}
	m.Del(&a)
	m.Del(&b)
	m.Del(&c)
	m.Del(&x)
}

func TestScopedNumeralScalarFailureAndAllThinOperations(t *testing.T) {
	for _, synch := range []bool{false, true} {
		m, b := rationalManager(t, synch)
		s, _ := NewScopedRational(m)
		defer s.Close()
		s.SetInt64(-17)
		three := NewRational()
		m.SetInt64(&three, 3)
		for _, f := range []func(*Rational) error{s.AddAssign, s.SubAssign, s.MulAssign, s.DivAssign, s.RemAssign} {
			if e := f(&three); e != nil {
				t.Fatal(e)
			}
		}
		if text, _ := s.Display(); text != "-2" {
			t.Fatal(text)
		} // exact rational / leaves -17, remainder -2
		s.SetInt64(3)
		for _, item := range []struct {
			f    func(*Rational) (bool, error)
			want bool
		}{{s.Equal, true}, {s.NotEqual, false}, {s.Less, false}, {s.Greater, false}, {s.LessEqual, true}, {s.GreaterEqual, true}} {
			v, e := item.f(&three)
			if e != nil || v != item.want {
				t.Fatal(v, e)
			}
		}
		for _, item := range []struct {
			f    func() (bool, error)
			want bool
		}{{s.IsZero, false}, {s.IsPos, true}, {s.IsNeg, false}, {s.IsNonpos, false}, {s.IsNonneg, true}} {
			v, e := item.f()
			if e != nil || v != item.want {
				t.Fatal(v, e)
			}
		}
		for _, f := range []func(*Rational) (*ScopedRational, error){s.Add, s.Sub, s.Mul, s.Div} {
			r, e := f(&three)
			if e != nil {
				t.Fatal(e)
			}
			r.Close()
		}
		abs, e := s.AbsCopy()
		if e != nil {
			t.Fatal(e)
		}
		abs.Neg()
		abs.Assign(s)
		abs.Swap(s)
		abs.SwapValue(&three)
		abs.Close()
		m.Del(&three)
		// Fill a genuinely cold allocation point rather than assuming a warmed
		// copy reserves. Header construction alone gives a deterministic boundary.
		live := m.Z.Cells.Stats().Live
		used := b.Used()
		b.FailAfter(0)
		if r, e := s.Clone(); r != nil || !errors.Is(e, arena.ErrInjectedFailure) || m.Z.Cells.Stats().Live != live || b.Used() != used {
			t.Fatal("clone header failure", r, e)
		}
		b.FailAfter(-1)
		fraction := NewRational()
		m.SetFraction(&fraction, 1, 2)
		if modularSourceDebug {
			bindMPNDebugActions(t, "T")
		}
		e = s.RemAssign(&fraction)
		divPublicationAudit(t, m, &s.num, &fraction)
		if modularSourceDebug {
			if !isMPNVerificationThrow(e) {
				t.Fatal(e)
			}
			if text, _ := s.Display(); text != "3" {
				t.Fatal("source debug leaves scope unchanged", text)
			}
		} else {
			qopsMust(t, e)
			if text, _ := s.Display(); text != "0" {
				t.Fatal("release rem3 by raw numerator1", text)
			}
		}
		m.Del(&fraction)
		s.Close()
	}
}

func TestScopedNumeralWarmReuseAndLifetimeGuards(t *testing.T) {
	m, b := integerManager(t)
	v, _ := NewScopedIntVector(m)
	v.Resize(20)
	v.Reset()
	x := NewInt(7)
	s, _ := NewScopedInt(m)
	s.Set(&x)
	b.FailAfter(0)
	if n := testing.AllocsPerRun(100, func() {
		if e := v.Push(&x); e != nil {
			panic(e)
		}
		if e := v.Set(0, &x); e != nil {
			panic(e)
		}
		var out Int
		if e := v.Read(0, &out); e != nil {
			panic(e)
		}
		if e := v.Pop(); e != nil {
			panic(e)
		}
		if e := s.AddAssign(&x); e != nil {
			panic(e)
		}
		if e := s.Reset(); e != nil {
			panic(e)
		}
	}); n != 0 {
		t.Fatal("warm allocation", n)
	}
	b.FailAfter(-1)
	copy := *v
	if e := copy.Close(); !errors.Is(e, ErrScopedCopy) {
		t.Fatal(e)
	}
	if e := v.Pop(); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	if e := v.Shrink(1); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	var out Int
	if e := v.Read(1, &out); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	m.Close()
	if _, e := s.Get(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
	if e := s.Close(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("dead borrowed owner guard", e)
	}
	if e := v.Close(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatal("release independent cold storage after invalid manager lifetime", b.Used())
	}
	s.Close()
	v.Close()
}
func TestScopedNumeralScalarOwnershipAndOperators(t *testing.T) {
	m, _ := integerManager(t)
	a, e := NewScopedInt(m)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	a.SetInt64(-19)
	c, e := a.Clone()
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	v := NewInt(3)
	c.AddAssign(&v)
	c.SubAssign(&v)
	c.MulAssign(&v)
	c.DivAssign(&v)
	c.RemAssign(&v)
	if s, _ := c.Display(); s != "-1" {
		t.Fatal(s)
	}
	c.Assign(a)
	c.Neg()
	p, e := c.IsPos()
	if e != nil || !p {
		t.Fatal(p, e)
	}
	abs, e := a.AbsCopy()
	if e != nil {
		t.Fatal(e)
	}
	defer abs.Close()
	if s, _ := abs.Display(); s != "19" {
		t.Fatal(s)
	}
	y, e := a.Add(&v)
	if e != nil {
		t.Fatal(e)
	}
	defer y.Close()
	if s, _ := y.Display(); s != "-16" {
		t.Fatal(s)
	}
	if s, _ := a.Display(); s != "-19" {
		t.Fatal("arithmetic copy changed input")
	}
	c.Swap(a)
	if s, _ := a.Display(); s != "19" {
		t.Fatal(s)
	}
	a.SwapValue(&v)
	if s, _ := a.Display(); s != "3" || v.Val != 19 {
		t.Fatal(s, v)
	}
	a.Reset()
	if z, e := a.IsZero(); e != nil || !z {
		t.Fatal(z, e)
	}
	shallow := *a
	if _, e := shallow.Get(); !errors.Is(e, ErrScopedCopy) {
		t.Fatal("shallow owner copy", e)
	}
	other, _ := integerManager(t)
	foreign, _ := NewScopedInt(other)
	defer foreign.Close()
	if e := a.Assign(foreign); !errors.Is(e, ErrScopedOwner) {
		t.Fatal(e)
	}
	if arena.ValidateRecordType(reflect.TypeOf(Int{})) != nil || arena.ValidateRecordType(reflect.TypeOf(Rational{})) != nil {
		t.Fatal("hot numeral records must be pointer-free")
	}
}
func TestScopedNumeralVectorIndependentLifecycle(t *testing.T) {
	m, _ := integerManager(t)
	baseline := m.Cells.Stats().Live
	v, e := NewScopedIntVector(m)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Close()
	rng := rand.New(rand.NewSource(1709))
	var model []int32
	for step := 0; step < 1000; step++ {
		switch rng.Intn(5) {
		case 0:
			x := NewInt(int32(rng.Intn(100) - 50))
			if e := v.Push(&x); e != nil {
				t.Fatal(e)
			}
			model = append(model, x.Val)
		case 1:
			if len(model) > 0 {
				if e := v.Pop(); e != nil {
					t.Fatal(e)
				}
				model = model[:len(model)-1]
			}
		case 2:
			n := uint32(rng.Intn(20))
			if e := v.Resize(n); e != nil {
				t.Fatal(e)
			}
			if int(n) <= len(model) {
				model = model[:n]
			} else {
				model = append(model, make([]int32, int(n)-len(model))...)
			}
		case 3:
			v.Reset()
			model = nil
		case 4:
			if len(model) > 0 {
				i := rng.Intn(len(model))
				x := NewInt(int32(rng.Intn(100)))
				v.Set(uint32(i), &x)
				model[i] = x.Val
			}
		}
		if v.Size() != uint32(len(model)) || v.Capacity() < v.Size() {
			t.Fatal("slice size/capacity", step, v.Size(), v.Capacity(), len(model))
		}
		for i, w := range model {
			var out Int
			if e := v.Read(uint32(i), &out); e != nil || out.Val != w {
				t.Fatal(step, i, out, e)
			}
			m.Del(&out)
		}
	}
	v.Reset()
	if m.Cells.Stats().Live != baseline {
		t.Fatal("vector leak")
	}
}
func TestScopedRationalVectorAndScalarLifetimes(t *testing.T) {
	for _, synch := range []bool{false, true} {
		m, _ := rationalManager(t, synch)
		base := m.Z.Cells.Stats().Live
		a, e := NewScopedRational(m)
		if e != nil {
			t.Fatal(e)
		}
		q, _ := a.Get()
		if e = m.SetFraction(q, -9223372036854775807, 18446744073709551615); e != nil {
			t.Fatal(e)
		}
		copy, e := a.Clone()
		if e != nil {
			t.Fatal(e)
		}
		v, e := NewScopedRationalVector(m)
		if e != nil {
			t.Fatal(e)
		}
		v.Push(q)
		v.Push(q)
		if m.Z.Cells.Stats().Live <= base {
			t.Fatal("real rational payloads absent")
		}
		v.Resize(5)
		for i := uint32(2); i < 5; i++ {
			out := NewRational()
			v.Read(i, &out)
			if s := rationalString(t, m, &out); s != "0" {
				t.Fatal("source numeral(0) default", s)
			}
			m.Del(&out)
		}
		copy.Reset()
		out := NewRational()
		v.Read(0, &out)
		if s := rationalString(t, m, &out); s != "-9223372036854775807/18446744073709551615" {
			t.Fatal("deep push", s)
		}
		m.Del(&out)
		v.Shrink(1)
		v.Pop()
		v.Reset()
		a.Close()
		copy.Close()
		v.Close()
		if m.Z.Cells.Stats().Live != base {
			t.Fatal("rational destructor balance", synch, m.Z.Cells.Stats().Live, base)
		}
	}
}
func TestScopedNumeralHeadersFailuresAndSourcePushPrefix(t *testing.T) {
	// This action-timing fixture uses the actual direct source profile so the
	// copied big numeral requires physical preparation despite warmed pool slots.
	m, b := mpzPublicationDirectOwner(t)
	base := b.Used()
	b.FailAfter(0)
	if s, e := NewScopedInt(m); s != nil || !errors.Is(e, arena.ErrInjectedFailure) || b.Used() != base {
		t.Fatal(s, e)
	}
	b.FailAfter(-1)
	v, e := NewScopedIntVector(m)
	if e != nil {
		t.Fatal(e)
	}
	v.Resize(2)
	v.Shrink(0)
	var big Int
	m.SetString(&big, "340282366920938463463374607431768211455")
	// Parsing can warm recyclable cells. Occupy every slot before injection so
	// the new deep-copy cell must actually reserve a page, rather than guessing
	// that the next allocation is cold.
	var fillers []Int
	for m.Cells.Stats().Live+m.Cells.Stats().Retired < m.Cells.Stats().Slots {
		var filler Int
		if e := m.SetUint64(&filler, 1<<63); e != nil {
			t.Fatal(e)
		}
		fillers = append(fillers, filler)
	}
	roots := []*Int{&big}
	for i := range fillers {
		roots = append(roots, &fillers[i])
	}
	live := m.Cells.Stats().Live
	usedBefore := b.Used()
	b.FailAfter(0)
	if e = v.Push(&big); !errors.Is(e, arena.ErrInjectedFailure) || v.Size() != 1 || v.Capacity() < 1 {
		t.Fatal("source default appended before copy", e, v.Size())
	}
	integerPhysicalAudit(t, m, roots...)
	if m.Cells.Stats().Live != live {
		t.Fatal("failed push leaked a temporary cell")
	}
	if b.Used() != usedBefore {
		t.Fatal("failed push leaked live payload or arena charge")
	}
	b.FailAfter(-1)
	var out Int
	v.Read(0, &out)
	if out.Val != 0 || out.Cell != 0 {
		t.Fatal("failed push not default")
	}
	v.Close()
	m.Del(&big)
	for i := range fillers {
		m.Del(&fillers[i])
	}
	m.Close()
	if b.Used() != 0 {
		t.Fatal("owner cleanup")
	}
}

func TestScopedRationalVectorIndependentLifecycleAndFailures(t *testing.T) {
	for _, synch := range []bool{false, true} {
		m, b := rationalManager(t, synch)
		small := NewRational()
		big := NewRational()
		m.SetFraction(&small, -2, 3)
		m.SetFraction(&big, -9223372036854775807, 18446744073709551615)
		// Warm caller-output formatting workspace before fixed live-graph oracle.
		rationalString(t, m, &big)
		baseCells := m.Z.Cells.Stats().Live
		v, e := NewScopedRationalVector(m)
		if e != nil {
			t.Fatal(e)
		}
		rng := rand.New(rand.NewSource(933))
		var model []uint8
		for step := 0; step < 400; step++ {
			switch rng.Intn(5) {
			case 0:
				code := uint8(rng.Intn(2) + 1)
				num := &small
				if code == 2 {
					num = &big
				}
				if e := v.Push(num); e != nil {
					t.Fatal(e)
				}
				model = append(model, code)
			case 1:
				if len(model) > 0 {
					v.Pop()
					model = model[:len(model)-1]
				}
			case 2:
				n := uint32(rng.Intn(16))
				if e := v.Resize(n); e != nil {
					t.Fatal(e)
				}
				if int(n) <= len(model) {
					model = model[:n]
				} else {
					model = append(model, make([]uint8, int(n)-len(model))...)
				}
			case 3:
				v.Reset()
				model = nil
			case 4:
				if len(model) > 0 {
					i := rng.Intn(len(model))
					v.Set(uint32(i), &big)
					model[i] = 2
				}
			}
			if v.Size() != uint32(len(model)) || v.Capacity() < v.Size() {
				t.Fatal("rational vector size/capacity")
			}
			bigCount := uint32(0)
			for i, code := range model {
				want := "0"
				if code == 1 {
					want = "-2/3"
				}
				if code == 2 {
					want = "-9223372036854775807/18446744073709551615"
					bigCount++
				}
				out := NewRational()
				if e := v.Read(uint32(i), &out); e != nil {
					t.Fatal(e)
				}
				if text := rationalString(t, m, &out); text != want {
					t.Fatal(step, i, text, want)
				}
				m.Del(&out)
			}
			if m.Z.Cells.Stats().Live != baseCells+bigCount*2 {
				t.Fatal("immediate rational cell/retained formatter lifecycle oracle", step, m.Z.Cells.Stats().Live, baseCells, bigCount)
			}
		}
		v.Reset()
		v.Resize(2)
		v.Shrink(0)
		// Occupy existing free cell slots so the deep copy is genuinely cold.
		var fillers []Int
		for m.Z.Cells.Stats().Live+m.Z.Cells.Stats().Retired < m.Z.Cells.Stats().Slots {
			var x Int
			m.Z.SetUint64(&x, 1<<63)
			fillers = append(fillers, x)
		}
		live := m.Z.Cells.Stats().Live
		b.FailAfter(0)
		if e := v.Push(&big); !numericPhysicalReservationRefusal(e) || v.Size() != 1 {
			t.Fatal("rational source append prefix", e)
		}
		roots := []*Int{&small.Num, &small.Den, &big.Num, &big.Den, &v.data[0].Num, &v.data[0].Den}
		for i := range fillers {
			roots = append(roots, &fillers[i])
		}
		qopsAuditLiveOwnership(t, m, roots)
		setterPairPrefix(t, m, &v.data[0], "0", "1", roundPublicationDecode(t, m.Z, &big.Num), roundPublicationDecode(t, m.Z, &big.Den), true)
		published := uint32(0)
		for _, n := range []*Int{&v.data[0].Num, &v.data[0].Den} {
			if n.Cell != 0 && !m.Z.IsFailureRef(n.Cell) {
				published++
			}
		}
		if m.Z.Cells.Stats().Live != live+published {
			t.Fatal("actual source copy descriptors", m.Z.Cells.Stats().Live, live, published)
		}
		// The numeric arena accounts for owned numerals and manager state. The Go
		// wrapper and its typed slice are ordinary Go storage.
		if b.Used() != m.HeaderBytes+integerActualOwnedBytes(m.Z) {
			t.Fatal("actual rational vector physical graph", b.Used(), integerActualOwnedBytes(m.Z), m.HeaderBytes)
		}
		b.FailAfter(-1)
		qopsMust(t, m.Z.CleanupPendingAllocation()) // explicit unassigned-object cleanup, after immediate source audit
		for i := range fillers {
			m.Z.Del(&fillers[i])
		}
		beforeClose := b.Used()
		integerBefore := integerActualOwnedBytes(m.Z)
		if e := v.Close(); e != nil {
			t.Fatal("actual failed-component vector Close", e)
		}
		if beforeClose-b.Used() != integerBefore-integerActualOwnedBytes(m.Z) {
			t.Fatal("vector numeral refund", beforeClose, b.Used())
		}
		if m.Z.Cells.Stats().Live != baseCells {
			t.Fatal("rational close live graph")
		}
		m.Del(&small)
		m.Del(&big)
	}
}

func TestScopedNumeralDeepCloneFailureCleanup(t *testing.T) {
	failed, passed, retained, partial := 0, 0, 0, 0
	for prefix := int64(0); prefix < 24; prefix++ {
		m, b := rationalManager(t, true)
		s, _ := NewScopedRational(m)
		num, _ := s.Get()
		// An armed (never reached) injection keeps GCD on the Lehmer path,
		// whose cell layout this sweep was calibrated on.
		b.FailAfter(1 << 40)
		m.SetFraction(num, -9223372036854775807, 18446744073709551615)
		b.FailAfter(-1)
		var fillers []Int
		for m.Z.Cells.Stats().Live+m.Z.Cells.Stats().Retired < m.Z.Cells.Stats().Slots {
			var x Int
			if e := m.Z.SetUint64(&x, 1<<63); e != nil {
				t.Fatal(e)
			}
			fillers = append(fillers, x)
		}
		cells := m.Z.Cells.Stats().Live
		b.FailAfter(prefix)
		copy, e := s.Clone()
		var pending *ScopedRationalCopyCleanupError
		errors.As(e, &pending)
		roots := []*Int{&num.Num, &num.Den}
		for i := range fillers {
			roots = append(roots, &fillers[i])
		}
		if copy != nil {
			roots = append(roots, &copy.num.Num, &copy.num.Den)
		}
		if pending != nil {
			retained++
			if pending.owner.num.Num.Cell != 0 && !m.Z.IsFailureRef(pending.owner.num.Num.Cell) {
				partial++
			}
			roots = append(roots, &pending.owner.num.Num, &pending.owner.num.Den)
		}
		qopsAuditLiveOwnership(t, m, roots)
		ownerCharge := uint64(reflect.TypeOf(ScopedRational{}).Size())
		if copy != nil || pending != nil {
			ownerCharge *= 2
		}
		if b.Used() != m.HeaderBytes+integerActualOwnedBytes(m.Z)+ownerCharge {
			t.Fatal("actual clone owners and physical roots", prefix, b.Used(), ownerCharge)
		}
		if e != nil {
			failed++
			if !numericPhysicalReservationRefusal(e) || copy != nil {
				t.Fatal("clone constructor refusal", prefix, e)
			}
			b.FailAfter(-1)
			if pending != nil {
				qopsMust(t, pending.Cleanup())
				if pending.owner != nil {
					t.Fatal("clone construction owner not drained", prefix)
				}
			}
		} else {
			passed++
			b.FailAfter(-1)
			if e := copy.Close(); e != nil {
				t.Fatal(e)
			}
			if m.Z.Cells.Stats().Live != cells {
				t.Fatal("clone close immediate graph")
			}
		}
		b.FailAfter(-1)
		qopsMust(t, m.Z.CleanupPendingAllocation()) // explicit release after pre-cleanup audit
		if m.Z.Cells.Stats().Live != cells {
			t.Fatal("clone owning cleanup did not restore live roots", prefix)
		}
		for i := range fillers {
			m.Z.Del(&fillers[i])
		}
		s.Close()
		m.Close()
		if b.Used() != 0 {
			t.Fatal("clone header unwind", prefix, b.Used())
		}
	}
	if failed == 0 || passed == 0 || retained == 0 || partial == 0 {
		t.Fatal("clone sweep endpoints and retained partial copy", failed, passed, retained, partial)
	}
}
