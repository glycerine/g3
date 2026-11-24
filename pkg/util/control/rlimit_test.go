package control

import (
	"math"
	"sync"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Added source-derived tests: rlimit has no dedicated source assertion suite.
func TestResourceLimitSourceScopes(t *testing.T) {
	r, err := NewReslimit(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !r.Inc() || !r.IncOffset(4) || r.Count() != 5 {
		t.Fatal("unlimited count")
	}
	if err := r.Push(3); err != nil {
		t.Fatal(err)
	}
	if !r.IncOffset(3) || r.Inc() || r.Count() != 9 {
		t.Fatal("inclusive limit")
	}
	r.Pop()
	if r.Count() != 8 || !r.Inc() {
		t.Fatal("pop clamps to inner limit")
	}
	if err := r.Push(10); err != nil {
		t.Fatal(err)
	}
	if err := r.Push(0); err != nil {
		t.Fatal(err)
	}
	// Source min(outer limit, zero) drops the outer bound while nested.
	if !r.IncOffset(100) {
		t.Fatal("source zero nested bound")
	}
	r.Pop()
	if r.IncOffset(0) {
		t.Fatal("outer bound restored without zero-bound clamp")
	}
	r.Pop()
	if r.Count() != 19 {
		t.Fatal(r.Count())
	}
	r.Steps = math.MaxUint64 - 1
	if err := r.Push(3); err != nil {
		t.Fatal(err)
	}
	if r.Limit != 0 || !r.IncOffset(3) || r.Count() != 1 {
		t.Fatal("source uint64 wrap")
	}
	r.Pop()
}

func TestResourceLimitSourceCancellation(t *testing.T) {
	var parent, child, grand Reslimit
	if err := parent.PushChild(&child); err != nil {
		t.Fatal(err)
	}
	if err := child.PushChild(&grand); err != nil {
		t.Fatal(err)
	}
	parent.Cancel()
	parent.IncCancel()
	if !parent.CancelFlag() || !child.CancelFlag() || !grand.CancelFlag() || grand.Inc() {
		t.Fatal("propagation")
	}
	parent.DecCancel()
	if grand.cancelCount.Load() != 1 {
		t.Fatal("nested cancel count")
	}
	parent.ResetCancel()
	parent.DecCancel()
	if grand.CancelFlag() || !grand.Inc() {
		t.Fatal("reset or zero decrement")
	}
	parent.Cancel()
	if err := child.Push(0); err != nil {
		t.Fatal(err)
	}
	if child.CancelFlag() || !grand.CancelFlag() {
		t.Fatal("push clears local cancel only")
	}
	child.Pop()
	parent.cancelCount.Store(math.MaxUint32)
	parent.IncCancel()
	if parent.CancelFlag() || child.CancelFlag() || grand.CancelFlag() {
		t.Fatal("source cancel unsigned wrap")
	}
	parent.PopChild()
	parent.Cancel()
	if child.CancelFlag() {
		t.Fatal("popped child detached")
	}
	if parent.CancelMessage() != "canceled" || child.CancelMessage() != "max. resource limit exceeded" {
		t.Fatal("source messages")
	}
	parent.Close()
	child.Close()
	grand.Close()
}

func TestResourceLimitSourceRAII(t *testing.T) {
	var r, child Reslimit
	scope, err := NewLimitScope(&r, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Inc() || r.Inc() {
		t.Fatal("scope")
	}
	scope.Close()
	scope.Close()
	if r.Count() != 1 || !r.Inc() {
		t.Fatal("scope closes once")
	}
	r.Cancel()
	outer := NewSuspendLimit(&r)
	inner := NewSuspendLimit(&r)
	if r.CancelFlag() || !r.Inc() {
		t.Fatal("suspended cancel")
	}
	inner.Close()
	// Source destructor always clears suspend; nesting does not restore it.
	if !r.CancelFlag() || r.Inc() {
		t.Fatal("source nested suspend defect")
	}
	outer.Close()
	children := newUtilChildLimits(&r)
	if err := children.PushChild(&child); err != nil {
		t.Fatal(err)
	}
	children.Close()
	children.Close()
	r.ResetCancel()
	r.Cancel()
	if child.CancelFlag() {
		t.Fatal("child scope cleanup")
	}
	r.Close()
	child.Close()
}

func TestResourceLimitIndependentStackOracle(t *testing.T) {
	var r Reslimit
	defer r.Close()
	var count, bound uint64
	var stack []uint64
	state := uint32(17)
	for step := 0; step < 4000; step++ {
		state = state*1664525 + 1013904223
		switch state % 3 {
		case 0:
			if len(stack) == 20 {
				continue
			}
			delta := uint64(state>>16) % 20
			stack = append(stack, bound)
			candidate := count + delta
			if delta == 0 {
				candidate = 0
			}
			if bound == 0 || candidate < bound {
				bound = candidate
			}
			if err := r.Push(uint32(delta)); err != nil {
				t.Fatal(err)
			}
		case 1:
			if len(stack) == 0 {
				continue
			}
			if bound != 0 && count > bound {
				count = bound
			}
			bound = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			r.Pop()
		case 2:
			offset := uint32(state>>20) % 9
			count += uint64(offset)
			if r.IncOffset(offset) != (bound == 0 || count <= bound) {
				t.Fatal(step, "admission")
			}
		}
		if r.Count() != count || r.Limit != bound {
			t.Fatal(step, "stack state")
		}
	}
}

func TestResourceLimitBudgetsAndWarmScopes(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	b.FailAfter(0)
	if r, err := NewReslimit(b); err == nil || r != nil || b.Used() != 0 {
		t.Fatal("constructor rollback")
	}
	b.FailAfter(-1)
	r, err := NewReslimit(b)
	if err != nil {
		t.Fatal(err)
	}
	before := b.Used()
	r.Cancel()
	b.FailAfter(0)
	if err := r.Push(5); err == nil || r.Limit != 0 || !r.CancelFlag() || b.Used() != before {
		t.Fatal("failed push mutation")
	}
	var child Reslimit
	if err := r.PushChild(&child); err == nil || len(r.Children) != 0 || b.Used() != before {
		t.Fatal("failed child publication")
	}
	b.FailAfter(-1)
	if err := r.Push(5); err != nil {
		t.Fatal(err)
	}
	r.Pop()
	if err := r.PushChild(&child); err != nil {
		t.Fatal(err)
	}
	r.PopChild()
	if a := testing.AllocsPerRun(100, func() {
		if err := r.Push(2); err != nil {
			panic(err)
		}
		r.Inc()
		r.Pop()
		if err := r.PushChild(&child); err != nil {
			panic(err)
		}
		r.Cancel()
		r.ResetCancel()
		r.PopChild()
	}); a != 0 {
		t.Fatal(a)
	}
	r.Close()
	r.Close()
	if b.Used() != 0 {
		t.Fatal("close leaked", b.Used())
	}
}

func TestResourceLimitConcurrentCancel(t *testing.T) {
	var r, child Reslimit
	if err := r.PushChild(&child); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer child.Close()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				r.IncCancel()
				child.CancelFlag()
				r.DecCancel()
			}
		}()
	}
	for i := 0; i < 1000; i++ {
		r.Inc()
	}
	wg.Wait()
	if r.CancelFlag() || child.CancelFlag() {
		t.Fatal("balanced concurrent cancellation")
	}
}

func TestResourceLimitSuspensionAndLateChild(t *testing.T) {
	var r, child Reslimit
	defer r.Close()
	defer child.Close()
	if err := r.Push(1); err != nil {
		t.Fatal(err)
	}
	if !r.Inc() || r.Inc() || r.CancelFlag() {
		t.Fatal("exhaustion is separate from cancel flag")
	}
	s := NewSuspendLimit(&r)
	if !r.IncOffset(10) || r.Count() != 12 {
		t.Fatal("suspension bypasses bound, retains work")
	}
	r.Cancel()
	if r.CancelFlag() || r.CancelMessage() != "canceled" {
		t.Fatal("message ignores suspension")
	}
	if err := r.PushChild(&child); err != nil {
		t.Fatal(err)
	}
	if child.CancelFlag() {
		t.Fatal("registration does not inherit previous cancellation")
	}
	r.IncCancel()
	if !child.CancelFlag() || child.cancelCount.Load() != 2 {
		t.Fatal("next propagation overwrites child count")
	}
	s.Close()
	r.Pop()
	if r.CancelFlag() || !child.CancelFlag() || r.Count() != 1 {
		t.Fatal("pop clamps and clears local flag only")
	}
	r.PopChild()
}

func TestResourceLimitGrowthFailureAndPeak(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, err := NewReslimit(b)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := r.Push(9); err != nil {
			t.Fatal(err)
		}
	}
	r.Cancel()
	before := b.Used()
	b.FailAfter(0)
	if err := r.Push(3); err == nil || len(r.Limits) != 2 || r.Limit != 9 || !r.CancelFlag() || b.Used() != before {
		t.Fatal("growth failure changed limit")
	}
	b.FailAfter(-1)
	r.Close()
	// Constructor header plus an existing two-entry vector fits, replacement
	// plus the old backing store does not. This checks peak growth charging.
	header := before - 16
	b2 := arena.NewBudget(header + 24)
	r, err = NewReslimit(b2)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Push(1); err != nil {
		t.Fatal(err)
	}
	if err := r.Push(1); err != nil {
		t.Fatal(err)
	}
	if err := r.Push(1); err != arena.ErrBudget || b2.Used() != header+16 {
		t.Fatal("old+new vector peak", err, b2.Used())
	}
	r.Pop()
	r.Pop()
	r.Close()
	if b.Used() != 0 || b2.Used() != 0 {
		t.Fatal("budget leak")
	}
}

func TestTryIncOffsetPreservesSingleStepFallback(t *testing.T) {
	var r Reslimit
	if err := r.Push(3); err != nil {
		t.Fatal(err)
	}
	if r.TryIncOffset(4) || r.Count() != 0 || !r.TryIncOffset(3) || r.Count() != 3 {
		t.Fatal("expected exact inclusive charge and no partial charge")
	}
	if r.TryIncOffset(1) || r.Count() != 3 || r.Inc() || r.Count() != 4 {
		t.Fatal("fallback must charge the first failing step")
	}
	r.Pop()
	r.Cancel()
	if r.TryIncOffset(2) || r.Count() != 3 {
		t.Fatal("consumed a canceled block")
	}
	if err := r.Uncharged(func() error {
		if !r.TryIncOffset(2) || r.Count() != 5 {
			t.Fatal("suspended limit rejected a block")
		}
		return nil
	}); err != nil || r.Count() != 3 {
		t.Fatal("owner count after suspension", err, r.Count())
	}
}
