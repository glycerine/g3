package transforms_test

// Source: src/muz/transforms/dl_mk_array_instantiation.cpp predicate-cell
// expansion, select linkage, and output-predicate remapping.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestArrayInstantiationExpandsPredicateCellsAndIndices(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, context.Close()) })
	manager, err := context.Manager()
	coneMust(t, err)
	rules, err := base.NewRuleManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, set.Close()) })

	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	pName, err := manager.Symbols.Intern("array-instantiation-p")
	coneMust(t, err)
	pDecl, err := manager.MkFuncDecl(pName, []ast.ASTHandle{arraySort}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	goalName, err := manager.Symbols.Intern("array-instantiation-goal")
	coneMust(t, err)
	goalDecl, err := manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(pDecl))
	coneMust(t, rules.RegisterPredicate(goalDecl))
	arrayVariable, err := manager.MkVar(0, arraySort)
	coneMust(t, err)
	indexVariable, err := manager.MkVar(1, boolean)
	coneMust(t, err)
	pApp, err := manager.MkApp(pDecl, []ast.ASTHandle{arrayVariable})
	coneMust(t, err)
	goalApp, err := manager.MkApp(goalDecl, nil)
	coneMust(t, err)
	arrayFamily, err := manager.ArrayFamilyID()
	coneMust(t, err)
	selected, err := manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{arrayVariable, indexVariable})
	coneMust(t, err)
	falseIndex, err := manager.MkFalse()
	coneMust(t, err)
	selectedFalse, err := manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{arrayVariable, falseIndex})
	coneMust(t, err)
	fact, err := rules.NewRule(pApp, []ast.ASTHandle{selected}, nil, "array-cell-fact")
	coneMust(t, err)
	query, err := rules.NewRule(goalApp, []ast.ASTHandle{pApp, selected, selectedFalse}, nil, "array-cell-query")
	coneMust(t, err)
	coneMust(t, set.Add(fact))
	coneMust(t, set.Add(query))
	coneMust(t, fact.Close())
	coneMust(t, query.Close())
	coneMust(t, set.SetOutputPredicate(goalDecl))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("array-instantiation source is unstratified")
	}

	plugin := transforms.NewArrayInstantiationPlugin(manager, rules, 2, true, 34999)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("array instantiation did not modify the rule set")
	}
	mapping := plugin.FreshPredicates()
	pFresh, goalFresh := mapping[pDecl], mapping[goalDecl]
	if pFresh.IsNull() || goalFresh.IsNull() {
		t.Fatal("missing predicate mappings", pFresh, goalFresh)
	}
	if arity, getErr := manager.GetArity(pFresh); getErr != nil || arity != 4 {
		t.Fatal("instantiated array predicate arity", arity, getErr)
	}
	if arity, getErr := manager.GetArity(goalFresh); getErr != nil || arity != 0 {
		t.Fatal("instantiated goal arity", arity, getErr)
	}
	if output, getErr := set.IsOutputPredicate(goalFresh); getErr != nil || !output {
		t.Fatal("instantiated output mapping", output, getErr)
	}
	if original, getErr := set.OriginalPredicate(pFresh); getErr != nil || original != pDecl {
		t.Fatal("instantiated original mapping", original, getErr)
	}

	count, err := set.Len()
	coneMust(t, err)
	if count != 2 {
		t.Fatal("transformed rule count", count)
	}
	cellPredicates, foundSelectEquality := 0, false
	for i := 0; i < count; i++ {
		rule, getErr := set.Rule(i)
		coneMust(t, getErr)
		tails, getErr := rule.TailSize()
		coneMust(t, getErr)
		for j := 0; j < tails; j++ {
			tail, _, tailErr := rule.Tail(j)
			coneMust(t, tailErr)
			declaration, tailErr := manager.GetDecl(tail)
			if tailErr == nil && declaration == pFresh {
				cellPredicates++
			}
			family, familyErr := manager.GetFamilyID(tail)
			if familyErr == nil && family == manager.BasicFamilyID() {
				op, opErr := manager.GetDeclKind(tail)
				if opErr == nil && (op == ast.ASTOpEq || op == ast.ASTOpIff) {
					foundSelectEquality = true
				}
			}
		}
		coneMust(t, rule.Close())
	}
	// The native chosen-vector traversal does not reset exhausted lower
	// positions: two quantifier slots over two reads produce three, not four,
	// predicate instances. Preserve that actual source behavior.
	if cellPredicates != 3 || !foundSelectEquality {
		t.Fatal("cell predicate/select definitions", cellPredicates, foundSelectEquality)
	}
}

func TestArrayInstantiationKeepsArrayAndCreatesFallbackIndex(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	arrayVariable, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	predicate := declareTransformPredicate(t, f, "array-instantiation-fallback", []ast.ASTHandle{arraySort}, []ast.ASTHandle{arrayVariable})
	addTransformRule(t, f, predicate.application, []ast.ASTHandle{predicate.application}, nil, "fallback")
	coneMust(t, f.set.SetOutputPredicate(predicate.declaration))

	plugin := transforms.NewArrayInstantiationPlugin(f.manager, f.rules, 1, false, 34999)
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	if result == nil {
		t.Fatal("non-enforced array instantiation returned no transformed rules")
	}
	t.Cleanup(func() { coneMust(t, result.Close()) })
	fresh := plugin.FreshPredicates()[predicate.declaration]
	if fresh.IsNull() {
		t.Fatal("missing transformed predicate")
	}
	if arity, getErr := f.manager.GetArity(fresh); getErr != nil || arity != 3 {
		t.Fatal("fallback predicate arity", arity, getErr)
	}
	if retained, getErr := f.manager.GetDomain(fresh, 1); getErr != nil || retained != arraySort {
		t.Fatal("non-enforced transform did not retain the array argument", retained, getErr)
	}
	rule, err := result.Rule(0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rule.Close()) })
	if tails, getErr := rule.TailSize(); getErr != nil || tails != 2 {
		t.Fatal("fallback transform should emit a predicate and select definition", tails, getErr)
	}
}

func TestArrayInstantiationRejectsNegatedPredicateTail(t *testing.T) {
	f := newConeFixture(t, "P", "Q")
	addTransformRule(t, f, f.applications["P"], []ast.ASTHandle{f.applications["Q"]}, []bool{true}, "negative")
	plugin := transforms.NewArrayInstantiationPlugin(f.manager, f.rules, 1, true, 34999)
	result, err := plugin.Transform(f.set)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	if result != nil {
		coneMust(t, result.Close())
		t.Fatal("array instantiation accepted a negated predicate tail")
	}
	if err == nil || !strings.Contains(err.Error(), "does not accept negated predicate tails") {
		t.Fatal("negated-tail error", err)
	}
}
