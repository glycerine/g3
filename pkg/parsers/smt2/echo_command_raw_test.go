package smt2

import "testing"

func TestSMT2EchoRetainsRawNoncompliantMode(t *testing.T) {
	output, err := runApplyTacticScript(t, "(echo \"raw text\")")
	if err != nil || output != "raw text\n" {
		t.Fatal(err, output)
	}
}
