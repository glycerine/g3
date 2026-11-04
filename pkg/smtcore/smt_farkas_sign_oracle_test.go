package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// A single unit-coefficient premise must yield its logical complement. The
// values straddle zero, including half units for reals, to distinguish strict
// bounds from non-strict bounds and integer rounding from real arithmetic.
func TestSMTFarkasSignConsequenceSourceOracle(t *testing.T) {
	for _, real := range []bool{false, true} {
		for _, neg := range []bool{false, true} {
			for _, rel := range []struct {
				name string
				op   int32
			}{{"le", smtArithOpLE}, {"lt", smtArithOpLT}, {"ge", smtArithOpGE}, {"gt", smtArithOpGT}, {"eq", ast.ASTOpEq}} {
				name := "int"
				if real {
					name = "real"
				}
				if neg {
					name += "-not"
				}
				name += "-" + rel.name
				t.Run(name, func(t *testing.T) {
					m, u, family, intSort := smtFarkasFixture(t)
					sort := intSort
					if real {
						var err error
						sort, err = m.MkFamilySort(family, 0, nil)
						astTestMust(t, err)
					}
					x := smtFarkasConst(t, m, "sign-oracle-x", sort)
					zero, err := m.ArithmeticNumeralRat(new(big.Rat), !real)
					astTestMust(t, err)
					var bound ast.ASTHandle
					if rel.name == "eq" {
						bound, err = m.MkEq(x, zero)
						astTestMust(t, err)
					} else {
						bound = smtFarkasOp(t, m, family, rel.op, x, zero)
					}
					if neg {
						bound, err = m.MkNot(bound)
						astTestMust(t, err)
					}
					err = u.Add(big.NewRat(1, 1), bound)
					if neg && rel.name == "eq" {
						if err != ast.ErrASTDomain {
							t.Fatalf("negated equality: got %v, want domain error", err)
						}
						return
					}
					astTestMust(t, err)
					out, err := m.NewRef(ast.ASTHandle{})
					astTestMust(t, err)
					defer m.DelRef(&out)
					astTestMust(t, u.Get(out))
					h, err := m.RefGet(out)
					astTestMust(t, err)
					values := []*big.Rat{big.NewRat(-2, 1), big.NewRat(-1, 1), big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(2, 1)}
					if real {
						values = append(values, big.NewRat(-1, 2), big.NewRat(1, 2))
					}
					for _, v := range values {
						cmp := v.Sign()
						var premise bool
						switch rel.name {
						case "le":
							premise = cmp <= 0
						case "lt":
							premise = cmp < 0
						case "ge":
							premise = cmp >= 0
						case "gt":
							premise = cmp > 0
						case "eq":
							premise = cmp == 0
						}
						if neg {
							premise = !premise
						}
						if got := evalFarkasOracle(t, m, h, x, v, family); got == premise {
							t.Errorf("x=%s: consequence = %v, premise = %v", v, got, premise)
						}
					}
				})
			}
		}
	}
}

func evalFarkasOracle(t *testing.T, m *ast.ASTManager, h, x ast.ASTHandle, v *big.Rat, family ast.FamilyID) bool {
	t.Helper()
	neg, child, err := m.IsNot(h)
	astTestMust(t, err)
	if neg {
		return !evalFarkasOracle(t, m, child, x, v, family)
	}
	left, err := m.GetArg(h, 0)
	astTestMust(t, err)
	right, err := m.GetArg(h, 1)
	astTestMust(t, err)
	value := func(a ast.ASTHandle) *big.Rat {
		if a == x {
			return v
		}
		text, _, ok, err := m.ArithmeticNumeralText(a)
		astTestMust(t, err)
		if !ok {
			t.Fatalf("expected variable or numeral, got %v", a)
		}
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatalf("invalid numeral %q", text)
		}
		return r
	}
	cmp := value(left).Cmp(value(right))
	f, err := m.GetFamilyID(h)
	astTestMust(t, err)
	op, err := m.GetDeclKind(h)
	astTestMust(t, err)
	if f == m.BasicFamilyID() && op == ast.ASTOpEq {
		return cmp == 0
	}
	if f != family {
		t.Fatalf("unexpected relation family %d", f)
	}
	switch op {
	case smtArithOpLE:
		return cmp <= 0
	case smtArithOpLT:
		return cmp < 0
	case smtArithOpGE:
		return cmp >= 0
	case smtArithOpGT:
		return cmp > 0
	}
	t.Fatalf("unexpected relation operation %d", op)
	return false
}
