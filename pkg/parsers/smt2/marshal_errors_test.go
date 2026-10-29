package smt2

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMT2MarshalInputAndMalformedStreamGuards(t *testing.T) {
	if err := Marshal(nil, ast.ASTHandle{}, nil); !errors.Is(err, ErrMarshalInput) {
		t.Fatalf("nil marshal error %v", err)
	}
	if _, err := Unmarshal(nil, nil); !errors.Is(err, ErrMarshalInput) {
		t.Fatalf("nil unmarshal error %v", err)
	}
	_, manager, _ := marshalTestFormula(t, `(assert true)`)
	if _, err := UnmarshalString("(assert", manager); err == nil {
		t.Fatal("malformed stream accepted")
	}
	if _, err := Unmarshal(strings.NewReader("(assert true) ["), manager); err == nil {
		t.Fatal("trailing token accepted")
	}
}

func TestSMT2MarshalNonBooleanRetainsSourceBareTermFrontier(t *testing.T) {
	_, manager, _ := marshalTestFormula(t, `(set-logic QF_LIA)(assert true)`)
	numeral, err := manager.ArithmeticIntegerNumeralText("42")
	if err != nil {
		t.Fatal(err)
	}
	text, err := MarshalString(numeral, manager)
	if err != nil || text != "42" {
		t.Fatalf("non-Boolean marshal %q, %v", text, err)
	}
	if _, err = UnmarshalString(text, manager); err == nil {
		t.Fatal("bare non-command term unexpectedly unmarshaled")
	}
}
