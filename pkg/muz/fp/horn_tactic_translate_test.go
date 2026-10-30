package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestHornTacticTranslateRequiresSharedServices(t *testing.T) {
	f := newHornLifecycleFixture(t, true)
	other, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: f.manager.Symbols, Numerals: f.manager.NumeralService(), Budget: f.manager.Budget()})
	fpMust(t, err)
	fpMust(t, other.RegisterDeclarationPlugins())
	t.Cleanup(func() { fpMust(t, other.Close()) })
	translated, err := f.tactic.Translate(other)
	fpMust(t, err)
	fpMust(t, translated.IncRef())
	fpMust(t, translated.DecRef())
}
