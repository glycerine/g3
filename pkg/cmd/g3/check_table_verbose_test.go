package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// AUDIT.md muz_rel_02-F1: check_table_plugin::mk_empty, the join/union/
// rename/negation functors and check_table::add_fact/clone print their
// __FUNCTION__ at -v:1. The expected lines are build/z3 -v:1 stderr without
// the rule_transformer "(transform ..." lines and the verbose_action
// "join_project...0s" line.
func TestCheckTableVerboseTrace(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "check_table_verbose.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	level, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	previous, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, err := range []error{diagnostic.SetProcessVerbosityLevel(level), diagnostic.SetProcessVerboseStream(previous), stream.Close()} {
			if err != nil {
				t.Error(err)
			}
		}
	}()
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		t.Fatal(err)
	}
	if err = diagnostic.SetProcessVerbosityLevel(1); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = run(bytes.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "sat\n" {
		t.Fatalf("output %q", output.String())
	}
	text, err := stream.String()
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line == "" || strings.HasPrefix(line, "(transform ") || strings.Contains(line, "...") {
			continue
		}
		lines = append(lines, line)
	}
	want := []string{"mk_empty", "mk_empty", "mk_empty", "add_fact", "mk_empty", "operator()", "mk_empty", "add_fact", "operator()", "operator()", "operator()", "mk_empty", "operator()"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("verbose lines\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}
