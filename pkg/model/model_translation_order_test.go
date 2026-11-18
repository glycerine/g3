package model_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// AUDIT.md model_01-F7: model::translate's register_decl(translator(key),
// translator(value)) translates the value first in the release build, so
// the value's declarations get lower ids in the target than the key's.
func TestModelTranslateCreationOrder(t *testing.T) {
	from := astTestManager(t)
	to, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: from.Symbols, Numerals: from.NumeralService(), Budget: from.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, to.Close()) })
	translation, err := ast.NewASTTranslation(from, to, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, translation.Close()) })
	source, err := model.NewModel(from)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	key, err := from.GetDecl(goalTestAtom(t, from, "order-key"))
	astTestMust(t, err)
	f := modelTestFunctionDecl(t, from, "order-f")
	a := goalTestAtom(t, from, "order-a")
	value, err := from.MkApp(f, []ast.ASTHandle{a})
	astTestMust(t, err)
	astTestMust(t, source.Core().RegisterConstant(key, value))
	translated, err := source.Translate(translation)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	tf, err := translation.Translate(f)
	astTestMust(t, err)
	tkey, err := translation.Translate(key)
	astTestMust(t, err)
	fID, err := to.GetASTID(tf)
	astTestMust(t, err)
	keyID, err := to.GetASTID(tkey)
	astTestMust(t, err)
	if fID >= keyID {
		t.Fatalf("value declaration id %d not below key declaration id %d", fID, keyID)
	}
}
