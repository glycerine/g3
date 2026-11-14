package sat

// Source: src/sat/sat_solver.cpp:2773-2779,3652-3714; src/util/params.{h,cpp}

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSATSolverReportEveryStatusStreamReservationAndStatePrefix(t *testing.T) {
	h := int64(0)
	baseline := ""
	t.Run("calibrate", func(t *testing.T) {
		s := satInspectionFixture(t)
		o := satInspectionStream(t, arena.NewBudget(math.MaxUint64))
		const n = int64(1 << 50)
		o.Budget.FailAfter(n)
		satOK(t, s.displayStatus(o))
		h = n - testFieldBySourceName(reflect.ValueOf(o.Budget).Elem(), "remaining").Int()
		satInspectionStreamAudit(t, s, o, o.Budget)
		baseline, _ = o.String()
		o.Budget.FailAfter(-1)
	})
	failed, success := int64(0), 0
	for fault := int64(0); fault <= h; fault++ {
		t.Run(fmt.Sprint(fault), func(t *testing.T) {
			s := satInspectionFixture(t)
			b := arena.NewBudget(math.MaxUint64)
			o := satInspectionStream(t, b)
			b.FailAfter(fault)
			e := s.displayStatus(o)
			satInspectionStreamAudit(t, s, o, b)
			text, x := o.String()
			satOK(t, x)
			if !strings.HasPrefix(baseline, text) {
				t.Fatal(text, baseline)
			}
			state := o.State()
			if strings.Contains(text, "  :memory          ") {
				if state.FloatMode != utext.NumericFixedFloat || state.Precision != 2 {
					t.Fatal("reached source manipulator prefix", state, text)
				}
			} else if state.FloatMode != utext.NumericDefaultFloat || state.Precision != 6 {
				t.Fatal(state, text)
			}
			if e != nil {
				if !errors.Is(e, arena.ErrInjectedFailure) {
					t.Fatal(e)
				}
				failed++
			} else {
				if text != baseline {
					t.Fatal(text)
				}
				success++
			}
			b.FailAfter(-1)
		})
	}
	if failed != h || success != 1 {
		t.Fatal(failed, success, h)
	}
	t.Log("real status stream horizon", h)
}
func TestSATSolverReportEveryDescriptorOwnerReservationPrefix(t *testing.T) {
	h := int64(0)
	t.Run("calibrate", func(t *testing.T) {
		sy := symbolManagerTest(t, nil, false)
		b := arena.NewBudget(math.MaxUint64)
		d := pdnew(t, sy, b)
		const n = int64(1 << 50)
		b.FailAfter(n)
		satOK(t, collectSolverParamDescrs(d))
		h = n - testFieldBySourceName(reflect.ValueOf(b).Elem(), "remaining").Int()
		if pdtotal(d.Stats()) != b.Used() {
			t.Fatal(d.Stats(), b.Used())
		}
		b.FailAfter(-1)
	})
	failed, success := int64(0), 0
	names := satSolverDescriptorNames()
	for fault := int64(0); fault <= h; fault++ {
		t.Run(fmt.Sprint(fault), func(t *testing.T) {
			sy := symbolManagerTest(t, nil, false)
			b := arena.NewBudget(math.MaxUint64)
			d := pdnew(t, sy, b)
			b.FailAfter(fault)
			e := collectSolverParamDescrs(d)
			q := d.Stats()
			if pdtotal(q) != b.Used() {
				t.Fatal("actual descriptor graph before refusal disable", q, b.Used())
			}
			n, x := d.Size()
			satOK(t, x)
			for i := uint32(0); i < n; i++ {
				k, x := d.ParamName(i)
				satOK(t, x)
				name, yes, x := sy.BareString(k)
				satOK(t, x)
				if !yes || name != names[i] {
					t.Fatal("actual source child prefix", i, name)
				}
			}
			if e != nil {
				if !errors.Is(e, arena.ErrInjectedFailure) {
					t.Fatal(e)
				}
				failed++
			} else {
				if n != 45 {
					t.Fatal(n)
				}
				success++
			}
			b.FailAfter(-1)
		})
	}
	if failed != h || success != 1 {
		t.Fatal(failed, success, h)
	}
	t.Log("real descriptor-only reservation horizon", h)
}
func TestSATSolverReportWarmPhysicalPlateauAndLifetime(t *testing.T) {
	s := satInspectionFixture(t)
	b := arena.NewBudget(math.MaxUint64)
	o := satInspectionStream(t, b)
	satOK(t, s.displayStatus(o))
	before := b.Used()
	solver := s.budget.Used()
	for i := 0; i < 8; i++ {
		satOK(t, o.ResetText())
		b.FailAfter(0)
		satOK(t, s.displayStatus(o))
		satInspectionStreamAudit(t, s, o, b)
		if b.Used() != before || s.budget.Used() != solver {
			t.Fatal("warm real graph")
		}
		b.FailAfter(-1)
	}
	copy := *s
	if e := copy.displayStatus(o); !errors.Is(e, ErrSATOwner) {
		t.Fatal(e)
	}
	satOK(t, s.Close())
	if e := s.displayStatus(o); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
	sy := symbolManagerTest(t, nil, false)
	d := pdnew(t, sy, nil)
	satOK(t, d.Close())
	if e := collectSolverParamDescrs(d); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}
