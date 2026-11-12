package protomodel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func datalogFactoryFixture(t *testing.T, size uint64) (*ast.ASTManager, *DatalogFactory, ast.ASTHandle) {
	t.Helper()
	m, _ := protoFixture(t)
	sort, err := m.MkDatalogFiniteSort(protoSymbol(t, m, "factory-domain"), size)
	protoMust(t, err)
	protoPin(t, m, sort)
	f, err := NewDatalogFactory(m)
	protoMust(t, err)
	t.Cleanup(func() { protoMust(t, f.Close()) })
	return m, f, sort
}

func datalogFactoryNumber(t *testing.T, m *ast.ASTManager, value ast.ASTHandle, want uint64) {
	t.Helper()
	got, matched, err := m.DatalogNumeralValue(value)
	protoMust(t, err)
	if !matched || got != want {
		t.Fatal("factory numeral", got, want)
	}
}

func TestDatalogFactorySourceSomeValuesRegisteredBranches(t *testing.T) {
	for _, registered := range [][]uint64{nil, {0}, {1}, {4}, {4, 3}, {1, 0}} {
		t.Run(fmt.Sprint(registered), func(t *testing.T) {
			m, f, sort := datalogFactoryFixture(t, 8)
			var old []ast.ASTHandle
			for _, n := range registered {
				v, err := m.MkDatalogNumeral(n, sort)
				protoMust(t, err)
				protoMust(t, f.RegisterValue(v))
				old = append(old, v)
			}
			a, b, ok, err := protoTestSomeValues(t, m, f, sort)
			protoMust(t, err)
			if !ok || a == b {
				t.Fatal("source some-values requires two distinct values", a, b, ok)
			}
			switch len(old) {
			case 0:
				datalogFactoryNumber(t, m, a, 0)
				datalogFactoryNumber(t, m, b, 1)
			case 1:
				if a != old[0] {
					t.Fatal("single registered value lost")
				}
				want := uint64(0)
				if registered[0] == 0 {
					want = 1
				}
				datalogFactoryNumber(t, m, b, want)
			default:
				// Source obj_hashtable iteration order; it must reuse existing
				// registered values instead of manufacturing numeral one.
				keys := f.sets[sort].table.FirstKeys(2)
				if a != keys[0] || b != keys[1] || (a != old[0] && a != old[1]) || (b != old[0] && b != old[1]) {
					t.Fatal("registered pair was replaced", a, b, old)
				}
			}
			before := len(f.values)
			a2, b2, ok, err := protoTestSomeValues(t, m, f, sort)
			protoMust(t, err)
			keys := f.sets[sort].table.FirstKeys(2)
			if !ok || a2 != keys[0] || b2 != keys[1] || len(f.values) != before {
				t.Fatal("warm pair selection changed or retained duplicates")
			}
		})
	}
}

func TestDatalogFactorySourceSingletonAndExhaustionPrefixes(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(fmt.Sprint(warm), func(t *testing.T) {
			m, f, sort := datalogFactoryFixture(t, 1)
			if warm {
				_, err := f.SomeValue(sort)
				protoMust(t, err)
			}
			first, second, ok, err := protoTestSomeValues(t, m, f, sort)
			if !errors.Is(err, ast.ErrASTDomain) || ok || (!warm && !second.IsNull()) || (warm && second != first) {
				t.Fatal("source attempts out-of-range numeral one", first, second, ok, err)
			}
			datalogFactoryNumber(t, m, first, 0)
			if len(f.values) != 1 {
				t.Fatal("failed pair lost source first-value prefix")
			}
		})
	}
	m, f, sort := datalogFactoryFixture(t, 2)
	for n := uint64(0); n < 2; n++ {
		v, err := f.FreshValue(sort)
		protoMust(t, err)
		datalogFactoryNumber(t, m, v, n)
	}
	// Exhaustion attempts mk_value(next) before checking the source bound.
	// Its exception leaves next unchanged, so retries encounter the same value.
	for retry := 0; retry < 2; retry++ {
		v, err := f.FreshValue(sort)
		if !errors.Is(err, ast.ErrASTDomain) || v != (ast.ASTHandle{}) || f.sets[sort].next != 2 || len(f.values) != 2 {
			t.Fatal("source exhausted-prefix changed", v, err, f.sets[sort].next, len(f.values))
		}
	}
}

func TestDatalogFactorySourceFreshSkipsRegisteredValues(t *testing.T) {
	m, f, sort := datalogFactoryFixture(t, 8)
	for _, n := range []uint64{0, 1, 4} {
		value, err := m.MkDatalogNumeral(n, sort)
		protoMust(t, err)
		protoMust(t, f.RegisterValue(value))
	}
	for i, n := range []uint64{2, 3, 5} {
		value, err := f.FreshValue(sort)
		protoMust(t, err)
		datalogFactoryNumber(t, m, value, n)
		if f.sets[sort].next != uint32(n+1) {
			t.Fatal("collision scan cursor", i, f.sets[sort].next, n)
		}
	}
}

func TestDatalogFactorySourceRegistrationOwnershipAndCursor(t *testing.T) {
	m, f, sort := datalogFactoryFixture(t, (uint64(1)<<32)+7)
	symbolic, err := m.MkConst(protoSymbol(t, m, "registered-symbolic-value"), sort)
	protoMust(t, err)
	protoPin(t, m, symbolic)
	before, err := m.GetRefCount(symbolic)
	protoMust(t, err)
	protoMust(t, f.RegisterValue(symbolic))
	protoMust(t, f.RegisterValue(symbolic))
	count, err := m.GetRefCount(symbolic)
	protoMust(t, err)
	if count != before+1 {
		t.Fatal("registered value must own exactly one reference", count, before)
	}
	v, err := f.SomeValue(sort)
	protoMust(t, err)
	if v != symbolic {
		t.Fatal("register_value must accept and reuse the supplied expression")
	}
	high, err := m.MkDatalogNumeral((uint64(1)<<32)+3, sort)
	protoMust(t, err)
	protoPin(t, m, high)
	protoMust(t, f.RegisterValue(high))
	// Registering an arbitrary or high value does not change the unsigned
	// enumeration cursor. This assignment also checks its concrete source type.
	var cursor uint32 = f.sets[sort].next
	if cursor != 0 {
		t.Fatal("registration changed next", cursor)
	}
	fresh, err := f.FreshValue(sort)
	protoMust(t, err)
	datalogFactoryNumber(t, m, fresh, 0)
	protoMust(t, f.Close())
	protoMust(t, f.Close())
	count, err = m.GetRefCount(symbolic)
	protoMust(t, err)
	if count != before {
		t.Fatal("factory close lost independent owner or leaked its reference", count, before)
	}
	if _, err = f.SomeValue(sort); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed factory value", err)
	}
	if _, _, _, err = protoTestSomeValues(t, m, f, sort); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed factory pair", err)
	}
	if _, err = f.FreshValue(sort); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed factory fresh", err)
	}
	if err = f.RegisterValue(symbolic); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed factory registration", err)
	}
}

func TestDatalogFactorySourceFreshAllocationRefusalRetry(t *testing.T) {
	failures := 0
	for step := int64(0); step < 8; step++ {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			m, f, sort := datalogFactoryFixture(t, 4)
			symbolic, err := m.MkConst(protoSymbol(t, m, "registered-before-refusal"), sort)
			protoMust(t, err)
			protoMust(t, f.RegisterValue(symbolic))
			// Fill the actual reference descriptor storage to its first reserve.
			var padding []ast.ASTRef
			defer func() {
				m.Budget().FailAfter(-1)
				for i := range padding {
					protoMust(t, m.DelRef(&padding[i]))
				}
			}()
			m.Budget().FailAfter(0)
			cold := false
			for i := 0; i < 8192; i++ {
				r, e := m.NewRef(symbolic)
				if e != nil {
					cold = errors.Is(e, arena.ErrInjectedFailure)
					break
				}
				padding = append(padding, r)
			}
			m.Budget().FailAfter(-1)
			if !cold {
				t.Fatal("no actual allocation frontier reached")
			}
			m.Budget().FailAfter(step)
			value, err := f.FreshValue(sort)
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				if !errors.Is(err, arena.ErrInjectedFailure) || f.sets[sort].next != 0 || len(f.values) != 1 {
					t.Fatal("failed mk_value advanced cursor or published value", err, f.sets[sort].next, len(f.values))
				}
				value, err = f.FreshValue(sort)
			}
			protoMust(t, err)
			datalogFactoryNumber(t, m, value, 0)
			if f.sets[sort].next != 1 {
				t.Fatal("successful fresh value did not advance once")
			}
		})
	}
	if failures == 0 {
		t.Fatal("allocation campaign did not encounter a refusal")
	}
}

func TestDatalogFactorySourceAssertionOwnerAndWarmSelection(t *testing.T) {
	m, f, sort := datalogFactoryFixture(t, 8)
	_, _, _, err := protoTestSomeValues(t, m, f, sort)
	protoMust(t, err)
	b := arena.NewBudget(1 << 20)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	protoMust(t, err)
	d, err := diagnostic.NewDebugContext(region, b)
	protoMust(t, err)
	previous, err := diagnostic.BindProcessDebugContext(d)
	protoMust(t, err)
	defer func() {
		_, err := diagnostic.BindProcessDebugContext(previous)
		protoMust(t, err)
		protoMust(t, d.Close())
		protoMust(t, region.Close())
		if b.Used() != 0 {
			t.Fatal("debug owner leak", b.Used())
		}
	}()
	for _, enabled := range []bool{false, true} {
		protoMust(t, d.EnableAssertions(enabled))
		a, b, ok, err := protoTestSomeValues(t, m, f, sort)
		protoMust(t, err)
		if !ok || a == b {
			t.Fatal("healthy assertion owner changed pair")
		}
	}
	protoMust(t, d.Close())
	// Source get_some_value's existing nonempty fast path has no assertion.
	v, err := f.SomeValue(sort)
	protoMust(t, err)
	if v != f.sets[sort].table.FirstKeys(1)[0] {
		t.Fatal("warm selection changed the stored expression")
	}
	a, bv, ok, err := protoTestSomeValues(t, m, f, sort)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) || ok || a == (ast.ASTHandle{}) || bv == (ast.ASTHandle{}) {
			t.Fatal("line171 must consult owner after pair publication", a, bv, ok, err)
		}
	} else if err != nil || !ok {
		t.Fatal("release pair evaluated erased assertion", err)
	}
	_, err = f.FreshValue(sort)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) || f.sets[sort].next != 0 {
			t.Fatal("line105 precedes fresh generation", err, f.sets[sort].next)
		}
	} else {
		protoMust(t, err)
	}
}

func TestDatalogFactorySourceProtoModelDispatch(t *testing.T) {
	m, p := protoFixture(t)
	sort, err := m.MkDatalogFiniteSort(protoSymbol(t, m, "model-dispatch-domain"), 8)
	protoMust(t, err)
	protoPin(t, m, sort)
	f, err := NewDatalogFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	one, err := m.MkDatalogNumeral(1, sort)
	protoMust(t, err)
	protoMust(t, p.RegisterValue(one))
	first, second, distinct, err := protoTestSomeValues(t, m, p, sort)
	protoMust(t, err)
	if !distinct || first != one || first == second {
		t.Fatal("prototype-model dispatch did not preserve registered pair", first, second, distinct)
	}
	datalogFactoryNumber(t, m, second, 0)
	fresh, err := p.FreshValue(sort)
	protoMust(t, err)
	datalogFactoryNumber(t, m, fresh, 2)
	// A caller-owned value survives the generator/prototype factory teardown.
	protoPin(t, m, fresh)
	protoMust(t, p.Close())
	datalogFactoryNumber(t, m, fresh, 2)
}
