package base_test

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestStringToUint64ScanfPrefixSemantics(t *testing.T) {
	tests := []struct {
		text string
		want uint64
		ok   bool
	}{
		{"42tail", 42, true},
		{" \t+17 rest", 17, true},
		{"-1", math.MaxUint64, true},
		{"", 0, false},
		{"+x", 0, false},
		{"18446744073709551616", 0, false},
	}
	for _, test := range tests {
		got, ok := base.StringToUint64(test.text)
		if got != test.want || ok != test.ok {
			t.Fatal(test.text, got, ok, test.want, test.ok)
		}
	}
}
