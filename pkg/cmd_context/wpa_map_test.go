package cmd_context

import "testing"

func TestWPAMapLineSourceNameCleanup(t *testing.T) {
	cases := []struct {
		line    string
		number  uint64
		name    string
		present bool
	}{
		{"", 0, "", false},
		{"# comment", 0, "", false},
		{"17 Alpha SC_EXTERN ignored", 17, "Alpha", true},
		{"19 Beta _ZONE_tail", 19, "Beta", true},
		{"23 Gamma Constant ", 23, "Gamma", true},
		{"23 GammaConstant ", 23, "Gamma", true},
		{"23 GammaConstant", 23, "GammaConstant", true},
		{"23 Constant ", 23, "Constant", true},
		{"23  Gamma  ", 23, " Gamma ", true},
		{"23 Gamma\t ", 23, "Gamma\t", true},
		{"23 Gamma\rignored", 23, "Gamma", true},
		{"23  ", 23, "", true},
	}
	for _, tc := range cases {
		number, name, present, err := parseWPAMapLine(tc.line)
		if err != nil {
			t.Fatal(tc.line, err)
		}
		if number != tc.number || name != tc.name || present != tc.present {
			t.Fatal(tc.line, number, name, present)
		}
	}
	if _, _, _, err := parseWPAMapLine("not-a-number value"); err == nil {
		t.Fatal("accepted malformed map number")
	}
}

func TestWPAMapLineLiteralSpaceAndIgnoredNames(t *testing.T) {
	for _, line := range []string{" 23 name", "\t23 name", "+23 name", "23\tname", "23", "18446744073709551616 name", " # comment"} {
		if _, _, _, err := parseWPAMapLine(line); err == nil {
			t.Errorf("accepted non-source map grammar %q", line)
		}
	}
	for _, line := range []string{"23 ", "23  SC_EXTERN ignored"} {
		if _, _, _, err := parseWPAMapLine(line); err == nil {
			t.Errorf("accepted undefined source empty-name indexing %q", line)
		}
		number, name, present, err := parseWPAMapLineWithNames(line, false)
		if err != nil || number != 23 || name != "<names ignored>" || !present {
			t.Fatal("disabled name processing inspected ignored text", number, name, present, err)
		}
	}
}
