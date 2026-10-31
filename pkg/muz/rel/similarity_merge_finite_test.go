package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestSimilarityMergeFinitePublishesFactsAndRule(t *testing.T) {
	m := astTestManager(t)
	ctx, e := base.NewDatalogContext(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, ctx.Close()) })
	relations, e := NewRelationManager(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	rm, e := ctx.RuleManager()
	astTestMust(t, e)
	set, e := base.NewRuleSet(rm)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, set.Close()) })
	name, e := m.Symbols.Intern("finite")
	astTestMust(t, e)
	sort, e := m.MkDatalogFiniteSort(name, 5)
	astTestMust(t, e)
	astTestPin(t, m, sort)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	pred, e := m.MkFuncDecl(astTestSymbol(t, m, "finite-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, pred)
	astTestMust(t, ctx.RegisterPredicate(pred, false))
	var rules []*base.Rule
	for i := uint64(0); i < 3; i++ {
		value, e := m.MkDatalogNumeral(i, sort)
		astTestMust(t, e)
		astTestPin(t, m, value)
		head, e := m.MkApp(pred, []ast.ASTHandle{value})
		astTestMust(t, e)
		astTestPin(t, m, head)
		r, e := rm.NewRule(head, nil, nil, "")
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		astTestMust(t, set.Add(r))
		rules = append(rules, r)
	}
	groups, _, e := similarityClasses(set, rules, 2)
	astTestMust(t, e)
	if len(groups) != 1 || !groups[0].merge {
		t.Fatal("eligible group", groups)
	}
	aux, rewritten, e := similarityMergeFinite(ctx, relations, groups[0])
	astTestMust(t, e)
	defer func() { astTestMust(t, rewritten.Close()) }()
	registered, e := ctx.IsPredicate(aux)
	astTestMust(t, e)
	if !registered {
		t.Fatal("auxiliary predicate not registered")
	}
	saturated, e := relations.IsSaturated(aux)
	astTestMust(t, e)
	if !saturated {
		t.Fatal("relation not saturated")
	}
	table, found, e := relations.TryGetRelation(aux)
	astTestMust(t, e)
	if !found {
		t.Fatal("missing relation")
	}
	for _, tc := range []struct {
		value Element
		want  bool
	}{{0, true}, {1, true}, {2, true}, {3, false}} {
		ok, e := table.Contains(Fact{tc.value})
		astTestMust(t, e)
		if ok != tc.want {
			t.Fatal("fact", tc.value, ok)
		}
	}
	head, e := rewritten.Head()
	astTestMust(t, e)
	variable, e := m.GetArg(head, 0)
	astTestMust(t, e)
	index, e := m.GetVarIndex(variable)
	astTestMust(t, e)
	if index != 0 {
		t.Fatal("fresh index", index)
	}
	tail, neg, e := rewritten.Tail(0)
	astTestMust(t, e)
	d, e := m.GetDecl(tail)
	astTestMust(t, e)
	arg, e := m.GetArg(tail, 0)
	astTestMust(t, e)
	if neg || d != aux || arg != variable {
		t.Fatal("auxiliary rule tail")
	}
	// Execute the generated rule against the actual stored auxiliary relation.
	generated, e := base.NewRuleSet(rm)
	astTestMust(t, e)
	defer func() { astTestMust(t, generated.Close()) }()
	astTestMust(t, generated.Add(rewritten))
	valid, e := generated.Finalize()
	astTestMust(t, e)
	if !valid {
		t.Fatal("generated rules are not stratified")
	}
	execution, termination, e := CompileFiniteRules(relations, generated)
	astTestMust(t, e)
	defer func() { astTestMust(t, execution.Close()); astTestMust(t, termination.Close()) }()
	runtime, e := NewExecutionContextWithRelations(relations, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, runtime.Close()) }()
	complete, e := execution.Perform(runtime)
	astTestMust(t, e)
	if !complete {
		t.Fatal("execution did not complete")
	}
	complete, e = termination.Perform(runtime)
	astTestMust(t, e)
	if !complete {
		t.Fatal("termination did not complete")
	}
	result, found, e := relations.TryGetRelation(pred)
	astTestMust(t, e)
	if !found {
		t.Fatal("generated rule did not produce a relation")
	}
	for _, tc := range []struct {
		value Element
		want  bool
	}{{0, true}, {1, true}, {2, true}, {3, false}} {
		ok, e := result.Contains(Fact{tc.value})
		astTestMust(t, e)
		if ok != tc.want {
			t.Fatal("generated rule result", tc.value, ok)
		}
	}
}

func TestSimilarityMergeFiniteRejectsInfiniteDomainBeforeRegistration(t *testing.T) {
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
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	sort, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	pred, e := m.MkFuncDecl(astTestSymbol(t, m, "infinite-p"), []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, pred)
	astTestMust(t, ctx.RegisterPredicate(pred, false))
	var rules []*base.Rule
	for _, text := range []string{"1", "2", "3"} {
		value, e := m.ArithmeticIntegerNumeralText(text)
		astTestMust(t, e)
		astTestPin(t, m, value)
		head, e := m.MkApp(pred, []ast.ASTHandle{value})
		astTestMust(t, e)
		astTestPin(t, m, head)
		r, e := rm.NewRule(head, nil, nil, "")
		astTestMust(t, e)
		defer func() { astTestMust(t, r.Close()) }()
		rules = append(rules, r)
	}
	groups, _, e := similarityClasses(set, rules, 2)
	astTestMust(t, e)
	before, e := ctx.Predicates()
	astTestMust(t, e)
	_, rewritten, e := similarityMergeFinite(ctx, relations, groups[0])
	if e == nil || rewritten != nil {
		t.Fatal("infinite domain accepted", e)
	}
	after, e := ctx.Predicates()
	astTestMust(t, e)
	if len(after) != len(before) {
		t.Fatal("registered predicate on rejection")
	}
}
