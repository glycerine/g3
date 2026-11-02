package polynomial

// Source: src/math/polynomial/polynomial.cpp:2807-2952; src/test/polynomial.cpp:1651-1672.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialNewtonSourceInterpolation(t *testing.T) {
	m := polynomialCellFixture(t)
	serviceMust(t, m.SetZPUint64(97))
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	defer func() { serviceMust(t, n.Close()) }()
	// Exact dormant source samples: (-9)y-21, (-3)y+20, 5y-36.
	cs := [3]int32{-9, -3, 5}
	ks := [3]int32{-21, 20, -36}
	for i := 0; i < 3; i++ {
		as := [1]unumeral.Int{unumeral.NewInt(cs[i])}
		c := unumeral.NewInt(ks[i])
		p, e := m.MkLinearTake(as[:], []uint32{1}, &c)
		serviceMust(t, e)
		polynomialCaller(t, m, p)
		a := unumeral.NewInt(int32(i))
		serviceMust(t, n.Add(&a, p))
	}
	if newtonCount(t, n) != 3 {
		t.Fatal(newtonCount(t, n))
	}
	r, e := NewPolynomialRef(m)
	serviceMust(t, e)
	defer func() { serviceMust(t, r.Close()) }()
	serviceMust(t, n.Mk(0, r))
	p := newtonRefGet(t, r)
	info, e := m.Info(p)
	serviceMust(t, e)
	// Independent coefficient map modulo97, centered: x²y+5xy+41x-9y-21.
	wants := map[[2]uint32]int64{{2, 1}: 1, {1, 1}: 5, {1, 0}: 41, {0, 1}: -9, {0, 0}: -21}
	if info.Size != uint32(len(wants)) {
		t.Fatal(info)
	}
	for i := uint32(0); i < info.Size; i++ {
		mm, e := m.TermMonomial(p, i)
		serviceMust(t, e)
		x, e := m.monomials.DegreeOf(mm, 0)
		serviceMust(t, e)
		y, e := m.monomials.DegreeOf(mm, 1)
		serviceMust(t, e)
		a, e := m.Coefficient(p, i)
		serviceMust(t, e)
		v, e := m.integer.Int64(&a)
		serviceMust(t, e)
		key := [2]uint32{x, y}
		want, ok := wants[key]
		if !ok || v != want {
			t.Fatal(key, v, want)
		}
		delete(wants, key)
	}
	serviceMust(t, n.Reset())
	if newtonCount(t, n) != 0 {
		t.Fatal("reset")
	}
}

func TestPolynomialNewtonReferenceCallbacksAndLazyVector(t *testing.T) {
	m := polynomialCellFixture(t)
	serviceMust(t, m.SetZPUint64(97))
	r, e := NewPolynomialRef(m)
	serviceMust(t, e)
	serviceMust(t, r.Assign(m.unit))
	info, e := m.Info(m.unit)
	serviceMust(t, e)
	if info.RefCount != 2 {
		t.Fatal(info)
	}
	serviceMust(t, r.Assign(m.unit))
	info, e = m.Info(m.unit)
	serviceMust(t, e)
	if info.RefCount != 2 {
		t.Fatal("self", info)
	}
	v, e := NewPolynomialNewtonVector(m)
	serviceMust(t, e)
	serviceMust(t, v.SetOwner(m))
	a, e := v.At(2)
	serviceMust(t, e)
	if newtonVectorCount(t, v) != 3 {
		t.Fatal(newtonVectorCount(t, v))
	}
	b, e := v.At(2)
	serviceMust(t, e)
	if a != b {
		t.Fatal("unstable child")
	}
	serviceMust(t, v.Flush())
	if newtonVectorCount(t, v) != 0 {
		t.Fatal("flush")
	}
	serviceMust(t, v.Close())
	serviceMust(t, r.Close())
}

func TestPolynomialNewtonWholeMethodAndWarmSingleSample(t *testing.T) {
	m := polynomialCellFixture(t)
	serviceMust(t, m.SetZPUint64(97))
	r, e := NewPolynomialRef(m)
	serviceMust(t, e)
	defer func() { serviceMust(t, r.Close()) }()
	serviceMust(t, m.NewtonInterpolation(0, 0, []unumeral.Int{unumeral.NewInt(7)}, []Polynomial{m.unit}, r))
	if m.newtonLocal != nil || newtonRefGet(t, r) != m.unit {
		t.Fatal("source local was retained on success")
	}
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	defer func() { serviceMust(t, n.Close()) }()
	input := unumeral.NewInt(7)
	serviceMust(t, n.Add(&input, m.unit))
	serviceMust(t, n.Mk(0, r))
	serviceMust(t, n.Reset())
	before := numericWarmProfileSnapshot(t, m.integer, m.integer.Budget())
	allocs := testing.AllocsPerRun(20, func() {
		if e := n.Add(&input, m.unit); e != nil {
			panic(e)
		}
		if e := n.Mk(0, r); e != nil {
			panic(e)
		}
		if e := n.Reset(); e != nil {
			panic(e)
		}
	})
	numericWarmProfileAudit(t, m.integer, m.integer.Budget(), before, allocs, false)
	polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, []*PolynomialRef{r}, nil, nil, nil, nil)
}

func TestPolynomialNewtonRealAddGrowthRefusalPrefixes(t *testing.T) {
	var failures int
	for fail := int64(0); fail < 256; fail++ {
		m := polynomialCellFixture(t)
		serviceMust(t, m.SetZPUint64(1000000007))
		n, e := NewPolynomialNewtonInterpolator(m)
		serviceMust(t, e)
		input := unumeral.NewInt(0)
		serviceMust(t, n.Add(&input, m.unit))
		input = unumeral.NewInt(2)
		m.integer.Budget().FailAfter(fail)
		e = n.Add(&input, m.unit)
		polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, nil, nil, nil, nil, nil, &input)
		m.integer.Budget().FailAfter(-1)
		if e != nil {
			if !numericPhysicalReservationRefusal(e) {
				t.Fatal(fail, e)
			}
			failures++
		}
		serviceMust(t, n.Close())
		serviceMust(t, m.CleanupConstruction())
		serviceMust(t, m.monomials.CleanupCreation())
		serviceMust(t, m.integer.CleanupPendingAllocation())
		if e == nil {
			t.Log("refusals", failures, "first success", fail)
			return
		}
	}
	t.Fatal("never completed")
}

func TestPolynomialNewtonRefVectorSourceConsumedPrefix(t *testing.T) {
	polynomialThrowAssertionFixture(t)
	m := polynomialCellFixture(t)
	a, e := m.MkVariable(0, 1)
	serviceMust(t, e)
	b, e := m.MkVariable(1, 1)
	serviceMust(t, e)
	v, e := NewPolynomialRefVector(m)
	serviceMust(t, e)
	serviceMust(t, v.Push(a))
	serviceMust(t, v.Push(b))
	serviceMust(t, m.writeWord(b, 0, 0)) // Explicit malformed RC fixture, source SASSERT refusal.
	e = v.Reset()
	if !polynomialSourceDebug {
		serviceMust(t, e)
		info, x := m.Info(b)
		serviceMust(t, x)
		if info.RefCount != ^uint32(0) {
			t.Fatal(info)
		}
		polynomialNewtonPhysicalAudit(t, m, nil, nil, []*PolynomialRefVector{v}, nil, []Polynomial{b}, nil)
		serviceMust(t, m.writeWord(b, 0, 1))
		serviceMust(t, m.DecRef(b))
		serviceMust(t, v.Close())
		return
	}
	if !errors.Is(e, ErrPolynomialAssertion) {
		t.Fatal(e)
	}
	if v.state.drain != 1 {
		t.Fatal("completed first DEC not recorded", v.state.drain)
	}
	if v.state.drainDec {
		t.Fatal("source DEBUG zero-RC refusal incorrectly acknowledged as consumed")
	}
	if _, e = m.Info(a); e != ErrPolynomialOwner {
		t.Fatal("first deletion was not complete", e)
	}
	info, e := m.Info(b)
	serviceMust(t, e)
	if info.RefCount != 0 {
		t.Fatal(info)
	}
	polynomialNewtonPhysicalAudit(t, m, nil, nil, []*PolynomialRefVector{v}, nil, nil, nil)
	serviceMust(t, m.writeWord(b, 0, 1)) // Restore only intentionally malformed counter for teardown.
	serviceMust(t, v.Close())
}

func TestPolynomialNewtonConstructionAndLazyRefusals(t *testing.T) {
	for route := 0; route < 2; route++ {
		failures := 0
		for ordinal := int64(0); ordinal < 64; ordinal++ {
			m := polynomialCellFixture(t)
			base := m.integer.Budget().Used()
			if route == 0 {
				runtimeBefore := m.memory.Stats()
				m.integer.Budget().FailAfter(ordinal)
				n, e := NewPolynomialNewtonInterpolator(m)
				polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, nil, nil, nil, nil, nil)
				m.integer.Budget().FailAfter(-1)
				if e != nil {
					if !numericPhysicalReservationRefusal(e) {
						t.Fatal(route, ordinal, e)
					}
					failures++
				}
				if n != nil {
					serviceMust(t, n.Close())
				}
				serviceMust(t, m.integer.CleanupPendingAllocation())
				// Actual Runtime block-directory capacity survives child destruction.
				// Only its measured charged metadata may remain; every source root
				// and physical byte must return to the prior independently named graph.
				runtimeAfter := m.memory.Stats()
				polynomialNewtonPhysicalAudit(t, m, nil, nil, nil, nil, nil, nil)
				if runtimeAfter.OwnerBytes != runtimeBefore.OwnerBytes || runtimeAfter.PhysicalBytes != runtimeBefore.PhysicalBytes || runtimeAfter.InitializationBytes != runtimeBefore.InitializationBytes || runtimeAfter.LiveBlocks != runtimeBefore.LiveBlocks || runtimeAfter.DirectoryBytes < runtimeBefore.DirectoryBytes || m.integer.Budget().Used() != base+runtimeAfter.DirectoryBytes-runtimeBefore.DirectoryBytes {
					t.Fatal("constructor refund", ordinal, base, m.integer.Budget().Used(), runtimeBefore, runtimeAfter)
				}
				if e == nil {
					if failures == 0 {
						t.Fatal("missing failure")
					}
					t.Log("constructor failures", failures, "success", ordinal)
					break
				}
			} else {
				v, e := NewPolynomialNewtonVector(m)
				serviceMust(t, e)
				serviceMust(t, v.SetOwner(m))
				before := m.integer.Budget().Used()
				runtimeBefore := m.memory.Stats()
				rawBefore, re := m.memory.AllocationSize()
				serviceMust(t, re)
				m.integer.Budget().FailAfter(ordinal)
				_, e = v.At(3)
				polynomialNewtonPhysicalAudit(t, m, nil, nil, nil, []*PolynomialNewtonVector{v}, nil, nil)
				m.integer.Budget().FailAfter(-1)
				if e != nil {
					if !numericPhysicalReservationRefusal(e) {
						t.Fatal(route, ordinal, e)
					}
					failures++
				}
				serviceMust(t, v.Close())
				serviceMust(t, m.integer.CleanupPendingAllocation())
				// Real Runtime block-directory growth survives child destruction; it
				// is charged owner metadata, not a retained Newton allocation.
				runtimeAfter := m.memory.Stats()
				rawAfter, re := m.memory.AllocationSize()
				serviceMust(t, re)
				polynomialNewtonPhysicalAudit(t, m, nil, nil, nil, nil, nil, nil)
				if runtimeAfter.OwnerBytes != runtimeBefore.OwnerBytes || runtimeAfter.PhysicalBytes != runtimeBefore.PhysicalBytes || runtimeAfter.InitializationBytes != runtimeBefore.InitializationBytes || runtimeAfter.LiveBlocks != runtimeBefore.LiveBlocks || runtimeAfter.DirectoryBytes < runtimeBefore.DirectoryBytes || m.integer.Budget().Used() != base+runtimeAfter.DirectoryBytes-runtimeBefore.DirectoryBytes {
					t.Fatal("lazy refund", ordinal, before, base, m.integer.Budget().Used(), runtimeBefore, runtimeAfter, rawBefore, rawAfter)
				}
				if e == nil {
					if failures == 0 {
						t.Fatal("missing failure")
					}
					t.Log("lazy failures", failures, "success", ordinal)
					break
				}
			}
			if ordinal == 63 {
				t.Fatal("never succeeded", route)
			}
		}
	}
}

func TestPolynomialNewtonSourceModularAndDuplicateProfiles(t *testing.T) {
	m := polynomialCellFixture(t)
	polynomialAssertionActionsFixture(t, "TT")
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	input := unumeral.NewInt(0)
	e = n.Add(&input, m.unit)
	if polynomialSourceDebug {
		if !errors.Is(e, ErrPolynomialAssertion) {
			t.Fatal(e)
		}
		if newtonCount(t, n) != 0 {
			t.Fatal("assertion published")
		}
	} else {
		serviceMust(t, e)
		if newtonCount(t, n) != 1 {
			t.Fatal("release body absent")
		}
	}
	serviceMust(t, n.Close())
	serviceMust(t, m.SetZPUint64(97))
	n, e = NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	serviceMust(t, n.Add(&input, m.unit))
	e = n.Add(&input, m.unit)
	if polynomialSourceDebug {
		if !errors.Is(e, ErrPolynomialAssertion) {
			t.Fatal(e)
		}
		if newtonCount(t, n) != 1 {
			t.Fatal("duplicate assertion published")
		}
	} else {
		serviceMust(t, e)
		if newtonCount(t, n) != 2 {
			t.Fatal("release inverse continuation absent")
		}
	}
	polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, nil, nil, nil, nil, nil, &input)
	serviceMust(t, n.Close())
}

func TestPolynomialNewtonLazyDirectoryPlateau(t *testing.T) {
	m := polynomialCellFixture(t)
	v, e := NewPolynomialNewtonVector(m)
	serviceMust(t, e)
	serviceMust(t, v.SetOwner(m))
	_, e = v.At(3)
	serviceMust(t, e)
	serviceMust(t, v.Flush())
	before := m.integer.Budget().Used()
	for i := 0; i < 4; i++ {
		_, e = v.At(3)
		serviceMust(t, e)
		serviceMust(t, v.Flush())
		if m.integer.Budget().Used() != before {
			t.Fatal("unused cold-directory slots grew", before, m.integer.Budget().Used())
		}
	}
	serviceMust(t, v.Close())
}

func newtonCount(t *testing.T, n *PolynomialNewtonInterpolator) uint32 {
	t.Helper()
	v, e := n.NumSamplePoints()
	serviceMust(t, e)
	return v
}
func newtonVectorCount(t *testing.T, n *PolynomialNewtonVector) uint32 {
	t.Helper()
	v, e := n.Size()
	serviceMust(t, e)
	return v
}
func newtonRefGet(t *testing.T, n *PolynomialRef) Polynomial {
	t.Helper()
	v, e := n.Get()
	serviceMust(t, e)
	return v
}

// Public TLS recipe reaches the guarded source signed-overflow domain. No
// private counters, callback errors, or allocation failures are fabricated.
func TestPolynomialNewtonStoppedReferenceBeforeOwningAbort(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	runtime, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, ThreadLocal: true})
	serviceMust(t, e)
	profile := umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: unumeral.IntegerConfig{Memory: runtime, Profile: profile, Cells: arena.Config{PageSize: 2, Budget: b}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: b}}}})
	serviceMust(t, e)
	mm, e := NewMonomialManager(MonomialManagerConfig{Budget: b, Memory: runtime, Profile: profile})
	serviceMust(t, e)
	serviceMust(t, mm.IncManagerRef())
	m, e := NewMultivariatePolynomialCore(MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Monomials: mm, Checkpoint: func() bool { t.Fatal("cleanup checkpoint"); return false }, Cooperation: func() error { t.Fatal("cleanup cooperation"); return nil }})
	serviceMust(t, e)
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	defer func() {
		contextTLSFinish(t, runtime, n.Close)
		contextTLSFinish(t, runtime, m.CloseChecked)
		before := runtime.Stats().LiveBlocks
		e := mm.DecManagerRef()
		if e != nil && (!errors.Is(e, arena.ErrBounds) || runtime.Stats().LiveBlocks >= before) {
			t.Fatal("source manager DEC", e)
		}
		contextTLSFinish(t, runtime, mm.CloseChecked)
		contextTLSFinish(t, runtime, q.CloseChecked)
		contextTLSFinish(t, runtime, runtime.Close)
	}()
	inputValue := unumeral.NewInt(7)
	serviceMust(t, n.pushNumeral(&n.inputs, &inputValue))
	// Distinct raw core fields give the actual old polynomial a >100000-byte
	// physical extent. Duplicate terms are deliberately unsimplified raw input.
	const size = 4096
	as := make([]unumeral.Int, size)
	ms := make([]Monomial, size)
	unit, e := mm.Unit()
	serviceMust(t, e)
	for i := range as {
		as[i] = unumeral.NewInt(1)
		ms[i] = unit
		serviceMust(t, mm.IncRef(unit))
	}
	old, e := m.MkCoreTake(as, ms)
	serviceMust(t, e)
	small := unumeral.NewInt(2)
	incoming, e := m.MkConstantTake(&small)
	serviceMust(t, e)
	other := unumeral.NewInt(3)
	older, e := m.MkConstantTake(&other)
	serviceMust(t, e)
	n.scope.kind = 1
	n.scope.refsConstructed = true
	serviceMust(t, n.scope.auxPoly.assign(m, old))
	serviceMust(t, n.scope.temp.assign(m, older))
	// Prepare source recycle storage before the undefined-accounting boundary.
	warms := [8]Polynomial{}
	for i := range warms {
		a := unumeral.NewInt(int32(4 + i))
		p, e := m.MkConstantTake(&a)
		serviceMust(t, e)
		serviceMust(t, m.IncRef(p))
		warms[i] = p
	}
	for _, p := range warms {
		serviceMust(t, m.DecRef(p))
	}
	for range warms {
		_ = m.ids.Mk()
	}
	mmwarm := [4]Monomial{}
	for i := range mmwarm {
		a, e := mm.MkMonomial([]MonomialPower{{uint32(i + 100), 1}})
		serviceMust(t, e)
		serviceMust(t, mm.IncRef(a))
		mmwarm[i] = a
	}
	for _, a := range mmwarm {
		serviceMust(t, mm.DecRef(a))
	}
	for range mmwarm {
		_ = mm.ids.Mk()
	}
	contextTLSNearMinimum(t, runtime)
	e = n.scope.auxPoly.assign(m, incoming)
	if !errors.Is(e, arena.ErrBounds) {
		t.Fatal("actual postphysical free", e)
	}
	e = n.finish(e)
	if !errors.Is(e, arena.ErrBounds) {
		t.Fatal("retained source cause", e)
	}
	// The completed outer interpolator may not owning-ack a stopped younger
	// source destructor. Explicit caller cleanup remains a later operation.
	readInput, readErr := n.Inputs().Get(0)
	if readErr != nil || readInput != inputValue {
		t.Fatal("stopped local poisoned source const inputs", readInput, readErr)
	}
	e = n.sourceClose()
	if !errors.Is(e, arena.ErrBounds) || n.closeStage != 0 {
		t.Fatal("source outer destructor advanced", e, n.closeStage)
	}
	if n.scope.auxPoly.value != old || n.scope.auxPoly.incoming != incoming || n.scope.temp.value != older {
		t.Fatal("owning acknowledgement/abort ran before stopped source unwind", n.scope)
	}
	loc, e := m.polynomialLocation(old)
	serviceMust(t, e)
	if !loc.released {
		t.Fatal("old raw free did not physically complete")
	}
	info, e := m.Info(incoming)
	serviceMust(t, e)
	if info.RefCount != 1 {
		t.Fatal("unjoined INC was consumed early", info)
	}
	info, e = m.Info(older)
	serviceMust(t, e)
	if info.RefCount != 1 {
		t.Fatal("older source destructor reached early", info)
	}
	polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, nil, nil, nil, nil, nil)
}

func TestPolynomialNewtonMkGrowthAndCancellationPrefixes(t *testing.T) {
	failures := 0
	for ordinal := int64(0); ordinal < 256; ordinal++ {
		m := polynomialCellFixture(t)
		serviceMust(t, m.SetZPUint64(97))
		n, e := NewPolynomialNewtonInterpolator(m)
		serviceMust(t, e)
		outputs := [3]Polynomial{}
		for i := range outputs {
			a := unumeral.NewInt(int32(i*i + 1))
			p, e := m.MkConstantTake(&a)
			serviceMust(t, e)
			outputs[i] = p
			serviceMust(t, m.IncRef(p))
			input := unumeral.NewInt(int32(i))
			serviceMust(t, n.Add(&input, p))
		}
		r, e := NewPolynomialRef(m)
		serviceMust(t, e)
		serviceMust(t, r.Assign(m.unit))
		m.integer.Budget().FailAfter(ordinal)
		e = n.Mk(0, r)
		polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, []*PolynomialRef{r}, nil, nil, outputs[:], nil)
		m.integer.Budget().FailAfter(-1)
		if e != nil {
			if !numericPhysicalReservationRefusal(e) {
				t.Fatal(ordinal, e)
			}
			failures++
			if r.root.value != m.unit {
				t.Fatal("output changed before final assignment", ordinal, r.root)
			}
		}
		serviceMust(t, r.Close())
		serviceMust(t, n.Close())
		serviceMust(t, m.CleanupConstruction())
		serviceMust(t, m.monomials.CleanupCreation())
		serviceMust(t, m.integer.CleanupPendingAllocation())
		for _, p := range outputs {
			serviceMust(t, m.DecRef(p))
		}
		if e == nil {
			if failures == 0 {
				t.Fatal("missing actual Mk failures")
			}
			t.Log("Mk failures", failures, "success", ordinal)
			break
		}
		if ordinal == 255 {
			t.Fatal("never complete")
		}
	}
	m := polynomialCellFixture(t)
	serviceMust(t, m.SetZPUint64(97))
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	outputs := [3]Polynomial{}
	for i := int32(0); i < 3; i++ {
		a := unumeral.NewInt(i*i + 1)
		p, e := m.MkConstantTake(&a)
		serviceMust(t, e)
		outputs[i] = p
		serviceMust(t, m.IncRef(p))
		input := unumeral.NewInt(i)
		serviceMust(t, n.Add(&input, p))
	}
	r, e := NewPolynomialRef(m)
	serviceMust(t, e)
	serviceMust(t, r.Assign(m.unit))
	var visits uint32
	m.config.Checkpoint = func() bool { visits++; return false }
	m.config.Cooperation = func() error { t.Fatal("cooperate after cancel"); return nil }
	e = n.Mk(0, r)
	if e != ErrUPolynomialCanceled || visits != 1 || r.root.value != m.unit {
		t.Fatal(e, visits, r.root)
	}
	polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, []*PolynomialRef{r}, nil, nil, outputs[:], nil)
	serviceMust(t, r.Close())
	serviceMust(t, n.Close())
	for _, p := range outputs {
		serviceMust(t, m.DecRef(p))
	}
}

func TestPolynomialNewtonNativeInputBounds(t *testing.T) {
	m := polynomialCellFixture(t)
	polynomialAssertionActionsFixture(t, "TTTTT")
	serviceMust(t, m.SetZPUint64(97))
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, n.Close()) })
	for _, i := range []uint32{0, 1, 0x80000000, ^uint32(0)} {
		_, e := n.Input(i)
		want := arena.ErrBounds
		if polynomialSourceDebug {
			want = ErrPolynomialAssertion
		}
		if !errors.Is(e, want) {
			t.Fatal(i, e, want)
		}
	}
	input := unumeral.NewInt(7)
	serviceMust(t, n.Add(&input, m.unit))
	serviceMust(t, n.Reset())
	if _, e := n.Input(0); e == nil {
		t.Fatal("native input exposed reset tail")
	}
}

func TestPolynomialNewtonVectorSourceOwnerBinding(t *testing.T) {
	m := polynomialCellFixture(t)
	polynomialThrowAssertionFixture(t)
	v, e := NewPolynomialNewtonVector(m)
	serviceMust(t, e)
	if v.owner != nil {
		t.Fatal("source default constructor bound owner")
	}
	_, e = v.At(0)
	if polynomialSourceDebug {
		if !errors.Is(e, ErrPolynomialAssertion) {
			t.Fatal(e)
		}
	} else {
		if e != ErrPolynomialOwner {
			t.Fatal(e)
		}
	}
	serviceMust(t, v.SetOwner(m))
	serviceMust(t, v.SetOwner(m))
	_, e = v.At(0)
	serviceMust(t, e)
	serviceMust(t, v.Close())
}

func TestPolynomialNewtonIndependentBoundedInterpolants(t *testing.T) {
	// Known polynomials supply independent sample values; expected coefficients
	// do not use the interpolation implementation or its inverse recurrence.
	coefficients := [5]int64{-5, 3, -2, 7, 1}
	for degree := 0; degree < 5; degree++ {
		m := polynomialCellFixture(t)
		serviceMust(t, m.SetZPUint64(97))
		inputs := make([]unumeral.Int, degree+1)
		outputs := make([]Polynomial, degree+1)
		for i := range inputs {
			x := int64(i - 2)
			v := int64(0)
			for k := degree; k >= 0; k-- {
				v = (v*x + coefficients[k]) % 97
			}
			if v > 48 {
				v -= 97
			}
			if v < -48 {
				v += 97
			}
			inputs[i] = unumeral.NewInt(int32(x))
			a := unumeral.NewInt(int32(v))
			p, e := m.MkConstantTake(&a)
			serviceMust(t, e)
			outputs[i] = p
			serviceMust(t, m.IncRef(p))
		}
		r, e := NewPolynomialRef(m)
		serviceMust(t, e)
		serviceMust(t, m.NewtonInterpolation(0, uint32(degree), inputs, outputs, r))
		p := newtonRefGet(t, r)
		info, e := m.Info(p)
		serviceMust(t, e)
		if info.Size != uint32(degree+1) {
			t.Fatal(degree, info)
		}
		seen := [5]bool{}
		for i := uint32(0); i < info.Size; i++ {
			term, e := m.TermMonomial(p, i)
			serviceMust(t, e)
			power, e := m.monomials.DegreeOf(term, 0)
			serviceMust(t, e)
			a, e := m.Coefficient(p, i)
			serviceMust(t, e)
			v, e := m.integer.Int64(&a)
			serviceMust(t, e)
			if power > uint32(degree) || seen[power] || v != coefficients[power] {
				t.Fatal(degree, power, v)
			}
			seen[power] = true
		}
		polynomialNewtonPhysicalAudit(t, m, nil, []*PolynomialRef{r}, nil, nil, outputs, nil)
		serviceMust(t, r.Close())
		for _, p := range outputs {
			serviceMust(t, m.DecRef(p))
		}
	}
}

func TestPolynomialNewtonRefVectorSourceResetStop(t *testing.T) {
	m := polynomialCellFixture(t)
	p, e := m.MkVariable(0, 1)
	serviceMust(t, e)
	v, e := NewPolynomialRefVector(m)
	serviceMust(t, e)
	serviceMust(t, v.Push(p))
	serviceMust(t, m.writeWord(p, 8, 2))
	e = v.Reset()
	if e != arena.ErrBounds {
		t.Fatal(e)
	}
	polynomialNewtonPhysicalAudit(t, m, nil, nil, []*PolynomialRefVector{v}, nil, nil, nil)
	serviceMust(t, m.writeWord(p, 8, 1))
	if second := v.Reset(); second != e {
		t.Fatal("source Reset owning-acknowledged stopped DEC", second, e)
	}
	serviceMust(t, v.Close())
}

// Native collection growth cannot consume Runtime reservations; numeric SET
// and source destruction still belong to the actual modular numeral owner.
func TestPolynomialNewtonNativeNumeralGrowthResetAndReuse(t *testing.T) {
	m := polynomialCellFixture(t)
	serviceMust(t, m.SetZPUint64(97))
	before := m.memory.Stats()
	used := m.integer.Budget().Used()
	m.integer.Budget().FailAfter(1) // Only the actual class header is charged.
	n, e := NewPolynomialNewtonInterpolator(m)
	serviceMust(t, e)
	t.Cleanup(func() { m.integer.Budget().FailAfter(-1); serviceMust(t, n.Close()) })
	if m.memory.Stats() != before || m.integer.Budget().Used() != used+n.header {
		t.Fatal("native zero inverse constructor charged backing")
	}
	m.integer.Budget().FailAfter(0)
	for pass := 0; pass < 2; pass++ {
		for i := int32(0); i < 4097; i++ {
			a := unumeral.NewInt(i)
			serviceMust(t, n.pushNumeral(&n.inputs, &a))
			serviceMust(t, n.pushNumeral(&n.invs, &a))
		}
		if newtonCount(t, n) != 4097 || len(n.invs) != 4098 {
			t.Fatal("native numeric association length")
		}
		for i := uint32(0); i < 4097; i++ {
			a, e := n.Input(i)
			serviceMust(t, e)
			value, e := m.integer.Int64(&a)
			serviceMust(t, e)
			inverse, e := m.integer.Int64(&n.invs[i+1])
			serviceMust(t, e)
			want := int64(i % 97)
			if want > 48 {
				want -= 97
			}
			if value != want || inverse != want {
				t.Fatal("native numeric association", i, value, inverse, want)
			}
		}
		polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, nil, nil, nil, nil, nil)
		serviceMust(t, n.Reset())
		if len(n.inputs) != 0 || len(n.invs) != 1 {
			t.Fatal("source reset must retain only inverse zero")
		}
		if m.memory.Stats() != before || m.integer.Budget().Used() != used+n.header {
			t.Fatal("native numeral growth/reset charged Runtime")
		}
	}
	serviceMust(t, n.Close())
	if n.inputs != nil || n.invs != nil || m.memory.Stats() != before || m.integer.Budget().Used() != used {
		t.Fatal("native numeral owning close retained storage")
	}
}

func TestPolynomialNewtonNativeInputActualNumericCopyRefusals(t *testing.T) {
	failures := 0
	for ordinal := int64(0); ordinal < 64; ordinal++ {
		m := polynomialCellFixture(t)
		serviceMust(t, m.SetZPUint64(97))
		n, e := NewPolynomialNewtonInterpolator(m)
		serviceMust(t, e)
		m.integer.Budget().FailAfter(ordinal)
		e = n.Add(&m.integer.Two64, m.unit)
		polynomialNewtonPhysicalAudit(t, m, []*PolynomialNewtonInterpolator{n}, nil, nil, nil, nil, nil)
		m.integer.Budget().FailAfter(-1)
		if e != nil {
			if !numericPhysicalReservationRefusal(e) {
				t.Fatal("actual input SET allocation", ordinal, e)
			}
			failures++
		} else {
			a, e := n.Input(0)
			serviceMust(t, e)
			value, e := m.integer.Int64(&a)
			serviceMust(t, e)
			if value != -36 { // Independently: 2^64 mod 97 = 61, centered to -36.
				t.Fatal("native input retained wrong numeral", value)
			}
		}
		serviceMust(t, n.Close())
		serviceMust(t, m.integer.CleanupPendingAllocation())
		polynomialNewtonPhysicalAudit(t, m, nil, nil, nil, nil, nil, nil)
		if e == nil {
			if failures == 0 {
				t.Fatal("no actual numeric allocation refusals")
			}
			return
		}
	}
	t.Fatal("no actual numeric copy completion")
}
