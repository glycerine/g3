package api

import "testing"

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
