package api

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

// AUDIT.md api_03-F9: z3_replayer.cpp's scanner messages, CRLF handling
// and modular arithmetic (the messages checked against build/z3).
func TestZ3ReplayScannerDiagnostics(t *testing.T) {
	for _, tc := range []struct{ log, want string }{
		{"U x\n", "invalid unsigned"},
		{"P z\n", "invalid ptr"},
		{"S \"a\\9x\"\n", "invalid scaped character"},
		{"S \"abc", "unexpected end of file"},
		{"D x\n", "invalid double"},
		{"F x\n", "invalid float"},
		{"I -x\n", "invalid integer"},
		{"R\r\n", "unknown log command"},
	} {
		r, err := NewZ3Replayer(strings.NewReader(tc.log), io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		err = r.Parse()
		var replayErr *Z3ReplayError
		if !errors.As(err, &replayErr) || replayErr.Message != tc.want || replayErr.Line != 1 {
			t.Errorf("%q: %v, want line 1 %q", tc.log, err, tc.want)
		}
		_ = r.Close()
	}
}

func TestZ3ReplayScannerNumbers(t *testing.T) {
	scan := func(text string) *z3ReplayScanner {
		return newZ3ReplayScanner(bufio.NewReader(strings.NewReader(text)))
	}
	if v, err := scan("18446744073709551617 ").uint64(); err != nil || v != 1 {
		t.Fatal("uint64 wrap", v, err)
	}
	if v, err := scan("1x5 ").hexadecimal(); err != nil || v != 0x15 {
		t.Fatal("pointer x at position 1", v, err)
	}
	if v, err := scan("1-2 ").floating(64); err != nil || v != 1 {
		t.Fatal("strtod prefix", v, err)
	}
	if v, err := scan("- ").floating(64); err != nil || v != 0 {
		t.Fatal("strtod no prefix", v, err)
	}
	if v, err := scan("-9223372036854775808 ").int64(); err != nil || v != -9223372036854775808 {
		t.Fatal("int64 minimum", v, err)
	}
	// FORK FIX #135: read_int64 wraps modulo 2^64 (tst_z3_4_7_1_replayer_int64_wrap).
	for text, want := range map[string]int64{"9223372036854775807 ": 9223372036854775807, "9223372036854775808 ": -9223372036854775808, "18446744073709551617 ": 1} {
		if v, err := scan(text).int64(); err != nil || v != want {
			t.Fatal("int64 wrap", text, v, err)
		}
	}
}
