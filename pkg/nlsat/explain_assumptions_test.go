package nlsat

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"reflect"
	"testing"
)

func explanationAssumptionFixture(t *testing.T) (*Evaluator, *searchState, *explanationAssumptions) {
	t.Helper()
	e, s, limit := clauseStoreFixtureConfigured(t, true)
	r, err := newSearchState(s, e, searchConfig{Limit: limit, MaxMemory: uint64(^uint32(0)), MaxConflicts: ^uint32(0)})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, r.close()) })
	n, err := newExplanationAssumptions(r)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, n.close()) })
	return e, r, n
}
func TestExplanationZeroAssumptionSelectsOnlyVanishingFactors(t *testing.T) {
	for _, value := range []int64{-1, 1, 2} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			e, r, n := explanationAssumptionFixture(t)
			evaluatorSetFraction(t, e, 0, value, 1)
			// (y-1)(y+1)(y-2) = y^3-2*y^2-y+2.
			p := evaluatorPolynomial(t, e, evaluatorTerm{1, 3, 0}, evaluatorTerm{-2, 2, 0}, evaluatorTerm{-1, 1, 0}, evaluatorTerm{2, 0, 0})
			assignmentMust(t, n.collectZero(p))
			if len(n.output.result) != 1 || n.resultActive || n.localFactor != nil {
				t.Fatal("zero assumption output/lifetime", n.output.result)
			}
			l := n.output.result[0]
			a, ok := r.store.atoms.bools.atoms[l.Variable()].(*InequalityAtom)
			if !ok || a.kind != AtomEQ || !l.Sign() || len(a.factors) != 1 || a.factors[0].Even {
				t.Fatal("zero assumption retained nonvanishing factors or wrong polarity", l, a)
			}
			eval, err := e.Eval(a, l.Sign())
			assignmentMust(t, err)
			if eval {
				t.Fatal("explanation assumption must be false in interpretation")
			}
			degree, err := e.core.Degree(a.factors[0].Polynomial, 0)
			assignmentMust(t, err)
			if degree != 1 {
				t.Fatal("selected product still contains other roots", degree)
			}
			for _, mark := range n.alreadyAdded {
				if mark {
					t.Fatal("completed output did not clear its marks")
				}
			}
		})
	}
}
func TestExplanationAssumptionsCanonicalDuplicatesAndRawSign(t *testing.T) {
	e, r, n := explanationAssumptionFixture(t)
	evaluatorSetFraction(t, e, 0, -2, 1)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 0})
	sign, err := n.sign(p)
	assignmentMust(t, err)
	if sign != -2 {
		t.Fatal("source optimistic rational sign was normalized", sign)
	}
	assignmentMust(t, n.start())
	assignmentMust(t, n.addSimpleAssumption(AtomLT, p, false))
	assignmentMust(t, n.addSimpleAssumption(AtomLT, p, false))
	assignmentMust(t, n.finish(nil))
	if len(n.output.result) != 1 {
		t.Fatal("duplicate literal appended within one result scope", n.output.result)
	}
	l := n.output.result[0]
	if !l.Sign() || arithmeticAtomHeader(r.store.atoms.bools.atoms[l.Variable()]).refs != 1 {
		t.Fatal("literal polarity or scoped output reference")
	}
	assignmentMust(t, n.collectSimple(AtomLT, p, false))
	if len(n.output.result) != 2 || n.output.result[1] != l || arithmeticAtomHeader(r.store.atoms.bools.atoms[l.Variable()]).refs != 2 {
		t.Fatal("new source scope did not append its own occurrence")
	}
	assignmentMust(t, n.reset())
	if !r.store.atoms.bools.dead[l.Variable()] {
		t.Fatal("result reset did not retire atom")
	}
}

func TestExplanationZeroAssumptionKeepsEveryVanishingFactor(t *testing.T) {
	e, r, n := explanationAssumptionFixture(t)
	evaluatorSetFraction(t, e, 0, 0, 1)
	evaluatorSetFraction(t, e, 1, 0, 1)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 1})
	assignmentMust(t, n.collectZero(p))
	if len(n.output.result) != 1 {
		t.Fatal("multi-factor output")
	}
	l := n.output.result[0]
	a, ok := r.store.atoms.bools.atoms[l.Variable()].(*InequalityAtom)
	if !ok || len(a.factors) != 2 {
		t.Fatal("multiple vanishing factors were omitted")
	}
	for _, f := range a.factors {
		if f.Even {
			t.Fatal("source zero factors must be odd")
		}
	}
}

func TestExplanationAssumptionsAllocationStopsAndOwningRetry(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 512; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			e, r, n := explanationAssumptionFixture(t)
			evaluatorSetFraction(t, e, 0, 0, 1)
			p := evaluatorPolynomial(t, e, evaluatorTerm{1, 3, 0}, evaluatorTerm{-1, 1, 0})
			e.budget.FailAfter(stop)
			err := n.collectZero(p)
			e.budget.FailAfter(-1)
			if err == nil {
				success = true
				return
			}
			failures++
			// Explicitly acknowledge the actual borrowed services used by this call.
			// The outer component's cleanup is not given their allocation authority.
			assignmentMust(t, e.am.CleanupRetained())
			assignmentMust(t, r.store.atoms.ineq.polynomials.CleanupFactor())
			assignmentMust(t, r.store.atoms.ineq.polynomials.CleanupUnique())
			assignmentMust(t, r.store.atoms.ineq.pm.CleanupFlipSignIfLeadingMonomialNegative())
			assignmentMust(t, r.store.atoms.cleanup())
			assignmentMust(t, n.cleanup())
			if n.localFactor != nil || n.resultActive || n.failed {
				t.Fatal("assumption lexical owners remained after acknowledged cleanup", stop)
			}
			// The C++ cache publishes a new entry before factorization. An empty
			// interrupted cache hit is preserved; use the actual Reset owner for retry.
			assignmentMust(t, r.store.atoms.ineq.polynomials.Reset())
			assignmentMust(t, n.collectZero(p))
			if len(n.output.result) != 1 {
				t.Fatal("cache-reset retry did not collect actual assumption", n.output.result)
			}
		})
		if success {
			break
		}
	}
	if !success || failures < 2 {
		t.Fatal("did not cover factor/scoped-reference frontiers through success", success, failures)
	}
	t.Log("failed consecutive reservations", failures)
}

func TestExplanationAssumptionsCleanupPreservesOlderIntervalScope(t *testing.T) {
	e, r, n := explanationAssumptionFixture(t)
	_, _, _ = searchReasonDomain(t, e, r)
	var older []sat.Literal
	e.budget.FailAfter(1)
	err := r.ism.GetJustifications(r.store.vars.infeasible[1], &older)
	e.budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.ism.justifications == nil || len(older) != 1 {
		t.Fatal("older owner not qualified", err)
	}
	scope := r.ism.justifications
	visited := append([]bool(nil), r.ism.visited...)
	stats, used := r.ism.allocator.RuntimeOwner().Stats(), e.budget.Used()
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 0})
	// Polynomial construction precedes the captured memory state for this probe.
	stats, used = r.ism.allocator.RuntimeOwner().Stats(), e.budget.Used()
	if err := n.collectSimple(AtomEQ, p, false); err == nil {
		t.Fatal("accepted another caller's pending marks")
	}
	assignmentMust(t, n.cleanup())
	if r.ism.justifications != scope || !reflect.DeepEqual(visited, r.ism.visited) || r.ism.allocator.RuntimeOwner().Stats() != stats || e.budget.Used() != used {
		t.Fatal("assumption cleanup adopted unrelated interval work")
	}
	assignmentMust(t, r.ism.Cleanup())
}
func TestExplanationAssumptionsConstructorAndSourceGuards(t *testing.T) {
	atomDebugFixture(t)
	for _, stop := range []int64{0, 1} {
		_, r := searchStateFixture(t)
		b := r.store.atoms.ineq.budget
		before := b.Used()
		b.FailAfter(stop)
		n, err := newExplanationAssumptions(r)
		b.FailAfter(-1)
		if !errors.Is(err, arena.ErrInjectedFailure) {
			t.Fatal("constructor refusal", stop, err)
		}
		if n != nil {
			assignmentMust(t, n.cleanup())
			if err := r.newLevel(); !errors.Is(err, ErrSearchPending) {
				t.Fatal("incomplete constructor allowed search", err)
			}
			assignmentMust(t, n.close())
		}
		if r.explanationAssumptions != nil || b.Used() != before {
			t.Fatal("partial constructor retained storage", stop)
		}
	}
	_, r, n := explanationAssumptionFixture(t)
	assignmentMust(t, n.start())
	assignmentMust(t, n.addLiteral(FalseLiteral))
	assignmentMust(t, n.finish(nil))
	if len(n.output.result) != 0 {
		t.Fatal("false assumption was stored")
	}
	assignmentMust(t, r.close())
	if !n.closed {
		t.Fatal("parent did not close assumption generator")
	}
}

func TestExplanationAssumptionsResetCallbackCannotReenter(t *testing.T) {
	e, r, n := explanationAssumptionFixture(t)
	p, err := e.core.MkVariable(0, 7)
	assignmentMust(t, err)
	assignmentMust(t, n.collectSimple(AtomGT, p, false))
	assignmentMust(t, r.store.atoms.ineq.polynomials.Reset())
	stop := errors.New("stop actual assumption factor destruction")
	var searchErr, cleanupErr error
	calls := 0
	h, err := polynomial.NewPolynomialDeletionHandler(e.budget, func(_ *polynomial.PolynomialDeletionHandler, _ *polynomial.MultivariatePolynomialCore, v polynomial.Polynomial) error {
		if v == p {
			calls++
			searchErr = r.newLevel()
			cleanupErr = n.cleanup()
			return stop
		}
		return nil
	})
	assignmentMust(t, err)
	assignmentMust(t, e.core.AddDeletionHandler(h))
	t.Cleanup(func() { assignmentMust(t, e.core.RemoveDeletionHandler(h)); assignmentMust(t, h.CloseChecked()) })
	err = n.reset()
	if !errors.Is(err, stop) || calls != 1 || !errors.Is(searchErr, ErrSearchPending) || !errors.Is(cleanupErr, ErrSearchPending) {
		t.Fatal("reset allowed reentry before its consumed DEC completed", err, calls, searchErr, cleanupErr)
	}
	assignmentMust(t, n.cleanup())
	if calls != 1 || len(n.output.result) != 0 || n.failed || n.output.resetting {
		t.Fatal("owning continuation replayed the actual deletion", calls)
	}
}

func TestExplanationSimpleAssumptionsBothPolaritiesAndLeadingSign(t *testing.T) {
	for _, kind := range []AtomKind{AtomEQ, AtomLT, AtomGT} {
		for _, positive := range []bool{false, true} {
			t.Run(fmt.Sprint(kind, positive), func(t *testing.T) {
				e, r, n := explanationAssumptionFixture(t)
				evaluatorSetFraction(t, e, 0, -2, 1)
				p := evaluatorPolynomial(t, e, evaluatorTerm{-1, 1, 0}) // p evaluates to +2.
				assignmentMust(t, n.collectSimple(kind, p, positive))
				if len(n.output.result) != 1 {
					t.Fatal("simple assumption output")
				}
				l := n.output.result[0]
				a := r.store.atoms.bools.atoms[l.Variable()]
				got, err := e.Eval(a, l.Sign())
				assignmentMust(t, err)
				want := kind == AtomGT
				if !positive {
					want = !want
				}
				if got != want || l.Sign() == positive {
					t.Fatal("source relation/polarity after leading-sign normalization", kind, positive, got, want)
				}
			})
		}
	}
}
func TestExplanationAssumptionLiteralMarksIncludePolarity(t *testing.T) {
	e, _, n := explanationAssumptionFixture(t)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 0})
	assignmentMust(t, n.start())
	assignmentMust(t, n.addSimpleAssumption(AtomEQ, p, false))
	assignmentMust(t, n.addSimpleAssumption(AtomEQ, p, true))
	assignmentMust(t, n.addSimpleAssumption(AtomEQ, p, false))
	assignmentMust(t, n.finish(nil))
	if len(n.output.result) != 2 || n.output.result[0] != n.output.result[1].Negated() {
		t.Fatal("literal-index marking collapsed opposite polarities", n.output.result)
	}
}
