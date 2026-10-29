package transforms_test

// Source: src/muz/transforms/dl_mk_separate_negated_tails.cpp/.h.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func separateDeclaration(t *testing.T, f *coneFixture, name string, domain []ast.ASTHandle, boolean ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := f.manager.Symbols.Intern("separate-" + name)
	coneMust(t, err)
	declaration, err := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(declaration))
	return declaration
}

func TestSeparateNegatedTailsAbstractsOnlyPrivateVariables(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pDecl := separateDeclaration(t, f, "P", []ast.ASTHandle{integer}, boolean)
	qDecl := separateDeclaration(t, f, "Q", []ast.ASTHandle{integer, integer}, boolean)
	rDecl := separateDeclaration(t, f, "R", []ast.ASTHandle{integer, integer}, boolean)
	tDecl := separateDeclaration(t, f, "T", []ast.ASTHandle{integer, integer}, boolean)
	variables := make([]ast.ASTHandle, 4)
	for index := range variables {
		variables[index], err = f.manager.MkVar(uint32(index), integer)
		coneMust(t, err)
	}
	application := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, makeErr := f.manager.MkApp(declaration, arguments)
		coneMust(t, makeErr)
		return result
	}
	head := application(pDecl, variables[0])
	q := application(qDecl, variables[0], variables[1])
	r := application(rDecl, variables[1], variables[2])
	tailT := application(tDecl, variables[2], variables[3])
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{q, r, tailT}, []bool{false, true, true}, "private-tail")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(pDecl))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("source rule set is not stratified")
	}
	plugin := transforms.NewSeparateNegatedTailsPlugin(f.manager, f.rules, 21000)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("private negated variable was not abstracted")
	}
	if output, outputErr := f.set.OutputPredicate(); outputErr != nil || output != pDecl {
		t.Fatal("output predicate was not inherited", output, outputErr)
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 2 {
		t.Fatalf("rewritten rule count %d, want 2", count)
	}
	var fresh ast.ASTHandle
	for index := 0; index < count; index++ {
		current, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		declaration, getErr := current.HeadDeclaration()
		if getErr == nil && declaration == pDecl {
			positive, sizeErr := current.PositiveTailSize()
			uninterpreted, uninterpretedErr := current.UninterpretedTailSize()
			getErr = errors.Join(getErr, sizeErr, uninterpretedErr)
			if getErr == nil && (positive != 2 || uninterpreted != 3) {
				t.Fatalf("rewritten partition positive=%d uninterpreted=%d", positive, uninterpreted)
			}
			seen := map[ast.ASTHandle]bool{}
			for tailIndex := 0; getErr == nil && tailIndex < uninterpreted; tailIndex++ {
				var tailDecl ast.ASTHandle
				tailDecl, getErr = current.TailDeclaration(tailIndex)
				if getErr == nil {
					seen[tailDecl] = true
				}
			}
			if getErr == nil && (seen[tDecl] || !seen[qDecl] || !seen[rDecl] || len(seen) != 3) {
				t.Fatalf("rewritten tail declarations %#v", seen)
			}
			if getErr == nil {
				for declaration := range seen {
					if declaration != qDecl && declaration != rDecl {
						fresh = declaration
					}
				}
			}
		} else if getErr == nil {
			fresh = declaration
			positive, sizeErr := current.PositiveTailSize()
			uninterpreted, uninterpretedErr := current.UninterpretedTailSize()
			getErr = errors.Join(sizeErr, uninterpretedErr)
			if getErr == nil && (positive != 0 || uninterpreted != 1) {
				t.Fatalf("definition partition positive=%d uninterpreted=%d", positive, uninterpreted)
			}
			if getErr == nil {
				var definitionTail ast.ASTHandle
				definitionTail, getErr = current.TailDeclaration(0)
				if getErr == nil && definitionTail != tDecl {
					t.Fatal("definition does not negate the original predicate", definitionTail, tDecl)
				}
			}
		}
		getErr = errors.Join(getErr, current.Close())
		coneMust(t, getErr)
	}
	if fresh.IsNull() {
		t.Fatal("fresh abstraction predicate not found")
	}
	if arity, arityErr := f.manager.GetArity(fresh); arityErr != nil || arity != 1 {
		t.Fatal("fresh predicate should retain only shared z", arity, arityErr)
	}
}

func TestSeparateNegatedTailsReturnsNoChangeWithoutPrivateVariables(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pDecl := separateDeclaration(t, f, "no-change-P", []ast.ASTHandle{integer}, boolean)
	qDecl := separateDeclaration(t, f, "no-change-Q", []ast.ASTHandle{integer, integer}, boolean)
	rDecl := separateDeclaration(t, f, "no-change-R", []ast.ASTHandle{integer, integer}, boolean)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	head, err := f.manager.MkApp(pDecl, []ast.ASTHandle{x})
	coneMust(t, err)
	q, err := f.manager.MkApp(qDecl, []ast.ASTHandle{x, y})
	coneMust(t, err)
	r, err := f.manager.MkApp(rDecl, []ast.ASTHandle{y, x})
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{q, r}, []bool{false, true}, "no-private-tail")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("source rule set is not stratified")
	}
	plugin := transforms.NewSeparateNegatedTailsPlugin(f.manager, f.rules, 21000)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if modified {
		t.Fatal("rule without private negated variables was rewritten")
	}
	if count, countErr := f.set.Len(); countErr != nil || count != 1 {
		t.Fatal("unchanged rule count", count, countErr)
	}
}
