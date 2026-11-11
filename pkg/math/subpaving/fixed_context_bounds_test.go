package subpaving

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingFixedContextBoundFlowAndNormalization(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	value, err := m.New()
	must(err)
	lo, err := m.New()
	must(err)
	hi, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{false, true}, 20, 1, 8192)
	must(err)
	root, err := ctx.Root()
	must(err)
	set := func(n int64) { t.Helper(); must(m.SetInt64(value, n)) }
	relevant := func(variable uint32, k int64, lower, open, want bool) {
		t.Helper()
		set(k)
		got, err := ctx.RelevantNewBound(root, variable, value, lower, open)
		must(err)
		if got != want {
			t.Fatalf("relevance x%d %d lower=%v open=%v = %v, want %v", variable, k, lower, open, got, want)
		}
	}
	relevant(0, 11, true, false, false)
	relevant(0, -11, false, false, false)
	set(0)
	must(ctx.PropagateBound(root, 0, value, true, false))
	set(10)
	must(ctx.PropagateBound(root, 0, value, false, false))
	if len(ctx.queue) != 2 || ctx.queue[0].timestamp != 0 || ctx.queue[1].timestamp != 1 {
		t.Fatalf("queue/timestamps = %d, %d, %d", len(ctx.queue), ctx.queue[0].timestamp, ctx.queue[1].timestamp)
	}
	relevant(0, 0, true, false, false)
	relevant(0, 1, true, false, true)
	relevant(0, 10, true, true, true) // equal to the upper bound; openness conflicts
	relevant(0, 10, false, false, false)
	relevant(0, 9, false, false, true)
	relevant(0, 0, false, true, true)   // equal to the lower bound; openness conflicts
	relevant(0, -1, false, false, true) // conflict has priority
	child, err := ctx.NewChild(root)
	must(err)
	view, err := NewFixedInterval(m)
	must(err)
	must(view.SetConstant(child, 0))
	set(5)
	must(ctx.PropagateBound(root, 0, value, true, false))
	var out bytes.Buffer
	must(view.Display(&out))
	if got := out.String(); got != "[0, 10]" {
		t.Fatalf("persistent child bounds = %q", got)
	}
	set(-1)
	must(ctx.PropagateBound(root, 0, value, false, false))
	if !root.inconsistent || root.conflictVar != 0 || len(ctx.queue) != 4 || ctx.timestamp != 4 {
		t.Fatalf("conflict/queue state: conflict=%v var=%d queue=%d timestamp=%d", root.inconsistent, root.conflictVar, len(ctx.queue), ctx.timestamp)
	}
	if err := ctx.Close(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("close with live child = %v", err)
	}
	must(view.Close())
	must(child.Close())
	must(ctx.Close())
	if _, err := ctx.Root(); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed context = %v", err)
	}
	zero, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{true}, 0, 1, 8192)
	must(err)
	zroot, err := zero.Root()
	must(err)
	set(2)
	must(zero.PropagateBound(zroot, 0, value, true, false))
	got, err := zero.RelevantNewBound(zroot, 0, value, true, false)
	must(err)
	if got {
		t.Fatal("equal closed lower bound is relevant with zero epsilon")
	}
	got, err = zero.RelevantNewBound(zroot, 0, value, true, true)
	must(err)
	if !got {
		t.Fatal("equal open lower bound is irrelevant with zero epsilon")
	}
	must(zero.Close())
	must(lo.Close())
	must(hi.Close())
	must(value.Close())
	must(m.Close())
}

func TestSubpavingFixedContextIntegerBoundNormalization(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{true}, 0, 1, 8192)
	must(err)
	for _, tc := range []struct {
		num, den             int64
		lower, initiallyOpen bool
		want                 int64
	}{
		{3, 2, true, true, 2}, {3, 2, false, true, 1},
		{2, 1, true, true, 3}, {2, 1, false, true, 1},
	} {
		must(m.SetRatio(v, tc.num, uint64(tc.den)))
		open := tc.initiallyOpen
		must(ctx.NormalizeBound(0, v, tc.lower, &open))
		if open {
			t.Fatalf("normalized integer bound stayed open: %+v", tc)
		}
		want, err := m.New()
		must(err)
		must(m.SetInt64(want, tc.want))
		cmp, err := m.Compare(v, want)
		must(err)
		must(want.Close())
		if cmp != 0 {
			t.Fatalf("normalized %+v did not yield %d", tc, tc.want)
		}
	}
	root, err := ctx.Root()
	must(err)
	must(m.SetRatio(v, 3, 2))
	must(ctx.PropagateBound(root, 0, v, true, true))
	bound, err := root.bound(0, true)
	must(err)
	if bound.open {
		t.Fatal("published fractional integer bound remained open")
	}
	value, err := m.Rational(bound.value)
	must(err)
	if value.Num().Int64() != 2 || value.Denom().Int64() != 1 {
		t.Fatalf("published integer lower = %s", value)
	}
	must(ctx.Close())
	must(v.Close())
	must(m.Close())
}

func TestSubpavingFixedContextPropagateCandidateAndRootPrecision(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(2, 1)
	must(err)
	v, err := m.New()
	must(err)
	lo, err := m.New()
	must(err)
	hi, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{false}, 0, 1, 8192)
	must(err)
	root, err := ctx.Root()
	must(err)
	must(m.SetInt64(v, 0))
	published, err := ctx.PropagateCandidate(root, 0, v, true, false)
	must(err)
	if !published {
		t.Fatal("first lower bound was skipped")
	}
	published, err = ctx.PropagateCandidate(root, 0, v, true, false)
	must(err)
	if published || len(ctx.queue) != 1 {
		t.Fatalf("equal candidate published=%v queue=%d", published, len(ctx.queue))
	}
	must(m.SetInt64(v, 1))
	published, err = ctx.PropagateCandidate(root, 0, v, true, false)
	must(err)
	if !published || len(ctx.queue) != 2 {
		t.Fatalf("stronger candidate published=%v queue=%d", published, len(ctx.queue))
	}
	src, err := NewFixedInterval(m)
	must(err)
	out, err := NewFixedInterval(m)
	must(err)
	must(m.SetInt64(lo, 4))
	must(m.SetInt64(hi, 9))
	must(src.SetFinite(lo, hi, false, false))
	must(ctx.XnEqY(out, src, 2))
	var text bytes.Buffer
	must(out.Display(&text))
	if got := text.String(); got != "[-3, 3]" {
		t.Fatalf("context root precision result = %q", got)
	}
	must(m.SetInt64(v, 0))
	published, err = ctx.PropagateCandidate(root, 0, v, false, false)
	must(err)
	if !published || !root.inconsistent || len(ctx.queue) != 3 {
		t.Fatalf("conflicting candidate published=%v conflict=%v queue=%d", published, root.inconsistent, len(ctx.queue))
	}
	must(src.Close())
	must(out.Close())
	must(ctx.Close())
	must(v.Close())
	must(lo.Close())
	must(hi.Close())
	must(m.Close())
}

func TestSubpavingFixedContextRelevanceArithmeticFailure(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	m, err := numeral.NewFixedPointManager(1, 1)
	must(err)
	v, err := m.New()
	must(err)
	ctx, err := NewFixedContext(m, &control.Reslimit{Limit: 1000}, []bool{false}, 20, 1, 8192)
	must(err)
	n, err := ctx.Root()
	must(err)
	must(m.SetInt64(v, -4294967295))
	must(ctx.PropagateBound(n, 0, v, true, false))
	must(m.SetInt64(v, 4294967295))
	must(ctx.PropagateBound(n, 0, v, false, false))
	must(m.SetInt64(v, 0))
	relevant, err := ctx.RelevantNewBound(n, 0, v, true, false)
	must(err)
	if relevant || !ctx.arithmeticFailed {
		t.Fatalf("overflow relevance=%v arithmeticFailed=%v", relevant, ctx.arithmeticFailed)
	}
	must(ctx.Close())
	must(v.Close())
	must(m.Close())
}
