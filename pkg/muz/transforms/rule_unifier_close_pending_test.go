package transforms

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp rule_unifier ownership.
// Its Go converter owners must remain reachable when native cleanup is refused.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleUnifierClosePreservesSubstitutionOwners(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *RuleUnifier
	active, callbacks, intact := false, 0, true
	var original *substitution.Substitution
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && plugin.manager != nil && plugin.subst == original
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	n, err := symbols.Intern("pending-unifier-P")
	must(err)
	d, err := m.MkFuncDecl(n, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(d))
	x, err := m.MkVar(0, integer)
	must(err)
	px, err := m.MkApp(d, []ast.ASTHandle{x})
	must(err)
	pzero, err := m.MkApp(d, []ast.ASTHandle{zero})
	must(err)
	n, err = symbols.Intern("pending-unifier-Q")
	must(err)
	d, err = m.MkFuncDecl(n, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(d))
	qx, err := m.MkApp(d, []ast.ASTHandle{x})
	must(err)
	qzero, err := m.MkApp(d, []ast.ASTHandle{zero})
	must(err)
	target, err := rules.NewRule(qx, []ast.ASTHandle{px}, nil, "target")
	must(err)
	defer func() { must(target.Close()) }()
	source, err := rules.NewRule(pzero, nil, nil, "source")
	must(err)
	defer func() { must(source.Close()) }()
	plugin, err = NewRuleUnifier(m)
	must(err)
	defer func() {
		if plugin.manager != nil {
			must(plugin.Close())
		}
	}()
	ok, err := plugin.UnifyRules(target, 0, source)
	must(err)
	if !ok {
		t.Fatal("fixture did not unify")
	}
	resolved, err := plugin.ResolveRaw(rules, target, 0, source)
	must(err)
	defer func() { must(resolved.Close()) }()
	h, err := resolved.Head()
	must(err)
	if count, e := resolved.TailSize(); e != nil || count != 0 || h != qzero {
		t.Fatal("resolution did not produce Q(0)", count, e)
	}
	original = plugin.subst
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused close lost rule substitution", body, callbacks, intact)
	}
	if ok, cause := plugin.UnifyRules(target, 0, source); ok || !errors.Is(cause, ast.ErrASTHandle) {
		t.Fatal("closing unifier accepted unification", cause)
	}
	if refs, cause := plugin.RuleSubstitution(target, true); refs != nil || !errors.Is(cause, ast.ErrASTHandle) {
		t.Fatal("closing unifier published substitution", cause)
	}
	if next, cause := plugin.ResolveRaw(rules, target, 0, source); next != nil || !errors.Is(cause, ast.ErrASTHandle) {
		t.Fatal("closing unifier accepted resolution", cause)
	}
	must(plugin.Close())
	if cause := plugin.Close(); !errors.Is(cause, ast.ErrASTHandle) {
		t.Fatal("closed-owner guard changed", cause)
	}
	if plugin.manager != nil || plugin.subst != nil || plugin.unifier != nil {
		t.Fatal("retry did not release unifier children")
	}
}
