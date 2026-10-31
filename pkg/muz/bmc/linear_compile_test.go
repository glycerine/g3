package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCLinearCompilationSharesRuleVariables(t *testing.T) {
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
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(rm.RegisterPredicate(d))
		return d
	}
	a, b := decl("linear-a"), decl("linear-b")
	app := func(d, value ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, []ast.ASTHandle{value})
		must(err)
		return h
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	add := func(head ast.ASTHandle, tails []ast.ASTHandle) {
		r, err := rm.NewRule(head, tails, nil, "")
		must(err)
		must(set.Add(r))
		must(r.Close())
	}
	add(app(a, zero), nil)
	add(app(a, one), nil)
	x, err := m.MkVar(0, integer)
	must(err)
	positive, err := m.MkArithmeticGT(x, zero)
	must(err)
	add(app(b, x), []ast.ASTHandle{app(a, x), positive})
	ok, err := set.Finalize()
	must(err)
	if !ok {
		t.Fatal("rule set did not finalize")
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	smt, err := context.NewSimpleSolver()
	must(err)
	defer func() { must(smt.Close()) }()
	for level := uint32(0); level <= 1; level++ {
		formulas, err := compiler.CompileLinear(set, level)
		must(err)
		for i := range formulas {
			f, err := m.RefGet(formulas[i])
			must(err)
			must(smt.Assert(f))
			must(m.DelRef(&formulas[i]))
		}
	}
	zeroLevelRef, err := compiler.LinearQuery(b, 0)
	must(err)
	zeroLevel, err := m.RefGet(zeroLevelRef)
	must(err)
	status, err := smt.CheckSat([]ast.ASTHandle{zeroLevel})
	must(err)
	must(m.DelRef(&zeroLevelRef))
	if status != lbool.False {
		t.Fatal("recursive predicate reached at level zero", status)
	}
	queryRef, err := compiler.LinearQuery(b, 1)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	query, err := m.RefGet(queryRef)
	must(err)
	argName, err := m.Symbols.Intern("linear-b#1_0")
	must(err)
	arg, err := m.MkConst(argName, integer)
	must(err)
	argRef, err := m.NewRef(arg)
	must(err)
	defer func() { must(m.DelRef(&argRef)) }()
	for _, tc := range []struct {
		value ast.ASTHandle
		want  lbool.LBool
	}{{zero, lbool.False}, {one, lbool.True}} {
		must(smt.Push())
		eq, err := m.MkEq(arg, tc.value)
		must(err)
		must(smt.Assert(eq))
		status, err := smt.CheckSat([]ast.ASTHandle{query})
		must(err)
		must(smt.Pop(1))
		if status != tc.want {
			t.Fatal("linear query", tc.value, status, tc.want)
		}
	}
	for _, tc := range []struct {
		depth uint32
		want  lbool.LBool
	}{{1, lbool.Undef}, {2, lbool.True}} {
		raw, err := context.NewSimpleSolver()
		must(err)
		backend, ok := raw.(*smtcore.SMTSolver)
		if !ok {
			t.Fatal("simple solver is not native SMT")
		}
		status, level, err := compiler.SearchLinear(set, b, backend, tc.depth)
		must(err)
		if status != tc.want || level != 1 {
			t.Fatal("bounded linear search", tc.depth, status, level)
		}
		if status == lbool.True {
			md, err := backend.AcquireModel()
			must(err)
			if md == nil {
				t.Fatal("missing linear model")
			}
			proofRef, err := compiler.LinearProof(set, b, level, md)
			must(err)
			proof, err := m.RefGet(proofRef)
			must(err)
			fact, err := m.GetProofFact(proof)
			must(err)
			positiveOne, err := m.MkArithmeticGT(one, zero)
			must(err)
			wantFact, err := m.MkImplies(positiveOne, app(b, one))
			must(err)
			if fact != wantFact {
				t.Fatal("linear BMC proof conclusion", fact, wantFact)
			}
			must(m.DelRef(&proofRef))
			trace, err := compiler.LinearTrace(set, b, level, md)
			must(err)
			if len(trace) != 2 {
				t.Fatal("linear trace length", len(trace))
			}
			bindings, err := compiler.LinearModelBindings(trace[0], 0, level, md)
			must(err)
			if len(bindings) != 1 {
				t.Fatal("linear model binding count", len(bindings))
			}
			bound, err := m.RefGet(bindings[0])
			must(err)
			if bound != one {
				t.Fatal("linear model binding", bound, one)
			}
			must(m.DelRef(&bindings[0]))
			empty, err := model.NewModelCore(m)
			must(err)
			bindings, err = compiler.LinearModelBindings(trace[0], 0, level, empty)
			must(err)
			bound, err = m.RefGet(bindings[0])
			must(err)
			if bound != x {
				t.Fatal("uninterpreted linear binding", bound, x)
			}
			must(m.DelRef(&bindings[0]))
			must(empty.Close())
			for i := len(trace) - 1; i >= 0; i-- {
				must(trace[i].Close())
			}
			must(md.DecRef())
		}
		must(raw.Close())
	}
	// At level zero a ground fact uses its asserted proof directly.
	raw, err := context.NewSimpleSolver()
	must(err)
	backend, ok := raw.(*smtcore.SMTSolver)
	if !ok {
		t.Fatal("simple solver is not native SMT")
	}
	status, level, err := compiler.SearchLinear(set, a, backend, 1)
	must(err)
	if status != lbool.True || level != 0 {
		t.Fatal("fact search", status, level)
	}
	md, err := backend.AcquireModel()
	must(err)
	proofRef, err := compiler.LinearProof(set, a, level, md)
	must(err)
	proof, err := m.RefGet(proofRef)
	must(err)
	fact, err := m.GetProofFact(proof)
	must(err)
	if fact != app(a, zero) && fact != app(a, one) {
		t.Fatal("ground proof conclusion", fact)
	}
	must(m.DelRef(&proofRef))
	must(md.DecRef())
	must(raw.Close())
}
