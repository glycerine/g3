package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// C++ recognizes equality and arithmetic predicates with their declaration
// families, even when their operation numbers coincide.
func TestSMTFarkasDistinguishesEqualityAndInequality(t *testing.T) {
	m, utility, family, integer := smtFarkasFixture(t)
	x := smtFarkasConst(t, m, "family-oracle-x", integer)
	zero := smtFarkasInt(t, m, 0)
	eq, err := m.MkEq(x, zero)
	astTestMust(t, err)
	le := smtFarkasOp(t, m, family, smtArithOpLE, x, zero)
	for _, tc := range []struct {
		name   string
		input  ast.ASTHandle
		family ast.FamilyID
		op     int32
	}{
		{"equality", eq, m.BasicFamilyID(), ast.ASTOpEq},
		{"inequality", le, family, smtArithOpLE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			astTestMust(t, utility.Reset())
			astTestMust(t, utility.Add(big.NewRat(1, 1), tc.input))
			out, err := m.NewRef(ast.ASTHandle{})
			astTestMust(t, err)
			defer m.DelRef(&out)
			astTestMust(t, utility.Get(out))
			result, err := m.RefGet(out)
			astTestMust(t, err)
			negated, body, err := m.IsNot(result)
			astTestMust(t, err)
			if !negated {
				t.Fatal("expected complemented consequence")
			}
			actualFamily, err := m.GetFamilyID(body)
			astTestMust(t, err)
			op, err := m.GetDeclKind(body)
			astTestMust(t, err)
			if actualFamily != tc.family || op != tc.op {
				t.Fatalf("consequence family/op = %d/%d, want %d/%d", actualFamily, op, tc.family, tc.op)
			}
		})
	}
}
