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
	traceTags.Enable("dl")
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
	if err = rules.RegisterPredicate(leftDecl); err != nil {
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
	if err = rules.RegisterPredicate(rightDecl); err != nil {
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
	makeUnary := func(name string) ast.ASTHandle {
		t.Helper()
		declName, x := manager.Symbols.Intern("simple-cost-repeated-" + name)
		if x != nil {
			t.Fatal(x)
		}
		decl, x := manager.MkFuncDecl(declName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		if x != nil {
			t.Fatal(x)
		}
		if x = rules.RegisterPredicate(decl); x != nil {
			t.Fatal(x)
		}
		application, x := manager.MkApp(decl, []ast.ASTHandle{variable})
		if x != nil {
			t.Fatal(x)
		}
		return application
	}
	c, d := makeUnary("c"), makeUnary("d")
	leftPin, err := manager.NewRef(left)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.DelRef(&leftPin); err != nil {
			t.Error(err)
		}
	}()
	rightPin, err := manager.NewRef(right)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.DelRef(&rightPin); err != nil {
			t.Error(err)
		}
	}()
	cPin, err := manager.NewRef(c)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.DelRef(&cPin); err != nil {
			t.Error(err)
		}
	}()
	dPin, err := manager.NewRef(d)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.DelRef(&dPin); err != nil {
			t.Error(err)
		}
	}()
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
	rule, err := rules.NewRule(left, []ast.ASTHandle{left, right}, nil, "cost-trace-rule")
	if err != nil {
		t.Fatal(err)
	}
	state, err := newSimpleJoinState(plugin, rule)
	if err != nil {
		t.Fatal(err)
	}
	if err = plugin.tracePairRegistration(state, left, right, []ast.ASTHandle{left}, 4, 2, true); err != nil {
		t.Fatal(err)
	}
	if err = rule.Close(); err != nil {
		t.Fatal(err)
	}
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := set.Close(); err != nil {
			t.Error(err)
		}
	}()
	plannerRule, err := rules.NewRule(left, []ast.ASTHandle{left, right}, nil, "planner-trace-rule")
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(plannerRule); err != nil {
		t.Fatal(err)
	}
	if err = plannerRule.Close(); err != nil {
		t.Fatal(err)
	}
	longRule, err := rules.NewRule(left, []ast.ASTHandle{left, right, c, d}, nil, "planner-long-trace-rule")
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(longRule); err != nil {
		t.Fatal(err)
	}
	if err = longRule.Close(); err != nil {
		t.Fatal(err)
	}
	transformed, err := plugin.Transform(set)
	if err != nil {
		t.Fatal(err)
	}
	if transformed != nil {
		if err = transformed.Close(); err != nil {
			t.Fatal(err)
		}
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
	if traceTags.ShouldTrace("dl") {
		output := traceOutput.String()
		for _, want := range []string{"cost-trace-rule:\nsimple-cost-repeated-left(#0,#0) :- \n simple-cost-repeated-left(#0,#0),\n simple-cost-repeated-right(#0).\n", "cost: 0.125", "planner-trace-rule:\nsimple-cost-repeated-left(#0,#0) :- \n simple-cost-repeated-left(#0,#0),\n simple-cost-repeated-right(#0).\n"} {
			if !strings.Contains(output, want) {
				t.Fatalf("dl registration trace %q is missing %q", output, want)
			}
		}
		if got := strings.Count(output, "simple-cost-repeated-c(#0) simple-cost-repeated-d(#0) 0\n"); got != 1 {
			t.Fatalf("existing pair registration trace count = %d, want one after active graph rebuild", got)
		}
	}
	if err := plugin.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSimpleJoinPairRegistrationHistoryKeepsSourceOrder(t *testing.T) {
	key := normalizedPairKey{}
	first, second := &simpleJoinRuleState{}, &simpleJoinRuleState{}
	history := map[normalizedPairKey][]*simpleJoinRuleState{key: {first, second}}
	traceCounts := map[normalizedPairKey]map[*simpleJoinRuleState]int{key: {first: 1, second: 1}}
	if err := updateSimpleJoinRegistrationHistory(history, traceCounts, first, map[normalizedPairKey]int{key: 1}, map[normalizedPairKey]int{key: 2}); err != nil {
		t.Fatal(err)
	}
	registrations := history[key]
	if len(registrations) != 3 || registrations[0] != first || registrations[1] != second || registrations[2] != first {
		t.Fatal("dynamic pair registrations do not preserve append order")
	}
	if err := updateSimpleJoinRegistrationHistory(history, traceCounts, first, map[normalizedPairKey]int{key: 2}, map[normalizedPairKey]int{key: 1}); err != nil {
		t.Fatal(err)
	}
	registrations = history[key]
	if len(registrations) != 2 || registrations[0] != second || registrations[1] != first {
		t.Fatal("removed pair registration does not delete the first matching rule")
	}
	if err := updateSimpleJoinRegistrationHistory(history, traceCounts, &simpleJoinRuleState{}, map[normalizedPairKey]int{key: 1}, map[normalizedPairKey]int{}); err == nil {
		t.Fatal("missing source pair registration was silently ignored")
	}
}
