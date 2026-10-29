package api

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestZ3ReplayerTypedScalarArgumentsAndEscapes(t *testing.T) {
	var output bytes.Buffer
	r, err := NewZ3Replayer(strings.NewReader("V \"4.8.0\"\nR\nI -5\nU 9\nF 1.25\nD -2.5e1\nS \"a\\032b\"\n$ |symbol|\n# 42\nC 7\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	err = r.RegisterCommand(7, "scalars", func(r *Z3Replayer) error {
		i, e := r.Int64(0)
		if e != nil || i != -5 {
			t.Fatal(i, e)
		}
		u, e := r.Uint64(1)
		if e != nil || u != 9 {
			t.Fatal(u, e)
		}
		f, e := r.Float(2)
		if e != nil || f != 1.25 {
			t.Fatal(f, e)
		}
		d, e := r.Double(3)
		if e != nil || math.Abs(d+25) > 1e-9 {
			t.Fatal(d, e)
		}
		s, e := r.String(4)
		if e != nil || s != "a b" {
			t.Fatal(s, e)
		}
		sym, e := r.Symbol(5)
		if e != nil || sym != "symbol" {
			t.Fatal(sym, e)
		}
		num, e := r.Symbol(6)
		if e != nil || num != "42" {
			t.Fatal(num, e)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
