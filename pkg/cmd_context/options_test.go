package cmd_context

import (
	"math/big"
	"testing"
)

// AUDIT.md cmd_context_02-F5: get_option_cmd compares the raw keyword, so
// underscore spellings of the built-in options go to gparams (unsupported).
func TestGetOptionMatchesRawBuiltinKeywords(t *testing.T) {
	context, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	for _, name := range []string{":produce_models", ":print_success", ":random_seed", ":int_real_coercions"} {
		if _, supported, err := context.GetOption(name); err != nil || supported {
			t.Fatal(name, supported, err)
		}
	}
	if value, supported, err := context.GetOption(":produce-models"); err != nil || !supported || value != "true" {
		t.Fatal(value, supported, err)
	}
}

// AUDIT.md cmd_context_01-F6: a non-builtin numeral option reaches gparams
// as rational::to_string ("3/2"), which the double parameter reads as 3.
func TestSetOptionNumeralUsesRationalText(t *testing.T) {
	context, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err = context.SetOptionNumeral(":smt.qi.eager_threshold", big.NewRat(3, 2)); err != nil {
		t.Fatal(err)
	}
	if value, supported, err := context.GetOption(":smt.qi.eager_threshold"); err != nil || !supported || value != "3" {
		t.Fatal(value, supported, err)
	}
	if err = context.SetOptionNumeral(":print-success", big.NewRat(3, 1)); err == nil || err.Error() != "option value is not a numeral" {
		t.Fatal(err)
	}
}
