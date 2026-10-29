package smt2

import (
	"bytes"
	"testing"
)

func TestSMT2MarshalStringAndWriterFormsAreIdentical(t *testing.T) {
	_, manager, formula := marshalTestFormula(t, `(declare-const p Bool)(declare-const q Bool)(assert (or p q))`)
	text, err := MarshalString(formula, manager)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = Marshal(&output, formula, manager); err != nil {
		t.Fatal(err)
	}
	if output.String() != text {
		t.Fatalf("writer/string mismatch\nwriter: %q\nstring: %q", output.String(), text)
	}
	fromReader, err := Unmarshal(bytes.NewBufferString(text), manager)
	if err != nil {
		t.Fatal(err)
	}
	fromString, err := UnmarshalString(text, manager)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := manager.RefGet(fromReader)
	right, _ := manager.RefGet(fromString)
	if left != right || left != formula {
		t.Fatalf("reader/string results %v %v, want %v", left, right, formula)
	}
	if err = manager.DelRef(&fromString); err != nil {
		t.Fatal(err)
	}
	if err = manager.DelRef(&fromReader); err != nil {
		t.Fatal(err)
	}
}
