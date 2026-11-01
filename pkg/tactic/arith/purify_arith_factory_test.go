package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

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
