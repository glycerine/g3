package tab_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/tab"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestClauseEqualityReduction(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(session.Close()) }()
	m, err := session.Manager()
	must(err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	must(err)
	defer func() { must(params.Close()) }()
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	must(err)
	defer func() { must(catalog.Close()) }()
	globals, err := uparams.NewGlobalParamsService(m.Symbols, params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	must(err)
	defer func() { must(globals.Close()) }()
	must(globals.InitializeSource())
	rw, err := rewriter.NewTheoryRewriter(m, params, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	value, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	integer, err := m.GetSort(value)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("tab-clause-p")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	eq, err := m.MkEq(x, value)
	must(err)
	c, err := tab.NewClause(m, rw, head, []ast.ASTHandle{head}, eq)
	must(err)
	got, err := c.Head()
	must(err)
	want, err := m.MkApp(p, []ast.ASTHandle{value})
	must(err)
	if got != want {
		t.Fatal("head substitution", got, want)
	}
	predicates, err := c.Predicates()
	must(err)
	if len(predicates) != 1 || predicates[0] != want {
		t.Fatal("predicate substitution", predicates)
	}
	condition, err := c.Constraint()
	must(err)
	if !m.IsTrue(condition) || c.NumVars() != 1 {
		t.Fatal("constraint and original variable count", condition, c.NumVars())
	}
	must(c.Close())

	// The source checks occurs-cycles before committing constructor bindings.
	_, err = m.RegisterDatatypePlugin()
	must(err)
	sym := func(name string) usymbol.Symbol { s, e := m.Symbols.Intern(name); must(e); return s }
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: sym("tab-cycle"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: sym("tab-nil"), Recognizer: sym("is-tab-nil")},
		{Name: sym("tab-cons"), Recognizer: sym("is-tab-cons"), Accessors: []ast.DatatypeAccessorDefinition{{Name: sym("tab-tail"), Recursive: true}}},
	}}})
	must(err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	must(err)
	variable, err := m.MkVar(0, sorts[0])
	must(err)
	term, err := m.MkApp(constructors[1], []ast.ASTHandle{variable})
	must(err)
	predicate, err := m.MkFuncDecl(sym("tab-cycle-p"), []ast.ASTHandle{sorts[0]}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	cyclicHead, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	must(err)
	cyclicEq, err := m.MkEq(variable, term)
	must(err)
	cyclic, err := tab.NewClause(m, rw, cyclicHead, nil, cyclicEq)
	must(err)
	cyclicResult, err := cyclic.Head()
	must(err)
	if cyclicResult != cyclicHead {
		t.Fatal("accepted cyclic binding", cyclicResult, cyclicHead)
	}
	cyclicConstraint, err := cyclic.Constraint()
	must(err)
	if m.IsTrue(cyclicConstraint) {
		t.Fatal("discarded cyclic constraint")
	}
	must(cyclic.Close())
}

func TestClauseDoesNotSubstituteUninterpretedTerms(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(session.Close()) }()
	m, err := session.Manager()
	must(err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	must(err)
	defer func() { must(params.Close()) }()
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	must(err)
	defer func() { must(catalog.Close()) }()
	globals, err := uparams.NewGlobalParamsService(m.Symbols, params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	must(err)
	defer func() { must(globals.Close()) }()
	must(globals.InitializeSource())
	rw, err := rewriter.NewTheoryRewriter(m, params, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	sort, err := m.MkBoolSort()
	must(err)
	x, err := m.MkVar(0, sort)
	must(err)
	name, err := m.Symbols.Intern("tab-clause-symbolic")
	must(err)
	a, err := m.MkConst(name, sort)
	must(err)
	name, err = m.Symbols.Intern("tab-clause-q")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	eq, err := m.MkEq(x, a)
	must(err)
	c, err := tab.NewClause(m, rw, head, nil, eq)
	must(err)
	defer func() { must(c.Close()) }()
	got, err := c.Head()
	must(err)
	if got != head {
		t.Fatal("substituted uninterpreted constant", got, head)
	}
	constraint, err := c.Constraint()
	must(err)
	if m.IsTrue(constraint) {
		t.Fatal("lost symbolic equality")
	}
}

func TestClauseFromRuleInterpretedTailAndGroundCount(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(session.Close()) }()
	m, err := session.Manager()
	must(err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	must(err)
	defer func() { must(params.Close()) }()
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	must(err)
	defer func() { must(catalog.Close()) }()
	globals, err := uparams.NewGlobalParamsService(m.Symbols, params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	must(err)
	defer func() { must(globals.Close()) }()
	must(globals.InitializeSource())
	rw, err := rewriter.NewTheoryRewriter(m, params, globals, nil)
	must(err)
	defer func() { must(rw.Close()) }()
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	seven, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	integer, err := m.GetSort(seven)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	symbol, err := m.Symbols.Intern("tab-rule-p")
	must(err)
	p, err := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(p))
	symbol, err = m.Symbols.Intern("tab-rule-q")
	must(err)
	q, err := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(q))
	x, err := m.MkVar(0, integer)
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	tail, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	eq, err := m.MkEq(x, seven)
	must(err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{tail, eq}, nil, "tab-rule")
	must(err)
	defer func() { must(rule.Close()) }()
	c, err := tab.NewClauseFromRule(rule, rw)
	must(err)
	got, err := c.Head()
	must(err)
	want, err := m.MkApp(p, []ast.ASTHandle{seven})
	must(err)
	if got != want {
		t.Fatal("rule head reduction", got, want)
	}
	predicates, err := c.Predicates()
	must(err)
	expected, err := m.MkApp(q, []ast.ASTHandle{seven})
	must(err)
	if len(predicates) != 1 || predicates[0] != expected {
		t.Fatal("rule tail reduction", predicates, expected)
	}
	condition, err := c.Constraint()
	must(err)
	if !m.IsTrue(condition) || c.NumVars() != 1 {
		t.Fatal("rule constraint", condition, c.NumVars())
	}
	ground, err := rules.NewRule(want, nil, nil, "tab-ground")
	must(err)
	must(c.Close())
	defer func() { must(ground.Close()) }()
	c, err = tab.NewClauseFromRule(ground, rw)
	must(err)
	defer func() { must(c.Close()) }()
	condition, err = c.Constraint()
	must(err)
	if !m.IsTrue(condition) || c.NumVars() != 1 {
		t.Fatal("source ground rule count", condition, c.NumVars())
	}
	set, err := base.NewRuleSet(rules)
	must(err)
	must(set.Add(rule))
	must(set.Add(ground))
	index, err := tab.NewRuleIndex(set, rw)
	must(err)
	must(set.Close())
	count, err := index.Count(p)
	must(err)
	if count != 2 {
		t.Fatal("indexed predicate count", count)
	}
	count, err = index.Count(q)
	must(err)
	if count != 0 {
		t.Fatal("unexpected head predicate", count)
	}
	for i := 0; i < 2; i++ {
		indexed, e := index.Rule(p, i)
		must(e)
		position, e := indexed.Index()
		must(e)
		if position != uint32(i) {
			t.Fatal("source rule index", position, i)
		}
		sequential, e := index.Clause(i)
		must(e)
		if sequential != indexed {
			t.Fatal("rule index order", i)
		}
	}
	if _, err = index.Rule(p, 2); err == nil {
		t.Fatal("missing bounds error")
	}
	must(index.Close())
}
