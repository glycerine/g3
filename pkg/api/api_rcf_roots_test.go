package api

import (
	"strings"
	"testing"
)

// rootDecimals isolates the roots of the ascending coefficients and returns
// their decimal strings at precision 10, keeping the roots for f.close.
func (f *rcfFixture) rootDecimals(coefficients ...*RCFNumber) ([]*RCFNumber, []string) {
	f.t.Helper()
	roots, err := f.context.RCFRoots(coefficients)
	if err != nil {
		f.t.Fatal(err)
	}
	decimals := make([]string, len(roots))
	for i, root := range roots {
		f.values = append(f.values, root)
		decimal, err := f.context.RCFDecimalString(root, 10)
		if err != nil {
			f.t.Fatal(err)
		}
		decimals[i] = decimal
	}
	return roots, decimals
}

func (f *rcfFixture) expectDecimals(label string, got []string, want ...string) {
	f.t.Helper()
	if len(got) != len(want) {
		f.t.Fatalf("%s: got %d roots %q, want %q", label, len(got), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			f.t.Fatalf("%s: root %d decimal %q, want %q", label, i, got[i], want[i])
		}
	}
}

// AUDIT math_realclosure_01-F2: mk_neg_interval/mk_pos_interval keep the
// inverted Knuth bounds of an empty half-line; root isolation must not fail.
func TestAPIRCFRootsAllPositive(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	zero := f.rational("0")
	_, got := f.rootDecimals(f.rational("-2"), zero, zero, f.rational("1"))
	f.expectDecimals("x^3-2", got, "1.2599210498?")
	_, got = f.rootDecimals(f.rational("2"), f.rational("-3"), f.rational("1"))
	f.expectDecimals("x^2-3x+2", got, "0.9999999999?", "1.9999999999?")
	_, got = f.rootDecimals(f.rational("1/3"), zero, zero, f.rational("1"))
	f.expectDecimals("x^3+1/3", got, "-0.6933612743?")
}

// AUDIT math_realclosure_01-F5: determine_infinitesimal_sign sets open
// intervals, so a value infinitely close to 0 has an interval excluding 0
// and can be inverted.
func TestAPIRCFInverseOfInfinitesimalDifference(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	e := f.keep(f.context.RCFInfinitesimal())
	d := f.keep(f.context.RCFSub(e, f.keep(f.context.RCFMul(e, e))))
	inverse := f.keep(f.context.RCFInv(d))
	if got, want := f.text(inverse), "(-1)/(eps!1^2 + -1*eps!1)"; got != want {
		t.Fatalf("1/(eps-eps^2) = %q, want %q", got, want)
	}
}

// AUDIT math_realclosure_01-F3: square_free, gcd, div_rem, rem and prem
// operate on coefficients of every extension kind.
func TestAPIRCFRootsTranscendentalCoefficient(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	pi := f.keep(f.context.RCFPi())
	_, got := f.rootDecimals(f.keep(f.context.RCFNeg(pi)), f.rational("0"), f.rational("1"))
	f.expectDecimals("x^2-pi", got, "-1.7724538509?", "1.7724538509?")
}

// AUDIT math_realclosure_01-F1: clean_denominators cleans the denominators
// of rational-function coefficients, so the roots are those of the cleaned
// polynomial (pi + 1)*x^2 - 1.
func TestAPIRCFRootsRationalFunctionCoefficient(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	pi := f.keep(f.context.RCFPi())
	pi1 := f.keep(f.context.RCFAdd(pi, f.rational("1")))
	c := f.keep(f.context.RCFNeg(f.keep(f.context.RCFInv(pi1))))
	roots, got := f.rootDecimals(c, f.rational("0"), f.rational("1"))
	f.expectDecimals("x^2-1/(pi+1)", got, "-0.4913786798?", "0.4913786798?")
	for i, root := range roots {
		if text := f.text(root); !strings.Contains(text, "root((pi + 1)*x^2 + -1, ") {
			t.Fatalf("root %d = %q", i, text)
		}
	}
	// Structurally equal non-rational denominators are not multiplied into
	// their own term, but they are multiplied into the rational ones.
	third := f.rational("1/3")
	inverse := f.keep(f.context.RCFDiv(f.rational("1"), pi1))
	roots, got = f.rootDecimals(c, inverse, third)
	f.expectDecimals("x^2/3+x/(pi+1)-1/(pi+1)", got, "-0.5147034125?", "0.3398047487?")
	for i, root := range roots {
		if text := f.text(root); !strings.Contains(text, "root((pi^2 + 2*pi + 1)*x^2 + 3*x + -3, ") {
			t.Fatalf("root %d = %q", i, text)
		}
	}
}

// AUDIT math_realclosure_01-F3 and 01-F4: roots infinitely close to each
// other are separated by sign determination.
func TestAPIRCFRootsInfinitesimalClose(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	e := f.keep(f.context.RCFInfinitesimal())
	e2 := f.keep(f.context.RCFMul(e, e))
	roots, _ := f.rootDecimals(f.keep(f.context.RCFMul(f.rational("2"), e2)), f.keep(f.context.RCFMul(f.rational("-3"), e)), f.rational("1"))
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2", len(roots))
	}
	for i, want := range []string{"{2*x + -3*eps!1 > 0}", "{2*x + -3*eps!1 < 0}"} {
		if text := f.text(roots[i]); !strings.Contains(text, want) {
			t.Fatalf("root %d = %q, want it to contain %q", i, text, want)
		}
	}
}
