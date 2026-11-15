package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Duality zbug round 2: dl_interface::get_answer is unsupported. z3 4.7.1
// returned a null answer that fixedpoint.print_answer dereferenced (crash);
// the fork throws "duality does not support get_answer", which the query
// command reports after its sat line.
func TestDualityPrintAnswerUnsupported(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "duality_print_answer_unsupported.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = run(bytes.NewReader(script), &out)
	const message = "line 8 column 8: duality does not support get_answer"
	if err == nil || err.Error() != message {
		t.Fatal("print_answer error", err)
	}
	if want := "sat\n(error \"" + message + "\")\n"; out.String() != want {
		t.Fatalf("output %q want %q", out.String(), want)
	}
}
