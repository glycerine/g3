package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticNormalizationCleanupUnderNumericalGuard(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	n := arithmeticNormalization{manager: m}
	value, err := m.ArithmeticRealNumeralText("123456789012345678901234567890123456789012345678901234567891/987654321098765432109876543210987654321098765432109876543211")
	closureMust(t, err)
	_, err = n.keep(value, nil)
	closureMust(t, err)
	numbers := m.NumeralService().Manager.Z
	active := false
	closureMust(t, numbers.SetCallerExecutionGuard(func() bool { return active }))
	defer func() {
		active = false
		closureMust(t, numbers.SetCallerExecutionGuard(nil))
		closureMust(t, n.close())
	}()
	active = true
	err = n.close()
	active = false
	closureMust(t, err)
	if _, e := m.GetKind(value); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("cleanup did not consume its numeral root", e)
	}
	closureMust(t, n.close())
	if len(n.refs) != 0 {
		t.Fatal("cleanup retry left owned roots", len(n.refs))
	}
}
