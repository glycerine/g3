package polynomial

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestUPolynomialSturmIsolationTraceTextAndFailedStream(t *testing.T) {
	for _, closed := range []bool{false, true} {
		name := "text"
		if closed {
			name = "closed"
		}
		t.Run(name, func(t *testing.T) {
			c, m, _ := algebraicNegTLSFixture(t, nil)
			b := c.config.Binary
			roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
			p := upolySourceNew(t, m, []int32{-6, 11, -6, 1})
			var text strings.Builder
			refused := &linearTraceFailWriter{}
			var out io.Writer = &text
			if closed {
				out = refused
			}
			tags := diagnostic.NewTraceTagsWithWriter(out)
			tags.Enable("upolynomial")
			serviceMust(t, m.config.Integer.BindTrace(tags))
			body := m.SturmIsolateRootsWithBounds(mustUPolyView(t, m, p), 3, 3, b, roots, lo, hi)
			serviceMust(t, m.config.Integer.BindTrace(nil))
			serviceMust(t, body)
			if tags.ShouldTrace("upolynomial") {
				if closed {
					if refused.calls != 1 {
						t.Fatal("failed stream retried", refused.calls)
					}
				} else {
					want := []string{"p: x^3 - 6 x^2 + 11 x - 6\nSturm seq:\nx^3 - 6 x^2 + 11 x - 6\n3 x^2 - 12 x + 11\nx - 2\n1\n\n", "depth: 0\nlower_sv: 3\nupper_sv: 0\ncurr_lower: 0\ncurr_upper: 8\nmid: 4\n"}
					pos := 0
					for _, part := range want {
						i := strings.Index(text.String()[pos:], part)
						if i < 0 {
							t.Fatal("missing/out-of-order trace", part, text.String())
						}
						pos += i + len(part)
					}
				}
			} else if text.Len() != 0 || refused.calls != 0 {
				t.Fatal("erased trace emitted output")
			}
			counts := upolyIsolationCounts{}
			upolyIsolationMatch(t, c.config.Rationals, b, roots, lo, hi, []upolyIsolationRational{{1, 1}, {2, 1}, {3, 1}}, &counts)
			if m.sturmIsolation.active || m.sturmIsolation.tracePending() {
				t.Fatal("successful trace retained debt")
			}
		})
	}
}

func TestUPolynomialSturmIsolationTraceDisabledOperandsAndCallbacks(t *testing.T) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	b, z := c.config.Binary, m.config.Integer
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	serviceMust(t, z.BindTrace(tags))
	q, e := m.beginSturmIsolation(UPolynomialView{}, b)
	serviceMust(t, e)
	before, used := *q, z.Budget().Used()
	z.Budget().FailAfter(0)
	serviceMust(t, q.traceInitial(UPolynomialView{}))
	serviceMust(t, q.traceMidpoint(0, 0))
	z.Budget().FailAfter(-1)
	if !reflect.DeepEqual(before, *q) || used != z.Budget().Used() || text.Len() != 0 {
		t.Fatal("disabled trace evaluated source operands")
	}
	serviceMust(t, q.finish(nil))
	serviceMust(t, z.BindTrace(nil))
	roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
	p := upolySourceNew(t, m, []int32{-6, 11, -6, 1})
	writer := &linearOperationProbeWriter{probe: func() {
		used, stats := z.Budget().Used(), z.RuntimeOwner().Stats()
		for _, e := range []error{m.Close(), m.SourceFinalize(), m.CleanupSturmIsolation(), m.DelPolynomial(&p)} {
			if !errors.Is(e, ErrUPolynomialCleanupPending) {
				t.Fatal("trace callback acquired source owner", e)
			}
		}
		context := m.sturmIsolation.context()
		value := unumeral.NewBinaryInt(7)
		if context.sturmIsolationFrame(z) != nil || b.WithAllocationContext(context.utility(z)).Div2(&value) == nil || value.KField != 0 || value.Num.Val != 7 || used != z.Budget().Used() || stats != z.RuntimeOwner().Stats() {
			t.Fatal("trace callback acquired numeric source context")
		}
	}}
	tags = diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("upolynomial")
	serviceMust(t, z.BindTrace(tags))
	body := m.SturmIsolateRootsWithBounds(mustUPolyView(t, m, p), 3, 3, b, roots, lo, hi)
	serviceMust(t, z.BindTrace(nil))
	serviceMust(t, body)
	if writer.called != tags.ShouldTrace("upolynomial") {
		t.Fatal("trace callback erasure", writer.called)
	}
}

func TestUPolynomialSturmIsolationTraceDisplayStopProtectsLocals(t *testing.T) {
	for _, row := range []bool{false, true} {
		name := "polynomial"
		if row {
			name = "sequence-row"
		}
		t.Run(name, func(t *testing.T) { sturmIsolationTraceDisplayStop(t, row) })
	}
}
func sturmIsolationTraceDisplayStop(t *testing.T, row bool) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	root := algebraicLargeDyadicPromotionRoot(t, c)
	location, e := c.location(&root)
	serviceMust(t, e)
	r, e := c.load(location)
	serviceMust(t, e)
	v, e := c.sourcePolynomial(&r)
	serviceMust(t, e)
	b, z := c.config.Binary, m.config.Integer
	roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
	writer := &rootIDRemainderDisplayStopWriter{t: t, m: m}
	var output io.Writer = writer
	if row {
		output = &sturmIsolationSequenceStopWriter{inner: writer}
	}
	tags := diagnostic.NewTraceTagsWithWriter(output)
	tags.Enable("upolynomial")
	serviceMust(t, z.BindTrace(tags))
	body := m.SturmIsolateRootsWithBounds(v, 3, 3, b, roots, lo, hi)
	serviceMust(t, z.BindTrace(nil))
	q := &m.sturmIsolation
	if tags.ShouldTrace("upolynomial") {
		index := 0
		if row {
			index = 1
		}
		receipt := &q.traceDisplays[index]
		if !writer.reached || !errors.Is(body, arena.ErrBounds) || !m.displayReceiptPending(receipt) || m.displayReceiptSourceCause(receipt) == nil || q.childStop == nil || q.sourceStage != 0 || q.sourceCoefficient != 0 || q.live != ([5]bool{true, true, true, true, true}) {
			t.Fatal("trace stop lost exact child or older declarations", body, q.live)
		}
		for _, value := range m.values(&q.sequence.state.coefficients)[:q.sequence.state.coefficients.size] {
			if value.Cell != 0 && z.Cell(value.Cell).Released {
				t.Fatal("older sequence coefficient destroyed")
			}
		}
		before, used, stats := *q, z.Budget().Used(), z.RuntimeOwner().Stats()
		if e := m.SourceFinalize(); !errors.Is(e, arena.ErrBounds) {
			t.Fatal("manager crossed stopped trace", e)
		}
		if e := q.finishSource(nil); !errors.Is(e, arena.ErrBounds) {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(before, *q) || used != z.Budget().Used() || stats != z.RuntimeOwner().Stats() {
			t.Fatal("source finalization replayed stopped trace")
		}
		cell := z.Cell(writer.ref)
		cell.Backing.Requested = writer.requested
		z.Store(writer.ref, cell)
		serviceMust(t, m.CleanupSturmIsolation())
		if q.active || q.sourceStage != 0 || q.sourceCoefficient != 0 || q.tracePending() {
			t.Fatal("trace acknowledgement rewrote source history")
		}
	} else {
		serviceMust(t, body)
	}
	for _, output := range []*BinaryRationalVector{roots, lo, hi} {
		serviceMust(t, output.Reset())
	}
	algebraicComparisonAudit(t, c, &root)
	serviceMust(t, c.Del(&root))
}

func TestUPolynomialSturmIsolationTraceReachedReservations(t *testing.T) {
	success, failed, traceDebt := false, false, false
	enabled := false
	for after := int64(0); after < 1024 && !success; after++ {
		t.Run(strconv.FormatInt(after, 10), func(t *testing.T) {
			c, m, _ := algebraicNegTLSFixture(t, nil)
			b := c.config.Binary
			roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
			p := upolySourceNew(t, m, []int32{-6, 11, -6, 1})
			z := m.config.Integer
			var input [4]unumeral.Int
			for i, text := range []string{"-110680464442257309696", "202914184810805067776", "-110680464442257309696", "18446744073709551616"} {
				serviceMust(t, z.SetString(&input[i], text))
			}
			serviceMust(t, m.SetInts(p, input[:]))
			for i := range input {
				serviceMust(t, z.Del(&input[i]))
			}
			tags := diagnostic.NewTraceTagsWithWriter(io.Discard)
			tags.Enable("upolynomial")
			serviceMust(t, z.BindTrace(tags))
			enabled = tags.ShouldTrace("upolynomial")
			z.Budget().FailAfter(after)
			body := m.SturmIsolateRootsWithBounds(mustUPolyView(t, m, p), 3, 3, b, roots, lo, hi)
			z.Budget().FailAfter(-1)
			serviceMust(t, z.BindTrace(nil))
			q := &m.sturmIsolation
			if body != nil {
				for i := range q.traceDisplays {
					r := &q.traceDisplays[i]
					if m.matchesDisplayReceipt(r) && r.frame.bodyCause != nil {
						traceDebt = true
					}
				}
			}
			if body == nil {
				success = true
				counts := upolyIsolationCounts{}
				upolyIsolationMatch(t, c.config.Rationals, b, roots, lo, hi, []upolyIsolationRational{{1, 1}, {2, 1}, {3, 1}}, &counts)
			} else {
				failed = true
				if !numericPhysicalReservationRefusal(body) {
					t.Fatal("unexpected isolation refusal", body)
				}
			}
			stage, stack, coefficient := q.sourceStage, q.sourceStack, q.sourceCoefficient
			serviceMust(t, m.CleanupSturmIsolation())
			if q.active || q.pending() || stage != q.sourceStage || stack != q.sourceStack || coefficient != q.sourceCoefficient || m.sturmPending != nil {
				t.Fatal("isolation cleanup changed source history")
			}
			for _, v := range []*BinaryRationalVector{roots, lo, hi} {
				serviceMust(t, v.Reset())
			}
			algebraicCellSourcePhysical(t, c, []UPolynomial{p}, nil, nil)
		})
	}
	if !success || !failed || enabled && !traceDebt {
		t.Fatal("isolation reservation campaign incomplete", success, failed, traceDebt)
	}
}

func TestUPolynomialSturmIsolationTraceFractionalMidpoints(t *testing.T) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	b, z := c.config.Binary, m.config.Integer
	roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
	p := upolySourceNew(t, m, []int32{-15, 184, -576, 512})
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("upolynomial")
	serviceMust(t, z.BindTrace(tags))
	body := m.SturmIsolateRootsWithBounds(mustUPolyView(t, m, p), 1, 1, b, roots, lo, hi)
	serviceMust(t, z.BindTrace(nil))
	serviceMust(t, body)
	if tags.ShouldTrace("upolynomial") && !strings.Contains(text.String(), "curr_lower: 0\ncurr_upper: 1/2\nmid: 1/2^2\n") {
		t.Fatal("binary numerator/exponent source text", text.String())
	}
	counts := upolyIsolationCounts{}
	upolyIsolationMatch(t, c.config.Rationals, b, roots, lo, hi, []upolyIsolationRational{{1, 8}, {3, 8}, {5, 8}}, &counts)
}

func TestUPolynomialSturmIsolationTraceFormatterAfterFailedOutput(t *testing.T) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	b, z := c.config.Binary, m.config.Integer
	q, e := m.beginSturmIsolation(UPolynomialView{}, b)
	serviceMust(t, e)
	q.constructCore()
	serviceMust(t, q.constructStack())
	ops := b.WithAllocationContext(integerAllocationContextRef(q.context()).utility(z))
	serviceMust(t, ops.PowerOfTwo(512, &q.lower, &q.copy))
	serviceMust(t, ops.Neg(&q.lower))
	serviceMust(t, ops.PowerOfTwo(511, &q.upper, &q.copy))
	serviceMust(t, ops.AddSub(&q.lower, &q.upper, &q.mid, false))
	serviceMust(t, ops.Div2(&q.mid))
	writer := &upolyNormalizeFailedProbeWriter{probe: func() {
		cell := z.Cell(q.lower.Num.Cell)
		size, e := cell.Backing.ReadWord(z.Allocator, 0)
		serviceMust(t, e)
		for i := size; i < cell.Capacity; i++ {
			serviceMust(t, z.RuntimeOwner().WriteUint32(cell.Backing.Raw, cell.Backing.Offset+8+4*uint64(i), 0))
		}
		serviceMust(t, z.RuntimeOwner().WriteUint32(cell.Backing.Raw, cell.Backing.Offset, cell.Capacity+1))
	}}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("upolynomial")
	serviceMust(t, z.BindTrace(tags))
	polynomialAssertionActionsFixture(t, "")
	if polynomialSourceDebug {
		serviceMust(t, diagnostic.BorrowProcessDebugContext().EnableAssertions(false))
	}
	body := q.traceMidpoint(3, 0)
	serviceMust(t, z.BindTrace(nil))
	if tags.ShouldTrace("upolynomial") {
		if !writer.called || !errors.Is(body, arena.ErrBounds) || !q.traceInsertions[0].Active() {
			t.Fatal("failed output skipped actual numeral conversion", body, writer.called, q.traceInsertions[0].Active())
		}
	} else {
		serviceMust(t, body)
		if writer.called {
			t.Fatal("erased trace evaluated output")
		}
	}
	_ = q.finish(body)
	stage, coefficient := q.sourceStage, q.sourceCoefficient
	serviceMust(t, m.CleanupSturmIsolation())
	if q.active || q.tracePending() || q.sourceStage != stage || q.sourceCoefficient != coefficient {
		t.Fatal("formatter acknowledgement changed source history")
	}
	algebraicCellSourcePhysical(t, c, nil, nil, nil)
}

type sturmIsolationSequenceStopWriter struct {
	inner *rootIDRemainderDisplayStopWriter
	row   bool
}

func (w *sturmIsolationSequenceStopWriter) Write(p []byte) (int, error) {
	if string(p) == "\nSturm seq:\n" {
		w.row = true
	}
	if w.row {
		return w.inner.Write(p)
	}
	return len(p), nil
}

func TestUPolynomialSturmIsolationTracePreservesOlderDisplay(t *testing.T) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	root := algebraicLargeDyadicPromotionRoot(t, c)
	l, e := c.location(&root)
	serviceMust(t, e)
	r, e := c.load(l)
	serviceMust(t, e)
	v, e := c.sourcePolynomial(&r)
	serviceMust(t, e)
	z := m.config.Integer
	older := upolySourceNew(t, m, nil)
	constant, e := m.read(v, 0)
	serviceMust(t, e)
	serviceMust(t, m.SetInts(older, []unumeral.Int{*constant}))
	ov, e := m.wholeView(older)
	serviceMust(t, e)
	var ref unumeral.NumericCellRef
	var requested uint64
	sink := &upolyDisplaySink{numeralHook: func(string) error {
		ref = m.currentDisplay.numeral.Cell
		cell := z.Cell(ref)
		requested = cell.Backing.Requested
		cell.Backing.Requested = uint64(^uint32(0)) + 1
		z.Store(ref, cell)
		return nil
	}}
	var oldReceipt upolyDisplayInvocationReceipt
	if e := m.displayIntoWithReceipt(ov, sink, "x", false, &oldReceipt); !errors.Is(e, arena.ErrBounds) || !m.displayReceiptPending(&oldReceipt) {
		t.Fatal("older display did not stop", e)
	}
	before := *oldReceipt.frame
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("upolynomial")
	serviceMust(t, z.BindTrace(tags))
	b := c.config.Binary
	roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
	e = m.SturmIsolateRootsWithBounds(v, 3, 3, b, roots, lo, hi)
	serviceMust(t, z.BindTrace(nil))
	serviceMust(t, e)
	serviceMust(t, m.CleanupSturmIsolation())
	if !reflect.DeepEqual(before, *oldReceipt.frame) || !m.displayReceiptPending(&oldReceipt) || z.Cell(ref).Released {
		t.Fatal("isolation adopted older display")
	}
	if tags.ShouldTrace("upolynomial") && text.Len() == 0 {
		t.Fatal("trace did not enter fresh display")
	}
	for _, output := range []*BinaryRationalVector{roots, lo, hi} {
		serviceMust(t, output.Reset())
	}
	if !reflect.DeepEqual(before, *oldReceipt.frame) || !m.displayReceiptPending(&oldReceipt) {
		t.Fatal("isolation output reset adopted older display")
	}
	cell := z.Cell(ref)
	cell.Backing.Requested = requested
	z.Store(ref, cell)
	serviceMust(t, m.cleanupDisplayReceipt(&oldReceipt))
	serviceMust(t, c.Del(&root))
}
