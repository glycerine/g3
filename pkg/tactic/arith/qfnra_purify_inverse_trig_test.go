package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestPurifyInverseTrigRangesProofAndModel(t *testing.T) {
	for _, row := range []struct {
		name                   string
		op, forward, undefined int32
	}{{"asin", qfnraPurifyASin, forwardTrigSin, 34}, {"acos", qfnraPurifyACos, forwardTrigCos, 35}, {"atan", qfnraPurifyATan, 21, 0}} {
		for _, complete := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/complete=%v", row.name, complete), func(t *testing.T) {
				runner, m := qfnraPurifier(t, true, complete)
				x := fixDLConst(t, m, false, "inverse-x")
				term := fixDLApp(t, m, row.op, x)
				zero := fixDLNumber(t, m, false, "0")
				formula, err := m.MkEq(term, zero)
				equationMust(t, err)
				fixDLPin(t, m, formula)
				p, err := m.MkProofAsserted(formula)
				equationMust(t, err)
				goal, err := tactic.NewGoal(m, true, true, false)
				equationMust(t, err)
				t.Cleanup(func() { equationMust(t, goal.Close()) })
				equationMust(t, goal.AssertWithProof(formula, p, ast.ASTDependency{}))
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				equationMust(t, runner.Apply(goal, &result))
				size, err := goal.Size()
				equationMust(t, err)
				want := uint32(2)
				if (complete && row.op != qfnraPurifyATan) || row.op == qfnraPurifyATan {
					want = 4
				}
				if size != want || result.ModelConverter == nil {
					t.Fatalf("got %d formulas and converter %v", size, result.ModelConverter)
				}
				rewritten, err := goal.Form(0)
				equationMust(t, err)
				fresh, err := m.GetArg(rewritten, 0)
				equationMust(t, err)
				constraint, err := goal.Form(1)
				equationMust(t, err)
				inside := constraint
				if row.op != qfnraPurifyATan {
					inside, err = m.GetArg(constraint, 1)
					equationMust(t, err)
				}
				equality := inside
				if row.op != qfnraPurifyATan {
					equality, err = m.GetArg(inside, 0)
					equationMust(t, err)
				}
				forward, err := m.GetArg(equality, 1)
				equationMust(t, err)
				kind, err := m.GetDeclKind(forward)
				equationMust(t, err)
				argument, err := m.GetArg(forward, 0)
				equationMust(t, err)
				if kind != row.forward || argument != fresh {
					t.Fatal("incorrect inverse equation", kind)
				}
				var lower, upper ast.ASTHandle
				if row.op == qfnraPurifyATan {
					lower, err = goal.Form(2)
					equationMust(t, err)
					upper, err = goal.Form(3)
					equationMust(t, err)
				} else {
					bounds, e := m.GetArg(inside, 1)
					equationMust(t, e)
					lower, err = m.GetArg(bounds, 0)
					equationMust(t, err)
					upper, err = m.GetArg(bounds, 1)
					equationMust(t, err)
				}
				lowerKind, err := m.GetDeclKind(lower)
				equationMust(t, err)
				upperKind, err := m.GetDeclKind(upper)
				equationMust(t, err)
				if row.op == qfnraPurifyATan {
					if lowerKind != 5 || upperKind != 4 {
						t.Fatal("atan endpoints must be strict")
					}
				} else if lowerKind != 3 || upperKind != 2 {
					t.Fatal("asin/acos endpoints must be closed")
				}
				for j, bound := range []ast.ASTHandle{lower, upper} {
					lhs, e := m.GetArg(bound, 0)
					equationMust(t, e)
					if lhs != fresh {
						t.Fatal("inverse endpoint bounds wrong variable")
					}
					rhs, e := m.GetArg(bound, 1)
					equationMust(t, e)
					if row.op == qfnraPurifyACos {
						if j == 0 {
							text, _, ok, e := m.ArithmeticNumeralText(rhs)
							equationMust(t, e)
							if !ok || text != "0" {
								t.Fatal("acos lower endpoint", text)
							}
						} else {
							kind, e := m.GetDeclKind(rhs)
							equationMust(t, e)
							if kind != forwardTrigPi {
								t.Fatal("acos upper endpoint is not pi")
							}
						}
					} else {
						coefficient, e := m.GetArg(rhs, 0)
						equationMust(t, e)
						pi, e := m.GetArg(rhs, 1)
						equationMust(t, e)
						text, _, ok, e := m.ArithmeticNumeralText(coefficient)
						equationMust(t, e)
						wantCoefficient := "-1/2"
						if j == 1 {
							wantCoefficient = "1/2"
						}
						kind, e := m.GetDeclKind(pi)
						equationMust(t, e)
						if !ok || text != wantCoefficient || kind != forwardTrigPi {
							t.Fatal("inverse endpoint", text, kind)
						}
					}
				}
				if complete && row.op != qfnraPurifyATan {
					for j := uint32(2); j < 4; j++ {
						guard, e := goal.Form(j)
						equationMust(t, e)
						equality, e := m.GetArg(guard, 1)
						equationMust(t, e)
						undefined, e := m.GetArg(equality, 1)
						equationMust(t, e)
						kind, e := m.GetDeclKind(undefined)
						equationMust(t, e)
						if kind != row.undefined {
							t.Fatal("incorrect undefined inverse function", kind)
						}
					}
				}
				for j := uint32(0); j < size; j++ {
					proof, e := goal.Proof(j)
					equationMust(t, e)
					fact, e := m.GetProofFact(proof)
					equationMust(t, e)
					form, e := goal.Form(j)
					equationMust(t, e)
					if fact != form {
						t.Fatalf("proof %d conclusion", j)
					}
				}
				decl, err := m.GetDecl(fresh)
				equationMust(t, err)
				md, err := model.NewModel(m)
				equationMust(t, err)
				equationMust(t, md.CoreField.RegisterConstant(decl, zero))
				var out tactic.ModelRef
				equationMust(t, out.Set(&md.CoreField))
				t.Cleanup(func() { equationMust(t, out.Close()) })
				equationMust(t, result.ModelConverter.ApplyModel(&out, 0))
				filtered, err := out.Get()
				equationMust(t, err)
				present, err := filtered.HasInterpretation(decl)
				equationMust(t, err)
				if present {
					t.Fatal("inverse auxiliary escaped model")
				}
			})
		}
	}
}

func TestPurifyInverseTrigDisabledAndQuantified(t *testing.T) {
	runner, m := qfnraPurifier(t, false, true)
	x := fixDLConst(t, m, false, "inverse-disabled-x")
	term := fixDLApp(t, m, qfnraPurifyASin, x)
	zero := fixDLNumber(t, m, false, "0")
	formula, err := m.MkEq(term, zero)
	equationMust(t, err)
	fixDLPin(t, m, formula)
	params, err := runner.context.NewValue()
	equationMust(t, err)
	key := "elim_inverses"
	equationMust(t, runner.context.SetBoolName(&params, &key, false))
	equationMust(t, runner.UpdateParams(&params))
	equationMust(t, runner.context.Del(&params))
	goal, err := tactic.NewGoal(m, false, false, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(formula))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	got, err := goal.Form(0)
	equationMust(t, err)
	if got != formula {
		t.Fatal("disabled inverse elimination changed formula")
	}
	params, err = runner.context.NewValue()
	equationMust(t, err)
	equationMust(t, runner.UpdateParams(&params))
	equationMust(t, runner.context.Del(&params))
	sort, err := m.GetSort(x)
	equationMust(t, err)
	variable, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, variable)
	term = fixDLApp(t, m, qfnraPurifyASin, variable)
	body, err := m.MkEq(term, zero)
	equationMust(t, err)
	fixDLPin(t, m, body)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	equationMust(t, err)
	fixDLPin(t, m, q)
	quantGoal, err := tactic.NewGoal(m, false, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, quantGoal.Close()) })
	equationMust(t, quantGoal.Assert(q))
	var quantResult tactic.TacticResult
	t.Cleanup(func() { equationMust(t, quantResult.Close()) })
	equationMust(t, runner.Apply(quantGoal, &quantResult))
	size, err := quantGoal.Size()
	equationMust(t, err)
	if size != 1 || quantResult.ModelConverter == nil {
		t.Fatal("inverse constraints escaped quantifier", size)
	}
	assertEmptyPurifyFilter(t, quantResult.ModelConverter)
	outer, err := quantGoal.Form(0)
	equationMust(t, err)
	inner, err := m.GetQuantifierBody(outer)
	equationMust(t, err)
	count, err := m.GetNumDeclarations(inner)
	equationMust(t, err)
	if count != 1 {
		t.Fatal("inverse binder count", count)
	}
	conj, err := m.GetQuantifierBody(inner)
	equationMust(t, err)
	count, err = m.GetNumArgs(conj)
	equationMust(t, err)
	if count != 4 {
		t.Fatal("inverse quantified constraints", count)
	}
}
