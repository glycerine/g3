package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogRelationExplanationNestedCheckerMagicHeterogeneous(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	for key, value := range map[string]string{
		"generate_explanations":          "true",
		"explanations_on_relation_level": "true",
		"magic_sets_for_queries":         "true",
		"check_relation":                 "check",
		"default_table":                  "hashtable",
		"default_table_checked":          "true",
		"default_table_checker":          "sparse",
		"unbound_compressor":             "false",
	} {
		must(e.globals.SourceSetName("fixedpoint.datalog."+key, value))
	}
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(text string, domain []ast.ASTHandle, representation string) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(text)
		must(err)
		declaration, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(declaration))
		if representation != "" {
			must(e.SetPredicateRepresentation(declaration, []string{representation}))
		}
		return declaration
	}
	seed := declare("nested-check-seed", []ast.ASTHandle{boolean}, "tr_sparse")
	edge := declare("nested-check-edge", []ast.ASTHandle{boolean, boolean}, "tr_hashtable")
	reach := declare("nested-check-reach", []ast.ASTHandle{boolean}, "")
	goal := declare("nested-check-goal", []ast.ASTHandle{boolean}, "")
	falsehood, err := m.MkFalse()
	must(err)
	truth, err := m.MkTrue()
	must(err)
	must(e.AddFact(seed, []ast.ASTHandle{falsehood}))
	must(e.AddFact(edge, []ast.ASTHandle{falsehood, truth}))

	x, err := m.MkVar(0, boolean)
	must(err)
	y, err := m.MkVar(1, boolean)
	must(err)
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		application, err := m.MkApp(declaration, arguments)
		must(err)
		return application
	}
	baseRule, err := m.MkImplies(app(seed, x), app(reach, x))
	must(err)
	must(e.AddRule(baseRule, "nested-check-base"))
	recursiveBody, err := m.MkAnd(app(reach, x), app(edge, x, y))
	must(err)
	recursiveRule, err := m.MkImplies(recursiveBody, app(reach, y))
	must(err)
	must(e.AddRule(recursiveRule, "nested-check-recursive"))
	goalRule, err := m.MkImplies(app(reach, x), app(goal, x))
	must(err)
	must(e.AddRule(goalRule, "nested-check-goal"))

	query, err := m.NewRef(app(goal, truth))
	must(err)
	defer func() { must(m.DelRef(&query)) }()
	queryRoot, err := m.RefGet(query)
	must(err)
	status, err := e.QueryExpression(queryRoot)
	must(err)
	if status != lbool.True {
		t.Fatal("nested checked magic query", status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	answerRoot, err := m.RefGet(answer)
	must(err)
	names := datalogExplanationFormulaNames(t, m, answerRoot)
	for _, name := range []string{"nested-check-recursive", "nested-check-goal"} {
		if !names[name] {
			t.Fatal("nested checked answer lost proof", name, names)
		}
	}
	if _, ok := e.predicates[seed].table.(*rel.SparseTable); !ok {
		t.Fatalf("magic query changed explicit Sparse seed: %T", e.predicates[seed].table)
	}
	if _, ok := e.predicates[edge].table.(*rel.HashTable); !ok {
		t.Fatalf("magic query changed explicit Hash edge: %T", e.predicates[edge].table)
	}
}

func TestDatalogExplanationNestedCheckerRequiresInnerPair(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.SelectEngine("datalog"))
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.check_relation", "check"))
	fragmentMust(t, e.AddFact(f.declarations["p"], nil))
	status, err := e.QueryExpression(f.applications["p"])
	if !errors.Is(err, rel.ErrCompilerUnsupported) || status != lbool.Undef {
		t.Fatal("nested selector silently lost its inner pair", status, err)
	}
}

func TestDatalogExplanationNestedCheckerScopesExposeBothLayers(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "check"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "sparse"))
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("nested-check-scope-predicate")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(predicate))
	truth, err := m.MkTrue()
	must(err)
	must(e.AddFact(predicate, []ast.ASTHandle{truth}))
	must(e.AddRule(f.applications["q"], "nested-check-scope-rule"))

	checkedScope, enabled, err := e.beginDatalogExplanationCheckedScope()
	must(err)
	if !enabled || checkedScope == nil {
		t.Fatal("inner checked-table scope inactive")
	}
	defer func() { must(checkedScope.Close()) }()
	formulaScope, enabled, err := e.beginDatalogExplanationFormulaScope()
	must(err)
	if !enabled || formulaScope == nil || !formulaScope.nested {
		t.Fatal("outer nested formula scope inactive")
	}
	defer func() { must(formulaScope.Close()) }()
	transformer, err := rel.NewExplanationTransformer(m, e.rules)
	must(err)
	defer func() { must(transformer.Close()) }()
	target, err := transformer.TransformRulesOnly(e.set)
	must(err)
	defer func() { must(target.Close()) }()
	must(formulaScope.transferFacts(transformer, e.set, target))
	product, found, err := transformer.RelationProduct(predicate)
	must(err)
	if !found {
		t.Fatal("missing nested checked relation product")
	}
	formula, err := product.FormulaChecked()
	must(err)
	primary, checker, pair, err := product.CheckedConcreteKinds()
	must(err)
	if !formula || !pair || primary != "hashtable" || checker != "sparse" {
		t.Fatal("nested checker layers", formula, primary, checker, pair)
	}
	must(formulaScope.requireProduct(predicate, product))
	must(checkedScope.requireProduct(product))
}
