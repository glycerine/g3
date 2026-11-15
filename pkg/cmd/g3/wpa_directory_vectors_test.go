package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md cmd_context_02-F11: "z3 -dl DIRECTORY" runs the WPA loader
// (wpa_parser_impl). z3 4.7.1 prints NOT IMPLEMENTED YET! and hangs on
// POSIX; the fork implements get_file_names there (FORK FIX #66), formats
// the not-in-sort warning with PRIu64 (#B), reads ULLONG_MAX (#C) and
// accepts an empty map name (#D). The expected stdout (without the timing
// lines) and stderr are what the fixed fork prints, run from this directory.
var wpaDirectoryVectors = map[string]struct{ stdout, stderr string }{
	"basic": {stdout: "Tuples in Out: \n\t(x=alpha(0),y=beta(1),w=seven(0))\n\t(x=beta(1),y=gamma(2),w=seven(0))\n"},
	"warnings": {
		stdout: "Tuples in P: \n\t(x=a(0),y=b(0))\n\t(x=b(1),y=e(1))\n",
		stderr: "WARNING: mismatch of number names on line 1 in file test_vectors/wpa/warnings/W.map. old: \"b\" new: \"c\"\n" +
			"WARNING: symbol number 4294967301 on line 2 in file test_vectors/wpa/warnings/P.rel does not belong to sort V\n" +
			"WARNING: symbol number 4294967301 on line 4 in file test_vectors/wpa/warnings/P.rel does not belong to sort W\n",
	},
	"bool_map":      {stderr: "sort BOOL already declared\nERROR: failed to parse file\n"},
	"uint64_max":    {stdout: "Tuples in P: \n\t(x=top(0))\n\t(x=below(1))\n"},
	"empty_name":    {stdout: "Tuples in P: \n\t(x=(0))\n\t(x=c(1))\n"},
	"bad_separator": {stderr: "' ' expected to separate numbers on line 2 in file test_vectors/wpa/bad_separator/P.rel, got 'x 1 '\nERROR: failed to parse file\n"},
	"undeclared":    {stderr: "tuple file test_vectors/wpa/undeclared/sub/Q.rel for undeclared predicate Q\nERROR: failed to parse file\n"},
	"short_numbers": {stdout: "Tuples in P: \n\t(b=1(0),s=700(0))\n\t(b=0(1),s=3(1))\n\t(b=1(0),s=3(1))\n"},
	// Round 2: two output relations print in decl_set order (datalog_relation
	// is family 9, as reg_decl_plugins makes it).
	"two_outputs": {stdout: "Tuples in f: \n\t(x=1(0),y=700(0))\n\t(x=0(1),y=3(1))\nTuples in e: \n\t(x=<zero element>(0),y=b(1))\n\t(x=b(1),y=a(2))\n"},
}

var wpaTimingLines = regexp.MustCompile(`(?m)^Time: \d+ms\nParsing: \d+ms, other: \d+ms\n`)

func TestWPADirectoryVectors(t *testing.T) {
	warnings := diagnostic.BorrowProcessWarningService()
	for name, want := range wpaDirectoryVectors {
		t.Run(name, func(t *testing.T) {
			var output, diagnostics bytes.Buffer
			previous := warnings.WarningStream()
			if err := warnings.SetWarningStream(&diagnostics); err != nil {
				t.Fatal(err)
			}
			err := runArgumentsWithDiagnostics([]string{"-dl", filepath.Join("test_vectors", "wpa", name)}, strings.NewReader(""), &output, &diagnostics)
			if restore := warnings.SetWarningStream(previous); restore != nil {
				t.Fatal(restore)
			}
			if err != nil {
				t.Fatal(err, output.String(), diagnostics.String())
			}
			stdout := output.String()
			if want.stdout != "" {
				if !wpaTimingLines.MatchString(stdout) {
					t.Fatalf("no timing lines in %q", stdout)
				}
				stdout = wpaTimingLines.ReplaceAllString(stdout, "")
			}
			if stdout != want.stdout || diagnostics.String() != want.stderr {
				t.Fatalf("stdout %q stderr %q\nwant stdout %q stderr %q", stdout, diagnostics.String(), want.stdout, want.stderr)
			}
		})
	}
}

// With G3_CPP_REFERENCE set to a fork build that has FORK FIX #66, the C++
// output must match too. z3 4.7.1 without it hangs; that reference is
// skipped.
func TestWPADirectoryVectorsCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the C++ z3 executable")
	}
	run := func(name string, timeout time.Duration) (stdout, stderr bytes.Buffer, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, reference, "-dl", filepath.Join("test_vectors", "wpa", name))
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err = cmd.Run()
		return
	}
	if _, stderr, _ := run("basic", 5*time.Second); strings.HasPrefix(stderr.String(), "NOT IMPLEMENTED YET!") {
		t.Skip("the C++ reference lacks the POSIX get_file_names (FORK FIX #66)")
	}
	for name, want := range wpaDirectoryVectors {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := run(name, 20*time.Second)
			if err != nil {
				t.Fatal("C++ reference", err, stdout.String(), stderr.String())
			}
			got := wpaTimingLines.ReplaceAllString(stdout.String(), "")
			if got != want.stdout || stderr.String() != want.stderr {
				t.Fatalf("C++ stdout %q stderr %q\nwant stdout %q stderr %q", got, stderr.String(), want.stdout, want.stderr)
			}
		})
	}
}
