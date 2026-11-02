package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func similarityTestGoal(t *testing.T, engine *PropositionalEngine, m *ast.ASTManager, pred, sort ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	boolean, e := m.MkBoolSort()
	fragmentMust(t, e)
	name, e := m.Symbols.Intern("similarity-goal")
	fragmentMust(t, e)
	goal, e := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, e)
	fragmentMust(t, engine.RegisterPredicate(goal))
	variable, e := m.MkVar(0, sort)
	fragmentMust(t, e)
	fact, e := m.MkApp(pred, []ast.ASTHandle{variable})
	fragmentMust(t, e)
	target, e := m.MkApp(goal, []ast.ASTHandle{variable})
	fragmentMust(t, e)
	implication, e := m.MkImplies(fact, target)
	fragmentMust(t, e)
	fragmentMust(t, engine.AddRule(implication, "goal"))
	return goal
}

// Inspect transformation internals while their source query scope is alive;
// public QueryPredicate must retire these rules and generated tables on return.
func similarityTestInspectScope(t *testing.T, engine *PropositionalEngine, output ast.ASTHandle) func() {
	t.Helper()
	scope, err := engine.beginDatalogQueryScope([]ast.ASTHandle{output})
	fragmentMust(t, err)
	original := engine.set
	engine.set = scope.set
	valid, err := engine.set.Finalize()
	fragmentMust(t, err)
	if !valid {
		t.Fatal("similarity fixture not stratified")
	}
	fragmentMust(t, engine.applyDatalogSimilarity([]ast.ASTHandle{output}))
	finished := false
	finish := func() {
		if finished {
			return
		}
		engine.set = original
		fragmentMust(t, scope.Close())
		finished = true
	}
	t.Cleanup(finish)
	return finish
}

func TestDatalogSimilarityLiveQuery(t *testing.T) {
	engine, m := newFragmentTestEngine(t, "datalog")
	name, e := m.Symbols.Intern("similarity-domain")
	fragmentMust(t, e)
	sort, e := m.MkDatalogFiniteSort(name, 16)
	fragmentMust(t, e)
	boolean, e := m.MkBoolSort()
	fragmentMust(t, e)
	name, e = m.Symbols.Intern("similarity-p")
	fragmentMust(t, e)
	pred, e := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, e)
	fragmentMust(t, engine.RegisterPredicate(pred))
	for i := uint64(0); i < 12; i++ {
		value, e := m.MkDatalogNumeral(i, sort)
		fragmentMust(t, e)
		// Similarity compresses ground rules, not directly inserted table tuples.
		fact, e := m.MkApp(pred, []ast.ASTHandle{value})
		fragmentMust(t, e)
		fragmentMust(t, engine.AddRule(fact, ""))
	}
	goal := similarityTestGoal(t, engine, m, pred, sort)
	status, e := engine.QueryPredicate(goal)
	fragmentMust(t, e)
	if status != lbool.True {
		t.Fatal("query", status)
	}
	if len(engine.auxiliary) != 0 {
		t.Fatal("query leaked similarity predicates")
	}
	finishScope := similarityTestInspectScope(t, engine, goal)
	n, e := engine.set.Len()
	fragmentMust(t, e)
	if n != 2 {
		t.Fatal("similar rules not merged", n)
	}
	if len(engine.auxiliary) != 1 {
		t.Fatal("expected one auxiliary predicate", len(engine.auxiliary))
	}
	var aux ast.ASTHandle
	for d := range engine.auxiliary {
		aux = d
	}
	saturated, e := engine.relations.IsSaturated(aux)
	fragmentMust(t, e)
	if !saturated {
		t.Fatal("generated relation is not saturated")
	}
	table, found, e := engine.relations.TryGetRelation(aux)
	fragmentMust(t, e)
	if !found {
		t.Fatal("auxiliary table missing")
	}
	count, e := table.Len()
	fragmentMust(t, e)
	if count != 12 {
		t.Fatal("auxiliary facts", count)
	}
	output, found, e := engine.relations.TryGetRelation(pred)
	fragmentMust(t, e)
	if !found {
		t.Fatal("output table missing")
	}
	for i := rel.Element(0); i < 16; i++ {
		found, e := output.Contains(rel.Fact{i})
		fragmentMust(t, e)
		if found != (i < 12) {
			t.Fatal("output fact", i, found)
		}
	}
	finishScope()
	status, e = engine.QueryPredicate(goal)
	fragmentMust(t, e)
	if status != lbool.True {
		t.Fatal("repeat query", status)
	}
	if len(engine.auxiliary) != 0 {
		t.Fatal("repeat query leaked auxiliary predicate")
	}
	n, e = engine.set.Len()
	fragmentMust(t, e)
	if n != 13 {
		t.Fatal("query did not restore original rules", n)
	}
}

func TestDatalogSimilarityQueryParameterControls(t *testing.T) {
	for _, tc := range []struct {
		name      string
		enabled   bool
		threshold uint32
		wantMerge bool
	}{{"enabled", true, 2, true}, {"disabled", false, 2, false}, {"strict threshold", true, 3, false}} {
		t.Run(tc.name, func(t *testing.T) {
			engine, m := newFragmentTestEngine(t, "datalog")
			params, e := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
			fragmentMust(t, e)
			catalog, e := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
			fragmentMust(t, e)
			globals, e := uparams.NewGlobalParamsService(m.Symbols, params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
			fragmentMust(t, e)
			fragmentMust(t, globals.InitializeSource())
			engine.params = params
			engine.globals = globals
			defer func() {
				fragmentMust(t, engine.SetQueryParameters(nil))
				fragmentMust(t, engine.Close())
				fragmentMust(t, globals.Close())
				fragmentMust(t, catalog.Close())
				fragmentMust(t, params.Close())
			}()
			local := params.EmptyValue()
			key := "datalog.similarity_compressor"
			fragmentMust(t, params.SetBoolName(&local, &key, tc.enabled))
			key = "datalog.similarity_compressor_threshold"
			fragmentMust(t, params.SetUIntName(&local, &key, tc.threshold))
			fragmentMust(t, engine.SetQueryParameters(&local))
			fragmentMust(t, params.Del(&local))
			name, e := m.Symbols.Intern("param-domain")
			fragmentMust(t, e)
			sort, e := m.MkDatalogFiniteSort(name, 4)
			fragmentMust(t, e)
			boolean, e := m.MkBoolSort()
			fragmentMust(t, e)
			name, e = m.Symbols.Intern("param-p")
			fragmentMust(t, e)
			pred, e := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, e)
			fragmentMust(t, engine.RegisterPredicate(pred))
			for i := uint64(0); i < 3; i++ {
				v, e := m.MkDatalogNumeral(i, sort)
				fragmentMust(t, e)
				fact, e := m.MkApp(pred, []ast.ASTHandle{v})
				fragmentMust(t, e)
				fragmentMust(t, engine.AddRule(fact, ""))
			}
			goal := similarityTestGoal(t, engine, m, pred, sort)
			status, e := engine.QueryPredicate(goal)
			fragmentMust(t, e)
			if status != lbool.True {
				t.Fatal(status)
			}
			if len(engine.auxiliary) != 0 {
				t.Fatal("query leaked generated relation")
			}
			finishScope := similarityTestInspectScope(t, engine, goal)
			defer finishScope()
			merged := len(engine.auxiliary) != 0
			if merged != tc.wantMerge {
				t.Fatalf("merged=%v want %v", merged, tc.wantMerge)
			}
		})
	}
}

func TestDatalogSimilarityKeepsQueriedOutputUnmerged(t *testing.T) {
	engine, m := newFragmentTestEngine(t, "datalog")
	name, e := m.Symbols.Intern("output-domain")
	fragmentMust(t, e)
	sort, e := m.MkDatalogFiniteSort(name, 16)
	fragmentMust(t, e)
	boolean, e := m.MkBoolSort()
	fragmentMust(t, e)
	name, e = m.Symbols.Intern("queried-output")
	fragmentMust(t, e)
	pred, e := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, e)
	fragmentMust(t, engine.RegisterPredicate(pred))
	for i := uint64(0); i < 12; i++ {
		v, e := m.MkDatalogNumeral(i, sort)
		fragmentMust(t, e)
		fact, e := m.MkApp(pred, []ast.ASTHandle{v})
		fragmentMust(t, e)
		fragmentMust(t, engine.AddRule(fact, ""))
	}
	status, e := engine.QueryPredicate(pred)
	fragmentMust(t, e)
	if status != lbool.True {
		t.Fatal(status)
	}
	finishScope := similarityTestInspectScope(t, engine, pred)
	defer finishScope()
	if len(engine.auxiliary) != 0 {
		t.Fatal("queried output was compressed")
	}
	n, e := engine.set.Len()
	fragmentMust(t, e)
	if n != 12 {
		t.Fatal("output rule count", n)
	}
}
