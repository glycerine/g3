package fp

// Source: rel_context::inherit_predicate_kind after fact-level explanation
// lowering and the query-local MagicSets transform.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogFactExplanationMagicInheritsFiniteRepresentation(t *testing.T) {
	for _, kind := range []string{"tr_hashtable", "tr_sparse", "tr_lazy_sparse"} {
		for _, ground := range []bool{false, true} {
			shape := "open"
			if ground {
				shape = "ground"
			}
			t.Run(kind+"/"+shape, func(t *testing.T) {
				manager, globals := newPDRLifecycleServices(t)
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				must(globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
				must(globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", "true"))
				defaultTable := "hashtable"
				if kind == "tr_hashtable" {
					defaultTable = "sparse"
				}
				must(globals.SourceSetName("fixedpoint.datalog.default_table", defaultTable))
				engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
				must(err)
				defer func() { must(engine.Close()) }()
				must(engine.SelectEngine("datalog"))

				boolean, err := manager.MkBoolSort()
				must(err)
				declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
					t.Helper()
					symbol, declarationErr := manager.Symbols.Intern("explanation-magic-kind-" + kind + "-" + shape + "-" + name)
					must(declarationErr)
					declaration, declarationErr := manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
					must(declarationErr)
					must(engine.RegisterPredicate(declaration))
					return declaration
				}
				seed := declare("seed", []ast.ASTHandle{boolean})
				predicate := declare("predicate", []ast.ASTHandle{boolean})
				goal := declare("goal", nil)
				must(engine.SetPredicateRepresentation(predicate, []string{kind}))
				truth, err := manager.MkTrue()
				must(err)
				must(engine.AddFact(seed, []ast.ASTHandle{truth}))
				variable, err := manager.MkVar(0, boolean)
				must(err)
				app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
					t.Helper()
					application, appErr := manager.MkApp(declaration, arguments)
					must(appErr)
					return application
				}
				step, err := manager.MkImplies(app(seed, variable), app(predicate, variable))
				must(err)
				must(engine.AddRule(step, "explanation-magic-kind-step"))
				goalArgument := variable
				if ground {
					goalArgument = truth
				}
				goalRule, err := manager.MkImplies(app(predicate, goalArgument), app(goal))
				must(err)
				must(engine.AddRule(goalRule, "explanation-magic-kind-goal"))
				must(engine.applyDatalogDefaultFiniteTable())

				transformer, err := rel.NewExplanationTransformer(manager, engine.rules)
				must(err)
				defer func() { must(transformer.Close()) }()
				transformed, err := transformer.TransformRulesOnly(engine.set)
				must(err)
				defer func() { must(transformed.Close()) }()
				must(transformer.TransferFiniteFacts(engine.set, transformed, engine.relations, engine.predicateOrder))
				explainedGoal, err := transformer.ExplanationDeclaration(goal)
				must(err)
				origins, err := engine.datalogExplanationFiniteOrigins(transformer)
				must(err)
				valid, err := transformed.Finalize()
				must(err)
				if !valid {
					t.Fatal("explanation Magic fixture is unstratified")
				}
				plugin := transforms.NewMagicSetsPluginWithRelations(manager, engine.rules, explainedGoal, true, 0, engine.relations)
				defer func() { must(plugin.Close()) }()
				magicRules, err := plugin.Transform(transformed)
				must(err)
				if magicRules == nil {
					t.Fatal("MagicSets did not transform explanation rules")
				}
				defer func() { must(magicRules.Close()) }()
				must(engine.installDatalogExplanationMagicFinite(plugin, origins))

				matched := 0
				for _, declaration := range plugin.FreshPredicates() {
					if origins[declaration] != predicate {
						continue
					}
					arity, arityErr := manager.GetArity(declaration)
					must(arityErr)
					proofFree := true
					for column := uint32(0); column < arity; column++ {
						sort, domainErr := manager.GetDomain(declaration, column)
						must(domainErr)
						proof, proofErr := manager.IsDatalogRuleSort(sort)
						must(proofErr)
						proofFree = proofFree && !proof
					}
					table, found, tableErr := engine.relations.TryGetFiniteRelation(declaration)
					must(tableErr)
					if !proofFree {
						if found {
							t.Fatal("proof-carrying explanation predicate received a finite table")
						}
						continue
					}
					matched++
					wantArity := uint32(0)
					if ground {
						wantArity = 1
					}
					if !found || arity != wantArity {
						t.Fatal("missing proof-free inherited table", found, arity, wantArity)
					}
					if !ground || kind == "tr_hashtable" {
						if _, ok := table.(*rel.HashTable); !ok {
							t.Fatalf("nullary/Hash Magic table = %T", table)
						}
						continue
					}
					switch kind {
					case "tr_sparse":
						if _, ok := table.(*rel.SparseTable); !ok {
							t.Fatalf("inherited Sparse Magic table = %T", table)
						}
					case "tr_lazy_sparse":
						if _, ok := table.(*rel.LazySparseTable); !ok || engine.lazyFinite == nil || !engine.lazyFinite.Owns(table) {
							t.Fatalf("inherited LazySparse Magic table = %T", table)
						}
					}
				}
				if matched == 0 {
					t.Fatal("fixture did not create a proof-free Magic predicate")
				}
			})
		}
	}
}

func TestDatalogFactExplanationMagicRepresentationPublicQuery(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(globals.SourceSetName("fixedpoint.datalog.generate_explanations", "true"))
	must(globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", "true"))
	must(globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	must(err)
	defer func() { must(engine.Close()) }()
	must(engine.SelectEngine("datalog"))
	boolean, err := manager.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		symbol, declarationErr := manager.Symbols.Intern("explanation-magic-public-" + name)
		must(declarationErr)
		declaration, declarationErr := manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		must(declarationErr)
		must(engine.RegisterPredicate(declaration))
		return declaration
	}
	seed, predicate := declare("seed"), declare("predicate")
	must(engine.SetPredicateRepresentation(predicate, []string{"tr_sparse"}))
	truth, err := manager.MkTrue()
	must(err)
	must(engine.AddFact(seed, []ast.ASTHandle{truth}))
	variable, err := manager.MkVar(0, boolean)
	must(err)
	seedVariable, err := manager.MkApp(seed, []ast.ASTHandle{variable})
	must(err)
	predicateVariable, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	must(err)
	step, err := manager.MkImplies(seedVariable, predicateVariable)
	must(err)
	must(engine.AddRule(step, "explanation-magic-public-step"))
	query, err := manager.MkApp(predicate, []ast.ASTHandle{truth})
	must(err)
	queryRef, err := manager.NewRef(query)
	must(err)
	defer func() { must(manager.DelRef(&queryRef)) }()
	status, err := engine.QueryExpression(query)
	must(err)
	if status != lbool.True || engine.queryClone == nil {
		t.Fatal("fact-level explanation Magic query", status, engine.queryClone)
	}
	answer, err := engine.ExpressionAnswer()
	must(err)
	defer func() { must(manager.DelRef(&answer)) }()
	root, err := manager.RefGet(answer)
	must(err)
	names := datalogExplanationFormulaNames(t, manager, root)
	for _, name := range []string{"fact", "explanation-magic-public-step"} {
		if !names[name] {
			t.Fatal("fact-level Magic answer lost named proof", name, names)
		}
	}
	clone := engine.queryClone
	must(engine.clearQueryResources())
	if engine.queryClone != nil || !clone.closed {
		t.Fatal("fact-level Magic query clone was not released")
	}
	held, err := manager.RefGet(answer)
	must(err)
	if held != root {
		t.Fatal("owned explanation answer changed after clone cleanup")
	}
}
