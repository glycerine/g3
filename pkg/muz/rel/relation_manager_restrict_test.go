package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationManagerRestrictPredicatesPreservesOnlyScopedOwners(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		d, e := f.ast.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, f.ast, d)
		return d
	}
	keepTable := declare("scope-keep-table", []ast.ASTHandle{f.boolean})
	dropTable := declare("scope-drop-table", []ast.ASTHandle{f.boolean})
	keepExplanation := declare("scope-keep-explanation", []ast.ASTHandle{f.boolean, ruleSort})
	dropExplanation := declare("scope-drop-explanation", []ast.ASTHandle{f.boolean, ruleSort})
	preserved, err := f.relations.GetRelation(keepTable)
	astTestMust(t, err)
	_, err = preserved.Add(Fact{1})
	astTestMust(t, err)
	retired, err := f.relations.GetRelation(dropTable)
	astTestMust(t, err)
	_, err = retired.Add(Fact{0})
	astTestMust(t, err)
	keepProduct, err := f.relations.GetExplanationRelation(keepExplanation)
	astTestMust(t, err)
	dropProduct, err := f.relations.GetExplanationRelation(dropExplanation)
	astTestMust(t, err)
	attach := func(product *FiniteExplanationProduct, name string) ast.ASTHandle {
		symbol, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		term, e := f.ast.MkDatalogRule(symbol, nil)
		astTestMust(t, e)
		root, e := f.ast.NewRef(term)
		astTestMust(t, e)
		astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, term))
		astTestMust(t, f.ast.DelRef(&root))
		return term
	}
	keptTerm := attach(keepProduct, "scope-keep-fact")
	droppedTerm := attach(dropProduct, "scope-drop-fact")
	for _, predicate := range []ast.ASTHandle{keepTable, dropTable, keepExplanation, dropExplanation} {
		astTestMust(t, f.relations.MarkSaturated(predicate))
	}
	keep := map[ast.ASTHandle]bool{keepTable: true, keepExplanation: true}
	astTestMust(t, f.relations.RestrictPredicates(keep))
	astTestMust(t, f.relations.RestrictPredicates(keep))
	table, found, err := f.relations.TryGetRelation(keepTable)
	astTestMust(t, err)
	if !found || table != preserved {
		t.Fatal("kept table owner changed")
	}
	exists, err := table.Contains(Fact{1})
	astTestMust(t, err)
	if !exists {
		t.Fatal("kept table rows changed")
	}
	product, found, err := f.relations.TryGetExplanationRelation(keepExplanation)
	astTestMust(t, err)
	if !found || product != keepProduct {
		t.Fatal("kept explanation owner changed")
	}
	term, found, err := product.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found || term != keptTerm {
		t.Fatal("kept explanation changed", term, found)
	}
	if _, err = retired.Len(); err == nil {
		t.Fatal("removed table remained live")
	}
	if _, err = dropProduct.Len(); err == nil {
		t.Fatal("removed explanation remained live")
	}
	if f.ast.IsLiveHandle(droppedTerm) {
		t.Fatal("removed explanation term remained owned")
	}
	for _, predicate := range []ast.ASTHandle{keepTable, dropTable, keepExplanation, dropExplanation} {
		marked, e := f.relations.IsSaturated(predicate)
		astTestMust(t, e)
		if marked != keep[predicate] {
			t.Fatal("saturation intersection", predicate, marked)
		}
	}
	if table, found, e := f.relations.TryGetRelation(dropTable); e != nil || found || table != nil {
		t.Fatal("removed finite relation still stored", found, e)
	}
	if product, found, e := f.relations.TryGetExplanationRelation(dropExplanation); e != nil || found || product != nil {
		t.Fatal("removed explanation still stored", found, e)
	}
	astTestMust(t, f.relations.RestrictPredicates(nil))
	if _, err = preserved.Len(); err == nil {
		t.Fatal("empty keep set left finite owner live")
	}
	if f.ast.IsLiveHandle(keptTerm) {
		t.Fatal("empty keep set left explanation owner live")
	}
}
