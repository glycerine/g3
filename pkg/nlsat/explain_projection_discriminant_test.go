package nlsat

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"testing"
)

func TestProjectionDiscriminantsQuadraticCubicAndLinearSkip(t *testing.T) {
	for _, factor := range []bool{false, true} {
		e, _, n, todo := projectionCoefficientFixture(t, factor)
		evaluatorSetFraction(t, e, 0, 2, 1)
		ps := projectionOutput(t, e)
		// x^2-y and x^3-y have discriminant zero locus y=0.
		p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{-1, 1, 0})
		q := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 3}, evaluatorTerm{-1, 1, 0})
		linear := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{1, 1, 0})
		for _, p := range []polynomial.Polynomial{linear, p, q, linear} {
			assignmentMust(t, ps.Push(p))
		}
		assignmentMust(t, n.collectProjectionDiscriminants(ps, 1))
		values := projectionValues(t, todo.set)
		if len(values) < 1 || len(values) > 2 || (!factor && len(values) != 2) || len(n.output.result) != 0 {
			t.Fatal("discriminant root frontier", values, n.output.result)
		}
		if factor {
			for _, p := range values {
				degree, err := e.core.Degree(p, 0)
				assignmentMust(t, err)
				if degree != 1 {
					t.Fatal("discriminant factor multiplicity was not dropped", degree)
				}
			}
		}
		if len(projectionValues(t, ps)) != 4 || n.discriminant.active {
			t.Fatal("borrowed vector or caller scope")
		}
	}
}
func TestProjectionDiscriminantVanishingAndRepeatedRoot(t *testing.T) {
	for _, repeated := range []bool{false, true} {
		e, _, n, todo := projectionCoefficientFixture(t, true)
		evaluatorSetFraction(t, e, 0, 0, 1)
		ps := projectionOutput(t, e)
		p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{-1, 1, 0})
		if repeated {
			p = evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{2, 1, 1}, evaluatorTerm{1, 2, 0})
		}
		assignmentMust(t, ps.Push(p))
		assignmentMust(t, n.collectProjectionDiscriminants(ps, 1))
		want := 1
		if repeated {
			// An identically zero discriminant introduces no boundary.
			want = 0
		}
		if got := len(projectionValues(t, todo.set)); got != want {
			t.Fatalf("got %d discriminant factors in the cell, want %d", got, want)
		}
	}
}

func TestProjectionDiscriminantAllocationStopsAndRetry(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 256 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			e, r, n, todo := projectionCoefficientFixture(t, true)
			evaluatorSetFraction(t, e, 0, 2, 1)
			p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 3}, evaluatorTerm{-1, 1, 0})
			ps := projectionOutput(t, e)
			assignmentMust(t, ps.Push(p))
			e.budget.FailAfter(stop)
			err := n.collectProjectionDiscriminants(ps, 1)
			e.budget.FailAfter(-1)
			if err == nil {
				success = true
				return
			}
			failures++
			var oom umemory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &oom) && oom.Kind == umemory.FailureMemout && oom.ErrorCode() == 101) {
				t.Fatal("unexpected discriminant stop", stop, err)
			}
			if err := r.newLevel(); !errors.Is(err, ErrSearchPending) {
				t.Fatal("pending discriminant allowed parent", err)
			}
			assignmentMust(t, e.am.CleanupRetained())
			assignmentMust(t, r.store.atoms.ineq.polynomials.CleanupPSC())
			assignmentMust(t, r.store.atoms.ineq.polynomials.CleanupFactor())
			assignmentMust(t, r.store.atoms.ineq.polynomials.CleanupUnique())
			assignmentMust(t, r.store.atoms.ineq.pm.CleanupFlipSignIfLeadingMonomialNegative())
			assignmentMust(t, r.store.atoms.cleanup())
			assignmentMust(t, n.cleanup())
			assignmentMust(t, todo.cleanup())
			assignmentMust(t, todo.reset())
			assignmentMust(t, r.store.atoms.ineq.polynomials.Reset())
			if n.discriminant.active || n.psc.active {
				t.Fatal("owning scopes remained", stop)
			}
			assignmentMust(t, n.collectProjectionDiscriminants(ps, 1))
			if len(projectionValues(t, todo.set)) == 0 {
				t.Fatal("actual retry produced no discriminant", stop)
			}
		})
	}
	if !success || failures < 2 {
		t.Fatal("discriminant horizon incomplete", success, failures)
	}
	t.Log("consecutive failures", failures)
}
