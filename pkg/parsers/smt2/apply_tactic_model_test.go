package smt2

import (
	"strings"
	"testing"
)

func TestSMT2ApplyTacticPrintsModelConverter(t *testing.T) {
	script := `(set-logic QF_BV)(set-option :produce-models true)(declare-const x (_ BitVec 4))(assert (= x #x5))(apply bit-blast :print false :print_model_converter true)`
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.Contains(output, "(bit-blaster-model-converter") || !strings.Contains(output, "(x ") {
		t.Fatal(output)
	}
}
