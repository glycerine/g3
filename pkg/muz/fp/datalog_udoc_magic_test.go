package fp

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocMagicSetsTransfersSeededIntentionalFacts(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	must := func(err error) {
		t.Helper()
		fragmentMust(t, err)
	}
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", strconv.FormatBool(true)))
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	must(err)
	defer func() { must(engine.Close()) }()
	must(engine.SelectEngine("datalog"))

	boolean, err := manager.MkBoolSort()
	must(err)
	finite, err := manager.BitVectorSort(3)
	must(err)
	declare := func(name string) ast.ASTHandle {
		t.Helper()
		symbol, cause := manager.Symbols.Intern(name)
		must(cause)
		declaration, cause := manager.MkFuncDecl(symbol, []ast.ASTHandle{finite}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(engine.RegisterPredicate(declaration))
		return declaration
	}
	app := func(declaration ast.ASTHandle, argument ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		result, cause := manager.MkApp(declaration, []ast.ASTHandle{argument})
		must(cause)
		return result
	}
	extensional := declare("doc-magic-E")
	intentional := declare("doc-magic-Q")
	goal := declare("doc-magic-P")
	must(engine.AddTableFact(intentional, []uint32{3}))
	must(engine.AddTableFact(extensional, []uint32{5}))
	if engine.udocDatalog == nil {
		t.Fatal("doc storage was not selected")
	}

	variable, err := manager.MkVar(0, finite)
	must(err)
	extensionalBody := app(extensional, variable)
	intentionalHead := app(intentional, variable)
	intentionalRule, err := manager.MkImplies(extensionalBody, intentionalHead)
	must(err)
	must(engine.addRuleEager(intentionalRule, "doc-magic-intentional"))
	goalHead := app(goal, variable)
	goalRule, err := manager.MkImplies(intentionalHead, goalHead)
	must(err)
	must(engine.addRuleEager(goalRule, "doc-magic-goal"))

	// Query setup selects UDoc before MagicSets creates its adorned predicates.
	// Exercise that boundary directly so QueryPredicate's later idempotent sync
	// cannot hide a missing post-transform synchronization.
	seedValue, err := manager.BitVectorNumeralUint64(3, 3)
	must(err)
	seedQuery := app(goal, seedValue)
	prepared, err := engine.newQueryClone()
	must(err)
	queryDeclaration, headRef, err := prepared.makeQueryHead(seedQuery)
	must(err)
	defer func() { fragmentMust(t, closeASTRefs(manager, []ast.ASTRef{headRef})) }()
	queryHead, err := manager.RefGet(headRef)
	fragmentMust(t, err)
	must(prepared.RegisterPredicate(queryDeclaration))
	prepared.auxiliary[queryDeclaration] = true
	must(prepared.inheritDatalogQueryKind(seedQuery, queryDeclaration))
	queryRule, err := manager.MkImplies(seedQuery, queryHead)
	must(err)
	must(prepared.addRuleEager(queryRule, "doc-magic-prepared-query"))
	selected, err := prepared.ensureUDocDatalog()
	must(err)
	if !selected {
		t.Fatal("prepared query did not select UDoc")
	}
	beforeMagicPredicates := len(prepared.predicateOrder)
	must(prepared.applyQueryMagicSets(queryDeclaration))
	if len(prepared.predicateOrder) == beforeMagicPredicates {
		t.Fatal("prepared query did not create MagicSets predicates")
	}
	for _, declaration := range prepared.predicateOrder[beforeMagicPredicates:] {
		if prepared.udocDatalog.seeds[declaration] == nil {
			t.Fatal("MagicSets predicate was not synchronized before query execution")
		}
	}
	must(prepared.Close())

	persistentPredicates := make(map[ast.ASTHandle]bool, len(engine.predicateOrder))
	for _, declaration := range engine.predicateOrder {
		persistentPredicates[declaration] = true
	}
	persistentRules, err := engine.set.Len()
	must(err)
	for _, test := range []struct {
		value uint64
		want  lbool.LBool
	}{
		{value: 3, want: lbool.True}, // Requires the source transfer rule for Q's seed.
		{value: 5, want: lbool.True}, // Reached through Q's defining rule.
		{value: 2, want: lbool.False},
	} {
		numeral, cause := manager.BitVectorNumeralUint64(test.value, 3)
		must(cause)
		query := app(goal, numeral)
		queryRef, cause := manager.NewRef(query)
		must(cause)
		status, queryErr := engine.QueryExpression(query)
		closeErr := manager.DelRef(&queryRef)
		must(queryErr)
		must(closeErr)
		if status != test.want {
			t.Fatalf("P(%d) = %v, want %v", test.value, status, test.want)
		}
		clone := engine.queryClone
		if clone == nil || clone.udocDatalog == nil || clone.udocDatalog == engine.udocDatalog {
			t.Fatal("magic query did not keep its independently owned UDoc clone")
		}
		fresh := 0
		for _, declaration := range clone.predicateOrder {
			if persistentPredicates[declaration] || !clone.auxiliary[declaration] {
				continue
			}
			fresh++
			if clone.udocDatalog.seeds[declaration] == nil || clone.udocDatalog.result[declaration] == nil {
				t.Fatal("late MagicSets predicate was not synchronized into UDoc")
			}
		}
		if fresh < 2 {
			t.Fatalf("fresh query-local predicate count %d, want synthetic and MagicSets predicates", fresh)
		}
	}
	afterRules, err := engine.set.Len()
	must(err)
	if afterRules != persistentRules || len(engine.predicateOrder) != len(persistentPredicates) {
		t.Fatal("query-local UDoc MagicSets changed the persistent program")
	}
}
