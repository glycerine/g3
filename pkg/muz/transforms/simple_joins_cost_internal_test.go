package transforms

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSimpleJoinPairCostCountsRepeatedVariablePositions(t *testing.T) {
	budget := arena.NewBudget(1 << 30)
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
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
		if err := numerals.Close(); err != nil {
			t.Error(err)
		}
		if err := symbols.Close(); err != nil {
			t.Error(err)
		}
	})
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rules.Close(); err != nil {
			t.Error(err)
		}
	})
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	leftName, err := manager.Symbols.Intern("simple-cost-repeated-left")
	if err != nil {
		t.Fatal(err)
	}
	leftDecl, err := manager.MkFuncDecl(leftName, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	rightName, err := manager.Symbols.Intern("simple-cost-repeated-right")
	if err != nil {
		t.Fatal(err)
	}
	rightDecl, err := manager.MkFuncDecl(rightName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	variable, err := manager.MkVar(0, boolean)
	if err != nil {
		t.Fatal(err)
	}
	left, err := manager.MkApp(leftDecl, []ast.ASTHandle{variable, variable})
	if err != nil {
		t.Fatal(err)
	}
	right, err := manager.MkApp(rightDecl, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	plugin := NewSimpleJoinsPlugin(manager, rules, 1)
	got, err := plugin.estimatedPairCost([]ast.ASTHandle{left, right}, []ast.ASTHandle{left}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Source positional accounting matches both repeated left positions against
	// the right occurrence: 4*2/(2*2)=2. Set-based accounting yields 4.
	if math.Abs(float64(got-2)) > 1e-6 {
		t.Fatalf("repeated-position pair cost = %g, want 2", got)
	}
	if err := plugin.Close(); err != nil {
		t.Fatal(err)
	}
}
