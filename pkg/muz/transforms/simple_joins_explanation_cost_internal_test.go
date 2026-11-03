package transforms

import (
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSimpleJoinsUsesExplanationRelationSizeEstimate(t *testing.T) {
	budget := arena.NewBudget(1 << 28)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		simpleJoinCostCleanup(t, manager.Close())
		simpleJoinCostCleanup(t, numerals.Close())
		simpleJoinCostCleanup(t, symbols.Close())
	})
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { simpleJoinCostCleanup(t, rules.Close()) })
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { simpleJoinCostCleanup(t, relations.Close()) })
	ruleSort, err := manager.MkDatalogRuleSort()
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	plugin := NewSimpleJoinsPlugin(manager, rules, 0, relations)
	t.Cleanup(func() { simpleJoinCostCleanup(t, plugin.Close()) })

	makePredicate := func(name string, domain []ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
		symbol, makeErr := manager.Symbols.Intern(name)
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		declaration, makeErr := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		arguments := make([]ast.ASTHandle, len(domain))
		for index, sort := range domain {
			arguments[index], makeErr = manager.MkVar(uint32(index), sort)
			if makeErr != nil {
				t.Fatal(makeErr)
			}
		}
		application, makeErr := manager.MkApp(declaration, arguments)
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		return declaration, application
	}

	perFactDeclaration, perFactTail := makePredicate("simple-cost-per-fact-explanation", []ast.ASTHandle{boolean, ruleSort})
	perFact, err := rel.NewFiniteExplanationProduct(relations, []ast.ASTHandle{boolean})
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsehood, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	values := []ast.ASTHandle{truth, falsehood}
	for index := 0; index < 2; index++ {
		name, makeErr := manager.Symbols.Intern(fmt.Sprintf("simple-cost-proof-%d", index))
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		proof, makeErr := manager.MkDatalogRule(name, nil)
		if makeErr != nil {
			t.Fatal(makeErr)
		}
		if makeErr = perFact.AddFact([]ast.ASTHandle{values[index]}, proof); makeErr != nil {
			t.Fatal(makeErr)
		}
	}
	if err = relations.StoreExplanationRelation(perFactDeclaration, perFact); err != nil {
		t.Fatal(err)
	}
	if err = relations.MarkSaturated(perFactDeclaration); err != nil {
		t.Fatal(err)
	}
	cost, _, err := plugin.tailCost(perFactTail)
	if err != nil || cost != 2 {
		t.Fatalf("per-fact explanation cost = %g, %v, want 2", cost, err)
	}

	wholeDeclaration, wholeTail := makePredicate("simple-cost-whole-explanation", []ast.ASTHandle{ruleSort})
	whole, err := rel.NewRelationExplanationTotal(relations, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = relations.StoreRelationExplanation(wholeDeclaration, whole); err != nil {
		t.Fatal(err)
	}
	if err = relations.MarkSaturated(wholeDeclaration); err != nil {
		t.Fatal(err)
	}
	cost, _, err = plugin.tailCost(wholeTail)
	if err != nil || cost != float32(math.MaxUint32) {
		t.Fatalf("whole-relation explanation cost = %g, %v, want %g", cost, err, float32(math.MaxUint32))
	}
}

func simpleJoinCostCleanup(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Error(err)
	}
}
