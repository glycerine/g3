package ast

import "testing"

func TestASTPseudoBooleanIntegerAPIBoundaryPreservesSignedValues(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	a, err := m.MkTrue()
	astTestMust(t, err)
	b, err := m.MkFalse()
	astTestMust(t, err)
	for _, test := range []struct {
		kind int32
		make func([]int32, []ASTHandle, int32) (ASTHandle, error)
	}{
		{ASTPBOpLE, m.MkPseudoBooleanLEIntegers},
		{ASTPBOpGE, m.MkPseudoBooleanGEIntegers},
		{ASTPBOpEQ, m.MkPseudoBooleanEQIntegers},
	} {
		formula, e := test.make([]int32{-3, 7}, []ASTHandle{a, b}, -2)
		astTestMust(t, e)
		if kind, values := pbTestParameters(t, m, formula); kind != test.kind || len(values) != 3 || values[0] != -2 || values[1] != -3 || values[2] != 7 {
			t.Fatalf("kind %d parameters: %d %v", test.kind, kind, values)
		}
	}
}

func TestASTPseudoBooleanIntegerAPIBoundaryChecksArity(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	a, err := m.MkTrue()
	astTestMust(t, err)
	if _, err = m.MkPseudoBooleanLEIntegers(nil, []ASTHandle{a}, 0); err != ErrASTDomain {
		t.Fatalf("coefficient arity: %v", err)
	}
}
