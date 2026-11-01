package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astproofs "github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestTheoryRewriterQuantifierProofChecker(t *testing.T) {
	base, m := fixDLFixture(t, true)
	r, err := rewriter.NewTheoryRewriter(m, base.context, base.globals, nil)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	variable, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, variable)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: variable})
	equationMust(t, err)
	fixDLPin(t, m, inner)
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{sort, sort}, Names: []usymbol.Symbol{usymbol.Numerical(2), usymbol.Numerical(3)}, Body: inner})
	equationMust(t, err)
	fixDLPin(t, m, outer)
	out, err := m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.DelRef(&out)) })
	proofRef, err := m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.DelRef(&proofRef)) })
	equationMust(t, r.RewriteWithProof(outer, out, proofRef))
	proof, err := m.RefGet(proofRef)
	equationMust(t, err)
	checker, err := astproofs.NewChecker(m)
	equationMust(t, err)
	valid, side, err := checker.Check(proof)
	equationMust(t, err)
	equationMust(t, checker.ReleaseSideConditions(side))
	if !valid {
		t.Fatal("quantifier merge and unused variable elimination proof failed validation")
	}
}

func TestPurifyArithFactoryKeepsExistentialAfterParameterUpdate(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%v", proofs), func(t *testing.T) {
			base, m := fixDLFixture(t, proofs)
			p, err := base.context.NewValue()
			equationMust(t, err)
			t.Cleanup(func() { equationMust(t, base.context.Del(&p)) })
			key := "skolemize"
			equationMust(t, base.context.SetBoolName(&p, &key, true))
			strategy, err := MakePurifyArithTactic(m, base.context, base.globals, &p, base.services)
			equationMust(t, err)
			equationMust(t, strategy.IncRef())
			t.Cleanup(func() { equationMust(t, strategy.DecRef()) })
			equationMust(t, strategy.UpdateParams(&p))
			constant := fixDLConst(t, m, false, "quantifier-factory-sort")
			sort, err := m.GetSort(constant)
			equationMust(t, err)
			variable, err := m.MkVar(0, sort)
			equationMust(t, err)
			fixDLPin(t, m, variable)
			term := fixDLApp(t, m, qfnraPurifyToInt, variable)
			body := collectorEq(t, m, term, fixDLNumber(t, m, true, "0"))
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
			equationMust(t, err)
			fixDLPin(t, m, q)
			goal := fixDLGoal(t, m, proofs, false, false, q)
			var result tactic.TacticResult
			t.Cleanup(func() { equationMust(t, result.Close()) })
			equationMust(t, strategy.Apply(goal, &result))
			if len(result.Goals) != 1 {
				t.Fatal("unexpected goal count", len(result.Goals))
			}
			form, err := result.Goals[0].Form(0)
			equationMust(t, err)
			kind, err := m.GetKind(form)
			equationMust(t, err)
			if kind != ast.ASTQuantifier {
				t.Fatal("existential quantifier was skolemized")
			}
			forall, err := m.IsForall(form)
			equationMust(t, err)
			if forall {
				t.Fatal("existential quantifier changed polarity")
			}
			if proofs {
				proof, err := result.Goals[0].Proof(0)
				equationMust(t, err)
				fact, err := m.GetProofFact(proof)
				equationMust(t, err)
				if fact != form {
					actual, _ := fmExpressionText(m, base.services, fact, 0)
					want, _ := fmExpressionText(m, base.services, form, 0)
					t.Fatalf("quantified proof has wrong conclusion: %s; want %s", actual, want)
				}
				checker, err := astproofs.NewChecker(m)
				equationMust(t, err)
				valid, side, err := checker.Check(proof)
				equationMust(t, err)
				equationMust(t, checker.ReleaseSideConditions(side))
				if !valid {
					t.Fatal("quantified proof failed validation")
				}
				queue := []ast.ASTHandle{proof}
				foundPull := false
				for len(queue) > 0 {
					node := queue[0]
					queue = queue[1:]
					op, e := m.GetDeclKind(node)
					equationMust(t, e)
					if op == ast.ASTPrPullQuant {
						valid, side, e := checker.Check(node)
						equationMust(t, e)
						equationMust(t, checker.ReleaseSideConditions(side))
						if !valid {
							t.Fatal("quantifier merge proof failed validation")
						}
						foundPull = true
					}
					n, e := m.GetProofNumParents(node)
					equationMust(t, e)
					for j := uint32(0); j < n; j++ {
						parent, e := m.GetProofParent(node, j)
						equationMust(t, e)
						queue = append(queue, parent)
					}
				}
				if !foundPull {
					t.Fatal("quantifier merge proof is missing")
				}
			}
		})
	}
}

func TestPurifyArithFactoryApplyAllocationFrontiers(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		success := false
		for stop := int64(0); stop < 1024 && !success; stop++ {
			t.Run(fmt.Sprintf("proofs=%v/%d", proofs, stop), func(t *testing.T) {
				base, m := fixDLFixture(t, proofs)
				formula := generalPurifierFailureFormula(t, m, "integer")
				goal := fixDLGoal(t, m, proofs, true, false, formula)
				strategy, err := MakePurifyArithTactic(m, base.context, base.globals, nil, base.services)
				equationMust(t, err)
				equationMust(t, strategy.IncRef())
				t.Cleanup(func() { equationMust(t, strategy.DecRef()) })
				var result tactic.TacticResult
				t.Cleanup(func() { equationMust(t, result.Close()) })
				m.Budget().FailAfter(stop)
				err = strategy.Apply(goal, &result)
				m.Budget().FailAfter(-1)
				if err == nil {
					success = true
				} else {
					var failure memory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == memory.FailureMemout && m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(err)
					}
					goal = fixDLGoal(t, m, proofs, true, false, formula)
					var retry tactic.TacticResult
					t.Cleanup(func() { equationMust(t, retry.Close()) })
					equationMust(t, strategy.Cleanup())
					equationMust(t, strategy.Apply(goal, &retry))
					if len(retry.Goals) != 1 || retry.ModelConverter == nil {
						t.Fatal("retry lost goal or model conversion")
					}
				}
			})
		}
		if !success {
			t.Fatalf("no successful application frontier for proofs=%v", proofs)
		}
	}
}

func TestPurifyArithFactoryConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			base, m := fixDLFixture(t, false)
			m.Budget().FailAfter(stop)
			value, err := MakePurifyArithTactic(m, base.context, base.globals, nil, base.services)
			m.Budget().FailAfter(-1)
			if value != nil {
				equationMust(t, value.IncRef())
				equationMust(t, value.DecRef())
			}
			if err == nil {
				success = true
				return
			}
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
			retry, err := MakePurifyArithTactic(m, base.context, base.globals, nil, base.services)
			equationMust(t, err)
			equationMust(t, retry.IncRef())
			equationMust(t, retry.DecRef())
		})
	}
	if !success {
		t.Fatal("no successful constructor allocation frontier")
	}
}

func TestPurifyArithFactoryEliminatesRemainderAndModulo(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprintf("proofs=%v", proofs), func(t *testing.T) {
			base, m := fixDLFixture(t, proofs)
			params, err := base.context.NewValue()
			equationMust(t, err)
			defer func() { equationMust(t, base.context.Del(&params)) }()
			for _, name := range []string{"elim_rem", "complete"} {
				equationMust(t, base.context.SetBoolName(&params, &name, false))
			}
			x := fixDLConst(t, m, true, "purify-factory-x")
			three := fixDLNumber(t, m, true, "3")
			one := fixDLNumber(t, m, true, "1")
			rem := fixDLApp(t, m, 12, x, three)
			formula := collectorEq(t, m, rem, one)
			goal := fixDLGoal(t, m, proofs, true, false, formula)
			strategy, err := MakePurifyArithTactic(m, base.context, base.globals, &params, base.services)
			equationMust(t, err)
			equationMust(t, strategy.IncRef())
			defer func() { equationMust(t, strategy.DecRef()) }()
			var result tactic.TacticResult
			defer func() { equationMust(t, result.Close()) }()
			equationMust(t, strategy.Apply(goal, &result))
			if len(result.Goals) != 1 || result.ModelConverter == nil {
				t.Fatal("missing purified goal or model converter")
			}
			out := result.Goals[0]
			size, err := out.Size()
			equationMust(t, err)
			family, err := m.ArithmeticFamilyID()
			equationMust(t, err)
			seen := make(map[ast.ASTHandle]bool)
			var visit func(ast.ASTHandle)
			visit = func(h ast.ASTHandle) {
				if seen[h] {
					return
				}
				seen[h] = true
				kind, e := m.GetKind(h)
				equationMust(t, e)
				if kind == ast.ASTQuantifier {
					child, e := m.GetQuantifierBody(h)
					equationMust(t, e)
					visit(child)
					return
				}
				if kind != ast.ASTApp {
					return
				}
				fid, e := m.GetFamilyID(h)
				equationMust(t, e)
				op, e := m.GetDeclKind(h)
				equationMust(t, e)
				if fid == family && (op == 11 || op == 12 || op == 13) {
					t.Fatalf("arithmetic operator %d survived purification", op)
				}
				n, e := m.GetNumArgs(h)
				equationMust(t, e)
				for i := uint32(0); i < n; i++ {
					child, e := m.GetArg(h, i)
					equationMust(t, e)
					visit(child)
				}
			}
			for i := uint32(0); i < size; i++ {
				form, e := out.Form(i)
				equationMust(t, e)
				visit(form)
				if proofs {
					proof, e := out.Proof(i)
					equationMust(t, e)
					fact, e := m.GetProofFact(proof)
					equationMust(t, e)
					if fact != form {
						t.Fatal("proof conclusion", i)
					}
				}
			}
		})
	}
}
