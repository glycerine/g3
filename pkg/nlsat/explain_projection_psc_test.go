package nlsat

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"testing"
)

func TestProjectionPSCKeepsVanishingFactorsAndStopsAtConstant(t *testing.T) {
	for _, value := range []int64{0, 2} {
		e, _, n, todo := projectionCoefficientFixture(t, false)
		evaluatorSetFraction(t, e, 0, value, 1)
		p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
		q := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{1, 1, 0})
		pRef, qRef := vanishingRef(t, e, p), vanishingRef(t, e, q)
		assignmentMust(t, n.collectProjectionPSC(pRef, qRef, 1))
		values := projectionValues(t, todo.set)
		if len(values) != 1 || len(n.output.result) != 0 {
			t.Fatal("expected one cell boundary before the constant PSC", values, n.output.result)
		}
		degree, err := e.core.Degree(values[0], 0)
		assignmentMust(t, err)
		if degree != 1 {
			t.Fatal("wrong selected resultant", degree)
		}
	}
}
func TestProjectionPSCZeroChainAndPairwiseResultants(t *testing.T) {
	e, _, n, todo := projectionCoefficientFixture(t, true)
	evaluatorSetFraction(t, e, 0, 3, 1)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
	q := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{1, 1, 0})
	s := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{1, 0, 0})
	pRef := vanishingRef(t, e, p)
	assignmentMust(t, n.collectProjectionPSC(pRef, pRef, 1))
	if len(projectionValues(t, todo.set)) != 0 || len(n.output.result) != 0 {
		t.Fatal("identical inputs added a zero/constant PSC")
	}
	ps := projectionOutput(t, e)
	for _, p := range []polynomial.Polynomial{p, q, s} {
		assignmentMust(t, ps.Push(p))
	}
	assignmentMust(t, n.collectProjectionResultants(ps, 1))
	values := projectionValues(t, todo.set)
	// p/q yields y, p/s is constant, q/s yields y-1 (up to sign).
	if len(values) != 2 || len(n.output.result) != 0 {
		t.Fatal("pairwise resultant frontier", values, n.output.result)
	}
	for _, p := range values {
		degree, err := e.core.Degree(p, 0)
		assignmentMust(t, err)
		if degree != 1 {
			t.Fatal("nonlinear/constant added by linear pair", degree)
		}
	}
	if len(projectionValues(t, ps)) != 3 {
		t.Fatal("pairwise borrowed vector changed")
	}
}

func TestProjectionPSCRefusalPreservesOlderCacheScope(t *testing.T) {
	e, r, n, _ := projectionCoefficientFixture(t, true)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 2}, evaluatorTerm{1, 1, 1}, evaluatorTerm{1, 0, 1}, evaluatorTerm{2, 0, 0})
	q := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 1}, evaluatorTerm{3, 0, 0})
	pRef, qRef := vanishingRef(t, e, p), vanishingRef(t, e, q)
	oldOut := projectionOutput(t, e)
	sourceStop, ownStop := errors.New("old PSC source destructor stop"), errors.New("old PSC owning destructor stop")
	armed, calls := true, 0
	inLexicalCleanup := false
	minusQ := evaluatorPolynomial(t, e, evaluatorTerm{-1, 1, 1}, evaluatorTerm{-3, 0, 0})
	h, err := polynomial.NewPolynomialDeletionHandler(e.budget, func(_ *polynomial.PolynomialDeletionHandler, _ *polynomial.MultivariatePolynomialCore, value polynomial.Polynomial) error {
		if !armed {
			return nil
		}
		if !inLexicalCleanup {
			equal, err := e.core.Equal(value, minusQ)
			if err != nil {
				return err
			}
			if !equal {
				return nil
			}
			inLexicalCleanup = true
			calls++
			return sourceStop
		}
		calls++
		return ownStop
	})
	assignmentMust(t, err)
	assignmentMust(t, e.core.AddDeletionHandler(h))
	t.Cleanup(func() { assignmentMust(t, e.core.RemoveDeletionHandler(h)); assignmentMust(t, h.CloseChecked()) })
	pc := r.store.atoms.ineq.polynomials
	err = pc.PSCChain(p, q, 1, oldOut)
	armed = false
	if err == nil || calls < 2 {
		t.Fatal("actual independent cache continuation not reached", err, calls)
	}
	if _, err := pc.Manager(); err == nil {
		t.Fatal("old cache scope did not remain pending")
	}
	beforeCalls := calls
	runtime := e.ism.allocator.RuntimeOwner()
	used, state, stats := e.budget.Used(), runtime.State(), runtime.Stats()
	err = n.collectProjectionPSC(pRef, qRef, 1)
	if err == nil || n.psc.active || n.resultActive {
		t.Fatal("refused explanation adopted cache invocation", err)
	}
	assignmentMust(t, n.cleanup())
	if calls != beforeCalls || e.budget.Used() != used || runtime.State() != state || runtime.Stats() != stats {
		t.Fatal("outer cleanup changed independent cache ownership", calls)
	}
	if _, err := pc.Manager(); err == nil {
		t.Fatal("outer cleanup acknowledged old cache")
	}
	assignmentMust(t, pc.CleanupPSC())
	_, err = pc.Manager()
	assignmentMust(t, err)
}

func TestProjectionPSCStopsAfterFirstNonconstant(t *testing.T) {
	e, _, n, todo := projectionCoefficientFixture(t, false)
	evaluatorSetFraction(t, e, 0, 1, 1)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{1, 1, 0})
	q := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 2}, evaluatorTerm{1, 1, 1}, evaluatorTerm{1, 0, 0})
	assignmentMust(t, n.collectProjectionPSC(vanishingRef(t, e, p), vanishingRef(t, e, q), 1))
	values := projectionValues(t, todo.set)
	if len(values) != 1 || len(n.output.result) != 0 {
		t.Fatal("PSC continued beyond first nonvanishing nonconstant", values, n.output.result)
	}
	degree, err := e.core.Degree(values[0], 0)
	assignmentMust(t, err)
	if degree != 4 {
		t.Fatal("did not select quartic resultant before later PSC", degree)
	}
}
