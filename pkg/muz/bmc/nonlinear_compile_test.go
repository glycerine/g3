package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBMCNonlinearCompilationUnfoldsRulesByLevel(t *testing.T) {
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
	r, q := decl("compile-r"), decl("compile-q")
	app := func(d, value ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, []ast.ASTHandle{value})
		must(err)
		return h
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	fact, err := rm.NewRule(app(r, zero), nil, nil, "fact")
	must(err)
	must(set.Add(fact))
	must(fact.Close())
	x, err := m.MkVar(0, integer)
	must(err)
	step, err := rm.NewRule(app(q, x), []ast.ASTHandle{app(r, x)}, nil, "step")
	must(err)
	must(set.Add(step))
	must(step.Close())
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
		formulas, err := compiler.Compile(set, level)
		must(err)
		if len(formulas) != 4 {
			t.Fatal("compiled formulas", level, len(formulas))
		}
		for i := range formulas {
			value, err := m.RefGet(formulas[i])
			must(err)
			must(smt.Assert(value))
			must(m.DelRef(&formulas[i]))
		}
		queryRef, err := compiler.CompileQuery(q, level)
		must(err)
		query, err := m.RefGet(queryRef)
		must(err)
		guard, err := m.MkFreshConst("q", boolean)
		must(err)
		guardRef, err := m.NewRef(guard)
		must(err)
		implication, err := m.MkImplies(guard, query)
		must(err)
		must(smt.Assert(implication))
		status, err := smt.CheckSat([]ast.ASTHandle{guard})
		must(err)
		must(m.DelRef(&guardRef))
		must(m.DelRef(&queryRef))
		want := lbool.False
		if level == 1 {
			want = lbool.True
		}
		if status != want {
			t.Fatal("level query", level, status, want)
		}
	}
}

func TestBMCNonlinearCompilationSharesExistentialAcrossBranches(t *testing.T) {
	for _, tc := range []struct {
		name, second string
		want         lbool.LBool
	}{
		{"matching", "1", lbool.True}, {"different", "0", lbool.False},
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
			decl := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
				sym, err := m.Symbols.Intern(name)
				must(err)
				d, err := m.MkFuncDecl(sym, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(rm.RegisterPredicate(d))
				return d
			}
			a, b, goal := decl("branch-a", []ast.ASTHandle{integer}), decl("branch-b", []ast.ASTHandle{integer}), decl("branch-goal", nil)
			app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
				h, err := m.MkApp(d, args)
				must(err)
				return h
			}
			one, err := m.ArithmeticIntegerNumeralText("1")
			must(err)
			other, err := m.ArithmeticIntegerNumeralText(tc.second)
			must(err)
			add := func(head ast.ASTHandle, tails []ast.ASTHandle) {
				r, err := rm.NewRule(head, tails, nil, "")
				must(err)
				must(set.Add(r))
				must(r.Close())
			}
			add(app(a, one), nil)
			add(app(b, other), nil)
			x, err := m.MkVar(0, integer)
			must(err)
			add(app(goal), []ast.ASTHandle{app(a, x), app(b, x)})
			ok, err := set.Finalize()
			must(err)
			if !ok {
				t.Fatal("rule set did not finalize")
			}
			linear, err := bmc.IsLinear(set)
			must(err)
			if linear {
				t.Fatal("branching rule treated as linear")
			}
			compiler, err := bmc.NewCompiler(m)
			must(err)
			defer func() { must(compiler.Close()) }()
			smt, err := context.NewSimpleSolver()
			must(err)
			defer func() { must(smt.Close()) }()
			for level := uint32(0); level <= 1; level++ {
				formulas, err := compiler.Compile(set, level)
				must(err)
				for i := range formulas {
					value, err := m.RefGet(formulas[i])
					must(err)
					must(smt.Assert(value))
					must(m.DelRef(&formulas[i]))
				}
			}
			queryRef, err := compiler.CompileQuery(goal, 1)
			must(err)
			query, err := m.RefGet(queryRef)
			must(err)
			guard, err := m.MkFreshConst("q", boolean)
			must(err)
			guardRef, err := m.NewRef(guard)
			must(err)
			implication, err := m.MkImplies(guard, query)
			must(err)
			must(smt.Assert(implication))
			status, err := smt.CheckSat([]ast.ASTHandle{guard})
			must(err)
			must(m.DelRef(&guardRef))
			must(m.DelRef(&queryRef))
			if status != tc.want {
				t.Fatal("branch query", status, tc.want)
			}
		})
	}
}

func TestBMCNonlinearCompilationPreservesRepeatedHeadVariables(t *testing.T) {
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
	name, err := m.Symbols.Intern("repeated-head")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rm.RegisterPredicate(predicate))
	x, err := m.MkVar(0, integer)
	must(err)
	head, err := m.MkApp(predicate, []ast.ASTHandle{x, x})
	must(err)
	rule, err := rm.NewRule(head, nil, nil, "")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
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
	formulas, err := compiler.Compile(set, 0)
	must(err)
	for i := range formulas {
		f, err := m.RefGet(formulas[i])
		must(err)
		must(smt.Assert(f))
		must(m.DelRef(&formulas[i]))
	}
	queryRef, err := compiler.CompileQuery(predicate, 0)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	query, err := m.RefGet(queryRef)
	must(err)
	levelDecl, err := m.GetDecl(query)
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	oneRef, err := m.NewRef(one)
	must(err)
	defer func() { must(m.DelRef(&oneRef)) }()
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	twoRef, err := m.NewRef(two)
	must(err)
	defer func() { must(m.DelRef(&twoRef)) }()
	for _, tc := range []struct {
		second ast.ASTHandle
		want   lbool.LBool
	}{{one, lbool.True}, {two, lbool.False}} {
		atom, err := m.MkApp(levelDecl, []ast.ASTHandle{one, tc.second})
		must(err)
		guard, err := m.MkFreshConst("q", boolean)
		must(err)
		guardRef, err := m.NewRef(guard)
		must(err)
		implication, err := m.MkImplies(guard, atom)
		must(err)
		must(smt.Assert(implication))
		status, err := smt.CheckSat([]ast.ASTHandle{guard})
		must(err)
		must(m.DelRef(&guardRef))
		if status != tc.want {
			t.Fatal("repeated head query", status, tc.want)
		}
	}
}

// AUDIT.md muz_small_01-F5: nonlinear::compile walks the rule set's grouped
// rules (decl2rules table order), not the heads' first appearance in rule
// order. Each group ends with its level predicate's implication.
func TestBMCNonlinearCompilationFollowsGroupedOrder(t *testing.T) {
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
	names := []string{"grp-a", "grp-b", "grp-c", "grp-d", "grp-e", "grp-f", "grp-g"}
	decls := make([]ast.ASTHandle, len(names))
	for i, name := range names {
		sym, err := m.Symbols.Intern(name)
		must(err)
		decls[i], err = m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(rm.RegisterPredicate(decls[i]))
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	// Add the facts in reverse declaration order.
	var ruleOrder []ast.ASTHandle
	for i := len(decls) - 1; i >= 0; i-- {
		head, err := m.MkApp(decls[i], []ast.ASTHandle{zero})
		must(err)
		rule, err := rm.NewRule(head, nil, nil, names[i])
		must(err)
		must(set.Add(rule))
		must(rule.Close())
		ruleOrder = append(ruleOrder, decls[i])
	}
	ok, err := set.Finalize()
	must(err)
	if !ok {
		t.Fatal("rule set did not finalize")
	}
	grouped, err := set.GroupedPredicates()
	must(err)
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	formulas, err := compiler.Compile(set, 0)
	must(err)
	defer func() {
		for i := range formulas {
			must(m.DelRef(&formulas[i]))
		}
	}()
	if len(formulas) != 2*len(grouped) {
		t.Fatal("compiled formulas", len(formulas), len(grouped))
	}
	headName := func(formula ast.ASTHandle) string {
		kind, err := m.GetKind(formula)
		must(err)
		if kind == ast.ASTQuantifier {
			formula, err = m.GetQuantifierBody(formula)
			must(err)
		}
		head, err := m.GetArg(formula, 0)
		must(err)
		decl, err := m.GetDecl(head)
		must(err)
		name, err := m.GetASTName(decl)
		must(err)
		text, err := m.Symbols.Display(name)
		must(err)
		return text
	}
	for i, predicate := range grouped {
		value, err := m.RefGet(formulas[2*i+1])
		must(err)
		name, err := m.GetASTName(predicate)
		must(err)
		text, err := m.Symbols.Display(name)
		must(err)
		if got, want := headName(value), text+"#0"; got != want {
			t.Fatalf("group %d head %s, want %s (grouped order)", i, got, want)
		}
	}
	same := true
	for i := range grouped {
		same = same && grouped[i] == ruleOrder[i]
	}
	if same {
		t.Log("grouped order coincides with rule order for this fixture")
	}
}
