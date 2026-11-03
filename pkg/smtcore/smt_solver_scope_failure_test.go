package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTSolverFailedPushReportsOnlyPublishedScopes(t *testing.T) {
	failedBeforePublication, completed := false, false
	for _, direct := range []bool{false, true} {
		for _, allowance := range []int64{0, 1, 4, 16, 64, 256, 1024} {
			t.Run(fmt.Sprintf("direct=%v/allowance=%d", direct, allowance), func(t *testing.T) {
				m, s := smtSolverFixture(t)
				if direct {
					astTestMust(t, s.PushCore())
				}
				base := s.ScopeLevelCore()
				p := smtRelevancyBool(t, m, "scope-failure-p")
				astTestMust(t, s.Assert(p))
				m.Budget().FailAfter(allowance)
				failure := s.Push()
				m.Budget().FailAfter(-1)
				if failure != nil && !errors.Is(failure, arena.ErrInjectedFailure) {
					t.Fatal("unexpected push failure", failure)
				}
				level, err := s.ScopeLevel()
				astTestMust(t, err)
				if level+base != s.ScopeLevelCore() || s.ScopeLevelCore() != s.kernel.ScopeLevel() {
					t.Fatal("push published inconsistent scopes", level, base, s.ScopeLevelCore(), s.kernel.ScopeLevel(), failure)
				}
				if failure == nil && level != 1 {
					t.Fatal("successful push omitted named scope", level)
				}
				failedBeforePublication = failedBeforePublication || failure != nil && level == 0
				completed = completed || failure == nil
				if level != 0 {
					astTestMust(t, s.Pop(level))
				}
				if direct {
					astTestMust(t, s.PopCore(1))
				}
			})
		}
	}
	if !failedBeforePublication || !completed {
		t.Fatal("push campaign missed failure or publication", failedBeforePublication, completed)
	}
}

// Stage the exact frontier after the kernel consumed a scope but before its
// solver bookkeeping retires. Native callback refusal is covered by the BMC
// consumer; this checks that retry and destruction never pop that scope twice.
func TestSMTSolverPendingPopResumesAfterKernelPublication(t *testing.T) {
	for _, operation := range []string{"pop", "close", "decref"} {
		t.Run(operation, func(t *testing.T) {
			m, s := smtSolverFixture(t)
			if operation == "decref" {
				astTestMust(t, s.IncRef())
			}
			astTestMust(t, s.Push())
			astTestMust(t, s.kernel.Pop(1))
			s.pendingPop = &smtSolverPendingPop{count: 1, kernelPopped: true, kernelCleanup: true}
			level, err := s.ScopeLevel()
			astTestMust(t, err)
			if level != 1 || s.ScopeLevelCore() != 1 || s.kernel.ScopeLevel() != 0 {
				t.Fatal("pending pop lost its visible cleanup scope", level, s.ScopeLevelCore(), s.kernel.ScopeLevel())
			}
			if err = s.Push(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("pending pop accepted a new scope", err)
			}
			truth, err := m.MkTrue()
			astTestMust(t, err)
			if err = s.Assert(truth); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("pending pop accepted an assertion", err)
			}
			switch operation {
			case "pop":
				astTestMust(t, s.Pop(1))
				level, err = s.ScopeLevel()
				astTestMust(t, err)
				if level != 0 || s.ScopeLevelCore() != 0 || s.pendingPop != nil {
					t.Fatal("pop retry left bookkeeping", level, s.ScopeLevelCore(), s.pendingPop)
				}
				astTestMust(t, s.Push())
				astTestMust(t, s.Pop(1))
			case "close":
				astTestMust(t, s.Close())
			case "decref":
				astTestMust(t, s.DecRef())
			}
		})
	}
}

func TestSMTSolverCloseDoesNotReplayPartiallyConsumedKernelPop(t *testing.T) {
	_, s := smtSolverFixture(t)
	astTestMust(t, s.Push())
	// Model a refusal after the first native Pop stage consumed its scope,
	// before the kernel's top-level scope vector could be truncated.
	astTestMust(t, s.kernel.internalizer.PopScopes(1))
	s.pendingPop = &smtSolverPendingPop{count: 1}
	astTestMust(t, s.Close())
	astTestMust(t, s.Close())
}
