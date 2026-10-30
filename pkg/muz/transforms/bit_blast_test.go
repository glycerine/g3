package transforms_test

// Source: src/muz/transforms/dl_mk_bit_blast.cpp predicate expansion and
// zero-arity output retention.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestBitBlastExpandsBitVectorPredicateSignatures(t *testing.T) {
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
	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	bv, err := manager.BitVectorSort(3)
	coneMust(t, err)
	pName, err := manager.Symbols.Intern("bit-blast-p")
	coneMust(t, err)
	p, err := manager.MkFuncDecl(pName, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	goalName, err := manager.Symbols.Intern("bit-blast-goal")
	coneMust(t, err)
	goal, err := manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(p))
	coneMust(t, rules.RegisterPredicate(goal))
	x, err := manager.MkVar(0, bv)
	coneMust(t, err)
	pOfX, err := manager.MkApp(p, []ast.ASTHandle{x})
	coneMust(t, err)
	five, err := manager.BitVectorNumeralUnsigned(5, 3)
	coneMust(t, err)
	condition, err := manager.MkEq(x, five)
	coneMust(t, err)
	goalApp, err := manager.MkApp(goal, nil)
	coneMust(t, err)
	fact, err := rules.NewRule(pOfX, []ast.ASTHandle{condition}, nil, "bv-fact")
	coneMust(t, err)
	query, err := rules.NewRule(goalApp, []ast.ASTHandle{pOfX}, nil, "bv-query")
	coneMust(t, err)
	coneMust(t, set.Add(fact))
	coneMust(t, set.Add(query))
	coneMust(t, fact.Close())
	coneMust(t, query.Close())
	coneMust(t, set.SetOutputPredicate(goal))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("source rule set is not stratified")
	}
	plugin, err := transforms.NewBitBlastPlugin(manager, rules, globals.Context, globals, 35000)
	coneMust(t, err)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("bit blast did not modify the rule set")
	}
	fresh := plugin.FreshPredicates()[p]
	if fresh.IsNull() {
		t.Fatal("missing bit-blasted predicate")
	}
	if arity, getErr := manager.GetArity(fresh); getErr != nil || arity != 3 {
		t.Fatal("bit-blasted predicate arity", arity, getErr)
	}
	for i := uint32(0); i < 3; i++ {
		sort, getErr := manager.GetDomain(fresh, i)
		coneMust(t, getErr)
		if sort != boolean {
			t.Fatal("bit-blasted predicate domain", i, sort)
		}
	}
	if original, getErr := set.OriginalPredicate(fresh); getErr != nil || original != p {
		t.Fatal("bit-blasted predicate mapping", original, getErr)
	}
	if output, getErr := set.IsOutputPredicate(goal); getErr != nil || !output {
		t.Fatal("zero-arity output was not retained", output, getErr)
	}
}

func TestBitBlastStopsBeforeRulesWhenManagerIsCanceled(t *testing.T) {
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
	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	bv, err := manager.BitVectorSort(3)
	coneMust(t, err)
	name, err := manager.Symbols.Intern("bit-blast-cancel")
	coneMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(predicate))
	variable, err := manager.MkVar(0, bv)
	coneMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	coneMust(t, err)
	value, err := manager.BitVectorNumeralUnsigned(5, 3)
	coneMust(t, err)
	condition, err := manager.MkEq(variable, value)
	coneMust(t, err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{condition}, nil, "bit-blast-cancel")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	plugin, err := transforms.NewBitBlastPlugin(manager, rules, globals.Context, globals, 35000)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	manager.Limit.Cancel()
	result, err := plugin.Transform(set)
	coneMust(t, err)
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("canceled transform returned a partially built rule set")
	}
}
