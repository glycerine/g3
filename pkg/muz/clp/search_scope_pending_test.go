package clp_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

type clpScopeRefusalSolver struct {
	clp.Solver
	pushBefore, pushAfter map[uint32]error
	popBefore, popAfter   map[uint32]error
	pushes, pops          uint32
	popLevels             []uint32
}

func (s *clpScopeRefusalSolver) Push() error {
	s.pushes++
	if err := s.pushBefore[s.pushes]; err != nil {
		return err
	}
	if err := s.Solver.Push(); err != nil {
		return err
	}
	return s.pushAfter[s.pushes]
}

func (s *clpScopeRefusalSolver) Pop(count uint32) error {
	s.pops++
	level, err := s.Solver.ScopeLevel()
	if err != nil {
		return err
	}
	s.popLevels = append(s.popLevels, level)
	if err = s.popBefore[s.pops]; err != nil {
		return err
	}
	if err = s.Solver.Pop(count); err != nil {
		return err
	}
	return s.popAfter[s.pops]
}

func clpScopeLevel(t *testing.T, solver clp.Solver) uint32 {
	t.Helper()
	level, err := solver.ScopeLevel()
	if err != nil {
		t.Fatal(err)
	}
	return level
}

func TestCLPRuleScopeReconcilesPartialPush(t *testing.T) {
	for _, tc := range []struct {
		name        string
		beforePush  bool
		wantPops    uint32
		wantPushErr string
	}{
		{name: "before-publication", beforePush: true, wantPops: 0, wantPushErr: "CLP push refused before publication"},
		{name: "after-publication", wantPops: 1, wantPushErr: "CLP push refused after publication"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			refused := errors.New(tc.wantPushErr)
			wrapped := &clpScopeRefusalSolver{Solver: f.solver}
			if tc.beforePush {
				wrapped.pushBefore = map[uint32]error{1: refused}
			} else {
				wrapped.pushAfter = map[uint32]error{1: refused}
			}
			engine, err := clp.NewEngine(f.m, f.set, wrapped, 20)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			}()
			status, err := engine.Query(f.app["P"])
			if status != lbool.Undef || !errors.Is(err, refused) || clpScopeLevel(t, wrapped) != 0 || wrapped.pops != tc.wantPops {
				t.Fatal("partial push cleanup", status, err, clpScopeLevel(t, wrapped), wrapped.pops)
			}
		})
	}
}

func TestCLPRuleScopePopRefusalIsRetryableAndSuppressesResult(t *testing.T) {
	f := newFixture(t)
	refused := errors.New("CLP pop refused before consumption")
	wrapped := &clpScopeRefusalSolver{Solver: f.solver, popBefore: map[uint32]error{1: refused, 2: refused}}
	engine, err := clp.NewEngine(f.m, f.set, wrapped, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	}()
	before := f.m.Stats().LiveReferences
	status, failure := engine.Query(f.app["P"])
	if status != lbool.Undef || !errors.Is(failure, refused) || clpScopeLevel(t, wrapped) != 1 || wrapped.pops != 2 {
		t.Fatal("pop refusal published result or lost scope", status, failure, clpScopeLevel(t, wrapped), wrapped.pops)
	}
	if f.m.Stats().LiveReferences <= before {
		t.Fatal("pop refusal released grounded assertion owners")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("pop refusal has no retry owner", failure)
	}
	original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	if err = cleanup.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if err = cleanup.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if clpScopeLevel(t, wrapped) != 0 || wrapped.pops != 3 || cleanup.CleanupCause() != nil || f.m.Stats().LiveReferences != before {
		t.Fatal("pop retry did not restore exact ownership", clpScopeLevel(t, wrapped), wrapped.pops, cleanup.CleanupCause(), f.m.Stats().LiveReferences, before)
	}
	if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message {
		t.Fatal("cleanup retry changed failure evidence")
	}
	status, err = engine.Query(f.app["P"])
	if err != nil || status != lbool.True || clpScopeLevel(t, wrapped) != 0 {
		t.Fatal("solver was not reusable", status, err, clpScopeLevel(t, wrapped))
	}
}

func TestCLPRuleScopeRecognizesPopConsumedBeforeFailure(t *testing.T) {
	f := newFixture(t)
	refused := errors.New("CLP pop reported failure after consumption")
	wrapped := &clpScopeRefusalSolver{Solver: f.solver, popAfter: map[uint32]error{1: refused}}
	engine, err := clp.NewEngine(f.m, f.set, wrapped, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	}()
	status, failure := engine.Query(f.app["P"])
	if status != lbool.Undef || !errors.Is(failure, refused) || clpScopeLevel(t, wrapped) != 0 || wrapped.pops != 1 {
		t.Fatal("late pop failure published result", status, failure, clpScopeLevel(t, wrapped), wrapped.pops)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("late pop failure has no retry owner", failure)
	}
	if err = cleanup.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if err = cleanup.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if clpScopeLevel(t, wrapped) != 0 || wrapped.pops != 1 {
		t.Fatal("late pop failure was popped twice", clpScopeLevel(t, wrapped), wrapped.pops)
	}
	status, err = engine.Query(f.app["P"])
	if err != nil || status != lbool.True || wrapped.pops != 2 {
		t.Fatal("solver did not recover after late pop failure", status, err, wrapped.pops)
	}
}

func TestCLPNestedRuleScopeCleanupPrecedesParentAndPreservesCallerDepth(t *testing.T) {
	f := newFixture(t)
	if err := f.solver.Push(); err != nil {
		t.Fatal(err)
	}
	inner := errors.New("inner CLP pop refused")
	innerRetry := errors.New("inner CLP retry pop refused")
	outer := errors.New("outer CLP pop refused")
	wrapped := &clpScopeRefusalSolver{Solver: f.solver, popBefore: map[uint32]error{1: inner, 2: innerRetry, 4: outer}}
	engine, err := clp.NewEngine(f.m, f.set, wrapped, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	}()
	status, failure := engine.Query(f.app["Q"])
	if status != lbool.Undef || !errors.Is(failure, inner) || !errors.Is(failure, innerRetry) || !errors.Is(failure, outer) {
		t.Fatal("nested cleanup lost failure evidence", status, failure)
	}
	if clpScopeLevel(t, wrapped) != 2 || wrapped.pops != 4 {
		t.Fatal("nested cleanup lost its pending parent scope", clpScopeLevel(t, wrapped), wrapped.pops)
	}
	wantLevels := []uint32{3, 3, 3, 2}
	if len(wrapped.popLevels) != len(wantLevels) {
		t.Fatal("nested cleanup pop trace", wrapped.popLevels)
	}
	for i := range wantLevels {
		if wrapped.popLevels[i] != wantLevels[i] {
			t.Fatal("nested cleanup popped parent before child", wrapped.popLevels, wantLevels)
		}
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("nested cleanup has no retry owner", failure)
	}
	original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	if err = cleanup.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if err = cleanup.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if clpScopeLevel(t, wrapped) != 1 || wrapped.pops != 5 || cleanup.CleanupCause() != nil {
		t.Fatal("nested cleanup did not restore caller depth", clpScopeLevel(t, wrapped), wrapped.pops, cleanup.CleanupCause())
	}
	if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message {
		t.Fatal("nested retry changed failure evidence")
	}
	status, err = engine.Query(f.app["Q"])
	if err != nil || status != lbool.True || clpScopeLevel(t, wrapped) != 1 {
		t.Fatal("nested cleanup did not leave solver reusable", status, err, clpScopeLevel(t, wrapped))
	}
	if err = f.solver.Pop(1); err != nil {
		t.Fatal(err)
	}
}
