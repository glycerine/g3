package polynomial

import (
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// AUDIT.md math_polynomial_08-F1: core_manager::rem traces under rem_bug on
// every call, not only under the root-ID and Sturm frames.
func TestUPolynomialRemTracesOutsideSturmFrames(t *testing.T) {
	_, m, _ := algebraicNegTLSFixture(t, nil)
	a := upolySourceNew(t, m, []int32{2, 0, -2, 0, 2, 0, 1})
	b := upolySourceNew(t, m, []int32{0, 1, 0, -2, 0, 1})
	r := upolySourceNew(t, m, nil)
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("rem_bug")
	if !tags.ShouldTrace("rem_bug") {
		t.Skip("TRACE compiled out")
	}
	serviceMust(t, m.config.Integer.BindTrace(tags))
	var d uint32
	e := m.Rem(a, b, r, &d)
	serviceMust(t, m.config.Integer.BindTrace(nil))
	serviceMust(t, e)
	var lines []string
	for _, match := range regexp.MustCompile(`upolynomial\.cpp:(\d+) -`).FindAllStringSubmatch(text.String(), -1) {
		lines = append(lines, match[1])
	}
	if got, want := strings.Join(lines, " "), "532 551 532 535"; got != want {
		t.Fatalf("rem_bug records = %q, want %q\n%s", got, want, text.String())
	}
	if !strings.Contains(text.String(), "rem loop, p2:\nx^5 - 2 x^3 + x\nbuffer:\nx^6 + 2 x^4 - 2 x^2 + 2\n") {
		t.Fatalf("first record differs:\n%s", text.String())
	}
}

// AUDIT.md math_polynomial_08-F2: core_manager::div(sz, p, b) traces each
// coefficient b does not divide, before its SASSERT(m().divides(b, p[i])).
func TestUPolynomialDivScalarNonDivisibleTrace(t *testing.T) {
	_, m, _ := algebraicNegTLSFixture(t, nil)
	p := upolySourceNew(t, m, []int32{3, 4})
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("upolynomial")
	if !tags.ShouldTrace("upolynomial") {
		t.Skip("TRACE compiled out")
	}
	if asserting, e := refinementSourceAssertionsEnabled(); e != nil || asserting {
		serviceMust(t, e)
		t.Skip("source debug builds stop at SASSERT(m().divides(b, p[i]))")
	}
	serviceMust(t, m.modular.BindTrace(tags))
	two := unumeral.NewInt(2)
	e := m.DivScalar(p, &two)
	serviceMust(t, m.modular.BindTrace(nil))
	serviceMust(t, e)
	if !strings.Contains(text.String(), "[upolynomial] div src/math/polynomial/upolynomial.cpp:408 ---------\nb: 2, p[i]: 3\n") {
		t.Fatalf("missing non-divisible record:\n%s", text.String())
	}
}
