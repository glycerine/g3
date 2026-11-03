package polynomial

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestUPolynomialFiniteFieldSquareFreeTraceRecords(t *testing.T) {
	m, _, _ := upolySourceOwner(t)
	serviceMust(t, m.SetZPUint64(5))
	f := upolySourceNew(t, m, []int32{-1, 0, 1})
	fs, e := m.NewFactors()
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, fs.Close()) })
	var out strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("polynomial::factorization")
	tags.Enable("polynomial::factorization::bughunt")
	serviceMust(t, m.modular.BindTrace(tags))
	t.Cleanup(func() { serviceMust(t, m.modular.BindTrace(nil)) })

	serviceMust(t, m.SquareFreeFactorZP(mustUPolyView(t, m, f), fs))
	if !tags.ShouldTrace("polynomial::factorization") {
		if out.Len() != 0 {
			t.Fatal("erased finite-field TRACE executed", out.String())
		}
		return
	}
	text := out.String()
	ordered := []string{
		"upolynomial_factorization.cpp:256 ",
		"polynomial::square_free_factor(x^2 - 1) over Z_5\n",
		"upolynomial_factorization.cpp:268 ",
		"Initial factors: 1\nR.<x> = GF(5)['x']\nT_0 = x^2 - 1\n",
		"upolynomial_factorization.cpp:285 ",
		"k = 0\n",
		"upolynomial_factorization.cpp:289 ",
		"T_0_d = T_0.derivative(x)\nT_0_d == 2 x\n",
		"upolynomial_factorization.cpp:296 ",
		"T = T_0.gcd(T_0_d)\nT == 1\n",
		"upolynomial_factorization.cpp:303 ",
		"V = T_0.quo_rem(T)[0]\nV == x^2 - 1\n",
		"upolynomial_factorization.cpp:324 ",
		"W = T.gcd(V)\n1\n",
		"upolynomial_factorization.cpp:331 ",
		"A_ek = V.quo_rem(W)[0]\nA_ek == x^2 - 1\n",
		"upolynomial_factorization.cpp:338 ",
		"V = W\nV == 1\n",
		"upolynomial_factorization.cpp:345 ",
		"T = T.quo_rem(V)[0]\nT == 1\n",
		"upolynomial_factorization.cpp:352 ",
		"Factor: (x^2 - 1)^1\n",
		"upolynomial_factorization.cpp:364 ",
		"T_0 = 1\n",
		"upolynomial_factorization.cpp:369 ",
		"polynomial::square_free_factor(x^2 - 1) => 1 * (x^2 - 1)^1\n",
	}
	position := 0
	for _, fragment := range ordered {
		i := strings.Index(text[position:], fragment)
		if i < 0 {
			t.Fatalf("missing ordered TRACE fragment %q in %q", fragment, text)
		}
		position += i + len(fragment)
	}
}

func TestUPolynomialFiniteFieldFactorEntryAndOutcomeTrace(t *testing.T) {
	m, _, _ := upolySourceOwner(t)
	serviceMust(t, m.SetZPUint64(5))
	f := upolySourceNew(t, m, []int32{-1, 0, 1})
	fs, e := m.NewFactors()
	serviceMust(t, e)
	t.Cleanup(func() { serviceMust(t, fs.Close()) })
	var out strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("polynomial::factorization")
	serviceMust(t, m.modular.BindTrace(tags))
	t.Cleanup(func() { serviceMust(t, m.modular.BindTrace(nil)) })

	_, e = m.FactorZP(mustUPolyView(t, m, f), fs)
	serviceMust(t, e)
	if !tags.ShouldTrace("polynomial::factorization") {
		if out.Len() != 0 {
			t.Fatal("erased finite-field TRACE executed", out.String())
		}
		return
	}
	text := out.String()
	entry := strings.Index(text, "upolynomial_factorization.cpp:375 ")
	outcome := strings.Index(text, "upolynomial_factorization.cpp:399 ")
	if entry < 0 || outcome <= entry || !strings.Contains(text[entry:], "polynomial::factor(x^2 - 1) over Z_5\n") ||
		!strings.Contains(text[outcome:], "polynomial::factor(x^2 - 1) => 1 * (x + 1)^1 * (x - 1)^1\n") {
		t.Fatal("finite-field factor entry/outcome records", text)
	}
}
