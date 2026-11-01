package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyProjectionConstructorAllocationFrontiers(t *testing.T) {
	for _, name := range []string{"array", "arithmetic"} {
		t.Run(name, func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 1024 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := newFMFixture(t)
					m := f.r.manager
					fmMust(t, m.RegisterDeclarationPlugins())
					m.Budget().FailAfter(stop)
					var closeOwner func() error
					var err error
					if name == "array" {
						var owner *legacyArrayProjection
						owner, err = newLegacyArrayProjection(m, f.r.config)
						if owner != nil {
							closeOwner = owner.Close
						}
					} else {
						var owner *legacyArithmeticProjection
						owner, err = newLegacyArithmeticProjection(m, f.r.config)
						if owner != nil {
							closeOwner = owner.Close
						}
					}
					m.Budget().FailAfter(-1)
					if closeOwner != nil {
						fmMust(t, closeOwner())
					}
					if err == nil {
						success = true
						return
					}
					var construction memory.ConstructionFailure
					if errors.As(err, &construction) {
						fmMust(t, construction.Cleanup())
					}
					var memout memory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
						t.Fatal(err)
					}
					failures++
				})
				if t.Failed() {
					t.Fatal("constructor allocation frontier", stop)
				}
			}
			if !success || failures == 0 {
				t.Fatal("did not reach both failure and success", failures, success)
			}
		})
	}
}
