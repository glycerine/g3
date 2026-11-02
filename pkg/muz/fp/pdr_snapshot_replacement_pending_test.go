package fp

// Source: pdr_dl_interface::check_reset/m_old_rules replacement ownership.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRSnapshotReplacementPublishesOnlyAfterNativeCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var engine *PropositionalEngine
	var nextRules, nextSource *base.RuleSet
	active, callback, intact := false, false, true
	var closeErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			oldRules, oldSource := engine.pdrRules, engine.pdrSourceRules
			closeErr = engine.replacePDRRuleSnapshots(nextRules, nextSource)
			intact = errors.Is(closeErr, ast.ErrASTPending) && engine.pdrRules == oldRules && engine.pdrSourceRules == oldSource
			if _, err := nextRules.Len(); err != nil {
				intact = false
			}
			if _, err := nextSource.Len(); err != nil {
				intact = false
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	rules, err := base.NewRuleManager(manager)
	must(err)
	var oldRules, oldSource *base.RuleSet
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		for _, set := range []*base.RuleSet{oldRules, oldSource, nextRules, nextSource} {
			if set != nil {
				must(set.Close())
			}
		}
		must(rules.Close())
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("PDR snapshot replacement fixture leaked bytes", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("pending-pdr-snapshot")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(predicate))
	application, err := manager.MkApp(predicate, nil)
	must(err)
	rule, err := rules.NewRule(application, nil, nil, "pending-snapshot-fact")
	must(err)
	makeSet := func() *base.RuleSet {
		set, cause := base.NewRuleSet(rules)
		must(cause)
		must(set.Add(rule))
		must(set.SetOutputPredicate(predicate))
		return set
	}
	oldRules, oldSource, nextRules, nextSource = makeSet(), makeSet(), makeSet(), makeSet()
	must(rule.Close())
	engine = &PropositionalEngine{pdrRules: oldRules, pdrSourceRules: oldSource}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !intact || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("refused snapshot replacement lost an owner", body, callback, intact, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(engine.replacePDRRuleSnapshots(nextRules, nextSource))
	must(engine.replacePDRRuleSnapshots(nextRules, nextSource))
	if engine.pdrRules != nextRules || engine.pdrSourceRules != nextSource {
		t.Fatal("snapshot replacement retry did not publish new owners")
	}
	if _, err := oldRules.Len(); err == nil {
		t.Fatal("superseded transformed snapshot remained open")
	}
	if _, err := oldSource.Len(); err == nil {
		t.Fatal("superseded source snapshot remained open")
	}
}
