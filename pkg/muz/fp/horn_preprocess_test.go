package fp

// Source: src/muz/transforms/dl_transforms.cpp transformed-rule ownership.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornPreprocessResultOwnsRulesAndClearsMappings(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	quantifierResultMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	quantifierResultMust(t, err)
	t.Cleanup(func() {
		quantifierResultMust(t, numerals.Close())
		quantifierResultMust(t, symbols.Close())
		if used := budget.Used(); used != 0 {
			t.Errorf("owner graph retained %d bytes", used)
		}
	})
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	quantifierResultMust(t, err)
	quantifierResultMust(t, manager.RegisterDeclarationPlugins())
	t.Cleanup(func() { quantifierResultMust(t, manager.Close()) })
	rules, err := base.NewRuleManager(manager)
	quantifierResultMust(t, err)
	t.Cleanup(func() { quantifierResultMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	quantifierResultMust(t, err)
	result := &hornPreprocessResult{
		rules:                  set,
		slicedPredicates:       map[ast.ASTHandle]ast.ASTHandle{{}: {}},
		slicedColumns:          map[ast.ASTHandle][]bool{{}: {true}},
		quantifiedPredicates:   make(map[ast.ASTHandle]ast.ASTHandle),
		instantiatedPredicates: make(map[ast.ASTHandle]ast.ASTHandle),
		bitBlastedPredicates:   make(map[ast.ASTHandle]ast.ASTHandle),
		scaledPredicates:       make(map[ast.ASTHandle]ast.ASTHandle),
		magicPredicates:        make(map[ast.ASTHandle]ast.ASTHandle),
	}
	quantifierResultMust(t, result.Close())
	quantifierResultMust(t, result.Close())
	if !result.closed || result.rules != nil || result.slicedPredicates != nil || result.slicedColumns != nil ||
		result.quantifiedPredicates != nil || result.instantiatedPredicates != nil || result.bitBlastedPredicates != nil ||
		result.scaledPredicates != nil || result.magicPredicates != nil {
		t.Fatal("Horn preprocessing result retained closed state")
	}
	if _, err = set.Len(); err == nil {
		t.Fatal("Horn preprocessing result did not close its rule set")
	}
}
