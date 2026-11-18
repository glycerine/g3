package polynomial

// Source: algebraic_numbers.cpp:2899-2927.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func algebraicManagerDetailFixture(t *testing.T) *AlgebraicManager {
	t.Helper()
	cfg, _, budget := algebraicManagerFixture(t)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 32})
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, rt.Close()) })
	ic := serviceConfig(budget).Integers
	ic.Memory = rt
	ic.Profile = umemory.SmallAllocatorProfile{PointerBits: 32, Debug: true}
	q, e := unumeral.NewRationalManager(unumeral.RationalConfig{Integers: ic})
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, q.CloseChecked()) })
	cfg.Numerals = q
	a, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: rt, Profile: ic.Profile})
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, a.Close()) })
	cfg.Allocator = a
	m, e := NewAlgebraicManager(cfg)
	if m != nil {
		t.Cleanup(func() { serviceMust(t, m.Close()) })
	}
	serviceMust(t, e)
	return m
}

func TestAlgebraicManagerArithmeticDetailLiteralRecordsAndAliases(t *testing.T) {
	for _, kind := range []string{"power", "add", "sub", "mul", "integer"} {
		for _, alias := range []bool{false, true} {
			t.Run(fmt.Sprint(kind, "/", alias), func(t *testing.T) {
				m := algebraicManagerDetailFixture(t)
				var a, b, result Algebraic
				two, three := unumeral.NewInt(2), unumeral.NewInt(3)
				serviceMust(t, m.SetInteger(&a, &two))
				serviceMust(t, m.SetInteger(&b, &three))
				out := &result
				if alias {
					out = &a
				}
				var text strings.Builder
				tags := diagnostic.NewTraceTagsWithWriter(&text)
				tags.Enable("anum_detail")
				serviceMust(t, m.config.Numerals.BindTrace(tags))
				var e error
				op, want, before := "+", int64(5), "(# - 2, 1) + (# - 3, 1)\n"
				switch kind {
				case "power":
					e = m.Power(&a, 3, out)
					op, want, before = "^", 8, "(# - 2, 1)^3\n"
				case "add":
					e = m.Add(&a, &b, out)
				case "sub":
					e = m.Sub(&a, &b, out)
					op, want, before = "-", -1, "(# - 2, 1) - (# - 3, 1)\n"
				case "mul":
					e = m.Mul(&a, &b, out)
					op, want, before = "*", 6, "(# - 2, 1) * (# - 3, 1)\n"
				case "integer":
					e = m.AddInteger(&a, &three, out)
				}
				serviceMust(t, e)
				serviceMust(t, m.config.Numerals.BindTrace(nil))
				if tags.ShouldTrace("anum_detail") {
					root := fmt.Sprint("(# - ", want, ", 1)")
					if want < 0 {
						root = fmt.Sprint("(# + ", -want, ", 1)")
					}
					if strings.Count(text.String(), "[anum_detail]") != 2 || !strings.Contains(text.String(), before) || !strings.Contains(text.String(), op+" result: "+root+"\n") {
						t.Fatal("public wrapper literal/order/count", text.String())
					}
				} else if text.Len() != 0 {
					t.Fatal("erased manager trace wrote output")
				}
				q := unumeral.NewRational()
				serviceMust(t, m.config.Numerals.SetInt64(&q, want))
				cmp, e := m.CompareRational(out, &q)
				serviceMust(t, e)
				if cmp != 0 {
					t.Fatal("public arithmetic result", cmp)
				}
				if m.integerAdd.active || m.integerAdd.value.slot != 0 {
					t.Fatal("integer overload retained scoped value")
				}
				serviceMust(t, m.config.Numerals.Del(&q))
				serviceMust(t, m.Del(&result))
				serviceMust(t, m.Del(&b))
				serviceMust(t, m.Del(&a))
			})
		}
	}
}

func TestAlgebraicManagerArithmeticDetailFailurePublicationFrontiers(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			m := algebraicManagerDetailFixture(t)
			q := unumeral.NewRational()
			serviceMust(t, m.config.Numerals.SetString(&q, "18446744073709551619"))
			var a, out Algebraic
			serviceMust(t, m.SetRational(&a, &q))
			seven := unumeral.NewInt(7)
			serviceMust(t, m.SetInteger(&out, &seven))
			marker := ":2900 "
			if after {
				marker = ":2902 "
			}
			w := &algebraicBinaryDisplayStopWriter{core: m.AlgebraicRootCore, marker: marker}
			tags := diagnostic.NewTraceTagsWithWriter(w)
			tags.Enable("anum_detail")
			serviceMust(t, m.config.Numerals.BindTrace(tags))
			body := m.Power(&a, 1, &out)
			if tags.ShouldTrace("anum_detail") {
				if !errors.Is(body, arena.ErrBounds) || w.ref == 0 || !m.rootTracePending() || !m.rootDisplay.active {
					t.Fatal("actual public wrapper formatter stop", body)
				}
				if !after {
					l, e := m.locationCleanup(&out)
					serviceMust(t, e)
					r, e := m.load(l)
					serviceMust(t, e)
					if r.value.BorrowNumerator().Val != 7 || r.value.BorrowNumerator().Cell != 0 {
						t.Fatal("before record published arithmetic output")
					}
				}
				m.config.Numerals.Integers().Store(w.ref, w.original)
			} else {
				serviceMust(t, body)
			}
			serviceMust(t, m.config.Numerals.BindTrace(nil))
			serviceMust(t, m.CleanupRetained())
			if after || !tags.ShouldTrace("anum_detail") {
				cmp, e := m.CompareRational(&out, &q)
				serviceMust(t, e)
				if cmp != 0 {
					t.Fatal("after record lost already published arithmetic result", cmp)
				}
			}
			serviceMust(t, m.Del(&out))
			serviceMust(t, m.Del(&a))
			serviceMust(t, m.config.Numerals.Del(&q))
		})
	}
}

func TestAlgebraicManagerArithmeticDetailBodyExceptionOmitsResult(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	var zero, out Algebraic
	seven := unumeral.NewInt(7)
	serviceMust(t, m.SetInteger(&out, &seven))
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("anum_detail")
	serviceMust(t, m.config.Numerals.BindTrace(tags))
	body := m.Power(&zero, 0, &out)
	if !errors.Is(body, ErrAlgebraicZeroPower) {
		t.Fatal("public power exception", body)
	}
	if tags.ShouldTrace("anum_detail") && (!strings.Contains(text.String(), "(#, 1)^0\n") || strings.Contains(text.String(), "result:")) {
		t.Fatal("wrapper result record followed failed body", text.String())
	}
	serviceMust(t, m.config.Numerals.BindTrace(nil))
	serviceMust(t, m.Del(&out))
}

func TestAlgebraicManagerIntegerDetailStopRetainsRealTemporary(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			m := algebraicManagerDetailFixture(t)
			q := unumeral.NewRational()
			serviceMust(t, m.config.Numerals.SetString(&q, "18446744073709551619"))
			var a, out Algebraic
			serviceMust(t, m.SetRational(&a, &q))
			seven, three := unumeral.NewInt(7), unumeral.NewInt(3)
			serviceMust(t, m.SetInteger(&out, &seven))
			marker := ":2906 "
			if after {
				marker = ":2908 "
			}
			w := &algebraicBinaryDisplayStopWriter{core: m.AlgebraicRootCore, marker: marker}
			tags := diagnostic.NewTraceTagsWithWriter(w)
			tags.Enable("anum_detail")
			serviceMust(t, m.config.Numerals.BindTrace(tags))
			body := m.AddInteger(&a, &three, &out)
			var stop error
			if tags.ShouldTrace("anum_detail") {
				stop = m.rootDisplay.sourceStop
				if !errors.Is(body, arena.ErrBounds) || w.ref == 0 || stop == nil || !m.integerAdd.active || m.integerAdd.value.slot == 0 || m.integerAdd.sourceStop != stop {
					t.Fatal("public integer overload lost scoped source temporary", body)
				}
				l, e := m.locationCleanup(&m.integerAdd.value)
				serviceMust(t, e)
				r, e := m.load(l)
				serviceMust(t, e)
				if l.stage != 0 || r.value.BorrowNumerator().Val != 3 {
					t.Fatal("younger display advanced older tmp destruction")
				}
				m.config.Numerals.Integers().Store(w.ref, w.original)
			} else {
				serviceMust(t, body)
			}
			serviceMust(t, m.config.Numerals.BindTrace(nil))
			serviceMust(t, m.CleanupRetained())
			if m.integerAdd.active || m.integerAdd.cleaning || m.integerAdd.value.slot != 0 || m.integerAdd.sourceStop != stop {
				t.Fatal("integer temporary owning cleanup changed source history")
			}
			if after || !tags.ShouldTrace("anum_detail") {
				serviceMust(t, m.config.Numerals.SetString(&q, "18446744073709551622"))
				cmp, e := m.CompareRational(&out, &q)
				serviceMust(t, e)
				if cmp != 0 {
					t.Fatal("result display stop changed published integer sum", cmp)
				}
			}
			serviceMust(t, m.Del(&out))
			serviceMust(t, m.Del(&a))
			serviceMust(t, m.config.Numerals.Del(&q))
		})
	}
}

func TestAlgebraicManagerDetailDoesNotWrapImpCalls(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	var a, b, out Algebraic
	two, three := unumeral.NewInt(2), unumeral.NewInt(3)
	serviceMust(t, m.SetInteger(&a, &two))
	serviceMust(t, m.SetInteger(&b, &three))
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("anum_detail")
	serviceMust(t, m.config.Numerals.BindTrace(tags))
	serviceMust(t, m.AlgebraicRootCore.Add(&a, &b, &out))
	serviceMust(t, m.AlgebraicRootCore.Power(&a, 2, &out))
	serviceMust(t, m.config.Numerals.BindTrace(nil))
	if text.Len() != 0 {
		t.Fatal("internal imp operation emitted public wrapper records", text.String())
	}
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&b))
	serviceMust(t, m.Del(&a))
}

type managerDetailCharacterGuard struct {
	strings.Builder
	manager *AlgebraicManager
	guarded bool
}

func (w *managerDetailCharacterGuard) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *managerDetailCharacterGuard) Write(p []byte) (int, error) {
	n, e := w.Builder.Write(p)
	if q := w.manager.univariate.currentDisplay; !w.guarded && strings.Contains(w.String(), "src/util/mpn.cpp:395 ") && q != nil && q.insertion.Active() && q.insertion.Character != nil && q.insertion.Character.Completed {
		w.guarded = true
		q.insertion.Character.Buffer = q.insertion.Character.Buffer[:0]
	}
	return n, e
}
func TestAlgebraicManagerIntegerDetailCompletedFormatterDoesNotStopTemporary(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	q := unumeral.NewRational()
	serviceMust(t, m.config.Numerals.SetString(&q, "18446744073709551619"))
	var a, out Algebraic
	serviceMust(t, m.SetRational(&a, &q))
	seven, three := unumeral.NewInt(7), unumeral.NewInt(3)
	serviceMust(t, m.SetInteger(&out, &seven))
	w := &managerDetailCharacterGuard{manager: m}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("anum_detail")
	tags.Enable("mpn_to_string")
	serviceMust(t, m.config.Numerals.BindTrace(tags))
	body := m.AddInteger(&a, &three, &out)
	if tags.ShouldTrace("anum_detail") {
		if !w.guarded || !errors.Is(body, arena.ErrBounds) || !m.rootTracePending() || m.rootTraceSourceCause() != nil || m.integerAdd.active || m.integerAdd.value.slot != 0 || m.integerAdd.sourceStop != nil {
			t.Fatal("completed formatter source protected older integer temporary", body, m.integerAdd.sourceStop)
		}
	} else {
		serviceMust(t, body)
	}
	serviceMust(t, m.config.Numerals.BindTrace(nil))
	serviceMust(t, m.CleanupRetained())
	if tags.ShouldTrace("anum_detail") {
		serviceMust(t, m.config.Numerals.SetInt32(&q, 7))
		cmp, e := m.CompareRational(&out, &q)
		serviceMust(t, e)
		if cmp != 0 {
			t.Fatal("before record read failure reached arithmetic publication", cmp)
		}
	}
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&a))
	serviceMust(t, m.config.Numerals.Del(&q))
}

func TestAlgebraicManagerDetailDisabledDoesNotReadOperands(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	c := m.AlgebraicRootCore
	var zero, out Algebraic
	serviceMust(t, m.Mul(&zero, nil, &out))
	if out.slot != 0 {
		t.Fatal("disabled detail changed zero multiplication short circuit")
	}
	serviceMust(t, c.traceManagerPower(nil, ^uint32(0), false))
	serviceMust(t, c.traceManagerPower(nil, 0, true))
	for kind := uint8(0); kind < 3; kind++ {
		serviceMust(t, c.traceManagerBinary(nil, nil, kind, false))
		serviceMust(t, c.traceManagerBinary(nil, nil, kind, true))
	}
}

func TestAlgebraicManagerDetailPublishesInputRootIndexBeforePower(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	a := algebraicCompareRoot(t, m.AlgebraicRootCore, []int32{-2, 0, 1}, 1, 0, 2, 0, true)
	var out Algebraic
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("anum_detail")
	serviceMust(t, m.config.Numerals.BindTrace(tags))
	serviceMust(t, m.Power(&a, 2, &out))
	serviceMust(t, m.config.Numerals.BindTrace(nil))
	l, e := m.location(&a)
	serviceMust(t, e)
	flags, e := m.AlgebraicRootCore.config.Allocator.ReadUint32(l.raw, 16)
	serviceMust(t, e)
	index := uint32(0)
	if tags.ShouldTrace("anum_detail") {
		index = 2
	}
	if flags>>3 != index {
		t.Fatal("public wrapper cached index publication", flags>>3, index)
	}
	if tags.ShouldTrace("anum_detail") && (!strings.Contains(text.String(), "(#^2 - 2, 2)^2\n") || !strings.Contains(text.String(), "^ result: (# - 2, 1)\n")) {
		t.Fatal("root public power records", text.String())
	}
	q := unumeral.NewRational()
	serviceMust(t, m.config.Numerals.SetInt32(&q, 2))
	cmp, e := m.CompareRational(&out, &q)
	serviceMust(t, e)
	if cmp != 0 {
		t.Fatal("root power result", cmp)
	}
	serviceMust(t, m.config.Numerals.Del(&q))
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&a))
}

type managerDetailOwnerProbe struct {
	manager *AlgebraicManager
	t       *testing.T
	calls   int
}

func (w *managerDetailOwnerProbe) Write(p []byte) (int, error) {
	w.calls++
	m := w.manager
	c := m.AlgebraicRootCore
	z := m.config.Numerals.Integers()
	used, physical, cause := z.Budget().Used(), z.RuntimeOwner().Stats(), m.cleanupCause
	var zero, out Algebraic
	for _, call := range []func() error{m.Close, m.SourceFinalize, m.CleanupRetained, func() error { return m.Add(&zero, &zero, &out) }, func() error { return m.Power(&zero, 1, &out) }, func() error { return m.AddInteger(&zero, &z.Two64, &out) }} {
		if e := call(); !errors.Is(e, ErrAlgebraicPending) {
			w.t.Error("public detail callback acquired algebraic owner", e)
		}
	}
	if e := z.CloseChecked(); !errors.Is(e, ErrPolynomialPending) {
		w.t.Error("public detail callback closed numeric owner", e)
	}
	poly, e := m.univariate.NewPolynomial()
	if !errors.Is(e, ErrUPolynomialCleanupPending) || poly.slot != 0 {
		w.t.Error("public detail callback constructed UP workspace", e)
	}
	if !c.rootTrace.running || m.univariate.factorTrace != &c.rootTrace.borrow || m.finishing || m.closed || m.sourceStarted || m.cleanupCause != cause || out.slot != 0 || used != z.Budget().Used() || physical != z.RuntimeOwner().Stats() {
		w.t.Error("public detail callback changed source owner or physical graph")
	}
	return len(p), nil
}
func TestAlgebraicManagerDetailBorrowProtectsEntireStream(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	var a, b, out Algebraic
	two, three := unumeral.NewInt(2), unumeral.NewInt(3)
	serviceMust(t, m.SetInteger(&a, &two))
	serviceMust(t, m.SetInteger(&b, &three))
	w := &managerDetailOwnerProbe{manager: m, t: t}
	tags := diagnostic.NewTraceTagsWithWriter(w)
	tags.Enable("anum_detail")
	serviceMust(t, m.config.Numerals.BindTrace(tags))
	serviceMust(t, m.Add(&a, &b, &out))
	serviceMust(t, m.config.Numerals.BindTrace(nil))
	if (w.calls != 0) != tags.ShouldTrace("anum_detail") || m.rootTrace.running || m.rootTracePending() {
		t.Fatal("public arithmetic stream lifetime", w.calls)
	}
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&b))
	serviceMust(t, m.Del(&a))
}

// AUDIT.md math_polynomial_01-F2: t_eval's ValManager is the public
// algebraic_numbers::manager, so evaluating at algebraic values emits the
// manager::power/mul/add TRACE("anum_detail") records.
func TestPolynomialEvaluationAlgebraicUsesManagerWrappers(t *testing.T) {
	m := algebraicManagerDetailFixture(t)
	c := m.AlgebraicRootCore
	pm, p, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{3, [3]uint32{2, 1, 0}}, {-2, [3]uint32{1, 0, 1}}, {-1, [3]uint32{}}})
	values := algebraicSignTestValues{}
	var out Algebraic
	defer func() {
		serviceMust(t, c.Del(&out))
		for i := range values.values {
			serviceMust(t, c.Del(&values.values[i]))
		}
	}()
	for i, n := range []int32{2, 3, 5} {
		v := unumeral.NewInt(n)
		serviceMust(t, m.SetInteger(&values.values[i], &v))
	}
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("anum_detail")
	serviceMust(t, m.config.Numerals.BindTrace(tags))
	e := pm.EvaluateAlgebraic(p, c, &values, &out)
	serviceMust(t, m.config.Numerals.BindTrace(nil))
	serviceMust(t, e)
	q := unumeral.NewRational()
	defer func() { serviceMust(t, m.config.Numerals.Del(&q)) }()
	serviceMust(t, m.config.Numerals.SetInt64(&q, 3*2*2*3-2*2*5-1))
	cmp, e := m.CompareRational(&out, &q)
	serviceMust(t, e)
	if cmp != 0 {
		t.Fatal("evaluation result", cmp)
	}
	if !tags.ShouldTrace("anum_detail") {
		return
	}
	s := text.String()
	for _, want := range []string{"^ result: ", "* result: ", "+ result: "} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q record:\n%s", want, s)
		}
	}
	if strings.Index(s, "^ result: ") > strings.Index(s, "* result: ") {
		t.Fatalf("power does not precede mul:\n%s", s)
	}
}
