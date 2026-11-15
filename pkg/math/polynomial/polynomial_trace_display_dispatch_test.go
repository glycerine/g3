package polynomial

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// polynomial::display returns immediately for zero. The public display wrapper
// first constructs consistent_coeffs' scoped numeral, even for an empty cell.
// A closed reservation gate distinguishes these source entry points without
// poisoning source numerals or inventing constructor eligibility.
func TestPolynomialTraceDirectDisplayOmitsPublicCheck(t *testing.T) {
	for _, tc := range []struct {
		name, tag string
		run       func(*MultivariatePolynomialManager) error
	}{
		{"delete", "polynomial", func(m *MultivariatePolynomialManager) error { return m.core.traceDeletingPolynomial(m.core.zero) }},
		{"coefficient", "coeff_bug", func(m *MultivariatePolynomialManager) error {
			return m.core.traceCoefficientPolynomial(m.core.zero, 0, 1)
		}},
		{"iccp", "polynomial", func(m *MultivariatePolynomialManager) error { return m.core.traceICCPEntry(m.core.zero, 0) }},
		{"prs", "polynomial_gcd", func(m *MultivariatePolynomialManager) error {
			return m.core.traceGCDPRSEntry(m.core.zero, m.core.zero, 0)
		}},
		{"cra", "CRA", func(m *MultivariatePolynomialManager) error { return m.traceCRAImages(m.core.zero, m.core.zero) }},
		{"univariate", "mgcd", func(m *MultivariatePolynomialManager) error {
			return m.traceUniModGCDPolynomials(3770, "univ_modular_gcd\nu: ", m.core.zero, m.core.zero)
		}},
		{"modular", "mgcd", func(m *MultivariatePolynomialManager) error {
			return m.traceModGCDPolynomials("mgcd", 4239, "mod_gcd\nu: ", "\nv: ", true, m.core.zero, m.core.zero)
		}},
		{"recursive", "mgcd", func(m *MultivariatePolynomialManager) error {
			return m.traceModGCDRecPolynomials(4078, "mod_gcd_rec\nu: ", true, m.core.zero, m.core.zero)
		}},
		{"recursive-detail", "mgcd_detail", func(m *MultivariatePolynomialManager) error {
			z := unumeral.NewInt(0)
			p := m.core.zero
			return m.traceModGCDRecInitial(0, 0, p, p, p, p, p, p, &z, &z)
		}},
		{"zpx", "mgcd_detail", func(m *MultivariatePolynomialManager) error { return m.traceICCPZPXEntry(m.core.zero, 0) }},
		{"psc", "psc_chain_classic", func(m *MultivariatePolynomialManager) error { return m.core.tracePSCEntry(m.core.zero, m.core.zero) }},
		{"lazard", "Lazard", func(m *MultivariatePolynomialManager) error {
			return m.core.traceLazardEntry(m.core.zero, m.core.zero, 0)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			polynomialAssertionActionsFixture(t, "T")
			m := polynomialPSCFullFixture(t)
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			tags.Enable(tc.tag)
			if !tags.ShouldTrace(tc.tag) {
				t.Skip("TRACE compiled out")
			}
			m.core.monomials.config.Trace = tags
			// These source records also print scalar zero through to_string.
			z := unumeral.NewInt(0)
			_, e := m.core.modular.String(&z)
			serviceMust(t, e)
			budget := m.core.integer.Budget()
			budget.FailAfter(0)
			e = tc.run(m)
			budget.FailAfter(-1)
			tags.Disable(tc.tag)
			serviceMust(t, e)
			if out.Len() == 0 {
				t.Fatal("source writer not reached")
			}
			if len(m.core.display.frames) != 0 {
				t.Fatal("direct zero display entered public consistency check")
			}
		})
	}
}

func TestPolynomialTraceReferenceDisplayRetainsPublicCheck(t *testing.T) {
	if !polynomialSourceDebug {
		t.Skip("source assertions compiled out")
	}
	for _, which := range []string{"iccp-reference", "prs-reference", "recursive-reference", "newton-reference"} {
		t.Run(which, func(t *testing.T) {
			polynomialAssertionActionsFixture(t, "T")
			m := polynomialPSCFullFixture(t)
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			tags.Enable("polynomial")
			tags.Enable("polynomial_gcd_detail")
			tags.Enable("mgcd")
			tags.Enable("newton")
			if !tags.ShouldTrace("polynomial") {
				t.Skip("TRACE compiled out")
			}
			m.core.monomials.config.Trace = tags
			n, e := NewPolynomialNewtonInterpolator(m.core)
			serviceMust(t, e)
			t.Cleanup(func() { serviceMust(t, n.Close()) })
			budget := m.core.integer.Budget()
			budget.FailAfter(0)
			switch which {
			case "iccp-reference":
				e = m.core.traceICCPDivisor(m.core.zero, m.core.zero)
			case "prs-reference":
				e = m.core.traceGCDPRSIteration(0, m.core.zero, m.core.zero, 0)
			case "recursive-reference":
				e = m.traceModGCDRecResult(0, m.core.zero)
			case "newton-reference":
				e = n.tracePolynomial(2890, "mk", "u: ", m.core.zero)
			}
			budget.FailAfter(-1)
			tags.Finalize()
			if !errors.Is(e, arena.ErrInjectedFailure) {
				t.Fatalf("public consistency check was skipped: %v", e)
			}
		})
	}
}

func TestPolynomialTraceObjectDirectDisplayOmitsPublicCheck(t *testing.T) {
	polynomialAssertionActionsFixture(t, "T")
	m := polynomialPSCFullFixture(t)
	n, e := NewPolynomialNewtonInterpolator(m.core)
	serviceMust(t, e)
	defer func() { serviceMust(t, n.Close()) }()
	sk, e := NewPolynomialSkeleton(m.core, m.core.zero, 0)
	serviceMust(t, e)
	defer func() { serviceMust(t, sk.CloseChecked()) }()
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("newton")
	tags.Enable("skeleton")
	if !tags.ShouldTrace("newton") {
		t.Skip("TRACE compiled out")
	}
	m.core.monomials.config.Trace = tags
	defer tags.Finalize()
	z := unumeral.NewInt(0)
	_, e = m.core.modular.String(&z)
	serviceMust(t, e)
	budget := m.core.integer.Budget()
	budget.FailAfter(0)
	e = n.traceAddEntry(&z, m.core.zero)
	if e == nil {
		e = sk.traceConstruction(m.core.zero)
	}
	budget.FailAfter(-1)
	serviceMust(t, e)
	if out.Len() == 0 || len(m.core.display.frames) != 0 {
		t.Fatal("direct object trace entered public polynomial display")
	}
}

// AUDIT.md math_polynomial_04-F2: only the mod_gcd entry records pass
// use_star = true; the polynomial_ref records use the default false, and
// the second u_Zp/v_Zp operand is labelled "v_Zp".
func TestModGCDTraceRefRecordsUseNoStar(t *testing.T) {
	m := polynomialPSCFullFixture(t)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("mgcd")
	if !tags.ShouldTrace("mgcd") {
		t.Skip("TRACE compiled out")
	}
	m.core.monomials.config.Trace = tags
	u := polynomialQueryTerms(t, m.core, []int32{2, 1}, [][]MonomialPower{{{0, 1}, {1, 1}}, nil})
	v := polynomialQueryTerms(t, m.core, []int32{1, 1}, [][]MonomialPower{{{0, 1}}, {{1, 1}}})
	q := polynomialQueryTerms(t, m.core, []int32{2}, [][]MonomialPower{{{0, 1}, {1, 1}}})
	for _, tc := range []struct {
		run  func() error
		want string
	}{
		{func() error { return m.traceModGCDPolynomials("mgcd", 4301, "u_Zp: ", "\nv_Zp: ", false, u, v) }, "u_Zp: 2 x0 x1 + 1\nv_Zp: x1 + x0\n"},
		{func() error { return m.traceModGCDPolynomials("mgcd", 4308, "new q:\n", "", false, q) }, "new q:\n2 x0 x1\n"},
		{func() error { return m.traceModGCDPolynomials("mgcd", 4239, "mod_gcd\nu: ", "\nv: ", true, u, v) }, "mod_gcd\nu: 2*x0*x1 + 1\nv: x1 + x0\n"},
	} {
		out.Reset()
		serviceMust(t, tc.run())
		if !bytes.Contains(out.Bytes(), []byte(tc.want)) {
			t.Fatalf("trace %q does not contain %q", out.String(), tc.want)
		}
	}
}
