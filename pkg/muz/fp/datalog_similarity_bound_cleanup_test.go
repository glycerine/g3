package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBoundSimilarityQueryCleanupRefusalKeepsProgramAndInvalidatesResult(t *testing.T) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 28)
	var owner *boundDatalogRuntime
	var query ast.ASTHandle
	var refused error
	var refusedStatus lbool.LBool
	active, fired := false, false
	native, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			// A query after disabling similarity must first retire its former
			// evaluator, and may not expose that old result if cleanup refuses.
			refusedStatus, refused = owner.query([]ast.ASTHandle{query})
		}
		return failure
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: native, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(native.Close())
		if budget.Used() != 0 {
			t.Fatal("bound similarity cleanup leaked bytes", budget.Used())
		}
	}()
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	declare := func(text string) ast.ASTHandle {
		name, err := symbols.Intern(text)
		must(err)
		declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("bound-similarity-cleanup-P")
	query = declare("bound-similarity-cleanup-Q")
	for _, text := range []string{"0", "2", "4"} {
		value, err := m.ArithmeticIntegerNumeralText(text)
		must(err)
		head, err := m.MkApp(p, []ast.ASTHandle{value})
		must(err)
		rule, err := rules.NewRule(head, nil, nil, "source-fact")
		must(err)
		must(set.Add(rule))
		must(rule.Close())
	}
	x, err := m.MkVar(0, integer)
	must(err)
	body, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	head, err := m.MkApp(query, []ast.ASTHandle{x})
	must(err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{body}, []bool{false}, "source-step")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	owner, err = newBoundDatalogRuntime(m, set, []ast.ASTHandle{p, query})
	must(err)
	defer func() { must(owner.close()) }()
	status, err := owner.queryWithSimilarity([]ast.ASTHandle{query}, 2)
	must(err)
	if status != lbool.Undef || owner.similarity == nil || !owner.similarity.Modified() {
		t.Fatal("fixture did not construct compressed bound program", status)
	}
	program, evaluator := owner.similarity, owner.similarityRuntime
	answer, err := owner.relationFormula(query)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	number, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&number)) }()
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(native.SetMaxCount(uint64(native.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, cause := m.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(native.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if cause == nil || !fired || !errors.Is(refused, ast.ErrASTPending) || refusedStatus != lbool.Undef {
		t.Fatal("query did not reach native cleanup refusal", cause, fired, refused, refusedStatus)
	}
	if owner.similarity != program || owner.similarityRuntime != evaluator {
		t.Fatal("refused cleanup lost its program or borrowing evaluator")
	}
	if _, err := owner.relationFormula(query); !errors.Is(err, ErrBoundDatalogFragment) {
		t.Fatal("failed query exposed stale bound snapshot", err)
	}
	status, err = owner.queryWithSimilarity([]ast.ASTHandle{query}, 2)
	must(err)
	if status != lbool.Undef || owner.similarity == program || owner.similarityRuntime == evaluator {
		t.Fatal("query did not retire and replace its former owners", status)
	}
	if _, err := m.RefGet(answer); err != nil {
		t.Fatal("independent answer died with retired similarity owners", err)
	}
	if count, err := set.Len(); err != nil || count != 4 {
		t.Fatal("cleanup retry changed original rules", count, err)
	}
}
