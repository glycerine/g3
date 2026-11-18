package polynomial

import (
	"fmt"
	"strings"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// upolySmallKernelWorkload runs GCD (modular images, Zp Euclid, CRA),
// multiplication, division and factorization (Berlekamp, Hensel lifting,
// recombination) on polynomials with big and small coefficients, and
// returns every result as text. With armed set an injected failure is
// armed (but never reached), which turns the small and direct coefficient
// paths off, so the full numeral operations run instead.
func upolySmallKernelWorkload(t *testing.T, armed bool) string {
	t.Helper()
	q, s := preparationPhysicalFixture(t)
	m, z := s.manager, q.Integers()
	if armed {
		z.BudgetField.FailAfter(1 << 40)
	}
	if got := m.smallOps().direct; got == armed {
		if armed || m.modular.DirectKernel() {
			t.Fatal("direct coefficient paths selected", got, "with an armed injection", armed)
		}
	}
	poly := func(cs ...int64) UPolynomial {
		p := upolySourceNew(t, m, nil)
		vals := make([]unumeral.Int, len(cs))
		for i, c := range cs {
			serviceMust(t, z.SetInt64(&vals[i], c))
		}
		serviceMust(t, m.SetInts(p, vals))
		for i := range vals {
			serviceMust(t, z.Del(&vals[i]))
		}
		return p
	}
	power := func(base UPolynomial, k int) UPolynomial {
		out := poly(1)
		for i := 0; i < k; i++ {
			serviceMust(t, m.Mul(out, base, out))
		}
		return out
	}
	var text strings.Builder
	show := func(name string, p UPolynomial) {
		n, e := m.Size(p)
		serviceMust(t, e)
		fmt.Fprintf(&text, "%s:", name)
		for i := uint32(0); i < n; i++ {
			var v unumeral.Int
			serviceMust(t, m.ReadCoefficient(p, i, &v))
			str, e := z.String(&v)
			serviceMust(t, e)
			serviceMust(t, z.Del(&v))
			fmt.Fprintf(&text, " %s", str)
		}
		text.WriteString("\n")
	}
	shifted := poly(12345678901, 1)
	a := power(shifted, 6)
	serviceMust(t, m.Mul(a, poly(-7, 0, 1), a))
	serviceMust(t, m.Mul(a, poly(3, 2), a))
	b := power(shifted, 4)
	serviceMust(t, m.Mul(b, poly(5, 0, 0, 1), b))
	show("a", a)
	show("b", b)
	g := poly()
	serviceMust(t, m.GCD(a, b, g))
	show("gcd", g)
	prod := poly()
	serviceMust(t, m.Mul(a, b, prod))
	show("prod", prod)
	quo, rem := poly(), poly()
	var d uint32
	serviceMust(t, m.DivRem(prod, b, quo, rem, &d))
	show("quo", quo)
	show("rem", rem)
	r2 := poly()
	serviceMust(t, m.Rem(a, b, r2, &d))
	show("rem2", r2)
	fmt.Fprintf(&text, "d: %d\n", d)
	// y^24 - 3^7 (irreducible): Berlekamp, Hensel lifting and recombination.
	high := make([]int64, 25)
	high[0], high[24] = -2187, 1
	for i, p := range []UPolynomial{prod, poly(high...)} {
		fs, e := m.NewFactors()
		serviceMust(t, e)
		full, e := s.Factor(mustUPolyView(t, m, p), fs, DefaultUPolynomialFactorParams())
		serviceMust(t, e)
		fmt.Fprintf(&text, "factor %d full=%v: ", i, full)
		serviceMust(t, fs.DisplayInto(factorizationTraceStream{&text}))
		text.WriteString("\n")
		serviceMust(t, fs.Close())
	}
	// Zp mode: a field Euclid with an inline, a word (2^61-1) and a big
	// modulus.
	var p unumeral.Int
	for i, prime := range []string{"40009", "2305843009213693951", "1606938044258990275541962092341162602522202993782792835301611"} {
		serviceMust(t, z.SetString(&p, prime))
		serviceMust(t, m.modular.SetZP(&p))
		gz := poly()
		serviceMust(t, m.GCD(a, b, gz))
		show(fmt.Sprintf("gcd mod %d", i), gz)
		mz := poly()
		serviceMust(t, m.Mul(a, b, mz))
		show(fmt.Sprintf("prod mod %d", i), mz)
		serviceMust(t, m.modular.SetZ())
	}
	serviceMust(t, z.Del(&p))
	return text.String()
}

// The small and direct coefficient paths publish exactly the values of the
// full numeral operations.
func TestUPolynomialSmallKernelMatchesFullOperations(t *testing.T) {
	fast := upolySmallKernelWorkload(t, false)
	full := upolySmallKernelWorkload(t, true)
	if fast != full {
		t.Fatalf("coefficient paths differ\nfast:\n%s\nfull:\n%s", fast, full)
	}
	if !strings.Contains(full, "factor 1 full=") {
		t.Fatal(full)
	}
}
