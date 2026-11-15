package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDatalogCommandFile(t *testing.T, directory, name, contents string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertDatalogCommandOutput(t *testing.T, output, tuples string) {
	t.Helper()
	if !strings.HasPrefix(output, tuples+"Time: ") {
		t.Fatalf("Datalog output = %q, want tuple prefix %q followed by timing", output, tuples)
	}
	if strings.Count(output, "Time: ") != 1 || strings.Count(output, "\nParsing: ") != 1 {
		t.Fatalf("Datalog output did not contain exactly one statistics block: %q", output)
	}
	if strings.Count(output, "Tuples in ") != strings.Count(tuples, "Tuples in ") {
		t.Fatalf("Datalog output printed an unrequested relation: %q", output)
	}
}

func TestDatalogCommandFilesResolveIncludesAndDeriveTuples(t *testing.T) {
	directory := t.TempDir()
	writeDatalogCommandFile(t, directory, "domains.inc", "D 2\n\n")
	writeDatalogCommandFile(t, directory, "rules.inc", "Seed(x:D)\nReach(x:D) printtuples\nSeed(\"alpha\").\nReach(X) :- Seed(X).\n")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "query.dl"},
		{name: "query.datalog"},
		{name: "query.txt", args: []string{"-dl", "-st", "fixedpoint.engine=pdr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeDatalogCommandFile(t, directory, tc.name, ".include \"domains.inc\"\n\n.include \"rules.inc\"\n")
			args := append(append([]string(nil), tc.args...), path)
			var output bytes.Buffer
			if err := runArguments(args, strings.NewReader("ignored standard input"), &output); err != nil {
				t.Fatal(err, output.String())
			}
			assertDatalogCommandOutput(t, output.String(), "Tuples in Reach: \n\t(x=alpha(0))\n")
		})
	}
}

func TestDatalogCommandWPADirectoryUsesGlobalTupleOptions(t *testing.T) {
	directory := t.TempDir()
	writeDatalogCommandFile(t, directory, "V.map", "7 alpha\n19 alpha\n")
	writeDatalogCommandFile(t, directory, "program.rules", "P(x:V) outputtuples\n")
	writeDatalogCommandFile(t, directory, "P.rel", "19\n7\n")
	for _, tc := range []struct {
		name, option, want string
	}{
		{name: "default-names", want: "Tuples in P: \n\t(x=alpha(0))\n"},
		{name: "numeric-values", option: "fixedpoint.datalog.use_map_names=false", want: "Tuples in P: \n\t(x=19(0))\n\t(x=7(1))\n"},
		{name: "tuple-output-disabled", option: "fixedpoint.datalog.print.tuples=false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"-dl"}
			if tc.option != "" {
				args = append(args, tc.option)
			}
			args = append(args, directory)
			var output bytes.Buffer
			if err := runArguments(args, strings.NewReader(""), &output); err != nil {
				t.Fatal(err, output.String())
			}
			assertDatalogCommandOutput(t, output.String(), tc.want)
		})
	}
}

func TestDatalogCommandExplicitFormatOverridesExtensionAndEarlierFlags(t *testing.T) {
	directory := t.TempDir()
	smtPath := writeDatalogCommandFile(t, directory, "actually-smt.dl", "(assert true)\n(check-sat)\n")
	for _, flags := range [][]string{{"-smt2"}, {"-dl", "-smt2"}} {
		var output bytes.Buffer
		if err := runArguments(append(flags, smtPath), strings.NewReader(""), &output); err != nil {
			t.Fatal(flags, err, output.String())
		}
		if output.String() != "sat\n" {
			t.Fatalf("explicit SMT format %v produced %q", flags, output.String())
		}
	}
	datalogPath := writeDatalogCommandFile(t, directory, "actually-datalog.smt2", "\nReady printtuples\nReady.\n")
	var output bytes.Buffer
	if err := runArguments([]string{"-smt2", "-dl", datalogPath}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err, output.String())
	}
	assertDatalogCommandOutput(t, output.String(), "Tuples in Ready: \n\t()\n")
}

func TestDatalogCommandStandardInputRequiresPath(t *testing.T) {
	var output bytes.Buffer
	err := runArguments([]string{"-dl", "-in"}, strings.NewReader("\nReady\nReady.\n"), &output)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "requires") || !strings.Contains(strings.ToLower(err.Error()), "path") {
		t.Fatalf("Datalog standard-input error = %v, want a requires-path explanation", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unsupported standard input produced output: %q", output.String())
	}
}

func TestDatalogCommandHelpAdvertisesFormat(t *testing.T) {
	var output bytes.Buffer
	if err := runArguments([]string{"-h"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "-dl") {
		t.Fatalf("help omits the Datalog frontend: %q", output.String())
	}
}

// AUDIT.md muz_rel_04-F1: relation_manager::display_output_tables walks the
// output decl_set (obj_hashtable) in hash-table order, not declaration
// order. Expected prefixes are build/z3's output.
func TestDatalogOutputTuplesFollowDeclSetOrder(t *testing.T) {
	directory := t.TempDir()
	rows := "\t(x=a(0))\n\t(x=b(1))\n"
	for _, tc := range []struct {
		name  string
		preds []string
		want  []string
	}{
		{name: "zed.datalog", preds: []string{"Zed", "Alpha", "Mid"}, want: []string{"Alpha", "Mid", "Zed"}},
		{name: "pabc.datalog", preds: []string{"Pa", "Pb", "Pc"}, want: []string{"Pb", "Pc", "Pa"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var program strings.Builder
			program.WriteString("D 4\n\nSeed(x:D)\n")
			for _, pred := range tc.preds {
				program.WriteString(pred + "(x:D) printtuples\n")
			}
			program.WriteString("Seed(\"a\").\nSeed(\"b\").\n")
			for _, pred := range tc.preds {
				program.WriteString(pred + "(X) :- Seed(X).\n")
			}
			path := writeDatalogCommandFile(t, directory, tc.name, program.String())
			var output bytes.Buffer
			if err := runArguments([]string{path}, strings.NewReader(""), &output); err != nil {
				t.Fatal(err, output.String())
			}
			var want strings.Builder
			for _, pred := range tc.want {
				want.WriteString("Tuples in " + pred + ": \n" + rows)
			}
			assertDatalogCommandOutput(t, output.String(), want.String())
		})
	}
}
