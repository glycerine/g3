package smtcore

import (
	"math/big"
	"reflect"
	"testing"
)

func TestSMTArithmeticOppositeIndexExcludesIndependentBounds(t *testing.T) {
	theory := &SMTLinearArithmeticTheory{}
	add := func(column uint32, coefficient, constant int64) {
		form := newSMTLinearForm()
		form.terms[column] = big.NewRat(coefficient, 1)
		form.constant.SetInt64(constant)
		index := len(theory.atoms)
		theory.atoms = append(theory.atoms, smtLinearArithmeticAtom{supported: true, op: smtArithOpLE, form: form})
		theory.assertions = append(theory.assertions, smtIntegerDiffAssertion{atom: index, truth: true})
	}
	for i := uint32(0); i < 1024; i++ {
		add(i, 1, -3)
	}
	add(7, -1, 3)
	add(7, -1, 3)
	add(7, -1, 4) // Same coefficients, different endpoint: not an opposite.
	opposites := theory.oppositeArithmeticAssertions()
	for i := 0; i < 1024; i++ {
		if i == 7 {
			if !reflect.DeepEqual(opposites[i], []int{1024, 1025}) {
				t.Fatalf("ordered matches = %v", opposites[i])
			}
		} else if len(opposites[i]) != 0 {
			t.Fatalf("unrelated bound %d has candidates %v", i, opposites[i])
		}
	}
	for _, i := range []int{1024, 1025} {
		if !reflect.DeepEqual(opposites[i], []int{7}) {
			t.Fatalf("reverse matches = %v", opposites[i])
		}
	}
	if len(opposites[1026]) != 0 {
		t.Fatal("different endpoint matched")
	}
}
