package polynomial

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestAlgebraicMultivariateIsolationPreservesOlderFactorInvocation(t *testing.T) {
	found := false
	for allowance := uint64(0); allowance < 512 && !found; allowance++ {
		t.Run(strconv.FormatUint(allowance, 10), func(t *testing.T) {
			c, up, control := algebraicControlledFixture(t)
			pm, p, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{0, 0, 2}}, {-2, [3]uint32{}}})
			values := &algebraicIsolationAssignments{}
			roots, e := c.NewVector()
			serviceMust(t, e)
			defer func() { serviceMust(t, roots.Close()) }()
			factors, e := up.NewFactors()
			serviceMust(t, e)
			defer func() { serviceMust(t, c.config.Factors.CleanupRetained()); serviceMust(t, factors.Close()) }()
			poly := algebraicPolynomial(t, up, -2, 0, 0, 0, 1)
			view, e := up.wholeView(poly)
			serviceMust(t, e)
			control.limit = control.visits + allowance
			_, body := c.config.Factors.Factor(view, factors, c.factorParams)
			control.limit = 1000000
			service := c.config.Factors
			if body == nil || service.scope == nil {
				return
			}
			found = true
			oldReceipt, oldScope := service.frontend.receipt, service.scope
			e = c.IsolateMultivariateRoots(pm, p, values, roots)
			if !errors.Is(e, ErrUPolynomialCleanupPending) {
				t.Fatal("did not refuse older factor invocation", e)
			}
			serviceMust(t, c.CleanupRetained())
			if service.frontend.receipt != oldReceipt || service.scope != oldScope {
				t.Fatal("new isolation cleanup consumed older factor invocation")
			}
		})
	}
	if !found {
		t.Fatal("did not reach actual retained factor invocation")
	}
}

type algebraicIsolationOwnerProbe struct {
	algebraicIsolationAssignments
	t          *testing.T
	core       *AlgebraicRootCore
	pm         *MultivariatePolynomialManager
	roots      *AlgebraicVector
	polynomial Polynomial
	probes     uint32
}

func (v *algebraicIsolationOwnerProbe) Contains(x uint32) (bool, error) {
	v.probes++
	if e := v.core.CleanupRetained(); !errors.Is(e, ErrAlgebraicPending) {
		v.t.Fatal("executing isolation cleanup", e)
	}
	if e := v.core.Close(); !errors.Is(e, ErrAlgebraicPending) {
		v.t.Fatal("executing isolation owner close", e)
	}
	if e := v.pm.CloseChecked(); !errors.Is(e, ErrPolynomialPending) {
		v.t.Fatal("borrowed polynomial owner close", e)
	}
	if e := v.core.IsolateMultivariateRoots(v.pm, v.polynomial, v, v.roots); !errors.Is(e, ErrAlgebraicPending) {
		v.t.Fatal("recursive public isolation", e)
	}
	return v.algebraicIsolationAssignments.Contains(x)
}
func TestAlgebraicMultivariateIsolationExecutingOwnerRefusals(t *testing.T) {
	c, _, _ := algebraicControlledFixture(t)
	pm, p, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{1, 0, 1}}, {-2, [3]uint32{}}})
	roots, e := c.NewVector()
	serviceMust(t, e)
	defer func() { serviceMust(t, roots.Close()) }()
	values := algebraicIsolationOwnerProbe{t: t, core: c, pm: pm, roots: roots, polynomial: p}
	values.assigned[0] = true
	// Null is the actual basic zero assignment; it eliminates x from p.
	serviceMust(t, c.IsolateMultivariateRoots(pm, p, &values, roots))
	if values.probes == 0 || roots.Len() != 0 {
		t.Fatal("substitution probe not reached", values.probes, roots.Len())
	}
}
func TestAlgebraicMultivariateIsolationEmptyOutputAssertionBeforeLocals(t *testing.T) {
	if !polynomialSourceDebug {
		t.Skip("source assertion erased")
	}
	c, _, _ := algebraicControlledFixture(t)
	pm, p, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{1, 0, 1}}})
	roots, e := c.NewVector()
	serviceMust(t, e)
	defer func() { serviceMust(t, roots.Close()) }()
	zero := Algebraic{}
	serviceMust(t, roots.pushTake(&zero))
	polynomialAssertionActionsFixture(t, "T")
	e = c.IsolateMultivariateRoots(pm, p, &algebraicIsolationAssignments{}, roots)
	var exception z3exception.DefaultException
	if !errors.As(e, &exception) || len(c.multivariateIsolationFrames) != 0 || c.isolateTmp1.slot != 0 || roots.Len() != 1 {
		t.Fatal("output assertion after locals", e)
	}
}

func TestPolynomialIsolationCleanupOnlyOwnsMatchingOutput(t *testing.T) {
	found := false
	for allowance := uint64(0); allowance < 512 && !found; allowance++ {
		t.Run(strconv.FormatUint(allowance, 10), func(t *testing.T) {
			c, _, control := algebraicControlledFixture(t)
			pm, p, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{0, 0, 2}}, {-2, [3]uint32{}}})
			otherPM, _, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{0, 0, 1}}})
			roots, e := c.NewVector()
			serviceMust(t, e)
			other, e := c.NewVector()
			serviceMust(t, e)
			defer func() {
				serviceMust(t, c.CleanupRetained())
				serviceMust(t, other.Close())
				serviceMust(t, roots.Close())
			}()
			control.limit = control.visits + allowance
			body := c.IsolateMultivariateRoots(pm, p, &algebraicIsolationAssignments{}, roots)
			control.limit = 1000000
			if body == nil || !c.isolationConversion.active {
				return
			}
			found = true
			serviceMust(t, c.CleanupPolynomialIsolation(otherPM, roots))
			serviceMust(t, c.CleanupPolynomialIsolation(pm, other))
			if !c.isolationConversion.active || c.isolationConversion.ownerRoots != roots {
				t.Fatal("cleaned a different output's conversion")
			}
			serviceMust(t, c.CleanupPolynomialIsolation(pm, roots))
			if c.isolationConversion.active {
				t.Fatal("owned conversion remains active")
			}
			serviceMust(t, roots.Reset())
			serviceMust(t, c.IsolateMultivariateRoots(pm, p, &algebraicIsolationAssignments{}, roots))
			if roots.Len() != 2 {
				t.Fatal("retry roots", roots.Len())
			}
		})
	}
	if !found {
		t.Fatal("did not reach stopped polynomial isolation")
	}
}

func TestMultivariateIsolationCleanupOnlyOwnsMatchingOutput(t *testing.T) {
	found := false
	for allowance := uint64(0); allowance < 1024 && !found; allowance++ {
		t.Run(strconv.FormatUint(allowance, 10), func(t *testing.T) {
			c, up, control := algebraicControlledFixture(t)
			pm, p, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{0, 0, 1}}, {-1, [3]uint32{1, 0, 0}}})
			otherPM, _, _ := polynomialEvalFixtureWithRationals(t, c.config.Rationals, []polynomialEvalTerm{{1, [3]uint32{0, 0, 1}}})
			values := algebraicIsolationAssignmentsFixture(t, c, up, false)
			roots, e := c.NewVector()
			serviceMust(t, e)
			other, e := c.NewVector()
			serviceMust(t, e)
			defer func() {
				serviceMust(t, c.CleanupRetained())
				serviceMust(t, other.Close())
				serviceMust(t, roots.Close())
			}()
			control.limit = control.visits + allowance
			body := c.IsolateMultivariateRoots(pm, p, values, roots)
			control.limit = 1000000
			if body == nil || len(c.multivariateIsolationFrames) == 0 || !c.multivariateIsolationFrames[0].active {
				return
			}
			found = true
			frame := c.multivariateIsolationFrames[0]
			serviceMust(t, c.CleanupPolynomialIsolation(pm, other))
			serviceMust(t, c.CleanupPolynomialIsolation(otherPM, roots))
			if !frame.active || frame.ownerRoots != roots {
				t.Fatal("cleaned another caller's isolation")
			}
			serviceMust(t, c.CleanupPolynomialIsolation(pm, roots))
			if frame.active {
				t.Fatal("matching isolation remains active")
			}
			serviceMust(t, roots.Reset())
			serviceMust(t, c.IsolateMultivariateRoots(pm, p, values, roots))
			if roots.Len() != 1 {
				t.Fatal("retry root count", roots.Len())
			}
		})
	}
	if !found {
		t.Fatal("did not reach stopped multivariate isolation")
	}
}
