package opt_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestArithmeticSolverQuantifierCandidateRemainsUnknown(t *testing.T) {
	m, evaluator := sortMaxFixture(t)
	for _, name := range []string{"mbqi", "preprocess", "auto_config"} {
		if err := evaluator.Context.SetBoolName(evaluator.Params, &name, false); err != nil {
			t.Fatal(err)
		}
	}
	key := "relevancy"
	if err := evaluator.Context.SetUIntName(evaluator.Params, &key, 0); err != nil {
		t.Fatal(err)
	}
	solver, err := opt.NewArithmeticSolver(m, evaluator.Params, "AUFLIA", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := solver.Close(); err != nil {
			t.Error(err)
		}
	}()
	name, err := m.Symbols.Intern("opt-unchecked-sort")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.MkUninterpretedSort(name, nil)
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err = m.Symbols.Intern("opt-unchecked-predicate")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	variable, err := m.MkVar(0, sort)
	if err != nil {
		t.Fatal(err)
	}
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	pattern, err := m.MkPattern([]ast.ASTHandle{body})
	if err != nil {
		t.Fatal(err)
	}
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(quantifier); err != nil {
		t.Fatal(err)
	}
	p := sortMaxBool(t, m, "opt-forced-soft")
	if err = solver.Assert(p); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p}, []*big.Rat{big.NewRat(1, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := result.Close(); err != nil {
			t.Error(err)
		}
	}()
	if result.Status != lbool.Undef || result.Optimal || result.Model == nil || result.ReasonUnknown != "(incomplete quantifiers)" {
		t.Fatal("unchecked optimization result", result.Status, result.Optimal, result.Model, result.ReasonUnknown)
	}
	if result.Values[0].SoftCost.Sign() != 0 {
		t.Fatal("candidate did not satisfy forced soft term", &result.Values[0].SoftCost)
	}
}

// AUDIT.md cmd_context_01-F4: execute_min_max throws when an l_true
// objective is unbounded and the hard constraints contain quantifiers.
func TestArithmeticSolverUnboundedQuantifiedObjectiveThrows(t *testing.T) {
	for _, quantified := range []bool{false, true} {
		m, config := sortMaxFixture(t)
		solver, err := opt.NewArithmeticSolver(m, config.Params, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
		if err != nil {
			t.Fatal(err)
		}
		family, err := m.RegisterArithmeticPlugin()
		if err != nil {
			t.Fatal(err)
		}
		integer, err := m.MkFamilySort(family, 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		name, err := m.Symbols.Intern("unbounded-quantified-x")
		if err != nil {
			t.Fatal(err)
		}
		x, err := m.MkConst(name, integer)
		if err != nil {
			t.Fatal(err)
		}
		zero, err := m.ArithmeticIntegerNumeralText("0")
		if err != nil {
			t.Fatal(err)
		}
		lower, err := m.MkArithmeticLE(zero, x)
		if err != nil {
			t.Fatal(err)
		}
		// The caller owns the asserted formula and the objective term.
		lowerRef, err := m.NewRef(lower)
		if err != nil {
			t.Fatal(err)
		}
		xRef, err := m.NewRef(x)
		if err != nil {
			t.Fatal(err)
		}
		if err = solver.Assert(lower); err != nil {
			t.Fatal(err)
		}
		if err = solver.AddObjective(x, true); err != nil {
			t.Fatal(err)
		}
		solver.SetContainsQuantifiers(quantified)
		result, err := solver.Check()
		if quantified {
			var exception z3exception.Z3Exception
			if !errors.As(err, &exception) || exception.Message() != "unbounded objectives on quantified constraints is not supported" {
				t.Fatal("quantified unbounded objective", err)
			}
			if result != nil {
				t.Fatal("result with an exception")
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			value := result.Values[0].Arithmetic
			if result.Status != lbool.True || !(value.Unbounded || value.Finite.Infinity.Sign() > 0) {
				t.Fatal("unquantified unbounded objective", result.Status, value)
			}
			if err = result.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if err = solver.Close(); err != nil {
			t.Fatal(err)
		}
		if err = errors.Join(m.DelRef(&xRef), m.DelRef(&lowerRef)); err != nil {
			t.Fatal(err)
		}
	}
}
