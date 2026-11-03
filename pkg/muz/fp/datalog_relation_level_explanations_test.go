package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func datalogExplanationFormulaNames(t *testing.T, manager *ast.ASTManager, root ast.ASTHandle) map[string]bool {
	t.Helper()
	names := make(map[string]bool)
	var walk func(ast.ASTHandle)
	walk = func(value ast.ASTHandle) {
		kind, err := manager.GetKind(value)
		fragmentMust(t, err)
		if kind != ast.ASTApp {
			return
		}
		declaration, err := manager.GetDecl(value)
		fragmentMust(t, err)
		symbol, err := manager.GetASTName(declaration)
		fragmentMust(t, err)
		name, _, err := manager.Symbols.BareString(symbol)
		fragmentMust(t, err)
		names[name] = true
		count, err := manager.GetNumArgs(value)
		fragmentMust(t, err)
		for index := uint32(0); index < count; index++ {
			argument, err := manager.GetArg(value, index)
			fragmentMust(t, err)
			walk(argument)
		}
	}
	walk(root)
	return names
}

func TestDatalogRelationLevelExplanationAggregatesRulesAndOwnsAnswer(t *testing.T) {
	for _, magic := range []bool{false, true} {
		name := "plain"
		if magic {
			name = "magic"
		}
		t.Run(name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, manager := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
			must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
			if magic {
				must(e.globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", "true"))
			}
			must(e.AddFact(f.declarations["p"], nil))
			must(e.AddFact(f.declarations["u"], nil))
			left, err := manager.MkImplies(f.applications["p"], f.applications["q"])
			must(err)
			must(e.AddRule(left, "relation-left"))
			right, err := manager.MkImplies(f.applications["u"], f.applications["q"])
			must(err)
			must(e.AddRule(right, "relation-right"))

			status, err := e.QueryExpression(f.applications["q"])
			must(err)
			if status != lbool.True {
				t.Fatal("relation-level explanation status", status)
			}
			answer, err := e.ExpressionAnswer()
			must(err)
			defer func() { must(manager.DelRef(&answer)) }()
			root, err := manager.RefGet(answer)
			must(err)
			names := datalogExplanationFormulaNames(t, manager, root)
			for _, required := range []string{"fact", "relation-left", "relation-right", "e_union"} {
				if !names[required] {
					t.Fatal("relation-level answer lacks proof constructor", required, names)
				}
			}

			status, err = e.QueryPredicate(f.declarations["q"])
			must(err)
			if status != lbool.True {
				t.Fatal("direct query after relation explanation", status)
			}
			held, err := manager.RefGet(answer)
			must(err)
			if held != root {
				t.Fatal("owning relation-level answer changed after clone cleanup")
			}
		})
	}
}

func TestDatalogRelationLevelExplanationFalseOwnsFalseFormula(t *testing.T) {
	f := newDirectFactFixture(t)
	e, manager := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
	step, err := manager.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(e.AddRule(step, "unreached-relation-step"))
	status, err := e.QueryExpression(f.applications["q"])
	must(err)
	if status != lbool.False {
		t.Fatal("empty relation-level explanation status", status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(manager.DelRef(&answer)) }()
	formula, err := manager.RefGet(answer)
	must(err)
	if !manager.IsFalse(formula) {
		t.Fatal("empty relation-level explanation is not false", formula)
	}
	status, err = e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.False {
		t.Fatal("direct false query after explanation", status)
	}
	formula, err = manager.RefGet(answer)
	must(err)
	if !manager.IsFalse(formula) {
		t.Fatal("owned false answer expired after clone cleanup")
	}
}

func TestDatalogRelationLevelExplanationRecursiveHistoryOwnsAnswer(t *testing.T) {
	f := newDirectFactFixture(t)
	e, manager := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.explanations_on_relation_level", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "sparse"))
	must(e.AddFact(f.declarations["p"], nil))
	self, err := manager.MkImplies(f.applications["p"], f.applications["p"])
	must(err)
	must(e.AddRule(self, "relation-self"))

	status, err := e.QueryExpression(f.applications["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("recursive relation-level explanation status", status)
	}
	answer, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(manager.DelRef(&answer)) }()
	root, err := manager.RefGet(answer)
	must(err)
	names := datalogExplanationFormulaNames(t, manager, root)
	for _, required := range []string{"fact", "relation-self", "e_union"} {
		if !names[required] {
			t.Fatal("recursive relation-level history lacks constructor", required, names)
		}
	}

	status, err = e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("direct recursive predicate query", status)
	}
	held, err := manager.RefGet(answer)
	must(err)
	if held != root {
		t.Fatal("recursive relation-level answer expired after second query")
	}
}
