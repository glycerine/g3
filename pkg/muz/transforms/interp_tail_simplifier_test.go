package transforms_test

// Source: src/muz/transforms/dl_mk_interp_tail_simplifier.cpp interpreted-tail
// conjunction rewriting, infeasible-rule removal, and equivalence propagation.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type interpTailFixture struct {
	manager *ast.ASTManager
	rules   *base.RuleManager
	set     *base.RuleSet
	plugin  *transforms.InterpTailSimplifierPlugin
	globals *uparams.GlobalParamsService
}

func newInterpTailFixture(t *testing.T) *interpTailFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, context.Close()) })
	manager, err := context.Manager()
	coneMust(t, err)
	globals, err := context.GlobalParameters()
	coneMust(t, err)
	rules, err := base.NewRuleManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, set.Close()) })
	plugin, err := transforms.NewInterpTailSimplifierPlugin(manager, rules, globals.Context, globals, 0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	return &interpTailFixture{manager: manager, rules: rules, set: set, plugin: plugin, globals: globals}
}

func (f *interpTailFixture) predicate(t *testing.T, name string, domain []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	symbol, err := f.manager.Symbols.Intern("tail-" + name)
	coneMust(t, err)
	declaration, err := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(declaration))
	return declaration
}

func TestInterpTailSimplifierPropagatesEqualityAndRemovesTruth(t *testing.T) {
	f := newInterpTailFixture(t)
	one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(one)
	coneMust(t, err)
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	predicate := f.predicate(t, "P", []ast.ASTHandle{integer})
	head, err := f.manager.MkApp(predicate, []ast.ASTHandle{variable})
	coneMust(t, err)
	equality, err := f.manager.MkEq(variable, one)
	coneMust(t, err)
	truth, err := f.manager.MkTrue()
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{equality, truth}, nil, "propagate")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("tail simplifier fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(f.plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("interpreted tail was not simplified")
	}
	result, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	resultHead, err := result.Head()
	coneMust(t, err)
	argument, err := f.manager.GetArg(resultHead, 0)
	coneMust(t, err)
	if argument != one {
		t.Fatal("head equality was not propagated", argument, one)
	}
	if tails, getErr := result.TailSize(); getErr != nil || tails != 0 {
		t.Fatal("simplified equality was not removed", tails, getErr)
	}
}

func TestInterpTailSimplifierDropsFalseRule(t *testing.T) {
	f := newInterpTailFixture(t)
	predicate := f.predicate(t, "P", nil)
	head, err := f.manager.MkApp(predicate, nil)
	coneMust(t, err)
	falsehood, err := f.manager.MkFalse()
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{falsehood}, nil, "false")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("false-tail fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(f.plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("false rule was not removed")
	}
	if count, getErr := f.set.Len(); getErr != nil || count != 0 {
		t.Fatal("false rule remains", count, getErr)
	}
}

func TestInterpTailSimplifierPropagatesIffPolarity(t *testing.T) {
	for _, test := range []struct {
		name string
		make func(*ast.ASTManager, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) (ast.ASTHandle, error)
		want bool
	}{
		{"negated-iff-true", func(m *ast.ASTManager, variable, truth, _ ast.ASTHandle) (ast.ASTHandle, error) {
			iff, err := m.MkIff(variable, truth)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkNot(iff)
		}, false},
		{"negated-left", func(m *ast.ASTManager, variable, truth, _ ast.ASTHandle) (ast.ASTHandle, error) {
			negative, err := m.MkNot(variable)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkIff(negative, truth)
		}, false},
		{"double-polarity", func(m *ast.ASTManager, variable, _ ast.ASTHandle, falsehood ast.ASTHandle) (ast.ASTHandle, error) {
			negative, err := m.MkNot(variable)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			iff, err := m.MkIff(negative, falsehood)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkNot(iff)
		}, false},
		{"cancelled-negations", func(m *ast.ASTManager, variable, truth, _ ast.ASTHandle) (ast.ASTHandle, error) {
			negative, err := m.MkNot(variable)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			iff, err := m.MkIff(negative, truth)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkNot(iff)
		}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newInterpTailFixture(t)
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			variable, err := f.manager.MkVar(0, boolean)
			coneMust(t, err)
			truth, err := f.manager.MkTrue()
			coneMust(t, err)
			falsehood, err := f.manager.MkFalse()
			coneMust(t, err)
			condition, err := test.make(f.manager, variable, truth, falsehood)
			coneMust(t, err)
			predicate := f.predicate(t, test.name, []ast.ASTHandle{boolean})
			head, err := f.manager.MkApp(predicate, []ast.ASTHandle{variable})
			coneMust(t, err)
			rule, err := f.rules.NewRule(head, []ast.ASTHandle{condition}, nil, test.name)
			coneMust(t, err)
			coneMust(t, f.set.Add(rule))
			coneMust(t, rule.Close())
			coneMust(t, f.set.SetOutputPredicate(predicate))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("IFF polarity fixture rejected")
			}
			transformer := base.NewRuleTransformer()
			t.Cleanup(func() { coneMust(t, transformer.Close()) })
			coneMust(t, transformer.Register(f.plugin))
			modified, err := transformer.Apply(f.set)
			coneMust(t, err)
			if !modified {
				t.Fatal("IFF polarity was not propagated")
			}
			result, err := f.set.Rule(0)
			coneMust(t, err)
			defer func() { coneMust(t, result.Close()) }()
			resultHead, err := result.Head()
			coneMust(t, err)
			argument, err := f.manager.GetArg(resultHead, 0)
			coneMust(t, err)
			if test.want && !f.manager.IsTrue(argument) || !test.want && !f.manager.IsFalse(argument) {
				t.Fatal("IFF polarity head", argument, "want", test.want)
			}
			if tails, getErr := result.TailSize(); getErr != nil || tails != 0 {
				t.Fatal("IFF polarity tail remains", tails, getErr)
			}
		})
	}
}

func TestInterpTailSimplifierRecognizesPairedClauseEquivalence(t *testing.T) {
	f := newInterpTailFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	left, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	right, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	notLeft, err := f.manager.MkNot(left)
	coneMust(t, err)
	notRight, err := f.manager.MkNot(right)
	coneMust(t, err)
	first, err := f.manager.MkOr(notLeft, right)
	coneMust(t, err)
	second, err := f.manager.MkOr(left, notRight)
	coneMust(t, err)
	predicate := f.predicate(t, "paired-equivalence", []ast.ASTHandle{boolean, boolean})
	head, err := f.manager.MkApp(predicate, []ast.ASTHandle{left, right})
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{first, second}, nil, "paired-equivalence")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("paired-equivalence fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(f.plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("paired clauses were not normalized")
	}
	result, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	resultHead, err := result.Head()
	coneMust(t, err)
	firstArgument, err := f.manager.GetArg(resultHead, 0)
	coneMust(t, err)
	secondArgument, err := f.manager.GetArg(resultHead, 1)
	coneMust(t, err)
	if firstArgument != secondArgument {
		t.Fatal("paired equivalence was not propagated into the head", firstArgument, secondArgument)
	}
	if tails, getErr := result.TailSize(); getErr != nil || tails != 0 {
		t.Fatal("paired equivalence tail remains", tails, getErr)
	}
}

func TestInterpTailSimplifierPreservesRewriteProof(t *testing.T) {
	f := newInterpTailFixture(t)
	m := f.manager
	one, err := m.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := m.GetSort(one)
	coneMust(t, err)
	x, err := m.MkVar(0, integer)
	coneMust(t, err)
	predicate := f.predicate(t, "proof", []ast.ASTHandle{integer})
	head, err := m.MkApp(predicate, []ast.ASTHandle{x})
	coneMust(t, err)
	equality, err := m.MkEq(x, one)
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{equality}, nil, "proof")
	coneMust(t, err)
	coneMust(t, f.rules.MkRuleAssertedProof(rule))
	original, found, err := rule.Proof()
	coneMust(t, err)
	if !found {
		t.Fatal("missing original proof")
	}
	originalRef, err := m.NewRef(original)
	coneMust(t, err)
	defer func() { coneMust(t, m.DelRef(&originalRef)) }()
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	transformer := base.NewRuleTransformer()
	defer func() { coneMust(t, transformer.Close()) }()
	coneMust(t, transformer.Register(f.plugin))
	changed, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !changed {
		t.Fatal("rule was not simplified")
	}
	result, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	proof, found, err := result.Proof()
	coneMust(t, err)
	if !found {
		t.Fatal("simplified rule lost its proof")
	}
	parent, err := m.GetProofParent(proof, 0)
	coneMust(t, err)
	if parent != original {
		t.Fatal("proof does not use original rule")
	}
	formula, err := result.ToFormula()
	coneMust(t, err)
	defer func() { coneMust(t, m.DelRef(&formula)) }()
	want, err := m.RefGet(formula)
	coneMust(t, err)
	fact, err := m.GetProofFact(proof)
	coneMust(t, err)
	if fact != want {
		t.Fatal("proof has wrong conclusion")
	}
	checker, err := proofs.NewChecker(m)
	coneMust(t, err)
	valid, side, err := checker.Check(proof)
	coneMust(t, err)
	defer func() {
		for _, condition := range side {
			coneMust(t, m.DecRef(condition))
		}
	}()
	if !valid {
		t.Fatal("simplifier proof is invalid")
	}
	solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: f.globals.Context, Globals: f.globals})
	coneMust(t, err)
	defer func() { coneMust(t, solver.Close()) }()
	for _, condition := range side {
		coneMust(t, solver.Push())
		negated, e := m.MkNot(condition)
		coneMust(t, e)
		coneMust(t, solver.AssertFormula(negated))
		status, e := solver.CheckSat(nil)
		coneMust(t, e)
		coneMust(t, solver.Pop(1))
		if status != lbool.False {
			t.Fatal("invalid simplifier proof side condition", status)
		}
	}
}
