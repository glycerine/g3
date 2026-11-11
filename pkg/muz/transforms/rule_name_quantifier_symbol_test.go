package transforms_test

// Source: dl_mk_bit_blast.cpp, dl_mk_quantifier_abstraction.cpp and
// dl_mk_quantifier_instantiation.cpp pass r.name() to replacement rules.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func quantifierTransformNameCases(t *testing.T, manager *ast.ASTManager) []usymbol.Symbol {
	t.Helper()
	empty, err := manager.Symbols.Intern("")
	coneMust(t, err)
	spelling, err := manager.Symbols.Intern("k!7")
	coneMust(t, err)
	names := []usymbol.Symbol{{}, empty, usymbol.Numerical(7), spelling}
	for i, name := range names {
		for _, earlier := range names[:i] {
			if name == earlier {
				t.Fatal("fixture collapsed distinct rule-name symbols")
			}
		}
	}
	return names
}

type quantifierTransformRuleSnapshot struct {
	head     ast.ASTHandle
	tails    []ast.ASTHandle
	negative []bool
	name     usymbol.Symbol
}

func snapshotQuantifierTransformRules(t *testing.T, set *base.RuleSet) []quantifierTransformRuleSnapshot {
	t.Helper()
	count, err := set.Len()
	coneMust(t, err)
	result := make([]quantifierTransformRuleSnapshot, count)
	for i := range result {
		rule, err := set.Rule(i)
		coneMust(t, err)
		result[i].head, err = rule.Head()
		coneMust(t, err)
		result[i].name, err = rule.NameSymbol()
		coneMust(t, err)
		size, err := rule.TailSize()
		coneMust(t, err)
		for j := 0; j < size; j++ {
			tail, negative, err := rule.Tail(j)
			coneMust(t, err)
			result[i].tails = append(result[i].tails, tail)
			result[i].negative = append(result[i].negative, negative)
		}
		coneMust(t, rule.Close())
	}
	return result
}

func checkQuantifierTransformSource(t *testing.T, set *base.RuleSet, expected []quantifierTransformRuleSnapshot) {
	t.Helper()
	actual := snapshotQuantifierTransformRules(t, set)
	if len(actual) != len(expected) {
		t.Fatal("transform changed source rule count")
	}
	for i, want := range expected {
		got := actual[i]
		if got.head != want.head || got.name != want.name || len(got.tails) != len(want.tails) {
			t.Fatal("transform changed source rule shape, name or order", i)
		}
		for j, tail := range want.tails {
			if got.tails[j] != tail || got.negative[j] != want.negative[j] {
				t.Fatal("transform changed source tail or polarity", i, j)
			}
		}
	}
}

func addQuantifierTransformNamedRule(t *testing.T, f *coneFixture, head ast.ASTHandle, tails []ast.ASTHandle, name usymbol.Symbol) {
	t.Helper()
	rule, err := f.rules.NewRuleWithSymbol(head, tails, nil, name)
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
}

func checkCollapsedQuantifierTransformNames(t *testing.T, result *base.RuleSet, names []usymbol.Symbol) {
	t.Helper()
	collapse := func(in []usymbol.Symbol) []usymbol.Symbol {
		var out []usymbol.Symbol
		for _, name := range in {
			if len(out) == 0 || out[len(out)-1] != name {
				out = append(out, name)
			}
		}
		return out
	}
	count, err := result.Len()
	coneMust(t, err)
	var got []usymbol.Symbol
	for i := 0; i < count; i++ {
		rule, err := result.Rule(i)
		coneMust(t, err)
		name, err := rule.NameSymbol()
		coneMust(t, err)
		coneMust(t, rule.Close())
		got = append(got, name)
	}
	want, have := collapse(names), collapse(got)
	if len(want) != len(have) {
		t.Fatal("replacement rule names", have, "want", want)
	}
	for i := range want {
		if want[i] != have[i] {
			t.Fatal("replacement rule names", have, "want", want)
		}
	}
}

func checkQuantifierTransformNames(t *testing.T, result *base.RuleSet, names []usymbol.Symbol) {
	t.Helper()
	if result == nil {
		t.Fatal("transform did not return a changed rule set")
	}
	count, err := result.Len()
	coneMust(t, err)
	if count != len(names) {
		t.Fatal("replacement rule count", count, "want", len(names))
	}
	for i, want := range names {
		rule, err := result.Rule(i)
		coneMust(t, err)
		got, err := rule.NameSymbol()
		coneMust(t, err)
		coneMust(t, rule.Close())
		if got != want {
			t.Errorf("replacement rule %d changed name identity or source order: got %#v, want %#v", i, got, want)
		}
	}
}

func TestBitBlastPreservesRuleNameSymbolKinds(t *testing.T) {
	f := newConeFixture(t)
	names := quantifierTransformNameCases(t, f.manager)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	bv, err := f.manager.BitVectorSort(3)
	coneMust(t, err)
	name, err := f.manager.Symbols.Intern("named-bit-blast-p")
	coneMust(t, err)
	declaration, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(declaration))
	variable, err := f.manager.MkVar(0, bv)
	coneMust(t, err)
	head, err := f.manager.MkApp(declaration, []ast.ASTHandle{variable})
	coneMust(t, err)
	five, err := f.manager.BitVectorNumeralUnsigned(5, 3)
	coneMust(t, err)
	condition, err := f.manager.MkEq(variable, five)
	coneMust(t, err)
	for _, name := range names {
		addQuantifierTransformNamedRule(t, f, head, []ast.ASTHandle{condition}, name)
	}
	original := snapshotQuantifierTransformRules(t, f.set)
	plugin, err := transforms.NewBitBlastPlugin(f.manager, f.rules, nil, f.globals.Context, f.globals, 35000)
	coneMust(t, err)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	checkQuantifierTransformNames(t, result, names)
	defer func() { coneMust(t, result.Close()) }()
	fresh := plugin.FreshPredicates()[declaration]
	if fresh.IsNull() || fresh == declaration {
		t.Fatal("bit blast did not expand the predicate")
	}
	if arity, err := f.manager.GetArity(fresh); err != nil || arity != 3 {
		t.Fatal("bit-blasted predicate arity", arity, err)
	}
	for i := range names {
		rule, err := result.Rule(i)
		coneMust(t, err)
		got, err := rule.HeadDeclaration()
		coneMust(t, err)
		coneMust(t, rule.Close())
		if got != fresh {
			t.Fatal("named replacement did not reach the expanded predicate", i)
		}
	}
	checkQuantifierTransformSource(t, f.set, original)
}

func TestQuantifierAbstractionPreservesRuleNameSymbolKinds(t *testing.T) {
	f := newConeFixture(t, "Goal")
	names := quantifierTransformNameCases(t, f.manager)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	name, err := f.manager.Symbols.Intern("named-abstraction-p")
	coneMust(t, err)
	declaration, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{arraySort}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(declaration))
	variable, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	head, err := f.manager.MkApp(declaration, []ast.ASTHandle{variable})
	coneMust(t, err)
	var expected []usymbol.Symbol
	for _, name := range names {
		addQuantifierTransformNamedRule(t, f, head, nil, name)
		addQuantifierTransformNamedRule(t, f, f.applications["Goal"], []ast.ASTHandle{head}, name)
		expected = append(expected, name, name)
	}
	original := snapshotQuantifierTransformRules(t, f.set)
	plugin := transforms.NewQuantifierAbstractionPlugin(f.manager, f.rules, 38000)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	// rm.mk_rule's Horn normalization derives several rules per source rule,
	// all carrying its name, in source order.
	checkCollapsedQuantifierTransformNames(t, result, expected)
	fresh := plugin.FreshPredicates()[declaration]
	if fresh.IsNull() || fresh == declaration {
		t.Fatal("quantifier abstraction did not expand the array predicate")
	}
	if arity, err := f.manager.GetArity(fresh); err != nil || arity != 2 {
		t.Fatal("abstracted predicate arity", arity, err)
	}
	count, err := result.Len()
	coneMust(t, err)
	freshHead, quantifiedTail := false, false
	for i := 0; i < count; i++ {
		rule, err := result.Rule(i)
		coneMust(t, err)
		got, err := rule.HeadDeclaration()
		coneMust(t, err)
		freshHead = freshHead || got == fresh
		tails, err := rule.TailSize()
		coneMust(t, err)
		for j := 0; j < tails; j++ {
			tail, negative, err := rule.Tail(j)
			coneMust(t, err)
			quantified, err := f.manager.HasQuantifiers(tail)
			coneMust(t, err)
			quantifiedTail = quantifiedTail || (!negative && quantified)
		}
		coneMust(t, rule.Close())
	}
	if !freshHead || !quantifiedTail {
		t.Fatal("abstracted rules lost the fresh head or the quantified array tail", freshHead, quantifiedTail)
	}
	checkQuantifierTransformSource(t, f.set, original)
}

func TestQuantifierInstantiationPreservesRuleNameSymbolKinds(t *testing.T) {
	f := newConeFixture(t, "Goal")
	names := quantifierTransformNameCases(t, f.manager)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	falsity, err := f.manager.MkFalse()
	coneMust(t, err)
	truth, err := f.manager.MkTrue()
	coneMust(t, err)
	array, err := f.manager.MkConstArray(arraySort, falsity)
	coneMust(t, err)
	family, err := f.manager.ArrayFamilyID()
	coneMust(t, err)
	groundSelect, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, truth})
	coneMust(t, err)
	groundEquality, err := f.manager.MkEq(groundSelect, falsity)
	coneMust(t, err)
	bound, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	trigger, err := f.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, bound})
	coneMust(t, err)
	body, err := f.manager.MkEq(trigger, falsity)
	coneMust(t, err)
	quantifier, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	coneMust(t, err)
	quantifiedTail, err := f.manager.MkIff(quantifier, truth)
	coneMust(t, err)
	for _, name := range names {
		addQuantifierTransformNamedRule(t, f, f.applications["Goal"], []ast.ASTHandle{groundEquality, quantifiedTail}, name)
	}
	original := snapshotQuantifierTransformRules(t, f.set)
	plugin := transforms.NewQuantifierInstantiationPlugin(f.manager, f.rules, 37000)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	checkQuantifierTransformNames(t, result, names)
	defer func() { coneMust(t, result.Close()) }()
	for i := range names {
		rule, err := result.Rule(i)
		coneMust(t, err)
		size, err := rule.TailSize()
		coneMust(t, err)
		if size != 2 {
			t.Fatal("named instantiation missed its ground conjunct and instance", i, size)
		}
		for j := 0; j < size; j++ {
			tail, negative, err := rule.Tail(j)
			coneMust(t, err)
			quantified, err := f.manager.HasQuantifiers(tail)
			coneMust(t, err)
			if quantified || negative || tail != groundEquality {
				t.Fatal("named rule kept a quantifier or produced the wrong instance", i, j)
			}
		}
		coneMust(t, rule.Close())
	}
	checkQuantifierTransformSource(t, f.set, original)
}
