package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// AUDIT.md interp_02-F4: iz3translate::translate_main prints
// IF_VERBOSE(0, "Unsupported proof rule: " << expr_ref(proof) << "\n")
// before throwing on an unsupported rule.
func TestSMTLibRunnerInterpUnsupportedRuleMessage(t *testing.T) {
	script, err := os.ReadFile("test_vectors/interp_unsupported_rule_message.smt2")
	if err != nil {
		t.Fatal(err)
	}
	previous, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	level, err := diagnostic.ProcessVerbosityLevel()
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
	if err = diagnostic.SetProcessVerbosityLevel(0); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); !expectedParseError("interp_unsupported_rule_message.smt2", err) {
		t.Fatal("expected the interpolation command error", err)
	}
	const result = "(error \"line 5 column 131: theory not supported by interpolation or bad proof\")\n"
	if out.String() != result {
		t.Fatalf("result %q", out.String())
	}
	got, err := stream.String()
	if err != nil {
		t.Fatal(err)
	}
	const message = "Unsupported proof rule: (rewrite (= (or false (q b)) (q b)))\n"
	if got != message {
		t.Fatalf("stderr %q, want %q", got, message)
	}
}
