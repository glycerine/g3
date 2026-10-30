package transforms

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
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
	var traceOutput bytes.Buffer
	traceTags := diagnostic.NewTraceTagsWithWriter(&traceOutput)
	traceTags.Enable("report_costs")
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, TraceTags: traceTags})
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
	if err = plugin.tracePairCost(left, right, got); err != nil {
		t.Fatal(err)
	}
	if traceTags.ShouldTrace("report_costs") {
		output := traceOutput.String()
		for _, want := range []string{"simple-cost-repeated-left(#0,#0)", "simple-cost-repeated-right(#0)", "2\n"} {
			if !strings.Contains(output, want) {
				t.Fatalf("report_costs output %q is missing %q", output, want)
			}
		}
	} else if traceOutput.Len() != 0 {
		t.Fatalf("disabled report_costs emitted output: %q", traceOutput.String())
	}
	if err := plugin.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSimpleJoinPairRegistrationHistoryKeepsSourceOrder(t *testing.T) {
	key := normalizedPairKey{}
	first, second := &simpleJoinRuleState{}, &simpleJoinRuleState{}
	history := map[normalizedPairKey][]*simpleJoinRuleState{key: {first, second}}
	updateSimpleJoinRegistrationHistory(history, first, map[normalizedPairKey]int{key: 1}, map[normalizedPairKey]int{key: 2})
	registrations := history[key]
	if len(registrations) != 3 || registrations[0] != first || registrations[1] != second || registrations[2] != first {
		t.Fatal("dynamic pair registrations do not preserve append order")
	}
	updateSimpleJoinRegistrationHistory(history, first, map[normalizedPairKey]int{key: 2}, map[normalizedPairKey]int{key: 1})
	registrations = history[key]
	if len(registrations) != 2 || registrations[0] != second || registrations[1] != first {
		t.Fatal("removed pair registration does not delete the first matching rule")
	}
}
