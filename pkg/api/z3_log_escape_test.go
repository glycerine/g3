package api

import "testing"

func TestZ3LogEscapeMatchesLowLevelDecimalEncoding(t *testing.T) {
	got := z3LogEscape("Az 09~!@#$%^&*-_+.?/ <>:\n\té\x00ignored")
	want := "Az 09~!@#$%^&*-_+.?/ <>\\058\\010\\009\\195\\169"
	if got != want {
		t.Fatalf("escape mismatch\n got: %q\nwant: %q", got, want)
	}
	for _, c := range []byte{'[', ']', '\\', '"', '|'} {
		encoded := z3LogEscape(string([]byte{c}))
		if len(encoded) != 4 || encoded[0] != '\\' {
			t.Fatalf("byte %q was not escaped: %q", c, encoded)
		}
	}
}

func TestZ3LogCStringStopsAtFirstNUL(t *testing.T) {
	if got := z3LogCString("before\x00after"); got != "before" {
		t.Fatalf("got %q", got)
	}
}
