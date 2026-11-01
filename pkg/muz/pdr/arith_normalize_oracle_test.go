package pdr

// Exact rational oracle for pdr_util.cpp normalize_arithmetic. This evaluator
// uses Go arithmetic and does not invoke a project rewriter or model evaluator.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func normalizationOracleNumber(t *testing.T, m *ast.ASTManager, expression ast.ASTHandle, values map[ast.ASTHandle]*big.Rat) *big.Rat {
	t.Helper()
	if value, found := values[expression]; found {
		return new(big.Rat).Set(value)
	}
	text, _, numeral, err := m.ArithmeticNumeralText(expression)
	closureMust(t, err)
	if numeral {
		value, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatal("invalid rational", text)
		}
		return value
	}
	family, err := m.GetFamilyID(expression)
	closureMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	closureMust(t, err)
	op, err := m.GetDeclKind(expression)
	closureMust(t, err)
	if family != arithmetic || (op != 6 && op != 9) {
		t.Fatal("unexpected oracle term", expression, family, op)
	}
	result := new(big.Rat)
	if op == 9 {
		result.SetInt64(1)
	}
	count, err := m.GetNumArgs(expression)
	closureMust(t, err)
	for i := uint32(0); i < count; i++ {
		arg, err := m.GetArg(expression, i)
		closureMust(t, err)
		value := normalizationOracleNumber(t, m, arg, values)
		if op == 6 {
			result.Add(result, value)
		} else {
			result.Mul(result, value)
		}
	}
	return result
}

func normalizationOracleRelation(t *testing.T, m *ast.ASTManager, expression ast.ASTHandle, values map[ast.ASTHandle]*big.Rat) bool {
	t.Helper()
	negated, child, err := m.IsNot(expression)
	closureMust(t, err)
	if negated {
		return !normalizationOracleRelation(t, m, child, values)
	}
	left, err := m.GetArg(expression, 0)
	closureMust(t, err)
	right, err := m.GetArg(expression, 1)
	closureMust(t, err)
	comparison := normalizationOracleNumber(t, m, left, values).Cmp(normalizationOracleNumber(t, m, right, values))
	family, err := m.GetFamilyID(expression)
	closureMust(t, err)
	op, err := m.GetDeclKind(expression)
	closureMust(t, err)
	if family == m.BasicFamilyID() && op == ast.ASTOpEq {
		return comparison == 0
	}
	arithmetic, err := m.ArithmeticFamilyID()
	closureMust(t, err)
	if family != arithmetic {
		t.Fatal("unexpected oracle relation family", family)
	}
	switch op {
	case 2:
		return comparison <= 0
	case 3:
		return comparison >= 0
	case 4:
		return comparison < 0
	case 5:
		return comparison > 0
	default:
		t.Fatal("unexpected oracle relation", op)
		return false
	}
}

func TestPDRArithmeticNormalizationMatchesExactRationalOracle(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			closureMust(t, m.DelRef(&refs[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		closureMust(t, err)
		ref, err := m.NewRef(h)
		closureMust(t, err)
		refs = append(refs, ref)
		return h
	}
	realZero := own(m.ArithmeticRealNumeralText("0"))
	realSort, err := m.GetSort(realZero)
	closureMust(t, err)
	integerZero := own(m.ArithmeticIntegerNumeralText("0"))
	integerSort, err := m.GetSort(integerZero)
	closureMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	closureMust(t, err)
	cases := []struct {
		name            string
		coefficients    [6]string
		integer, scaled bool
		scale           string
	}{
		{"signed-fractions", [6]string{"-6", "3/2", "-9", "3", "-3/4", "15/2"}, false, true, "3/4"},
		{"thirds", [6]string{"-2/3", "4/3", "8/3", "2", "-10/3", "14/3"}, false, true, "2/3"},
		{"zero-reset", [6]string{"0", "2", "0", "4", "0", "6"}, false, false, "6"},
		{"unit-divisor", [6]string{"1", "2", "3", "-4", "5", "-6"}, false, false, "1"},
		{"integer-no-scaling", [6]string{"-6", "2", "-8", "4", "-2", "10"}, true, false, "1"},
	}
	samples := []string{"-3", "-1/2", "0", "2/3", "4"}
	checked := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sort := realSort
			if tc.integer {
				sort = integerSort
			}
			x := own(m.MkFreshConst("oracle-x", sort))
			y := own(m.MkFreshConst("oracle-y", sort))
			constants := make([]ast.ASTHandle, 6)
			for i, text := range tc.coefficients {
				if tc.integer {
					constants[i] = own(m.ArithmeticIntegerNumeralText(text))
				} else {
					constants[i] = own(m.ArithmeticRealNumeralText(text))
				}
			}
			polynomial := func(start int) ast.ASTHandle {
				a := own(m.MkArithmeticMul(constants[start], x))
				b := own(m.MkArithmeticMul(constants[start+1], y))
				return own(m.MkFamilyApp(arithmetic, 6, nil, []ast.ASTHandle{a, b, constants[start+2]}))
			}
			left, right := polynomial(0), polynomial(3)
			for _, reverse := range []bool{false, true} {
				lhs, rhs := left, right
				if reverse {
					lhs, rhs = rhs, lhs
				}
				for _, op := range []int32{-1, 2, 3, 4, 5} {
					input := ast.ASTHandle{}
					if op == -1 {
						input = own(m.MkEq(lhs, rhs))
					} else {
						input = own(m.MkFamilyApp(arithmetic, op, nil, []ast.ASTHandle{lhs, rhs}))
					}
					output, err := m.NewRef(input)
					closureMust(t, err)
					closureMust(t, NormalizeArithmeticRef(m, output))
					normalized, err := m.RefGet(output)
					closureMust(t, err)
					if tc.scaled && normalized == input {
						t.Fatal("non-unit rational coefficients were not normalized")
					}
					if tc.integer && op <= 3 && normalized != input {
						t.Fatal("integer coefficients were scaled")
					}
					core := normalized
					negated, child, err := m.IsNot(core)
					closureMust(t, err)
					if negated {
						core = child
					}
					normalizedLeft, err := m.GetArg(core, 0)
					closureMust(t, err)
					normalizedRight, err := m.GetArg(core, 1)
					closureMust(t, err)
					scaleText := tc.scale
					// Source update_coeff resets at zero; in this orientation
					// the last coefficient is zero, suppressing division.
					if tc.name == "zero-reset" && reverse {
						scaleText = "1"
					}
					scale, ok := new(big.Rat).SetString(scaleText)
					if !ok {
						t.Fatal("invalid expected scale")
					}
					for _, sx := range samples {
						for _, sy := range samples {
							if tc.integer && (sx == "-1/2" || sx == "2/3" || sy == "-1/2" || sy == "2/3") {
								continue
							}
							vx, _ := new(big.Rat).SetString(sx)
							vy, _ := new(big.Rat).SetString(sy)
							values := map[ast.ASTHandle]*big.Rat{x: vx, y: vy}
							for _, side := range []struct{ original, normalized ast.ASTHandle }{{lhs, normalizedLeft}, {rhs, normalizedRight}} {
								expectedNumber := normalizationOracleNumber(t, m, side.original, values)
								expectedNumber.Quo(expectedNumber, scale)
								actualNumber := normalizationOracleNumber(t, m, side.normalized, values)
								if expectedNumber.Cmp(actualNumber) != 0 {
									t.Fatalf("reverse=%v op=%d scale=%s: side evaluates to %s, want %s", reverse, op, scaleText, actualNumber.RatString(), expectedNumber.RatString())
								}
							}

							expected := normalizationOracleRelation(t, m, input, values)
							actual := normalizationOracleRelation(t, m, normalized, values)
							if expected != actual {
								t.Fatalf("op=%d reverse=%v x=%s y=%s: normalized=%v original=%v", op, reverse, sx, sy, actual, expected)
							}
							checked++
						}
					}
					closureMust(t, m.DelRef(&output))
				}
			}
		})
	}
	if checked != 1090 {
		t.Fatalf("oracle coverage = %d, want 1090", checked)
	}
}
