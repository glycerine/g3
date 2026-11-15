package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// verboseProgressLines keeps the IF_VERBOSE progress lines ported for
// AUDIT.md FUZZ-F5, in order.
func verboseProgressLines(log string) []string {
	var out []string
	for _, line := range strings.Split(log, "\n") {
		for _, prefix := range []string{"(started \"check-sat\")", "(combined-solver ", "(smt.searching)", "(smt.final-check ", "(smt.mbqi)", "(smt.giveup quantifiers)"} {
			if strings.HasPrefix(line, prefix) {
				out = append(out, line)
				break
			}
		}
	}
	return out
}

func TestVerboseProgressMessages(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		want         []string
	}{
		{"arith", "(declare-const x Int)\n(declare-const y Int)\n(assert (> (+ x (* 3 y)) 7))\n(assert (< (- x y) 2))\n(assert (or (= x 5) (= y 9)))\n(check-sat)\n",
			[]string{"(started \"check-sat\")", "(combined-solver \"using solver 1\")", "(smt.searching)", "(smt.final-check \"arithmetic\")"}},
		{"quantified", "(declare-fun f (Int) Int)\n(assert (forall ((x Int)) (= (f x) 3)))\n(assert (> (f 3) 1))\n(check-sat)\n",
			[]string{"(started \"check-sat\")", "(combined-solver \"using solver 1\")", "(smt.searching)",
				"(smt.final-check \"arithmetic\")", "(smt.final-check \"array\")", "(smt.final-check \"datatype\")",
				"(smt.final-check \"bit-vector\")", "(smt.final-check \"datalog\")", "(smt.final-check \"seq\")",
				"(smt.final-check \"unknown\")", "(smt.final-check \"fpa\")", "(smt.final-check \"quantifiers\")", "(smt.mbqi)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			t.Cleanup(func() {
				if err := diagnostic.SetProcessVerboseStream(previous); err != nil {
					t.Error(err)
				}
				if err := diagnostic.SetProcessVerbosityLevel(level); err != nil {
					t.Error(err)
				}
				if err := stream.Close(); err != nil {
					t.Error(err)
				}
			})
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerbosityLevel(100); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(strings.NewReader(tc.script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			log, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			got := verboseProgressLines(log)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("Go progress lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				var native, cppLog bytes.Buffer
				cmd := exec.Command(reference, "-v:100", "-in")
				cmd.Stdin = strings.NewReader(tc.script)
				cmd.Stdout = &native
				cmd.Stderr = &cppLog
				if err := cmd.Run(); err != nil {
					t.Fatal(err, native.String(), cppLog.String())
				}
				cpp := verboseProgressLines(cppLog.String())
				if strings.Join(got, "\n") != strings.Join(cpp, "\n") {
					t.Fatalf("Go/C++ progress lines differ\nGo:\n%s\nC++:\n%s", strings.Join(got, "\n"), strings.Join(cpp, "\n"))
				}
			}
		})
	}
}
