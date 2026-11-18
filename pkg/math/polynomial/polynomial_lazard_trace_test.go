package polynomial

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestPolynomialLazardSourceTraceAllBranches(t *testing.T) {
	m := polynomialCellFixture(t)
	out, e := NewPolynomialRef(m)
	serviceMust(t, e)
	defer func() { serviceMust(t, out.Close()) }()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("Lazard")
	m.monomials.config.Trace = tags
	serviceMust(t, m.seLazard(4, m.unit, m.unit, 0, &out.root))
	record := func(line int, body string) string {
		return fmt.Sprintf("-------- [Lazard] Se_Lazard src/math/polynomial/polynomial.cpp:%d ---------\n%s------------------------------------------------\n", line, body)
	}
	want := ""
	if tags.ShouldTrace("Lazard") {
		want = record(5661, "lc_S_d: 1\nS_d_1: 1\nn: 3\n") +
			record(5671, "a: 2\n") +
			record(5682, "loop a: 1\nC : 1\n") +
			record(5688, "if, C: 1\n") +
			record(5691, "C: 1\nY: "+lazardPointerToken(m.unit)+"\n")
	}
	if output.String() != want {
		t.Fatalf("Lazard source trace: got %q, want %q", output.String(), want)
	}
	if tags.ShouldTrace("Lazard") && !regexp.MustCompile(`\nY: 0x[0-9a-f]+\n-+\n$`).MatchString(output.String()) {
		t.Fatal("Y is not printed as a pointer", output.String())
	}
	output.Reset()
	// The entry TRACE precedes even the n==0 shortcut.
	serviceMust(t, m.seLazard(1, m.unit, m.unit, 0, &out.root))
	want = ""
	if tags.ShouldTrace("Lazard") {
		want = record(5661, "lc_S_d: 1\nS_d_1: 1\nn: 0\n")
	}
	if output.String() != want {
		t.Fatal("Lazard zero-exponent source trace ordering", output.String(), want)
	}
	tags.Disable("Lazard")
	unread := Polynomial{owner: m.origin, slot: ^uint32(0), generation: 1}
	serviceMust(t, m.traceLazardEntry(unread, unread, 0))
	serviceMust(t, m.traceLazardFinal(unread, unread))
	serviceMust(t, m.seLazard(1, unread, m.unit, 0, &out.root))
	if output.String() != want {
		t.Fatal("disabled Lazard trace emitted or inspected operands")
	}
	tags.Enable("Lazard")
	e = m.seLazard(1, unread, m.unit, 0, &out.root)
	if tags.ShouldTrace("Lazard") && e == nil {
		t.Fatal("enabled Lazard trace omitted polynomial formatting")
	}
	if !tags.ShouldTrace("Lazard") {
		serviceMust(t, e)
	}
	// The final record prints Y's address and does not format Y.
	serviceMust(t, m.traceLazardFinal(m.unit, unread))
}
