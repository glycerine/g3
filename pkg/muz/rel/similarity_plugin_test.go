package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestFiniteSimilarityCompressorTransformerAndExecution(t *testing.T) {
	m := astTestManager(t)
	ctx, e := base.NewDatalogContext(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ctx.Close()) }()
	relations, e := NewRelationManager(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, relations.Close()) }()
	rm, e := ctx.RuleManager()
	astTestMust(t, e)
	set, e := base.NewRuleSet(rm)
	astTestMust(t, e)
	defer func() { astTestMust(t, set.Close()) }()
	name, e := m.Symbols.Intern("plugin-domain")
	astTestMust(t, e)
	sort, e := m.MkDatalogFiniteSort(name, 5)
	astTestMust(t, e)
	astTestPin(t, m, sort)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	pred, e := m.MkFuncDecl(astTestSymbol(t, m, "plugin-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, pred)
	astTestMust(t, ctx.RegisterPredicate(pred, false))
	for i := uint64(0); i < 3; i++ {
		value, e := m.MkDatalogNumeral(i, sort)
		astTestMust(t, e)
		astTestPin(t, m, value)
		head, e := m.MkApp(pred, []ast.ASTHandle{value})
		astTestMust(t, e)
		astTestPin(t, m, head)
		rule, e := rm.NewRule(head, nil, nil, "")
		astTestMust(t, e)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}
	valid, e := set.Finalize()
	astTestMust(t, e)
	if !valid {
		t.Fatal("source stratification")
	}
	plugin, e := NewFiniteSimilarityCompressor(ctx, relations, 2)
	astTestMust(t, e)
	transformer := base.NewRuleTransformer()
	astTestMust(t, transformer.Register(plugin))
	defer func() { astTestMust(t, transformer.Close()) }()
	modified, e := transformer.Apply(set)
	astTestMust(t, e)
	if !modified {
		t.Fatal("compressor did not run")
	}
	n, e := set.Len()
	astTestMust(t, e)
	if n != 1 {
		t.Fatal("compressed rule count", n)
	}
	rule, e := set.Rule(0)
	astTestMust(t, e)
	auxTail, neg, e := rule.Tail(0)
	astTestMust(t, e)
	if neg {
		t.Fatal("negative auxiliary tail")
	}
	aux, e := m.GetDecl(auxTail)
	astTestMust(t, e)
	astTestMust(t, rule.Close())
	saturated, e := relations.IsSaturated(aux)
	astTestMust(t, e)
	if !saturated {
		t.Fatal("auxiliary relation not saturated")
	}
	execution, termination, e := CompileFiniteRules(relations, set)
	astTestMust(t, e)
	defer func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) }()
	runtime, e := NewExecutionContextWithRelations(relations, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, runtime.Close()) }()
	ok, e := execution.Perform(runtime)
	astTestMust(t, e)
	if !ok {
		t.Fatal("execution incomplete")
	}
	ok, e = termination.Perform(runtime)
	astTestMust(t, e)
	if !ok {
		t.Fatal("termination incomplete")
	}
	result, found, e := relations.TryGetRelation(pred)
	astTestMust(t, e)
	if !found {
		t.Fatal("result missing")
	}
	for _, tc := range []struct {
		value Element
		want  bool
	}{{0, true}, {1, true}, {2, true}, {3, false}} {
		ok, e := result.Contains(Fact{tc.value})
		astTestMust(t, e)
		if ok != tc.want {
			t.Fatalf("fact %d = %v", tc.value, ok)
		}
	}
}
