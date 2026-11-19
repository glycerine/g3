package shell

import (
	"errors"
	"testing"
)

func TestShellGlobalTimeoutOptionSourceBranch(t *testing.T) {
	if _, err := parseGlobalTimeoutOption(nil); !errors.Is(err, ErrMissingGlobalTimeout) {
		t.Fatal("missing colon argument", err)
	}
	for text, want := range map[string]uint32{
		"": 0, "not-a-number": 0, "0": 0, "-": 0,
		"  +12suffix": 12000, "4294967295garbage\x00ignored": 4294966296,
	} {
		if got, err := parseGlobalTimeoutOption(&text); err != nil || got != want {
			t.Errorf("%q returned %d, %v; want %d", text, got, err, want)
		}
	}
	for _, text := range []string{"9223372036854775807", "-9223372036854775808"} {
		if _, err := parseGlobalTimeoutOption(&text); !errors.Is(err, ErrGlobalTimeoutRange) {
			t.Fatal("undefined source multiplication", err)
		}
	}
}
