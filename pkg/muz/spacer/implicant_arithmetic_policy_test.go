package spacer

// Source: src/muz/spacer/spacer_util.cpp:740-772, implicant_picker::add_literal.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerImplicantArithmeticOrientationAndSoundness(t *testing.T) {
	cases := []struct {
		name, left, right, form string
		real, reverse           bool
	}{
		{"integer-forward", "-3", "2", "not-eq", false, false},
		{"integer-reverse", "5", "-2", "not-eq", false, true},
		{"integer-distinct", "0", "1", "distinct", false, false},
		{"real-negative", "-7/3", "-2/3", "not-eq", true, false},
		{"real-reverse", "3/7", "1/7", "distinct", true, true},
		{"large-rational", "18446744073709551617/18446744073709551619", "1", "not-eq", true, false},
		{"integer-equality", "4", "4", "eq", false, false},
		{"normalized-real-equality", "2/6", "1/3", "eq", true, false},
		{"integer-false-distinct", "4", "4", "not-distinct", false, true},
		{"real-false-distinct", "2/6", "1/3", "not-distinct", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, globals := newSpacerTestServices(t)
			config := smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals}
			owner, err := NewManager(m, nil, 1, config)
			depthMust(t, err)
			defer func() { depthMust(t, owner.Close()) }()
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			number := func(text string) ast.ASTHandle {
				if tc.real {
					return own(m.ArithmeticRealNumeralText(text))
				}
				return own(m.ArithmeticIntegerNumeralText(text))
			}
			leftValue, rightValue := number(tc.left), number(tc.right)
			sort, err := m.GetSort(leftValue)
			depthMust(t, err)
			x, y := own(m.MkFreshConst("arithmetic-picker-x", sort)), own(m.MkFreshConst("arithmetic-picker-y", sort))
			equality := own(m.MkEq(x, y))
			var formula, expected ast.ASTHandle
			switch tc.form {
			case "not-eq":
				formula = own(m.MkNot(equality))
			case "distinct":
				formula = own(m.MkDistinct([]ast.ASTHandle{x, y}))
			case "eq":
				formula = equality
				expected = equality
			case "not-distinct":
				distinct := own(m.MkDistinct([]ast.ASTHandle{x, y}))
				formula = own(m.MkNot(distinct))
				expected = own(m.MkEq(y, x))
			}
			if tc.form == "not-eq" || tc.form == "distinct" {
				if tc.reverse {
					expected = own(m.MkArithmeticLT(y, x))
				} else {
					expected = own(m.MkArithmeticLT(x, y))
				}
			}
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			for i, variable := range []ast.ASTHandle{x, y} {
				decl, err := m.GetDecl(variable)
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(decl, []ast.ASTHandle{leftValue, rightValue}[i]))
			}
			before := m.Stats().LiveReferences
			refs, err := computeImplicantLiterals(m, md.Core(), owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{formula})
			depthMust(t, err)
			if len(refs) != 1 {
				depthMust(t, releaseNormalizeRefs(m, refs))
				t.Fatal("arithmetic literal count", len(refs))
			}
			literal, err := m.RefGet(refs[0])
			depthMust(t, err)
			depthMust(t, releaseNormalizeRefs(m, refs))
			if literal != expected {
				t.Fatal("source arithmetic literal orientation", literal, expected)
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("temporary arithmetic references", got, before)
			}
			logic := "QF_LIA"
			if tc.real {
				logic = "QF_LRA"
			}
			backend, err := smtcore.NewSMTSolver(m, nil, logic, config)
			depthMust(t, err)
			defer func() { depthMust(t, backend.Close()) }()
			depthMust(t, backend.Assert(literal))
			depthMust(t, backend.Push())
			depthMust(t, backend.Assert(own(m.MkNot(formula))))
			status, err := backend.CheckSat(nil)
			depthMust(t, err)
			if status != lbool.False {
				t.Fatal("arithmetic literal does not imply formula", status)
			}
			depthMust(t, backend.Pop(1))
			depthMust(t, backend.Assert(own(m.MkEq(x, leftValue))))
			depthMust(t, backend.Assert(own(m.MkEq(y, rightValue))))
			status, err = backend.CheckSat(nil)
			depthMust(t, err)
			if status != lbool.True {
				t.Fatal("arithmetic literal contradicts model", status)
			}
		})
	}
}
