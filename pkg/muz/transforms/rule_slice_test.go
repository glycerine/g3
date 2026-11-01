package transforms_test

// Source: src/muz/transforms/dl_mk_slice.cpp predicate-position saturation
// and rule/predicate rewriting.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestRuleSliceRemovesUnconstrainedPredicateColumns(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P", []ast.ASTHandle{integer, integer})
	goal := declare("Goal", nil)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	pXY, err := f.manager.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, makeErr := f.rules.NewRule(head, tails, nil, name)
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	add(pXY, nil, "universal-P")
	add(goalApp, []ast.ASTHandle{pXY}, "goal")
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("slice fixture rejected")
	}
	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("slice plugin did not rewrite unconstrained columns")
	}
	derived, found := plugin.DerivedPredicate(p)
	if !found {
		t.Fatal("slice plugin did not publish predicate mapping")
	}
	predicateSnapshot, sliceSnapshot := plugin.PredicateSlices()
	if predicateSnapshot[p] != derived || len(sliceSnapshot[p]) != 2 || !sliceSnapshot[p][0] || !sliceSnapshot[p][1] {
		t.Fatal("slice plugin snapshot does not describe the removed predicate columns", predicateSnapshot, sliceSnapshot)
	}
	sliceSnapshot[p][0] = false
	_, freshSliceSnapshot := plugin.PredicateSlices()
	if !freshSliceSnapshot[p][0] {
		t.Fatal("slice plugin exposed its mutable internal column mask")
	}
	if arity, arityErr := f.manager.GetArity(derived); arityErr != nil || arity != 0 {
		t.Fatal("sliced predicate arity", arity, arityErr)
	}
	if mapped, mapErr := f.set.DerivedPredicate(p); mapErr != nil || mapped != derived {
		t.Fatal("sliced RuleSet mapping", mapped, derived, mapErr)
	}
	if output, outputErr := f.set.OutputPredicate(); outputErr != nil || output != goal {
		t.Fatal("slice output predicate", output, outputErr)
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 2 {
		t.Fatalf("sliced rule count %d, want 2", count)
	}
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		if getErr == nil && head == derived {
			var application ast.ASTHandle
			application, getErr = rule.Head()
			if getErr == nil {
				var arguments uint32
				arguments, getErr = f.manager.GetNumArgs(application)
				if getErr == nil && arguments != 0 {
					t.Fatalf("sliced head retained %d arguments", arguments)
				}
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		coneMust(t, getErr)
	}
}

func TestRuleSliceLeavesZeroArityOutputRulesUnchanged(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	symbol, err := f.manager.Symbols.Intern("slice-zero-arity-output")
	coneMust(t, err)
	goal, err := f.manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(goal))
	application, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	rule, err := f.rules.NewRule(application, nil, nil, "zero-arity-output")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("zero-arity slice fixture rejected")
	}

	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if modified {
		t.Fatal("zero-arity-only output rules should not produce a sliced replacement")
	}
	if _, found := plugin.DerivedPredicate(goal); found {
		t.Fatal("zero-arity output acquired a derived predicate")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 1 {
		t.Fatal("zero-arity source rules changed", count)
	}
}

func TestRuleSliceRetainsNegatedPredicateColumns(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-negated-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P", []ast.ASTHandle{integer, integer})
	goal := declare("Goal", nil)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	pXY, err := f.manager.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	rule, err := f.rules.NewRule(goalApp, []ast.ASTHandle{pXY}, []bool{true}, "negated-columns")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("negated slice fixture rejected")
	}

	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if modified {
		t.Fatal("negative-tail columns must remain unsliced")
	}
	if _, found := plugin.DerivedPredicate(p); found {
		t.Fatal("negative-tail predicate acquired a sliced declaration")
	}
}

func TestRuleSlicePropagatesInterpretedEqualityMasks(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-equality-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p, q := declare("P"), declare("Q")
	goalName, err := f.manager.Symbols.Intern("slice-equality-Goal")
	coneMust(t, err)
	goal, err := f.manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(goal))
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	pX, err := f.manager.MkApp(p, []ast.ASTHandle{x})
	coneMust(t, err)
	qY, err := f.manager.MkApp(q, []ast.ASTHandle{y})
	coneMust(t, err)
	equality, err := f.manager.MkEq(x, y)
	coneMust(t, err)
	rule, err := f.rules.NewRule(pX, []ast.ASTHandle{qY, equality}, nil, "interpreted-equality")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	z, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	pZ, err := f.manager.MkApp(p, []ast.ASTHandle{z})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	goalRule, err := f.rules.NewRule(goalApp, []ast.ASTHandle{pZ}, nil, "goal")
	coneMust(t, err)
	coneMust(t, f.set.Add(goalRule))
	coneMust(t, goalRule.Close())
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("interpreted-equality slice fixture rejected")
	}

	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("interpreted equality should permit removal of the solved output column")
	}
	for _, original := range []ast.ASTHandle{p, q} {
		derived, found := plugin.DerivedPredicate(original)
		if !found {
			t.Fatal("interpreted equality did not propagate the sliced predicate", original)
		}
		if arity, getErr := f.manager.GetArity(derived); getErr != nil || arity != 0 {
			t.Fatal("interpreted equality derived arity", original, arity, getErr)
		}
	}
	goalRules, err := f.set.PredicateRules(pluginDerived(t, plugin, p))
	coneMust(t, err)
	defer func() {
		for _, result := range goalRules {
			coneMust(t, result.Close())
		}
	}()
	if len(goalRules) != 1 {
		t.Fatalf("interpreted equality produced %d sliced rules, want 1", len(goalRules))
	}
	tailSize, err := goalRules[0].TailSize()
	coneMust(t, err)
	if tailSize != 2 {
		t.Fatalf("rewritten rule lost interpreted equality: tail size %d", tailSize)
	}
}

func pluginDerived(t *testing.T, plugin *transforms.RuleSlicePlugin, original ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	derived, found := plugin.DerivedPredicate(original)
	if !found {
		t.Fatal("missing derived predicate", original)
	}
	return derived
}

func TestRuleSliceRetainsConstantConstrainedColumn(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-constant-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P", []ast.ASTHandle{integer, integer})
	goal := declare("Goal", nil)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	pXY, err := f.manager.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, err)
	pZeroY, err := f.manager.MkApp(p, []ast.ASTHandle{zero, y})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
		name  string
	}{{pXY, nil, "universal-P"}, {goalApp, []ast.ASTHandle{pZeroY}, "goal"}} {
		rule, makeErr := f.rules.NewRule(spec.head, spec.tails, nil, spec.name)
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("constant slice fixture rejected")
	}
	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("constant slice fixture was not rewritten")
	}
	derived, found := plugin.DerivedPredicate(p)
	if !found {
		t.Fatal("constant slice mapping missing")
	}
	if arity, arityErr := f.manager.GetArity(derived); arityErr != nil || arity != 1 {
		t.Fatal("constant-constrained sliced arity", arity, arityErr)
	}
	rules, err := f.set.PredicateRules(goal)
	coneMust(t, err)
	defer func() {
		for _, rule := range rules {
			coneMust(t, rule.Close())
		}
	}()
	if len(rules) != 1 {
		t.Fatalf("constant slice goal rules %d, want 1", len(rules))
	}
	tail, _, err := rules[0].Tail(0)
	coneMust(t, err)
	if declaration, declarationErr := f.manager.GetDecl(tail); declarationErr != nil || declaration != derived {
		t.Fatal("constant slice tail declaration", declaration, derived, declarationErr)
	}
	argument, err := f.manager.GetArg(tail, 0)
	coneMust(t, err)
	if argument != zero {
		t.Fatal("constant slice did not retain constrained argument", argument, zero)
	}
}

func TestRuleSliceGeneratesProofsForRewrittenRules(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			f := newConeFixture(t)
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
			coneMust(t, err)
			integer, err := f.manager.GetSort(zero)
			coneMust(t, err)
			declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				symbol, e := f.manager.Symbols.Intern(name)
				coneMust(t, e)
				decl, e := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				coneMust(t, e)
				coneMust(t, f.rules.RegisterPredicate(decl))
				return decl
			}
			p := declare("slice-proof-P", []ast.ASTHandle{integer})
			goal := declare("slice-proof-goal", nil)
			v, err := f.manager.MkVar(0, integer)
			coneMust(t, err)
			pApp, err := f.manager.MkApp(p, []ast.ASTHandle{v})
			coneMust(t, err)
			goalApp, err := f.manager.MkApp(goal, nil)
			coneMust(t, err)
			originalProofs := make(map[ast.ASTHandle]ast.ASTHandle)
			for _, spec := range []struct {
				head  ast.ASTHandle
				tails []ast.ASTHandle
			}{{pApp, nil}, {goalApp, []ast.ASTHandle{pApp}}} {
				rule, e := f.rules.NewRule(spec.head, spec.tails, nil, "")
				coneMust(t, e)
				if enabled {
					coneMust(t, f.rules.MkRuleAssertedProof(rule))
					proof, found, e := rule.Proof()
					coneMust(t, e)
					if !found {
						t.Fatal("original proof missing")
					}
					decl, e := rule.HeadDeclaration()
					coneMust(t, e)
					originalProofs[decl] = proof
				}
				coneMust(t, f.set.Add(rule))
				coneMust(t, rule.Close())
			}
			coneMust(t, f.set.SetOutputPredicate(goal))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("invalid fixture")
			}
			plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 1)
			plugin.SetGenerateProofTrace(enabled)
			transformer := base.NewRuleTransformer()
			coneMust(t, transformer.Register(plugin))
			modified, err := transformer.Apply(f.set)
			coneMust(t, err)
			if !modified {
				t.Fatal("expected sliced rules")
			}
			derived, found := plugin.DerivedPredicate(p)
			if !found {
				t.Fatal("missing sliced predicate")
			}
			converter := plugin.TakeProofConverter()
			if (converter != nil) != enabled {
				t.Fatal("unexpected proof converter")
			}
			coneMust(t, transformer.Close())
			if converter != nil {
				defer func() { coneMust(t, converter.Close()) }()
				_, matched, e := converter.ConvertAsserted(originalProofs[p])
				coneMust(t, e)
				if matched {
					t.Fatal("unmapped original proof unexpectedly matched")
				}
			}
			count, err := f.set.Len()
			coneMust(t, err)
			if count != 2 {
				t.Fatalf("got %d rules", count)
			}
			for i := 0; i < count; i++ {
				rule, e := f.set.Rule(i)
				coneMust(t, e)
				proof, found, e := rule.Proof()
				coneMust(t, e)
				if found != enabled {
					t.Fatalf("proof present = %v, want %v", found, enabled)
				}
				if found {
					converted, matched, e := converter.ConvertAsserted(proof)
					coneMust(t, e)
					if !matched {
						t.Fatal("sliced rule was not mapped")
					}
					decl, e := rule.HeadDeclaration()
					coneMust(t, e)
					if decl == derived {
						decl = p
					}
					want := originalProofs[decl]
					got, e := f.manager.RefGet(converted)
					coneMust(t, e)
					if got != want {
						t.Fatal("converter returned the wrong original proof")
					}
					coneMust(t, f.manager.DelRef(&converted))
					formula, e := rule.ToFormula()
					coneMust(t, e)
					fact, e := f.manager.RefGet(formula)
					coneMust(t, e)
					proved, e := f.manager.GetProofFact(proof)
					coneMust(t, e)
					if proved != fact {
						t.Fatal("proof concludes a different formula")
					}
					coneMust(t, f.manager.DelRef(&formula))
				}
				coneMust(t, rule.Close())
			}
			mode, err := f.manager.ProofMode()
			coneMust(t, err)
			if mode != ast.ASTProofDisabled {
				t.Fatal("proof mode was not restored")
			}
		})
	}
}
