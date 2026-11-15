package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// AUDIT.md muz_fp_04-F2: spacer dl_interface::query prints the transformed
// rules under IF_VERBOSE(2, m_ctx.display_rules(verbose_stream())).
func TestSpacerVerboseTransformedRules(t *testing.T) {
	const script = `(set-option :fixedpoint.engine spacer)
(set-option :fixedpoint.print_answer true)
(declare-rel inv (Int Int))
(declare-var x Int)
(declare-var y Int)
(rule (=> (and (= x 0) (= y 5)) (inv x y)))
(rule (=> (and (inv x y) (< x 10)) (inv (+ x 1) y)))
(query inv)
`
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
	if err = diagnostic.SetProcessVerbosityLevel(2); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	verbose, err := stream.String()
	if err != nil {
		t.Fatal(err)
	}
	const want = "; rule count: 3\n; predicate count: 2\n; output: query!0\n<null>:\ninv(0,5).\n<null>:\ninv(#2,#0) :- \n inv(#1,#0),\n (not (<= 10 (:var 1))),\n (= (:var 2) (+ 1 (:var 1))).\n<null>:\nquery!0(#1,#0) :- \n inv(#0,#1).\n"
	if !strings.Contains(verbose, want) {
		t.Fatalf("verbose output lacks the transformed rule dump:\n%s", verbose)
	}
}
