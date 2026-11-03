package transforms_test

// Source: dl_mk_interp_tail_simplifier.cpp:401 makes variable-equivalence
// propagation optional while ordinary interpreted-tail simplification remains.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestInterpTailSimplifierVariableEquivalenceParameters(t *testing.T) {
	for _, test := range []struct {
		name, module, override string
		propagate              bool
	}{
		{name: "default", propagate: true},
		{name: "module-true", module: "true", propagate: true},
		{name: "module-false", module: "false", propagate: false},
		{name: "override-false-over-module-true", module: "true", override: "false", propagate: false},
		{name: "override-true-over-module-false", module: "false", override: "true", propagate: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newInterpTailFixture(t)
			if test.module != "" {
				coneMust(t, f.globals.SourceSetName("fixedpoint.xform.tail_simplifier_pve", test.module))
			}
			// Exercise the public constructor after the module option is set.
			coneMust(t, f.plugin.Close())
			var err error
			f.plugin, err = transforms.NewInterpTailSimplifierPlugin(f.manager, f.rules, f.globals.Context, f.globals, 0)
			coneMust(t, err)
			t.Cleanup(func() { coneMust(t, f.plugin.Close()) })
			if test.override != "" {
				params := transforms.DefaultRuleNormalizationParams()
				params.PropagateVariableEquivalences = test.override == "true"
				f.plugin.SetNormalizationParams(params)
			}
			checkInterpTailVariableEquivalence(t, f, test.propagate)
		})
	}
}

func checkInterpTailVariableEquivalence(t *testing.T, f *interpTailFixture, propagate bool) {
	t.Helper()
	one, err := f.manager.ArithmeticIntegerNumeralText("1")
	coneMust(t, err)
	integer, err := f.manager.GetSort(one)
	coneMust(t, err)
	variable, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	predicate := f.predicate(t, "parameter-P", []ast.ASTHandle{integer})
	head, err := f.manager.MkApp(predicate, []ast.ASTHandle{variable})
	coneMust(t, err)
	equality, err := f.manager.MkEq(variable, one)
	coneMust(t, err)
	truth, err := f.manager.MkTrue()
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{equality, truth}, nil, "parameter-propagation")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("parameter fixture rejected")
	}
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(f.plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("ordinary true-tail simplification did not run")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 1 {
		t.Fatal("parameter changed rule count", count)
	}
	result, err := f.set.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	resultHead, err := result.Head()
	coneMust(t, err)
	argument, err := f.manager.GetArg(resultHead, 0)
	coneMust(t, err)
	tails, err := result.TailSize()
	coneMust(t, err)
	if propagate {
		if argument != one || tails != 0 {
			t.Fatal("enabled propagation did not produce P(1)", argument, tails)
		}
		return
	}
	kind, err := f.manager.GetKind(argument)
	coneMust(t, err)
	if kind != ast.ASTVar || argument != variable || tails != 1 {
		t.Fatal("disabled propagation must preserve P(x) and one equality", kind, argument, tails)
	}
	tail, negative, err := result.Tail(0)
	coneMust(t, err)
	family, err := f.manager.GetFamilyID(tail)
	coneMust(t, err)
	op, err := f.manager.GetDeclKind(tail)
	coneMust(t, err)
	if negative || family != f.manager.BasicFamilyID() || op != ast.ASTOpEq {
		t.Fatal("disabled propagation lost its positive equality", tail, negative)
	}
	left, err := f.manager.GetArg(tail, 0)
	coneMust(t, err)
	right, err := f.manager.GetArg(tail, 1)
	coneMust(t, err)
	if !(left == variable && right == one || left == one && right == variable) {
		t.Fatal("disabled propagation changed x=1", left, right)
	}
}
