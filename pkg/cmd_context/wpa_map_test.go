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
