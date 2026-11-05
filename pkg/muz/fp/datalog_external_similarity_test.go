package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogExternalSimilarityPublishesAuxiliarySeedsAndCleansUp(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	fragmentMust(t, e.SelectEngine("datalog"))
	defer func() { fragmentMust(t, e.Close()) }()

	domainName, err := m.Symbols.Intern("external-similarity-finite-domain")
	fragmentMust(t, err)
	domain, err := m.MkDatalogFiniteSort(domainName, 4)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{domain}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(declaration))
		return declaration
	}
	seed, query := declare("external-similarity-seed"), declare("external-similarity-query")
	var stored []ast.ASTHandle
	fragmentMust(t, e.SetExternalRelationCallbacks(externalSimilaritySeedRecorder(m, t, &stored)))

	for i := uint64(0); i < 3; i++ {
		value, cause := m.MkDatalogNumeral(i, domain)
		fragmentMust(t, cause)
		head, cause := m.MkApp(seed, []ast.ASTHandle{value})
		fragmentMust(t, cause)
		fragmentMust(t, e.AddRule(head, "external-similarity-seed"))
	}
	x, err := m.MkVar(0, domain)
	fragmentMust(t, err)
	body, err := m.MkApp(seed, []ast.ASTHandle{x})
	fragmentMust(t, err)
	head, err := m.MkApp(query, []ast.ASTHandle{x})
	fragmentMust(t, err)
	rule, err := m.MkImplies(body, head)
	fragmentMust(t, err)
	fragmentMust(t, e.AddRule(rule, "external-similarity-query"))

	if _, err = e.externalRelations(); err != nil {
		t.Fatal(err)
	}
	transformed, err := e.prepareExternalSimilarity([]ast.ASTHandle{query}, 2)
	fragmentMust(t, err)
	count, err := transformed.Len()
	fragmentMust(t, err)
	if count != 2 {
		t.Fatalf("transformed rule count = %d, want 2", count)
	}
	want := make(map[ast.ASTHandle]bool)
	for i := uint64(0); i < 3; i++ {
		value, cause := m.MkDatalogNumeral(i, domain)
		fragmentMust(t, cause)
		want[value] = true
	}
	if len(stored) != len(want) {
		t.Fatalf("published auxiliary seed count = %d, want %d: %v", len(stored), len(want), stored)
	}
	for _, value := range stored {
		if !want[value] {
			t.Fatalf("unexpected auxiliary seed value %v", value)
		}
		delete(want, value)
	}
	if len(want) != 0 {
		t.Fatalf("missing auxiliary seed values: %v", want)
	}

	auxiliary := e.externalSimilarity.program.AuxiliaryPredicates()
	if len(auxiliary) != 1 {
		t.Fatalf("auxiliary predicate count = %d, want 1", len(auxiliary))
	}
	if err = e.discardExternalSimilarity(); err != nil {
		t.Fatal(err)
	}
	if _, found, err := e.external.TryGetPredicateRelation(auxiliary[0]); err != nil || found {
		t.Fatalf("auxiliary relation cleanup found=%t err=%v", found, err)
	}
	if status := e.lastResult; status != lbool.Undef {
		t.Fatalf("preparation unexpectedly set query result %v", status)
	}
}

func externalSimilaritySeedRecorder(m *ast.ASTManager, t *testing.T, stored *[]ast.ASTHandle) rel.ExternalRelationCallbacks {
	return rel.ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, arguments, _ []ast.ASTHandle) error {
		kind, err := m.GetDeclKind(declaration)
		if err != nil {
			return err
		}
		if kind == ast.DatalogOpStore {
			if len(arguments) != 2 {
				t.Fatalf("STORE argument count = %d, want 2", len(arguments))
			}
			*stored = append(*stored, arguments[1])
		}
		return nil
	}}
}
