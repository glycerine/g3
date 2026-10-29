package polynomial

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestAlgebraicPurificationRepresentationSourceOrder(t *testing.T) {
	c, p := algebraicFixture(t)
	poly := algebraicPolynomial(t, p, -2, 0, 1)
	v, e := p.wholeView(poly)
	serviceMust(t, e)
	b := c.config.Binary
	var lo, hi unumeral.BinaryRational
	serviceMust(t, b.SetInt32(&lo, 1))
	serviceMust(t, b.SetInt32(&hi, 2))
	defer func() { serviceMust(t, b.Del(&hi)); serviceMust(t, b.Del(&lo)) }()
	var a Algebraic
	serviceMust(t, c.SetRoot(&a, v, &lo, &hi, true))
	defer func() { serviceMust(t, c.Del(&a)) }()
	coeff, e := c.DefiningPolynomialText(&a)
	serviceMust(t, e)
	if len(coeff) != 3 || coeff[0] != "-2" || coeff[1] != "0" || coeff[2] != "1" {
		t.Fatal(coeff)
	}
	lower, upper, e := c.IrrationalBoundsText(&a)
	serviceMust(t, e)
	if lower != "1" || upper != "2" {
		t.Fatal(lower, upper)
	}
}
