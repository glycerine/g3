package protomodel

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func scalarFactoryDebugActions(t *testing.T, commands string) (*diagnostic.DebugContext, string) {
	t.Helper()
	b := arena.NewBudget(1 << 20)
	region, err := arena.NewBytesRegion(arena.BytesRegionConfig{DirectOnly: true, Descriptors: arena.Config{Budget: b}})
	protoMust(t, err)
	d, err := diagnostic.NewDebugContext(region, b)
	protoMust(t, err)
	inputFile, err := os.CreateTemp(t.TempDir(), "factory-actions-*")
	protoMust(t, err)
	_, err = inputFile.WriteString(commands)
	protoMust(t, err)
	_, err = inputFile.Seek(0, 0)
	protoMust(t, err)
	report, err := os.CreateTemp(t.TempDir(), "factory-report-*")
	protoMust(t, err)
	streams, err := diagnostic.NewMemoryStandardStreams(diagnostic.MemoryStandardStreamsConfig{Budget: b, Stdout: report, Stderr: report})
	protoMust(t, err)
	input, err := diagnostic.NewDebuggerCharInput(streams, inputFile)
	protoMust(t, err)
	actions, err := diagnostic.NewDebugActionContext(diagnostic.DebugActionConfig{Budget: b, Streams: streams, Input: input})
	protoMust(t, err)
	protoMust(t, d.BindAssertionStreams(streams))
	protoMust(t, d.BindAssertionActions(actions))
	previous, err := diagnostic.BindProcessDebugContext(d)
	protoMust(t, err)
	t.Cleanup(func() {
		_, err := diagnostic.BindProcessDebugContext(previous)
		protoMust(t, err)
		protoMust(t, actions.Close())
		protoMust(t, input.Close())
		protoMust(t, streams.Close())
		protoMust(t, inputFile.Close())
		protoMust(t, report.Close())
		protoMust(t, d.Close())
		protoMust(t, region.Close())
		if b.Used() != 0 {
			t.Fatal("factory debug owner leak", b.Used())
		}
	})
	return d, report.Name()
}

func TestScalarFactorySourceUserRegisteredValuesAndFreeze(t *testing.T) {
	m, p := protoFixture(t)
	sort, err := m.MkUninterpretedSort(protoSymbol(t, m, "source-user-sort"), nil)
	protoMust(t, err)
	protoPin(t, m, sort)
	one, err := m.MkModelValue(1, sort)
	protoMust(t, err)
	protoMust(t, p.RegisterValue(one))
	first, second, ok, err := protoTestSomeValues(t, m, p, sort)
	protoMust(t, err)
	zero, err := m.MkModelValue(0, sort)
	protoMust(t, err)
	if !ok || first != one || second != zero {
		t.Fatal("source one-value branch", first, second, ok)
	}
	var cursor uint32 = p.user.sets[sort].next
	if cursor != 0 {
		t.Fatal("pair creation changed unsigned cursor", cursor)
	}
	fresh, err := p.FreshValue(sort)
	protoMust(t, err)
	two, err := m.MkModelValue(2, sort)
	protoMust(t, err)
	if fresh != two || p.user.sets[sort].next != 3 {
		t.Fatal("unsigned collision scan", fresh, two, p.user.sets[sort].next)
	}
	protoMust(t, p.FreezeUniverse(sort))
	protoMust(t, p.FreezeUniverse(sort))
	size := len(p.user.KnownUniverse(sort))
	a, b, ok, err := protoTestSomeValues(t, m, p, sort)
	protoMust(t, err)
	if !ok || a != one || b != zero {
		t.Fatal("frozen pair selection", a, b, ok)
	}
	v, err := p.FreshValue(sort)
	protoMust(t, err)
	if !v.IsNull() || len(p.user.KnownUniverse(sort)) != size || len(p.user.Sorts()) != 1 {
		t.Fatal("frozen universe changed")
	}
	independent, err := m.NewRef(two)
	protoMust(t, err)
	defer func() { protoMust(t, m.DelRef(&independent)) }()
	count, err := m.GetRefCount(two)
	protoMust(t, err)
	protoMust(t, p.Close())
	after, err := m.GetRefCount(two)
	protoMust(t, err)
	if after+1 != count {
		t.Fatal("user owner did not release exactly one value reference", after, count)
	}
}

func TestScalarFactorySourceFrozenRegistrationActions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, action := range []string{"T", "C"} {
			t.Run(fmt.Sprintf("%t/%s", enabled, action), func(t *testing.T) {
				m, p := protoFixture(t)
				sort, err := m.MkUninterpretedSort(protoSymbol(t, m, "frozen-action-sort"), nil)
				protoMust(t, err)
				protoPin(t, m, sort)
				value, err := m.MkModelValue(2, sort)
				protoMust(t, err)
				protoPin(t, m, value)
				protoMust(t, p.FreezeUniverse(sort))
				d, path := scalarFactoryDebugActions(t, action)
				protoMust(t, d.EnableAssertions(enabled))
				err = p.RegisterValue(value)
				throws := diagnostic.DebugBuild && enabled && action == "T"
				if throws {
					var ex z3exception.DefaultException
					if !errors.As(err, &ex) || ex.Message() != "assertion violation" || len(p.user.KnownUniverse(sort)) != 1 {
						t.Fatal("source frozen-registration throw prefix", err)
					}
				} else {
					protoMust(t, err)
					if len(p.user.KnownUniverse(sort)) != 2 {
						t.Fatal("continued registration was suppressed")
					}
				}
				data, err := os.ReadFile(path)
				protoMust(t, err)
				want := 0
				if diagnostic.DebugBuild && enabled {
					want = 1
				}
				if got := strings.Count(string(data), "File: src/smt/proto_model/value_factory.cpp\nLine: 111\n!is_finite(m_manager.get_sort(n))\n"); got != want {
					t.Fatal("actual assertion diagnostic", got, want, string(data))
				}
			})
		}
	}
}

func TestScalarFactorySourceFrozenPathsAndClosedAssertionOwner(t *testing.T) {
	m, p := protoFixture(t)
	sort, err := m.MkUninterpretedSort(protoSymbol(t, m, "closed-frozen-sort"), nil)
	protoMust(t, err)
	protoPin(t, m, sort)
	value, err := p.SomeValue(sort)
	protoMust(t, err)
	protoMust(t, p.FreezeUniverse(sort))
	d, _ := scalarFactoryDebugActions(t, "T")
	protoMust(t, d.Close())
	a, b, ok, err := protoTestSomeValues(t, m, p, sort)
	protoMust(t, err)
	if ok || !a.IsNull() || !b.IsNull() {
		t.Fatal("frozen singleton produced pair")
	}
	fresh, err := p.FreshValue(sort)
	protoMust(t, err)
	if !fresh.IsNull() {
		t.Fatal("frozen fresh path ignored finite guard")
	}
	v, err := p.SomeValue(sort)
	if diagnostic.DebugBuild {
		if !errors.Is(err, arena.ErrClosed) {
			t.Fatal("finite SomeValue must consult source assertions", err)
		}
	} else if err != nil || v != value {
		t.Fatal("release finite value", v, err)
	}
}

func TestScalarFactorySourceNumeralExplicitValuesAndEnumeration(t *testing.T) {
	m, p := protoFixture(t)
	arith, err := NewArithmeticFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(arith))
	v, err := arith.MkNumValue(big.NewRat(3, 2), false)
	protoMust(t, err)
	sort, err := m.GetSort(v)
	protoMust(t, err)
	before, err := m.GetRefCount(v)
	protoMust(t, err)
	same, err := arith.MkNumValue(big.NewRat(3, 2), false)
	protoMust(t, err)
	after, err := m.GetRefCount(v)
	protoMust(t, err)
	if same != v || after != before {
		t.Fatal("explicit numeral acquired duplicate reference")
	}
	a, b, ok, err := protoTestSomeValues(t, m, p, sort)
	protoMust(t, err)
	if !ok || a != v || a == b {
		t.Fatal("registered fractional pair")
	}
	fresh, err := p.FreshValue(sort)
	protoMust(t, err)
	text, integer, matched, err := m.ArithmeticNumeralText(fresh)
	protoMust(t, err)
	if !matched || integer || text != "1" || arith.sets[sort].next.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("concrete rational cursor", text, arith.sets[sort].next.String())
	}
	bv, err := NewBitVectorFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(bv))
	high, err := bv.MkNumValue(big.NewRat(-1, 1), 8)
	protoMust(t, err)
	bs, err := m.GetSort(high)
	protoMust(t, err)
	a, b, ok, err = protoTestSomeValues(t, m, p, bs)
	protoMust(t, err)
	if !ok || a != high || a == b {
		t.Fatal("registered normalized BV pair")
	}
	fresh, err = p.FreshValue(bs)
	protoMust(t, err)
	text, matched, err = m.BitVectorNumeralText(fresh)
	protoMust(t, err)
	if !matched || text != "1" {
		t.Fatal("rational BV enumeration", text)
	}
}

func TestScalarFactorySourceBVExhaustionAndCursor(t *testing.T) {
	m, p := protoFixture(t)
	f, err := NewBitVectorFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(f))
	sort, err := m.BitVectorSort(1)
	protoMust(t, err)
	protoPin(t, m, sort)
	a, b, ok, err := protoTestSomeValues(t, m, p, sort)
	protoMust(t, err)
	if !ok || a == b {
		t.Fatal("one-bit pair")
	}
	fresh, err := p.FreshValue(sort)
	protoMust(t, err)
	if !fresh.IsNull() || f.sets[sort].next.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("source finite collision limit", fresh, f.sets[sort].next.String())
	}
	// Each retry starts a new relative scan instead of treating next as an
	// absolute domain bound. One-bit numeral construction wraps each candidate.
	fresh, err = p.FreshValue(sort)
	protoMust(t, err)
	if !fresh.IsNull() || f.sets[sort].next.Cmp(big.NewRat(6, 1)) != 0 {
		t.Fatal("source relative scan limit", fresh, f.sets[sort].next.String())
	}
}

func TestScalarFactorySourceAllocationFailureCursorRetry(t *testing.T) {
	for _, kind := range []string{"user", "arith", "bv"} {
		t.Run(kind, func(t *testing.T) {
			failures := 0
			for step := int64(0); step < 8; step++ {
				m, p := protoFixture(t)
				var sort ast.ASTHandle
				var factory ValueFactory
				switch kind {
				case "user":
					sort, _ = m.MkUninterpretedSort(protoSymbol(t, m, "failure-user"), nil)
					factory = p.user
				case "arith":
					f, err := NewArithmeticFactory(m)
					protoMust(t, err)
					protoMust(t, p.RegisterFactory(f))
					sort, err = m.MkFamilySort(f.FamilyID(), 0, nil)
					protoMust(t, err)
					factory = f
				case "bv":
					f, err := NewBitVectorFactory(m)
					protoMust(t, err)
					protoMust(t, p.RegisterFactory(f))
					sort, err = m.BitVectorSort(8)
					protoMust(t, err)
					factory = f
				}
				protoPin(t, m, sort)
				symbolic, err := m.MkConst(protoSymbol(t, m, "failure-registered"), sort)
				protoMust(t, err)
				protoMust(t, factory.RegisterValue(symbolic))
				var padding []ast.ASTRef
				m.Budget().FailAfter(0)
				cold := false
				for i := 0; i < 8192; i++ {
					ref, e := m.NewRef(symbolic)
					if e != nil {
						cold = errors.Is(e, arena.ErrInjectedFailure)
						break
					}
					padding = append(padding, ref)
				}
				m.Budget().FailAfter(-1)
				if !cold {
					t.Fatal("no actual reference reservation frontier")
				}
				m.Budget().FailAfter(step)
				value, err := factory.FreshValue(sort)
				m.Budget().FailAfter(-1)
				if err != nil {
					failures++
					if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal("unexpected allocation error", err)
					}
					switch f := factory.(type) {
					case *userSortFactory:
						if f.sets[sort].next != 0 {
							t.Fatal("failed unsigned constructor advanced cursor")
						}
					case *ArithmeticFactory:
						if f.sets[sort].next.Sign() != 0 {
							t.Fatal("failed rational constructor advanced cursor")
						}
					case *BitVectorFactory:
						if f.sets[sort].next.Sign() != 0 {
							t.Fatal("failed rational BV constructor advanced cursor")
						}
					}
					value, err = factory.FreshValue(sort)
				}
				protoMust(t, err)
				switch kind {
				case "user":
					zero, e := m.MkModelValue(0, sort)
					protoMust(t, e)
					if value != zero {
						t.Fatal("unsigned retry skipped failed value")
					}
				case "arith":
					text, _, _, e := m.ArithmeticNumeralText(value)
					protoMust(t, e)
					if text != "0" {
						t.Fatal("rational retry skipped failed value", text)
					}
				case "bv":
					text, _, e := m.BitVectorNumeralText(value)
					protoMust(t, e)
					if text != "0" {
						t.Fatal("BV retry skipped failed value", text)
					}
				}
				for i := range padding {
					protoMust(t, m.DelRef(&padding[i]))
				}
			}
			if failures == 0 {
				t.Fatal("no allocation refusal encountered")
			}
		})
	}
}

func TestScalarFactorySourceBasicRejectsOtherSortFamilies(t *testing.T) {
	m, _ := protoFixture(t)
	arith, err := NewArithmeticFactory(m)
	protoMust(t, err)
	defer func() { protoMust(t, arith.Close()) }()
	real, err := m.MkFamilySort(arith.FamilyID(), 0, nil)
	protoMust(t, err)
	protoPin(t, m, real)
	f := newBasicFactory(m)
	value, err := f.SomeValue(real)
	protoMust(t, err)
	if !value.IsNull() {
		t.Fatal("nonbasic kind-zero sort was treated as Bool", value)
	}
	_, _, ok, err := protoTestSomeValues(t, m, f, real)
	protoMust(t, err)
	if ok {
		t.Fatal("non-Bool basic pair")
	}
}

func TestScalarFactorySourceWarmSelectionAndClosedAssertionOwner(t *testing.T) {
	for _, kind := range []string{"user", "arith", "bv"} {
		t.Run(kind, func(t *testing.T) {
			m, p := protoFixture(t)
			var sort ast.ASTHandle
			switch kind {
			case "user":
				var err error
				sort, err = m.MkUninterpretedSort(protoSymbol(t, m, "warm-user"), nil)
				protoMust(t, err)
			case "arith":
				f, err := NewArithmeticFactory(m)
				protoMust(t, err)
				protoMust(t, p.RegisterFactory(f))
				sort, err = m.MkFamilySort(f.FamilyID(), 0, nil)
				protoMust(t, err)
			case "bv":
				f, err := NewBitVectorFactory(m)
				protoMust(t, err)
				protoMust(t, p.RegisterFactory(f))
				sort, err = m.BitVectorSort(4)
				protoMust(t, err)
			}
			protoPin(t, m, sort)
			first, second, ok, err := protoTestSomeValues(t, m, p, sort)
			protoMust(t, err)
			if !ok || first == second {
				t.Fatal("warm pair setup")
			}
			hold, err := m.NewRef(first)
			protoMust(t, err)
			defer func() { protoMust(t, m.DelRef(&hold)) }()
			// Warm selection reads the value set in its obj_hashtable
			// order, which need not be the order the pair was made in.
			warm, err := p.SomeValue(sort)
			protoMust(t, err)
			warmFirst, warmSecond, ok, err := protoTestSomeValues(t, m, p, sort)
			protoMust(t, err)
			if !ok || warmFirst != warm || (warmFirst != first && warmFirst != second) || (warmSecond != first && warmSecond != second) {
				t.Fatal("warm pair selection")
			}
			live := diagnostic.BorrowProcessDebugContext()
			d, _ := scalarFactoryDebugActions(t, "T")
			protoMust(t, d.Close())
			value, err := p.SomeValue(sort)
			protoMust(t, err)
			if value != warm {
				t.Fatal("assertion-free warm selection changed value")
			}
			a, b, ok, err := protoTestSomeValues(t, m, p, sort)
			if diagnostic.DebugBuild {
				if !errors.Is(err, arena.ErrClosed) || ok || a != warmFirst || b != warmSecond {
					t.Fatal("warm pair line171 prefix", a, b, ok, err)
				}
			} else if err != nil || !ok {
				t.Fatal("release warm pair consulted assertion owner", err)
			}
			// Restore before tearing down numeric owners; the helper cleanup
			// independently restores this same borrowed pointer afterward.
			_, err = diagnostic.BindProcessDebugContext(live)
			protoMust(t, err)
			before, err := m.GetRefCount(first)
			protoMust(t, err)
			protoMust(t, p.Close())
			after, err := m.GetRefCount(first)
			protoMust(t, err)
			if after+1 != before {
				t.Fatal("factory value reference cleanup", before, after)
			}
		})
	}
}

func scalarFactoryReferenceFrontier(t *testing.T, m *ast.ASTManager) []ast.ASTRef {
	t.Helper()
	var occupied []ast.ASTRef
	m.Budget().FailAfter(0)
	for len(occupied) < 4096 {
		ref, err := m.NewRef(ast.ASTHandle{})
		if errors.Is(err, arena.ErrInjectedFailure) {
			return occupied
		}
		protoMust(t, err)
		occupied = append(occupied, ref)
	}
	t.Fatal("reference page frontier not reached")
	return nil
}

func scalarFactoryReleaseFrontier(t *testing.T, m *ast.ASTManager, occupied []ast.ASTRef) {
	t.Helper()
	m.Budget().FailAfter(-1)
	for i := range occupied {
		protoMust(t, m.DelRef(&occupied[i]))
	}
}

func TestSimpleFactoriesPublishSetBeforeSortReference(t *testing.T) {
	t.Run("user", func(t *testing.T) {
		m, _ := protoFixture(t)
		f := newUserSortFactory(m)
		t.Cleanup(func() { protoMust(t, f.Close()) })
		sort, err := m.MkUninterpretedSort(protoSymbol(t, m, "published-user-sort"), nil)
		protoMust(t, err)
		protoPin(t, m, sort)
		occupied := scalarFactoryReferenceFrontier(t, m)
		t.Cleanup(func() { scalarFactoryReleaseFrontier(t, m, occupied) })
		set, err := f.valueSet(sort)
		if !errors.Is(err, arena.ErrInjectedFailure) || set != nil || f.sets[sort] == nil || len(f.sorts) != 0 {
			t.Fatal("user-sort publication prefix", set, err, len(f.sorts))
		}
		published := f.sets[sort]
		m.Budget().FailAfter(-1)
		set, err = f.valueSet(sort)
		protoMust(t, err)
		if set != published || len(f.sorts) != 0 {
			t.Fatal("user-sort warm lookup retried sort acquisition")
		}
	})

	t.Run("arithmetic", func(t *testing.T) {
		m, _ := protoFixture(t)
		f, err := NewArithmeticFactory(m)
		protoMust(t, err)
		t.Cleanup(func() { protoMust(t, f.Close()) })
		sort, err := m.MkFamilySort(f.FamilyID(), 0, nil)
		protoMust(t, err)
		protoPin(t, m, sort)
		occupied := scalarFactoryReferenceFrontier(t, m)
		t.Cleanup(func() { scalarFactoryReleaseFrontier(t, m, occupied) })
		set, err := f.valueSet(sort)
		if !errors.Is(err, arena.ErrInjectedFailure) || set != nil || f.sets[sort] == nil || len(f.sorts) != 0 {
			t.Fatal("arithmetic publication prefix", set, err, len(f.sorts))
		}
		published := f.sets[sort]
		m.Budget().FailAfter(-1)
		set, err = f.valueSet(sort)
		protoMust(t, err)
		if set != published || len(f.sorts) != 0 {
			t.Fatal("arithmetic warm lookup retried sort acquisition")
		}
	})

	t.Run("bit-vector", func(t *testing.T) {
		m, _ := protoFixture(t)
		f, err := NewBitVectorFactory(m)
		protoMust(t, err)
		t.Cleanup(func() { protoMust(t, f.Close()) })
		sort, err := m.BitVectorSort(8)
		protoMust(t, err)
		protoPin(t, m, sort)
		occupied := scalarFactoryReferenceFrontier(t, m)
		t.Cleanup(func() { scalarFactoryReleaseFrontier(t, m, occupied) })
		set, err := f.valueSet(sort)
		if !errors.Is(err, arena.ErrInjectedFailure) || set != nil || f.sets[sort] == nil || len(f.sorts) != 0 {
			t.Fatal("bit-vector publication prefix", set, err, len(f.sorts))
		}
		published := f.sets[sort]
		m.Budget().FailAfter(-1)
		set, err = f.valueSet(sort)
		protoMust(t, err)
		if set != published || len(f.sorts) != 0 {
			t.Fatal("bit-vector warm lookup retried sort acquisition")
		}
	})

	t.Run("datalog", func(t *testing.T) {
		m, _ := protoFixture(t)
		f, err := NewDatalogFactory(m)
		protoMust(t, err)
		t.Cleanup(func() { protoMust(t, f.Close()) })
		sort, err := m.MkDatalogFiniteSort(protoSymbol(t, m, "published-datalog-sort"), 8)
		protoMust(t, err)
		protoPin(t, m, sort)
		occupied := scalarFactoryReferenceFrontier(t, m)
		t.Cleanup(func() { scalarFactoryReleaseFrontier(t, m, occupied) })
		set, err := f.valueSet(sort)
		if !errors.Is(err, arena.ErrInjectedFailure) || set != nil || f.sets[sort] == nil || len(f.sorts) != 0 {
			t.Fatal("datalog publication prefix", set, err, len(f.sorts))
		}
		published := f.sets[sort]
		m.Budget().FailAfter(-1)
		set, err = f.valueSet(sort)
		protoMust(t, err)
		if set != published || len(f.sorts) != 0 {
			t.Fatal("datalog warm lookup retried sort acquisition")
		}
	})
}
