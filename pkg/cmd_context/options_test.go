package cmd_context

import "testing"

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
