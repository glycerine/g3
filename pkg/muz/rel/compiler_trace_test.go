package rel_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestFiniteCompilerStratumTrace(t *testing.T) {
	budget := arena.NewBudget(1 << 30)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = symbols.Close() })
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = numerals.Close() })
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("dl")
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, TraceTags: tags})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close() })
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rules.Close() })
	set, err := base.NewRuleSet(rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = set.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := symbols.Intern("trace-stratum")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = rules.RegisterPredicate(predicate); err != nil {
		t.Fatal(err)
	}
	fact, err := manager.MkApp(predicate, nil)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := rules.NewRule(fact, nil, nil, "trace-fact")
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(rule); err != nil {
		t.Fatal(err)
	}
	if err = rule.Close(); err != nil {
		t.Fatal(err)
	}
	if valid, x := set.Finalize(); x != nil || !valid {
		t.Fatal("finalize", valid, x)
	}
	execution, termination, err := rel.CompileFiniteRules(relations, set)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = execution.Close(); _ = termination.Close() })
	if tags.ShouldTrace("dl") && !strings.Contains(output.String(), "Stratum: trace-stratum ") {
		t.Fatalf("stratum trace missing source predicate name: %q", output.String())
	}
}
