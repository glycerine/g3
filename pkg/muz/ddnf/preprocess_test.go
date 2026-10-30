package ddnf

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type preprocessFixture struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	registry *Registry
	budget   *arena.Budget
	width    ast.ASTHandle
	x        ast.ASTHandle
	head     ast.ASTHandle
}

func newPreprocessFixture(t *testing.T) *preprocessFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	width, err := manager.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	x, err := manager.MkVar(0, width)
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	rules, err := base.NewRuleManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rules.Close(); err != nil {
			t.Error(err)
		}
	})
	name, err := manager.Symbols.Intern("ddnf-preprocess-head")
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{width}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = rules.RegisterPredicate(declaration); err != nil {
		t.Fatal(err)
	}
	head, err := manager.MkApp(declaration, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	budget := arena.NewBudget(1 << 20)
	registry := NewRegistry(arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: budget}})
	t.Cleanup(func() {
		registry.Close()
		if used := budget.Used(); used != 0 {
			t.Errorf("DDNF registry retained %d bytes", used)
		}
	})
	return &preprocessFixture{manager: manager, rules: rules, registry: registry, budget: budget, width: width, x: x, head: head}
}

func (f *preprocessFixture) rule(t *testing.T, tails ...ast.ASTHandle) *base.Rule {
	t.Helper()
	rule, err := f.rules.NewRule(f.head, tails, nil, "ddnf-preprocess")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rule.Close(); err != nil {
			t.Error(err)
		}
	})
	return rule
}

func ddnfBVNumeral(t *testing.T, manager *ast.ASTManager, value uint64, width uint32) ast.ASTHandle {
	t.Helper()
	service := manager.NumeralService()
	rational, err := service.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := service.Del(&rational); err != nil {
			t.Error(err)
		}
	}()
	if err = service.SetUint64(&rational, value); err != nil {
		t.Fatal(err)
	}
	result, err := manager.BitVectorNumeral(&rational, width)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestDDNFPreprocessorRecordsFullAndExtractEqualities(t *testing.T) {
	f := newPreprocessFixture(t)
	preprocessor, err := NewPreprocessor(f.manager, f.registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := preprocessor.Close(); err != nil {
			t.Error(err)
		}
	})
	one := ddnfBVNumeral(t, f.manager, 1, 4)
	full, err := f.manager.MkEq(f.x, one)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.ProcessRule(f.rule(t, full)); err != nil || !ok {
		t.Fatal("full bit-vector equality should be supported", ok, err)
	}
	reversed, err := f.manager.MkEq(one, f.x)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.ProcessRule(f.rule(t, reversed)); err != nil || !ok {
		t.Fatal("reversed full bit-vector equality should be supported", ok, err)
	}
	fullVector, found, err := preprocessor.Constraint(full)
	if err != nil || !found {
		t.Fatal("full equality constraint", found, err)
	}
	if reversedVector, found, err := preprocessor.Constraint(reversed); err != nil || !found || reversedVector != fullVector {
		t.Fatal("reversed equality vector", reversedVector, found, err)
	}
	core, err := f.registry.Get(4)
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []rel.TBit{rel.TBit1, rel.TBit0, rel.TBit0, rel.TBit0} {
		got, e := core.Manager().Get(fullVector, uint32(index))
		if e != nil || got != want {
			t.Fatalf("full equality bit %d = %v, want %v: %v", index, got, want, e)
		}
	}
	part, err := f.manager.MkBitVectorExtract(2, 1, f.x)
	if err != nil {
		t.Fatal(err)
	}
	two := ddnfBVNumeral(t, f.manager, 2, 2)
	partial, err := f.manager.MkEq(part, two)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.ProcessRule(f.rule(t, partial)); err != nil || !ok {
		t.Fatal("bit-vector extract equality should be supported", ok, err)
	}
	partialVector, found, err := preprocessor.Constraint(partial)
	if err != nil || !found {
		t.Fatal("extract equality constraint", found, err)
	}
	for index, want := range []rel.TBit{rel.TBitX, rel.TBit0, rel.TBit1, rel.TBitX} {
		got, e := core.Manager().Get(partialVector, uint32(index))
		if e != nil || got != want {
			t.Fatalf("extract equality bit %d = %v, want %v: %v", index, got, want, e)
		}
	}
}

func TestDDNFPreprocessorAcceptsVariableEqualityWithoutConstraint(t *testing.T) {
	f := newPreprocessFixture(t)
	preprocessor, err := NewPreprocessor(f.manager, f.registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preprocessor.Close() })
	y, err := f.manager.MkVar(1, f.width)
	if err != nil {
		t.Fatal(err)
	}
	equality, err := f.manager.MkEq(f.x, y)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.ProcessRule(f.rule(t, equality)); err != nil || !ok {
		t.Fatal("variable-to-variable bit-vector equality should be supported", ok, err)
	}
	if _, found, err := preprocessor.Constraint(equality); err != nil || found {
		t.Fatal("variable equality should not create a constant constraint", found, err)
	}
	if widths := f.registry.Widths(); len(widths) != 0 {
		t.Fatal("variable equality should not allocate a width core", widths)
	}
}

func TestDDNFPreprocessorTraversesRuleSetAndResetsQueryState(t *testing.T) {
	f := newPreprocessFixture(t)
	preprocessor, err := NewPreprocessor(f.manager, f.registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preprocessor.Close() })
	set, err := base.NewRuleSet(f.rules)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := set.Close(); err != nil {
			t.Error(err)
		}
	})
	value := ddnfBVNumeral(t, f.manager, 3, 4)
	equality, err := f.manager.MkEq(f.x, value)
	if err != nil {
		t.Fatal(err)
	}
	if err = set.Add(f.rule(t, equality)); err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.PreprocessRules(set); err != nil || !ok {
		t.Fatal("supported rule set should preprocess", ok, err)
	}
	if _, found, err := preprocessor.Constraint(equality); err != nil || !found {
		t.Fatal("rule-set traversal did not retain the equality vector", found, err)
	}
	if ok, err := preprocessor.PreprocessRules(set); err != nil || !ok {
		t.Fatal("repeat query should reset traversal state", ok, err)
	}
}

func TestDDNFPreprocessorRejectsUnsupportedAtom(t *testing.T) {
	f := newPreprocessFixture(t)
	preprocessor, err := NewPreprocessor(f.manager, f.registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preprocessor.Close() })
	name, err := f.manager.Symbols.Intern("ddnf-preprocess-opaque")
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{f.width}, f.width, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	application, err := f.manager.MkApp(declaration, []ast.ASTHandle{f.x})
	if err != nil {
		t.Fatal(err)
	}
	one := ddnfBVNumeral(t, f.manager, 1, 4)
	unsupported, err := f.manager.MkEq(application, one)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.ProcessRule(f.rule(t, unsupported)); err != nil || ok {
		t.Fatal("non-variable, non-extract equality should be rejected", ok, err)
	}
	if size := f.registry.Widths(); len(size) != 0 {
		t.Fatal("unsupported atom allocated a width core", size)
	}
}

func TestDDNFPreprocessorRejectsQuantifierReachedThroughBooleanConnective(t *testing.T) {
	f := newPreprocessFixture(t)
	preprocessor, err := NewPreprocessor(f.manager, f.registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = preprocessor.Close() })
	boolean, err := f.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	opaqueName, err := f.manager.Symbols.Intern("ddnf-quantifier-opaque")
	if err != nil {
		t.Fatal(err)
	}
	opaqueDecl, err := f.manager.MkFuncDecl(opaqueName, []ast.ASTHandle{f.width}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	opaque, err := f.manager.MkApp(opaqueDecl, []ast.ASTHandle{f.x})
	if err != nil {
		t.Fatal(err)
	}
	boundName, err := f.manager.Symbols.Intern("ddnf-quantifier-bound")
	if err != nil {
		t.Fatal(err)
	}
	quantified, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{boundName}, Body: opaque,
	})
	if err != nil {
		t.Fatal(err)
	}
	truth, err := f.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := f.manager.MkIff(quantified, truth)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := preprocessor.ProcessRule(f.rule(t, wrapped)); err != nil || ok {
		t.Fatal("reachable quantifier should be rejected", ok, err)
	}
}
