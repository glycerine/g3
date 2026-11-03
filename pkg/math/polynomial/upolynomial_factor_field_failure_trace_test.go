//go:build g3trace

package polynomial

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type factorFieldOutcomeFailure struct {
	t          *testing.T
	z          *unumeral.IntegerManager
	target     string
	seenTarget int
	held       []unumeral.Int
	heldCount  int
	armed      bool
	text       strings.Builder
}

func (w *factorFieldOutcomeFailure) Write(p []byte) (int, error) {
	if string(p) == w.target {
		w.seenTarget++
		if w.seenTarget == 2 {
			w.armed = true
			for {
				stats := w.z.Cells.Stats()
				if uint64(stats.Live)+uint64(stats.Retired) >= uint64(len(w.held)) {
					break
				}
				if e := w.z.Copy(&w.held[w.heldCount], &w.z.Two64); e != nil {
					w.t.Fatal("caller descriptor padding", e)
				}
				w.heldCount++
			}
		}
	}
	return w.text.Write(p)
}

func factorFieldOutcomeInput(t *testing.T, m *UPolynomialManager, z *unumeral.IntegerManager) UPolynomial {
	t.Helper()
	// Display checks the original coefficient's modular normalization before
	// copying it. Keep residue 2 in a real BIG cell so the later display Copy
	// still crosses the descriptor-allocation frontier without invalid input.
	big := unumeral.NewInt(2)
	serviceMust(t, z.EnsureOwnedCapacity(&big, 6))
	f := upolySourceNew(t, m, nil)
	serviceMust(t, m.SetInts(f, []unumeral.Int{big, unumeral.NewInt(0), unumeral.NewInt(1)}))
	serviceMust(t, z.Del(&big))
	serviceMust(t, m.SetZPUint64(5))
	coefficient, e := m.read(mustUPolyView(t, m, f), 0)
	serviceMust(t, e)
	normalized, e := m.modular.IsPNormalized(coefficient)
	serviceMust(t, e)
	value, e := z.Int64(coefficient)
	serviceMust(t, e)
	if coefficient.IsSmall() || !normalized || value != 2 {
		t.Fatal("finite-field display fixture lost normalized BIG coefficient", coefficient, normalized, value)
	}
	return f
}

func testFactorFieldOutcomeRetainsOlderSource(t *testing.T, whole bool) {
	q, s := preparationPhysicalFixtureWithSlots(t, 256)
	m, z := s.manager, q.Integers()
	f := factorFieldOutcomeInput(t, m, z)
	fs, e := m.NewFactors()
	serviceMust(t, e)
	target := "polynomial::square_free_factor("
	if whole {
		target = "polynomial::factor("
	}
	w := factorFieldOutcomeFailure{t: t, z: z, target: target, held: make([]unumeral.Int, 256)}
	tags := diagnostic.NewTraceTagsWithWriter(&w)
	tags.Enable("polynomial::factorization")
	serviceMust(t, m.modular.BindTrace(tags))
	defer func() {
		serviceMust(t, m.modular.BindTrace(nil))
		serviceMust(t, m.CleanupFactorFieldFactorization())
		for i := 0; i < w.heldCount; i++ {
			serviceMust(t, z.Del(&w.held[i]))
		}
		serviceMust(t, fs.Close())
	}()
	if whole {
		_, e = m.FactorZP(mustUPolyView(t, m, f), fs)
	} else {
		e = m.SquareFreeFactorZP(mustUPolyView(t, m, f), fs)
	}
	if !w.armed || !errors.Is(e, arena.ErrSlotLimit) || m.factorField == nil || m.factorField.executing || !m.displayReceiptPending(&m.factorField.polynomial) {
		t.Fatal("final finite-field TRACE did not retain exact child/source frame", whole, e, w.armed)
	}
	retained := m.factorField
	if whole {
		if retained.kind != factorFieldWhole || retained.sq == nil || retained.parent != nil {
			t.Fatal("outer factor frame/local factor collection", retained.kind, retained.sq != nil, retained.parent)
		}
	} else if retained.kind != factorFieldSquare || retained.constructed != 8 || retained.sourceStage != 0 {
		t.Fatal("square-free vector declaration frontier", retained.kind, retained.constructed, retained.sourceStage)
	}
	if e = m.SetZ(); !errors.Is(e, ErrUPolynomialCleanupPending) {
		t.Fatal("retained finite-field source allowed manager reuse", e)
	}
	if e = m.SourceFinalize(); !errors.Is(e, ErrUPolynomialCleanupPending) {
		t.Fatal("source finalization ignored retained finite-field display", e)
	}
	serviceMust(t, m.CleanupFactorFieldFactorization())
	if m.factorField != nil || m.displayReceiptPending(&retained.polynomial) || retained.active {
		t.Fatal("exact finite-field owning cleanup incomplete")
	}
}

func TestUPolynomialFiniteFieldSquareFreeOutcomeRetainsOlderSource(t *testing.T) {
	testFactorFieldOutcomeRetainsOlderSource(t, false)
}

func TestUPolynomialFiniteFieldFactorOutcomeRetainsLocalFactors(t *testing.T) {
	testFactorFieldOutcomeRetainsOlderSource(t, true)
}

func TestUPolynomialFiniteFieldFactorRetainsNestedSquareFreeSource(t *testing.T) {
	q, s := preparationPhysicalFixtureWithSlots(t, 256)
	m, z := s.manager, q.Integers()
	f := factorFieldOutcomeInput(t, m, z)
	fs, e := m.NewFactors()
	serviceMust(t, e)
	w := factorFieldOutcomeFailure{t: t, z: z, target: "polynomial::square_free_factor(", held: make([]unumeral.Int, 256)}
	tags := diagnostic.NewTraceTagsWithWriter(&w)
	tags.Enable("polynomial::factorization")
	serviceMust(t, m.modular.BindTrace(tags))
	defer func() {
		serviceMust(t, m.modular.BindTrace(nil))
		serviceMust(t, m.CleanupFactorFieldFactorization())
		for i := 0; i < w.heldCount; i++ {
			serviceMust(t, z.Del(&w.held[i]))
		}
		serviceMust(t, fs.Close())
	}()

	_, e = m.FactorZP(mustUPolyView(t, m, f), fs)
	inner := m.factorField
	if !w.armed || !errors.Is(e, arena.ErrSlotLimit) || inner == nil || inner.kind != factorFieldSquare || inner.parent == nil || inner.parent.kind != factorFieldWhole {
		t.Fatal("nested finite-field source stack", e, w.armed, inner)
	}
	outer := inner.parent
	if !m.displayReceiptPending(&inner.polynomial) || inner.constructed != 8 || inner.sourceStage != 0 || outer.sq == nil || outer.executing {
		t.Fatal("nested square-free/outer factor frontier", inner.constructed, inner.sourceStage, outer.sq != nil, outer.executing)
	}
	if e = m.SourceFinalize(); !errors.Is(e, ErrUPolynomialCleanupPending) {
		t.Fatal("source finalization ignored nested retained display", e)
	}
	serviceMust(t, m.CleanupFactorFieldFactorization())
	if m.factorField != nil || inner.active || outer.active || m.displayReceiptPending(&inner.polynomial) {
		t.Fatal("nested finite-field cleanup incomplete")
	}
}
