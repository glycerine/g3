//go:build g3trace

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The IF_VERBOSE progress lines ported for AUDIT.md FUZZ-F5 are compiled
// only into g3trace builds (owner decision 2026-11-15), so this test is
// tagged. It runs the g3 front end in a child process (the timeit lines go
// to the process standard error, as std::cerr does in the C++) and keeps
// the lines of every ported site, in order. With G3_CPP_REFERENCE set it
// also compares them with build/z3's standard error.

// verboseProgressPrefixes are the leading texts of the ported sites.
var verboseProgressPrefixes = []string{
	"(started \"check-sat\")", "(combined-solver ", "(smt.searching)",
	"(smt.final-check ", "(smt.mbqi)", "(smt.giveup quantifiers)",
}

// verboseProgressLines keeps the ported progress lines of log, in order.
// timeit's :time and memory fields are dropped.
func verboseProgressLines(log string) []string {
	var out []string
	for _, line := range strings.Split(log, "\n") {
		for _, prefix := range verboseProgressPrefixes {
			if strings.HasPrefix(line, prefix) {
				if head, _, found := strings.Cut(line, " :time "); found {
					line = head + ")"
				}
				out = append(out, line)
				break
			}
		}
	}
	return out
}

type verboseProgressCase struct {
	name, script string
	args         []string
	want         []string
}

var verboseProgressCases = []verboseProgressCase{
	{name: "arith", script: "(declare-const x Int)\n(declare-const y Int)\n(assert (> (+ x (* 3 y)) 7))\n(assert (< (- x y) 2))\n(assert (or (= x 5) (= y 9)))\n(check-sat)\n",
		want: []string{"(started \"check-sat\")", "(combined-solver \"using solver 1\")", "(smt.searching)", "(smt.final-check \"arithmetic\")"}},
	{name: "quantified", script: "(declare-fun f (Int) Int)\n(assert (forall ((x Int)) (= (f x) 3)))\n(assert (> (f 3) 1))\n(check-sat)\n",
		want: []string{"(started \"check-sat\")", "(combined-solver \"using solver 1\")", "(smt.searching)",
			"(smt.final-check \"arithmetic\")", "(smt.final-check \"array\")", "(smt.final-check \"datatype\")",
			"(smt.final-check \"bit-vector\")", "(smt.final-check \"datalog\")", "(smt.final-check \"seq\")",
			"(smt.final-check \"unknown\")", "(smt.final-check \"fpa\")", "(smt.final-check \"quantifiers\")", "(smt.mbqi)"}},
}

const verboseProgressChildEnv = "G3_TEST_VERBOSE_PROGRESS_ARGS"

// runVerboseProgressChild runs g3 with args in a child copy of the test
// binary and returns its standard error.
func runVerboseProgressChild(t *testing.T, args []string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestVerboseProgressMessages$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), verboseProgressChildEnv+"="+strings.Join(args, "\x1f"))
	var out, diagnostics bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostics
	if err := cmd.Run(); err != nil {
		t.Fatal(err, out.String(), diagnostics.String())
	}
	return diagnostics.String()
}

func TestVerboseProgressMessages(t *testing.T) {
	if args := os.Getenv(verboseProgressChildEnv); args != "" {
		os.Args = append([]string{"g3"}, strings.Split(args, "\x1f")...)
		main()
		return
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, tc := range verboseProgressCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name+".smt2")
			if err := os.WriteFile(path, []byte(tc.script), 0o644); err != nil {
				t.Fatal(err)
			}
			args := tc.args
			if args == nil {
				args = []string{"-v:100"}
			}
			args = append(append([]string(nil), args...), path)
			got := verboseProgressLines(runVerboseProgressChild(t, args))
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("Go progress lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			if reference == "" {
				return
			}
			var native, cppLog bytes.Buffer
			cmd := exec.Command(reference, args...)
			cmd.Stdout = &native
			cmd.Stderr = &cppLog
			if err := cmd.Run(); err != nil {
				t.Fatal(err, native.String(), cppLog.String())
			}
			cpp := verboseProgressLines(cppLog.String())
			if strings.Join(got, "\n") != strings.Join(cpp, "\n") {
				t.Fatalf("Go/C++ progress lines differ\nGo:\n%s\nC++:\n%s", strings.Join(got, "\n"), strings.Join(cpp, "\n"))
			}
		})
	}
}
