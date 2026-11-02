package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source: dl_bmc_engine.cpp bmc::is_linear, nonlinear::compile and
// nonlinear::level_replacer. Quantified interpreted tails select the nonlinear
// compiler, and registered predicates below the quantifier refer to the
// preceding unfolding level.
func TestBMCNonlinearCompilationExecutesQuantifiedPredicateTail(t *testing.T) {
	for _, tc := range []struct {
		name       string
		falseFact  bool
		wantLevel1 lbool.LBool
	}{
		{name: "universal-covered", falseFact: true, wantLevel1: lbool.True},
		{name: "universal-missing", wantLevel1: lbool.False},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			context, err := cmd_context.NewStandaloneContext()
			must(err)
			defer func() { must(context.Close()) }()
			m, err := context.Manager()
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			rm, err := base.NewRuleManager(m)
			must(err)
			defer func() { must(rm.Close()) }()
			set, err := base.NewRuleSet(rm)
			must(err)
			defer func() { must(set.Close()) }()
			declare := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
				name, cause := m.Symbols.Intern(text)
				must(cause)
				decl, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				must(rm.RegisterPredicate(decl))
				return decl
			}
			p := declare("quantified-tail-p", []ast.ASTHandle{boolean})
			goal := declare("quantified-tail-goal", nil)
			app := func(decl ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
				value, cause := m.MkApp(decl, args)
				must(cause)
				return value
			}
			add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
				rule, cause := rm.NewRule(head, tails, nil, "")
				must(cause)
				must(set.Add(rule))
				must(rule.Close())
			}
			yes, err := m.MkTrue()
			must(err)
			no, err := m.MkFalse()
			must(err)
			add(app(p, yes))
			if tc.falseFact {
				add(app(p, no))
			}
			x, err := m.MkVar(0, boolean)
			must(err)
			body := app(p, x)
			universal, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true,
				Sorts:  []ast.ASTHandle{boolean},
				Names:  []symbol.Symbol{symbol.Numerical(0)},
				Body:   body,
			})
			must(err)
			quantifiedTail, err := m.MkAnd(universal, yes)
			must(err)
			add(app(goal), quantifiedTail)
			valid, err := set.Finalize()
			must(err)
			if !valid {
				t.Fatal("quantified-tail fixture did not finalize")
			}
			linear, err := bmc.IsLinear(set)
			must(err)
			if linear {
				t.Fatal("quantified rule selected the linear compiler")
			}
			compiler, err := bmc.NewCompiler(m)
			must(err)
			defer func() { must(compiler.Close()) }()
			solver, err := context.NewSimpleSolver()
			must(err)
			defer func() { must(solver.Close()) }()
			for level := uint32(0); level <= 1; level++ {
				formulas, cause := compiler.Compile(set, level)
				must(cause)
				for i := range formulas {
					formula, cause := m.RefGet(formulas[i])
					must(cause)
					must(solver.Assert(formula))
					must(m.DelRef(&formulas[i]))
				}
				queryRef, cause := compiler.CompileQuery(goal, level)
				must(cause)
				query, cause := m.RefGet(queryRef)
				must(cause)
				status, cause := solver.CheckSat([]ast.ASTHandle{query})
				must(cause)
				must(m.DelRef(&queryRef))
				want := lbool.False
				if level == 1 {
					want = tc.wantLevel1
				}
				if status != want {
					t.Fatalf("quantified tail at level %d: got %v, want %v", level, status, want)
				}
			}
		})
	}
}
