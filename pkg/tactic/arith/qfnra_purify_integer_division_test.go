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
			// process_mod runs process_idiv with the modulus declaration, so
			// (div x y) is not cached and gets its own k3/k4 pair and
			// constraints (AUDIT.md FUZZ_C-F3).
			if size != 3+2*row.constraints {
				t.Fatalf("got %d formulas", size)
			}
			first, err := goal.Form(0)
			equationMust(t, err)
			second, err := goal.Form(1)
			equationMust(t, err)
			if first != second || first == formula {
				t.Fatal("modulus cache was not reused")
			}
			euclid := func(j uint32) (quotient, remainder ast.ASTHandle) {
				constraint, err := goal.Form(j)
				equationMust(t, err)
				equality, err := m.GetArg(constraint, 1)
				equationMust(t, err)
				sum, err := m.GetArg(equality, 1)
				equationMust(t, err)
				product, err := m.GetArg(sum, 0)
				equationMust(t, err)
				quotient, err = m.GetArg(product, 0)
				equationMust(t, err)
				remainder, err = m.GetArg(sum, 1)
				equationMust(t, err)
				return quotient, remainder
			}
			quotient, remainder := euclid(3)
			cachedRemainder, err := m.GetArg(first, 0)
			equationMust(t, err)
			if remainder != cachedRemainder {
				t.Fatal("modulus does not rewrite to its remainder k2")
			}
			divisionResult, err := goal.Form(2)
			equationMust(t, err)
			divisionQuotient, err := m.GetArg(divisionResult, 0)
			equationMust(t, err)
			k3, k4 := euclid(3 + row.constraints)
			if divisionQuotient != k3 || k3 == quotient || k4 == remainder {
				t.Fatal("division after modulus did not get a fresh quotient/remainder pair")
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
			// mk_apply_def(k, def, pr): (~ k1 (mod x y)).
			if definedTerm != quotient || definedValue != term {
				t.Fatal("quotient proof does not define k1 as the modulus term")
			}
			md, err := model.NewModel(m)
			equationMust(t, err)
			var decls []ast.ASTHandle
			for _, fresh := range []ast.ASTHandle{quotient, remainder, k3, k4} {
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
			// The filter holds the values of m_app2fresh: (div x y) -> k3 and
			// (mod x y) -> k4. k1 and k2 were overwritten and stay in the model.
			for j, decl := range decls {
				present, e := filtered.HasInterpretation(decl)
				equationMust(t, e)
				if present != (j < 2) {
					t.Fatal("model filter does not follow m_app2fresh", j, present)
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

// process_mod calls process_idiv with the modulus declaration: k1 is cached
// under (mod x y) and then overwritten by k2, which is created before
// mk_mod; the quotient (div x y) is never cached, and mk_def_proof builds
// (~ k def). AUDIT.md FUZZ_C-F3.
func TestPurifyModCachesRemainderOverQuotientDefinition(t *testing.T) {
	runner, m := qfnraPurifier(t, true, false)
	x := fixDLConst(t, m, true, "purify-mod-x")
	y := fixDLConst(t, m, true, "purify-mod-y")
	div := fixDLApp(t, m, qfnraPurifyIDiv, x, y)
	mod := fixDLApp(t, m, qfnraPurifyMod, x, y)
	goal := fixDLGoal(t, m, true, true, false)
	invocation, err := newQFNRAPurifyInvocation(runner, goal)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, invocation.close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.DelRef(&out)) })
	pr, err := m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.DelRef(&pr)) })
	decl, err := m.GetDecl(mod)
	equationMust(t, err)
	config := &qfnraPurifyConfig{invocation: invocation}
	_, err = config.ReduceApp(decl, ast.SliceArguments([]ast.ASTHandle{x, y}), out, pr)
	equationMust(t, err)
	if len(invocation.entries) < 2 {
		t.Fatal("entries", len(invocation.entries))
	}
	first, second := invocation.entries[0], invocation.entries[1]
	for _, e := range []*qfnraPurifyEntry{first, second} {
		term, err := m.RefGet(e.term)
		equationMust(t, err)
		if term != mod {
			t.Fatal("definition is not the modulus term", term, mod)
		}
	}
	if invocation.cache[mod] != second {
		t.Fatal("(mod x y) is not cached to the second fresh constant")
	}
	if invocation.cache[div] != nil {
		t.Fatal("(div x y) was cached")
	}
	if invocation.app2fresh.Len() != 1 {
		t.Fatal("m_app2fresh size", invocation.app2fresh.Len())
	}
	got, err := m.RefGet(out)
	equationMust(t, err)
	k2, err := m.RefGet(second.fresh)
	equationMust(t, err)
	if got != k2 {
		t.Fatal("(mod x y) does not rewrite to k2")
	}
	k1, err := m.RefGet(first.fresh)
	equationMust(t, err)
	proof, err := m.RefGet(first.proof)
	equationMust(t, err)
	fact, err := m.GetProofFact(proof)
	equationMust(t, err)
	lhs, err := m.GetArg(fact, 0)
	equationMust(t, err)
	if lhs != k1 {
		t.Fatal("apply-def fact is not (~ k def)")
	}
}
