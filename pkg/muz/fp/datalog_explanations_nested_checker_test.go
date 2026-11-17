package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

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
	must(e.addRuleEager(f.applications["q"], "nested-check-scope-rule"))

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
