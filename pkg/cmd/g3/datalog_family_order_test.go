package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AUDIT.md muz_rel_04-F1 follow-up (cmd_context_02-F11 round 2):
// read_datalog's ast_manager gets its plugins from reg_decl_plugins (via
// the datalog::context constructor), so datalog_relation is family 9, not
// the cmd_context family 12. The family id enters each finite sort's hash,
// hence each predicate's, hence the decl_set order in which
// display_output_tables prints the relations. Expected texts are build/z3's.
var datalogFamilyOrderVectors = map[string]string{
	"datalog_family_order.datalog":       "Tuples in f: \n\t(x=a(0),y=b(1))\nTuples in e: \n\t(x=a(0),y=b(1))\n",
	"datalog_family_order_sorts.datalog": "Tuples in f: \n\t(x=x(0),y=y(0))\nTuples in e: \n\t(x=a(0),y=b(1))\n",
}

func TestDatalogOutputOrderUsesFrontendFamilies(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	for name, want := range datalogFamilyOrderVectors {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("test_vectors", name)
			var output bytes.Buffer
			if err := runArguments([]string{path}, strings.NewReader(""), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if got := wpaTimingLines.ReplaceAllString(output.String(), ""); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
			if reference == "" {
				return
			}
			native, err := exec.Command(reference, path).Output()
			if err != nil {
				t.Fatal("C++ reference", err, string(native))
			}
			if got := wpaTimingLines.ReplaceAllString(string(native), ""); got != want {
				t.Fatalf("C++ reference %q, want %q", got, want)
			}
		})
	}
}
