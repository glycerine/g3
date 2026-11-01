package bmc_test

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestBMCQuantifiedLinearCompilationLevels(t *testing.T) {
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
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	decl := func(name string) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(d))
		return d
	}
	p, q, r := decl("qlinear-p"), decl("qlinear-q"), decl("qlinear-r")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	app := func(d, value ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, []ast.ASTHandle{value})
		must(e)
		return h
	}
	add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, e := rm.NewRule(head, tails, nil, "")
		must(e)
		must(set.Add(rule))
		must(rule.Close())
	}
	add(app(p, one))
	add(app(q, x), app(p, x))
	same, err := m.MkEq(x, y)
	must(err)
	positive, err := m.MkArithmeticGT(z, zero)
	must(err)
	add(app(r, x), app(p, y), same, positive)
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("rules did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	if invalid, e := compiler.CompileQuantifiedLinear(set, 0); e == nil || len(invalid) != 0 {
		t.Fatal("zero-width quantified index accepted", e)
	}
	formulas, err := compiler.CompileQuantifiedLinear(set, 4)
	must(err)
	defer func() {
		for i := range formulas {
			must(m.DelRef(&formulas[i]))
		}
	}()
	if len(formulas) != 6 {
		t.Fatal("one rule and one predicate implication per group", len(formulas))
	}
	solver, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(solver.Close()) }()
	sub, err := rewriter.NewVarSubstitution(m, true)
	must(err)
	defer func() { must(sub.Close()) }()
	var levels []ast.ASTHandle
	for _, value := range []int64{0, 1} {
		n, e := m.BitVectorNumeralRat(big.NewRat(value, 1), 4)
		must(e)
		ref, e := m.NewRef(n)
		must(e)
		defer func() { must(m.DelRef(&ref)) }()
		levels = append(levels, n)
	}
	for _, formula := range formulas {
		h, e := m.RefGet(formula)
		must(e)
		body, e := m.GetQuantifierBody(h)
		must(e)
		for _, level := range levels {
			ref, e := m.NewRef(ast.ASTHandle{})
			must(e)
			must(sub.Substitute(body, []ast.ASTHandle{level}, ref))
			inst, e := m.RefGet(ref)
			must(e)
			must(solver.Assert(inst))
			must(m.DelRef(&ref))
		}
	}
	indexSort, err := m.BitVectorSort(4)
	must(err)
	for _, tc := range []struct {
		name         string
		level, value ast.ASTHandle
		want         lbool.LBool
	}{
		{"qlinear-q", levels[0], one, lbool.False},
		{"qlinear-q", levels[1], zero, lbool.False},
		{"qlinear-q", levels[1], one, lbool.True},
		{"qlinear-r", levels[0], one, lbool.False},
		{"qlinear-r", levels[1], zero, lbool.False},
		{"qlinear-r", levels[1], one, lbool.True},
	} {
		qName, e := m.Symbols.Intern(tc.name + "#")
		must(e)
		qIndexed, e := m.MkFuncDecl(qName, []ast.ASTHandle{indexSort}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		argName, e := m.Symbols.Intern(tc.name + "#0")
		must(e)
		qArg, e := m.MkFuncDecl(argName, []ast.ASTHandle{indexSort}, integer, ast.ASTFuncDeclInfo{})
		must(e)
		must(solver.Push())
		query, e := m.MkApp(qIndexed, []ast.ASTHandle{tc.level})
		must(e)
		must(solver.Assert(query))
		argument, e := m.MkApp(qArg, []ast.ASTHandle{tc.level})
		must(e)
		eq, e := m.MkEq(argument, tc.value)
		must(e)
		must(solver.Assert(eq))
		status, e := solver.CheckSat(nil)
		must(e)
		if status != tc.want {
			t.Fatalf("quantified level result: got %v, want %v", status, tc.want)
		}
		must(solver.Pop(1))
	}
}

func TestBMCQuantifiedLinearCompilationAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 512 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			context, err := cmd_context.NewStandaloneContext()
			must(err)
			m, err := context.Manager()
			must(err)
			budget := m.Budget()
			t.Cleanup(func() {
				must(context.Close())
				if used := budget.Used(); used != 0 {
					t.Errorf("compilation left %d bytes", used)
				}
			})
			must(m.RegisterDeclarationPlugins())
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("qlinear-budget")
			must(err)
			decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			head, err := m.MkApp(decl, nil)
			must(err)
			rm, err := base.NewRuleManager(m)
			must(err)
			t.Cleanup(func() { must(rm.Close()) })
			must(rm.RegisterPredicate(decl))
			set, err := base.NewRuleSet(rm)
			must(err)
			t.Cleanup(func() { must(set.Close()) })
			rule, err := rm.NewRule(head, nil, nil, "fact")
			must(err)
			must(set.Add(rule))
			must(rule.Close())
			compiler, err := bmc.NewCompiler(m)
			must(err)
			t.Cleanup(func() { must(compiler.Close()) })
			budget.FailAfter(stop)
			formulas, err := compiler.CompileQuantifiedLinear(set, 4)
			budget.FailAfter(-1)
			for i := range formulas {
				must(m.DelRef(&formulas[i]))
			}
			if err == nil {
				success = true
				return
			}
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				must(failure.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal(err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("quantified compilation allocation frontier", stop)
		}
	}
	if !success || failures == 0 {
		t.Fatal("did not observe failure and success", failures, success)
	}
}
