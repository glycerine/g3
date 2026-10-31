package opt_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestArithmeticSolverSharedObjectiveRepairsStrictSupremum(t *testing.T) {
	m, config := sortMaxFixture(t)
	key := "auto_config"
	if err := config.Context.SetBoolName(config.Params, &key, false); err != nil {
		t.Fatal(err)
	}
	key = "arith.solver"
	if err := config.Context.SetUIntName(config.Params, &key, 2); err != nil {
		t.Fatal(err)
	}
	solver, err := opt.NewArithmeticSolver(m, config.Params, "QF_UFLRA", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := solver.Close(); err != nil {
			t.Error(err)
		}
	}()
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("shared-repair-x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := m.NewRef(x)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&ref); err != nil {
			t.Error(err)
		}
	}()
	five, err := m.ArithmeticRealNumeralText("5")
	if err != nil {
		t.Fatal(err)
	}
	upper, err := m.MkArithmeticLE(x, five)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(upper); err != nil {
		t.Fatal(err)
	}
	name, err = m.Symbols.Intern("shared-repair-f")
	if err != nil {
		t.Fatal(err)
	}
	function, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	fx, err := m.MkApp(function, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	f5, err := m.MkApp(function, []ast.ASTHandle{five})
	if err != nil {
		t.Fatal(err)
	}
	equality, err := m.MkEq(fx, f5)
	if err != nil {
		t.Fatal(err)
	}
	different, err := m.MkNot(equality)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(different); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddObjective(x, true); err != nil {
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
	if result.Status != lbool.True || !result.Optimal || result.Model == nil {
		t.Fatal("shared repair result", result.Status, result.Optimal, result.Model)
	}
	value := result.Values[0].Arithmetic
	if value.Unbounded || value.Finite.Main.Cmp(big.NewRat(5, 1)) != 0 || value.Finite.Epsilon.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("local unattainable maximum was not decremented", value)
	}
	decl, err := m.GetDecl(x)
	if err != nil {
		t.Fatal(err)
	}
	modelValue, err := result.Model.ConstantInterpretation(decl)
	if err != nil {
		t.Fatal(err)
	}
	text, _, matched, err := m.ArithmeticNumeralText(modelValue)
	if err != nil {
		t.Fatal(err)
	}
	number, ok := new(big.Rat).SetString(text)
	if !matched || !ok || number.Cmp(big.NewRat(5, 1)) >= 0 {
		t.Fatal("repair published an invalid shared assignment", text)
	}
}
