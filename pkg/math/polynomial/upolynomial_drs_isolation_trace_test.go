package polynomial

import (
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md math_polynomial_10-F1: sqf_isolate_roots, drs_isolate_roots and
// drs_isolate_0_1_roots emit their TRACE("upolynomial") records.
func TestDRSIsolationTraceRecords(t *testing.T) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	b := c.config.Binary
	roots, lo, hi := upolyIsolationVector(t, b), upolyIsolationVector(t, b), upolyIsolationVector(t, b)
	p := upolySourceNew(t, m, []int32{-2, 0, 0, 1})
	var text strings.Builder
	tags := diagnostic.NewTraceTagsWithWriter(&text)
	tags.Enable("upolynomial")
	if !tags.ShouldTrace("upolynomial") {
		t.Skip("TRACE compiled out")
	}
	serviceMust(t, m.config.Integer.BindTrace(tags))
	body := m.SquareFreeIsolateRoots(mustUPolyView(t, m, p), b, roots, lo, hi)
	serviceMust(t, m.config.Integer.BindTrace(nil))
	serviceMust(t, body)
	var lines []string
	for _, match := range regexp.MustCompile(`upolynomial\.cpp:(\d+) -`).FindAllStringSubmatch(text.String(), -1) {
		lines = append(lines, match[1])
	}
	if got, want := strings.Join(lines, " "), "2333 2300 2199 2207 2313 2199 2203"; got != want {
		t.Fatalf("record lines = %q, want %q\n%s", got, want, text.String())
	}
	for _, part := range []string{"Cauchy     U: 2^3\nKnuth  pos U: 2^2\nKnuth  neg U: 2^1\n", "isolating (0,1) roots of:\n64 x^3 - 2\n", "searching at (-1, 0) using:\n"} {
		if !strings.Contains(text.String(), part) {
			t.Fatalf("missing %q in\n%s", part, text.String())
		}
	}
}
