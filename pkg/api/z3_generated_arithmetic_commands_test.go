package api

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestGeneratedArithmeticReplay(t *testing.T) {
	cases := []struct {
		id   int
		args string
		kind int32
	}{
		{65, "U 2\nP 0x43\nP 0x44\np 2\n", 6}, {66, "U 2\nP 0x43\nP 0x44\np 2\n", 9}, {67, "U 2\nP 0x43\nP 0x44\np 2\n", 7},
		{68, "P 0x43\n", 8}, {69, "P 0x43\nP 0x44\n", 11}, {70, "P 0x43\nP 0x44\n", 13}, {71, "P 0x43\nP 0x44\n", 12},
		{72, "P 0x43\nP 0x44\n", 18}, {73, "P 0x43\nP 0x44\n", 4}, {74, "P 0x43\nP 0x44\n", 2},
		{75, "P 0x43\nP 0x44\n", 5}, {76, "P 0x43\nP 0x44\n", 3}, {77, "P 0x43\n", 14}, {78, "P 0x45\n", 15}, {79, "P 0x45\n", 16},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 33\n= 0x42\nR\nP 0x41\nI 3\nP 0x42\nC 150\n= 0x43\nR\nP 0x41\nU 7\nP 0x42\nC 151\n= 0x44\nR\nP 0x41\nI 1\nI 2\nC 149\n= 0x45\nR\nP 0x41\n" + tc.args + fmt.Sprintf("C %d\n= 0x46\nR\nP 0x41\nP 0x46\nC 900\n", tc.id)
			r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := r.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err = RegisterZ3GeneratedContextCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = RegisterZ3GeneratedASTCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = r.RegisterCommand(900, "check-arithmetic", func(r *Z3Replayer) error {
				owner, e := replayContext(r, 0)
				if e != nil {
					return e
				}
				value, e := r.astObject(owner, 1)
				if e != nil {
					return e
				}
				kind, e := owner.results.manager.GetDeclKind(value.handle)
				if e != nil || kind != tc.kind {
					t.Fatalf("kind %d want %d: %v", kind, tc.kind, e)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err = r.Parse(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGeneratedNumeralReplay(t *testing.T) {
	cases := []struct {
		command  int
		argument string
		want     string
	}{
		{148, "S \"12345678901234567890\"\n", "12345678901234567890"}, {150, "I -17\n", "-17"},
		{151, "U 4294967295\n", "4294967295"}, {152, "I -9223372036854775808\n", "-9223372036854775808"},
		{153, "U 18446744073709551615\n", "18446744073709551615"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.command), func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 33\n= 0x42\nR\nP 0x41\n" + tc.argument + fmt.Sprintf("P 0x42\nC %d\n= 0x43\nR\nP 0x41\nP 0x43\nC 900\n", tc.command)
			r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err = RegisterZ3GeneratedContextCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = RegisterZ3GeneratedASTCommands(r); err != nil {
				t.Fatal(err)
			}
			if err = r.RegisterCommand(900, "check-numeral", func(r *Z3Replayer) error {
				owner, e := replayContext(r, 0)
				if e != nil {
					return e
				}
				value, e := r.astObject(owner, 1)
				if e != nil {
					return e
				}
				text, e := owner.context.NumeralString(value.handle)
				if e != nil || text != tc.want {
					t.Fatalf("numeral %q want %q: %v", text, tc.want, e)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err = r.Parse(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
