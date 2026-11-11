package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtAssertedDistributionFixture(t *testing.T, proofs bool) (*ast.ASTManager, *SMTSolver, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m, solver := smtAssertedNNFFixture(t, proofs)
	smtDisableAutoConfig(t, solver)
	u, err := m.MkUninterpretedSort(astTestSymbol(t, m, "distribute-U"), nil)
	astTestMust(t, err)
	x, err := m.MkVar(1, u)
	astTestMust(t, err)
	y, err := m.MkVar(0, u)
	astTestMust(t, err)
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "distribute-p", u, b)
	r := smtQuickPredicate(t, m, "distribute-r", u, b)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	astTestMust(t, err)
	ry, err := m.MkApp(r, []ast.ASTHandle{y})
	astTestMust(t, err)
	body, err := m.MkAnd(px, ry)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{u, u}, Names: []symbol.Symbol{symbol.Numerical(1), symbol.Numerical(2)}, Body: body})
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "distribute-a"), u)
	astTestMust(t, err)
	pa, err := m.MkApp(p, []ast.ASTHandle{a})
	astTestMust(t, err)
	for _, value := range []ast.ASTHandle{q, pa} {
		ref, err := m.NewRef(value)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
	}
	return m, solver, q, pa
}

func TestSMTAssertedDistributionGuardsBindersAndProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, mode := range []string{"enabled", "disabled", "no-preprocessing"} {
			t.Run(fmt.Sprint(proofs, "/", mode), func(t *testing.T) {
				m, solver, q, _ := smtAssertedDistributionFixture(t, proofs)
				k := solver.kernel
				astTestMust(t, solver.Assert(q))
				// Like C++, this field has no public parameter accessor.
				k.preprocessState.DistributeForall = mode != "disabled"
				if mode == "no-preprocessing" {
					k.preprocess = false
				}
				astTestMust(t, k.setAssertedEliminateAnd(true))
				astTestMust(t, k.reducePendingAssertions())
				astTestMust(t, k.distributePendingUniversals())
				wantRows, wantBinders := 1, uint32(2)
				if mode == "enabled" {
					wantRows, wantBinders = 2, 1
				}
				if len(k.assertions) != wantRows {
					t.Fatal("distributed assertion count", len(k.assertions), wantRows)
				}
				for _, row := range k.assertions {
					n, err := m.GetNumDeclarations(row.Formula)
					astTestMust(t, err)
					if n != wantBinders {
						t.Fatal("unused binders were not removed", n, wantBinders)
					}
					if mode == "enabled" {
						body, err := m.GetQuantifierBody(row.Formula)
						astTestMust(t, err)
						v, err := m.GetArg(body, 0)
						astTestMust(t, err)
						index, err := m.GetVarIndex(v)
						astTestMust(t, err)
						if index != 0 {
							t.Fatal("remaining binder was not remapped", index)
						}
					}
					if proofs {
						fact, err := m.GetProofFact(row.Proof)
						astTestMust(t, err)
						if fact != row.Formula {
							t.Fatal("distributed assertion lost its proof")
						}
					}
				}
			})
		}
	}
}

func TestSMTAssertedDistributionScopedConflict(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m, solver, q, pa := smtAssertedDistributionFixture(t, proofs)
			k := solver.kernel
			k.preprocessState.DistributeForall = true
			for cycle := 0; cycle < 2; cycle++ {
				astTestMust(t, solver.Push())
				astTestMust(t, solver.Assert(q))
				negative, err := m.MkNot(pa)
				astTestMust(t, err)
				astTestMust(t, solver.Assert(negative))
				status, err := solver.CheckSat(nil)
				astTestMust(t, err)
				if status != lbool.False {
					t.Fatal("distributed universal did not refute its counterexample", status)
				}
				universals := 0
				for _, row := range k.assertions {
					kind, err := m.GetKind(row.Formula)
					astTestMust(t, err)
					if kind == ast.ASTQuantifier {
						universals++
					}
				}
				if universals != 2 {
					t.Fatal("solver did not distribute the quantified conjunction", universals)
				}
				if proofs {
					proof, err := solver.Proof()
					astTestMust(t, err)
					fact, err := m.GetProofFact(proof)
					astTestMust(t, err)
					if !m.IsFalse(fact) {
						t.Fatal("distribution conflict lost its proof")
					}
				}
				astTestMust(t, solver.Pop(1))
				if len(k.assertions) != 0 || !k.internalizer.quantifiers.Empty() {
					t.Fatal("distributed quantifiers survived scope rollback")
				}
			}
		})
	}
}

func TestSMTAssertedRewriterSwitchesConjunctionPhases(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m, solver := smtAssertedNNFFixture(t, proofs)
			b, err := m.MkBoolSort()
			astTestMust(t, err)
			a, err := m.MkConst(astTestSymbol(t, m, "phase-a"), b)
			astTestMust(t, err)
			c, err := m.MkConst(astTestSymbol(t, m, "phase-c"), b)
			astTestMust(t, err)
			conjunction, err := m.MkAnd(a, c)
			astTestMust(t, err)
			ref, err := m.NewRef(conjunction)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
			for i := 0; i < 2; i++ {
				f := smtQuickPredicate(t, m, fmt.Sprint("phase-f-", i), b, b)
				root, err := m.MkApp(f, []ast.ASTHandle{conjunction})
				astTestMust(t, err)
				astTestMust(t, solver.Assert(root))
				k := solver.kernel
				row := k.assertions[len(k.assertions)-1]
				arg, err := m.GetArg(row.Formula, 0)
				astTestMust(t, err)
				op, err := m.GetDeclKind(arg)
				astTestMust(t, err)
				if op != ast.ASTOpAnd || k.assertedElimAnd {
					t.Fatal("assertion phase eliminated its conjunction")
				}
				astTestMust(t, k.internalizePending())
				row = k.assertions[len(k.assertions)-1]
				arg, err = m.GetArg(row.Formula, 0)
				astTestMust(t, err)
				op, err = m.GetDeclKind(arg)
				astTestMust(t, err)
				if op != ast.ASTOpNot || !k.assertedElimAnd {
					t.Fatal("post-NNF reduction retained a conjunction", op)
				}
				if proofs {
					fact, err := m.GetProofFact(row.Proof)
					astTestMust(t, err)
					if fact != row.Formula {
						t.Fatal("phase transition lost its rewrite proof")
					}
				}
			}
		})
	}
}

func TestSMTAssertedRewriterPreservesParametersAcrossPhaseUpdates(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, false)
	k := solver.kernel
	local, err := k.paramContext.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, k.paramContext.Del(&local)) }()
	for _, setting := range []struct {
		name  string
		value bool
	}{{"flat", false}, {"elim_and", true}, {"som", false}} {
		name := setting.name
		astTestMust(t, k.paramContext.SetBoolName(&local, &name, setting.value))
	}
	astTestMust(t, solver.UpdateParams(&local))
	partial, err := k.paramContext.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, k.paramContext.Del(&partial)) }()
	name := "pull_nested_quantifiers"
	astTestMust(t, k.paramContext.SetBoolName(&partial, &name, true))
	astTestMust(t, k.setAssertedEliminateAnd(true))
	astTestMust(t, solver.UpdateParams(&partial))
	for _, setting := range []struct {
		name string
		want bool
	}{{"flat", false}, {"elim_and", true}, {"som", false}, {"arith_ineq_lhs", true}, {"rewrite_patterns", true}} {
		// smt_solver::updt_params passes the solver's accumulated parameters
		// (local's som=false included) to context::updt_params, which appends
		// them to m_params and does not run set_eliminate_and; the next phase
		// flip restores som.
		name := setting.name
		got, err := k.paramContext.GetBoolName(&k.assertedParams, &name, !setting.want)
		astTestMust(t, err)
		if got != setting.want {
			t.Fatal("asserted rewriter parameter was lost or overridden incorrectly", name, got)
		}
	}
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	values := make([]ast.ASTHandle, 3)
	for i := range values {
		values[i], err = m.MkConst(astTestSymbol(t, m, fmt.Sprint("phase-flat-", i)), b)
		astTestMust(t, err)
	}
	inner, err := m.MkOr(values[1], values[2])
	astTestMust(t, err)
	outer, err := m.MkOr(values[0], inner)
	astTestMust(t, err)
	f := smtQuickPredicate(t, m, "phase-flat-f", b, b)
	root, err := m.MkApp(f, []ast.ASTHandle{outer})
	astTestMust(t, err)
	astTestMust(t, solver.Assert(root))
	argument, err := m.GetArg(k.assertions[0].Formula, 0)
	astTestMust(t, err)
	n, err := m.GetNumArgs(argument)
	astTestMust(t, err)
	if n != 2 {
		t.Fatal("retained flat=false parameter did not reach the assertion rewriter", n)
	}
}

func TestSMTAssertedInputSurvivesPhaseCacheFlush(t *testing.T) {
	m, solver := smtAssertedNNFFixture(t, false)
	k := solver.kernel
	local, err := k.paramContext.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, k.paramContext.Del(&local)) }()
	name := "cache_all"
	astTestMust(t, k.paramContext.SetBoolName(&local, &name, true))
	astTestMust(t, solver.UpdateParams(&local))
	astTestMust(t, k.setAssertedEliminateAnd(true))
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "phase-cache-a"), b)
	astTestMust(t, err)
	f := smtQuickPredicate(t, m, "phase-cache-f", b, b)
	root, err := m.MkApp(f, []ast.ASTHandle{a})
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	parent, err := m.MkIte(truth, root, falsity)
	astTestMust(t, err)
	parentRef, err := m.NewRef(parent)
	astTestMust(t, err)
	value, proof, err := k.preprocessAssertion(parent, ast.ASTHandle{})
	astTestMust(t, err)
	if value != root {
		t.Fatal("cache fixture unexpectedly rewrote the formula")
	}
	astTestMust(t, m.DecRef(proof))
	astTestMust(t, m.DecRef(value))
	astTestMust(t, m.DelRef(&parentRef))
	n, err := k.preprocessor.CacheSize()
	astTestMust(t, err)
	if n == 0 {
		t.Fatal("fixture did not retain the input in its rewriter cache")
	}
	// Assert must pin this borrowed input before switching back to assertion
	// mode and flushing the cache that currently owns it.
	astTestMust(t, solver.Assert(root))
	if len(k.assertions) != 1 || k.assertions[0].Formula != root {
		t.Fatal("cache flush lost the asserted formula")
	}
}
