package duality

import (
	"errors"
	"testing"
)

func TestContextWithProfilerRestoresPreviousOnError(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	previous := NewProfiler(nil)
	active := NewProfiler(nil)
	ctx.activeProfiler = previous
	failure := errors.New("profiled operation failed")
	err := ctx.WithProfiler(active, func() error {
		scope, err := startProfile(ctx, "operation")
		if err != nil {
			return err
		}
		if err := scope.Close(); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) || ctx.activeProfiler != previous || active.current != active.top || active.top.children["operation"] == nil {
		t.Fatal("profiler was not restored after error", err)
	}
}
