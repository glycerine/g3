package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// datalogFrontEndResults holds the stdout (without the Time:/Parsing:
// timing lines) and stderr of the -dl front end on test_vectors/<name>, as
// build/z3 prints them. AUDIT.md cmd_context_01-F9: the Datalog front end
// is a port of dlexer/dparser (src/muz/fp/datalog_parser.cpp).
var datalogFrontEndResults = map[string]struct{ stdout, stderr string }{
	"datalog_dparser_domain_overflow.datalog":         {"", "sort V contains more constants than its declared size 2\nERROR: failed to parse file\n"},
	"datalog_dparser_domain_size_zero.datalog":        {"", "Domain size of sort 'V' may not be 0\nERROR: failed to parse file\n"},
	"datalog_dparser_fact_order.datalog":              {"Tuples in P: \n\t(x=3(0))\n\t(x=1(1))\n\t(x=2(2))\n", ""},
	"datalog_dparser_include.datalog":                 {"Tuples in P: \n\t(x=v(0),y=w(0))\nTuples in Q: \n\t(x=beta(1))\n\t(x=gamma(3))\n", ""},
	"datalog_dparser_int_domain_ends_domains.datalog": {"", "unexpected token at line 2 '5' found '<num>'\n\nERROR: failed to parse file\n"},
	"datalog_dparser_lexer_program.datalog":           {"Tuples in far: \nTuples in reach: \n\t(a=1(0),b=2(1))\n\t(a=2(1),b=3(2))\n\t(a=x<y(3),b=1(0))\n\t(a=1(0),b=3(2))\n\t(a=x<y(3),b=3(2))\n\t(a=x<y(3),b=2(1))\nTuples in edge: \n\t(a=1(0),b=2(1))\n\t(a=2(1),b=3(2))\n\t(a=x<y(3),b=1(0))\n\t(a=1(0),b=3(2))\n\t(a=3(2),b=3(2))\n\t(a=x<y(3),b=3(2))\nTuples in low: \n", ""},
	"datalog_dparser_lt_int_sort.datalog":             {"", "expecting finite domain sort\nERROR: failed to parse file\n"},
	"datalog_dparser_map_overflow.datalog":            {"", "sort F contains more constants than its declared size 2\nERROR: failed to parse file\n"},
	"datalog_dparser_missing_include.datalog":         {"", "ERROR: could not open file 'test_vectors/datalog_dparser_no_such_file.dl'.\nnewline expected after include statement at line 1 'datalog_dparser_no_such_file.dl' found '<error>'\n\nERROR: failed to parse file\n"},
	"datalog_dparser_missing_period.datalog":          {"", "'.' expected at the end of rule at line 5 '' found '\\n'\n\nERROR: failed to parse file\n"},
	"datalog_dparser_sort_mismatch.datalog":           {"", "Sorts V and W are incompatible\nERROR: failed to parse file\n"},
	"datalog_dparser_string_last_line.datalog":        {"", "too few arguments passed to predicate at line 4 '' found '<error>'\n\nERROR: failed to parse file\n"},
}

func withoutDatalogTimes(text string) string {
	var kept []string
	for _, line := range strings.SplitAfter(text, "\n") {
		if !strings.HasPrefix(line, "Time:") && !strings.HasPrefix(line, "Parsing:") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "")
}

func TestDatalogFrontEndVectorsMatchCPP(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	for name, want := range datalogFrontEndResults {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("test_vectors", name)
			var stdout, stderr bytes.Buffer
			if err := runArgumentsWithDiagnostics([]string{path}, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatal(err, stdout.String(), stderr.String())
			}
			if got := withoutDatalogTimes(stdout.String()); got != want.stdout || stderr.String() != want.stderr {
				t.Fatalf("g3 stdout %q stderr %q\nwant stdout %q stderr %q", got, stderr.String(), want.stdout, want.stderr)
			}
			if reference == "" {
				return
			}
			cmd := exec.Command(reference, path)
			var cppStderr bytes.Buffer
			cmd.Stderr = &cppStderr
			cppStdout, err := cmd.Output()
			if err != nil {
				t.Fatal("C++ reference", err, string(cppStdout), cppStderr.String())
			}
			if got := withoutDatalogTimes(string(cppStdout)); got != want.stdout || cppStderr.String() != want.stderr {
				t.Fatalf("C++ stdout %q stderr %q\nwant stdout %q stderr %q", got, cppStderr.String(), want.stdout, want.stderr)
			}
		})
	}
}
