package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestPurifyIntegerDivisionAndModConstraints(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%v", proofs), func(t *testing.T) {
			runner, m := qfnraPurifier(t, proofs, false)
			x := fixDLConst(t, m, true, "integer-div-x")
			y := fixDLConst(t, m, true, "integer-div-y")
			zero := fixDLNumber(t, m, true, "0")
			div := fixDLApp(t, m, qfnraPurifyIDiv, x, y)
			mod := fixDLApp(t, m, qfnraPurifyMod, x, y)
			goal, err := tactic.NewGoal(m, proofs, true, true)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, goal.Close()) })
			for _, term := range []ast.ASTHandle{div, mod} {
				formula, e := m.MkEq(term, zero)
				equationMust(t, e)
				fixDLPin(t, m, formula)
				var p ast.ASTHandle
				if proofs {
					p, e = m.MkProofAsserted(formula)
					equationMust(t, e)
				}
				equationMust(t, goal.AssertWithProof(formula, p, ast.ASTDependency{}))
			}
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, runner.Apply(goal, &result))
			size, err := goal.Size()
			equationMust(t, err)
			if size != 6 || result.ModelConverter == nil {
				t.Fatalf("got %d formulas, converter %v", size, result.ModelConverter)
			}
			decls := make([]ast.ASTHandle, 4)
			decls[0], err = m.GetDecl(x)
			equationMust(t, err)
			decls[1], err = m.GetDecl(y)
			equationMust(t, err)
			for j := uint32(0); j < 2; j++ {
				form, e := goal.Form(j)
				equationMust(t, e)
				fresh, e := m.GetArg(form, 0)
				equationMust(t, e)
				decls[j+2], e = m.GetDecl(fresh)
				equationMust(t, e)
			}
			if decls[2] == decls[3] {
				t.Fatal("quotient and remainder share a fresh constant")
			}
			for j := uint32(0); proofs && j < size; j++ {
				p, e := goal.Proof(j)
				equationMust(t, e)
				fact, e := m.GetProofFact(p)
				equationMust(t, e)
				form, e := goal.Form(j)
				equationMust(t, e)
				if fact != form {
					t.Fatalf("wrong proof conclusion for formula %d", j)
				}
			}
			for _, row := range []struct {
				x, y, q, r int
				valid      bool
			}{
				{7, 3, 2, 1, true}, {-7, 3, -3, 2, true}, {7, -3, -2, 1, true}, {-7, -3, 3, 2, true},
				{7, 3, 1, 4, false}, {7, 3, 3, -2, false}, {-7, -3, 2, -1, false}, {7, 0, 99, -50, true},
			} {
				md, e := model.NewModel(m)
				equationMust(t, e)
				for j, v := range []int{row.x, row.y, row.q, row.r} {
					num := fixDLNumber(t, m, true, fmt.Sprint(v))
					equationMust(t, md.CoreField.RegisterConstant(decls[j], num))
				}
				ev, e := model.NewModelEvaluator(&md.CoreField, runner.services)
				equationMust(t, e)
				out, e := m.NewRef(ast.ASTHandle{})
				equationMust(t, e)
				valid := true
				for j := uint32(2); j < size; j++ {
					form, e := goal.Form(j)
					equationMust(t, e)
					equationMust(t, ev.Evaluate(form, out))
					value, e := m.RefGet(out)
					equationMust(t, e)
					if !m.IsTrue(value) && !m.IsFalse(value) {
						t.Fatal("non-Boolean constraint result")
					}
					valid = valid && m.IsTrue(value)
				}
				equationMust(t, m.DelRef(&out))
				equationMust(t, ev.Close())
				equationMust(t, md.Close())
				if valid != row.valid {
					t.Fatalf("%+v yielded %v", row, valid)
				}
			}
		})
	}
}

func TestPurifyIntegerDivisionZeroGuardsAndModFirst(t *testing.T) {
	for _, row := range []struct {
		name        string
		divisor     string
		constraints uint32
	}{
		{"symbolic", "", 6},
		{"zero", "0", 6},
		{"nonzero", "-3", 4},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner, m := qfnraPurifier(t, true, true)
			x := fixDLConst(t, m, true, "mod-first-x")
			y := fixDLConst(t, m, true, "mod-first-y")
			if row.divisor != "" {
				y = fixDLNumber(t, m, true, row.divisor)
			}
			term := fixDLApp(t, m, qfnraPurifyMod, x, y)
			zero := fixDLNumber(t, m, true, "0")
			formula, err := m.MkEq(term, zero)
			equationMust(t, err)
			fixDLPin(t, m, formula)
			p, err := m.MkProofAsserted(formula)
			equationMust(t, err)
			goal, err := tactic.NewGoal(m, true, true, false)
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, goal.Close()) })
			for j := 0; j < 2; j++ {
				equationMust(t, goal.AssertWithProof(formula, p, ast.ASTDependency{}))
			}
			division := fixDLApp(t, m, qfnraPurifyIDiv, x, y)
			divisionFormula, err := m.MkEq(division, zero)
			equationMust(t, err)
			fixDLPin(t, m, divisionFormula)
			divisionProof, err := m.MkProofAsserted(divisionFormula)
			equationMust(t, err)
			equationMust(t, goal.AssertWithProof(divisionFormula, divisionProof, ast.ASTDependency{}))
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, runner.Apply(goal, &result))
			size, err := goal.Size()
			equationMust(t, err)
			if size != 3+row.constraints {
				t.Fatalf("got %d formulas", size)
			}
			first, err := goal.Form(0)
			equationMust(t, err)
			second, err := goal.Form(1)
			equationMust(t, err)
			if first != second || first == formula {
				t.Fatal("modulus cache was not reused")
			}
			// Both auxiliary declarations must be filtered from the model.
			constraint, err := goal.Form(3)
			equationMust(t, err)
			equality, err := m.GetArg(constraint, 1)
			equationMust(t, err)
			sum, err := m.GetArg(equality, 1)
			equationMust(t, err)
			product, err := m.GetArg(sum, 0)
			equationMust(t, err)
			quotient, err := m.GetArg(product, 0)
			equationMust(t, err)
			divisionResult, err := goal.Form(2)
			equationMust(t, err)
			cachedQuotient, err := m.GetArg(divisionResult, 0)
			equationMust(t, err)
			if quotient != cachedQuotient {
				t.Fatal("division did not reuse modulus-first quotient")
			}
			constraintProof, err := goal.Proof(3)
			equationMust(t, err)
			premises, err := m.GetProofNumParents(constraintProof)
			equationMust(t, err)
			if premises != 2 {
				t.Fatalf("Euclidean equation has %d proof premises", premises)
			}
			quotientProof, err := m.GetProofParent(constraintProof, 0)
			equationMust(t, err)
			quotientFact, err := m.GetProofFact(quotientProof)
			equationMust(t, err)
			definedTerm, err := m.GetArg(quotientFact, 0)
			equationMust(t, err)
			definedValue, err := m.GetArg(quotientFact, 1)
			equationMust(t, err)
			if definedTerm != division || definedValue != quotient {
				t.Fatal("quotient proof does not define integer division")
			}
			remainder, err := m.GetArg(first, 0)
			equationMust(t, err)
			md, err := model.NewModel(m)
			equationMust(t, err)
			var decls []ast.ASTHandle
			for _, fresh := range []ast.ASTHandle{quotient, remainder} {
				decl, e := m.GetDecl(fresh)
				equationMust(t, e)
				decls = append(decls, decl)
				equationMust(t, md.CoreField.RegisterConstant(decl, zero))
			}
			var output tactic.ModelRef
			equationMust(t, output.Set(&md.CoreField))
			t.Cleanup(func() { equationMust(t, output.Close()) })
			equationMust(t, result.ModelConverter.ApplyModel(&output, 0))
			filtered, err := output.Get()
			equationMust(t, err)
			for _, decl := range decls {
				present, e := filtered.HasInterpretation(decl)
				equationMust(t, e)
				if present {
					t.Fatal("integer division auxiliary escaped model filtering")
				}
			}
			for j := uint32(0); j < size; j++ {
				p, e := goal.Proof(j)
				equationMust(t, e)
				fact, e := m.GetProofFact(p)
				equationMust(t, e)
				form, e := goal.Form(j)
				equationMust(t, e)
				if fact != form {
					t.Fatalf("wrong proof conclusion at %d", j)
				}
			}
		})
	}
}
