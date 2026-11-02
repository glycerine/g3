package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestIntervalDatalogStoreSnapshotsQueriesAndOwnership(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("interval-store-p")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(p))
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	five, err := m.ArithmeticIntegerNumeralText("5")
	fragmentMust(t, err)

	store, err := NewIntervalDatalogStore(m)
	fragmentMust(t, err)
	before, err := m.GetRefCount(p)
	fragmentMust(t, err)
	fragmentMust(t, store.Register(p))
	after, err := m.GetRefCount(p)
	fragmentMust(t, err)
	if after != before+1 {
		t.Fatal("store did not pin declaration", before, after)
	}
	fragmentMust(t, store.AddFact(p, []ast.ASTHandle{one}))
	evaluator, err := store.Evaluator(engine.set)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	fragmentMust(t, store.AddFact(p, []ast.ASTHandle{five}))

	snapshot, err := evaluator.Relation(p)
	fragmentMust(t, err)
	contains, err := snapshot.ContainsFactExpressions(m, []ast.ASTHandle{five})
	fragmentMust(t, err)
	if contains {
		t.Fatal("post-snapshot store fact leaked into evaluator")
	}
	status, err := evaluator.Query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty imprecise interval query was not unknown", status)
	}
	undeclaredName, err := m.Symbols.Intern("interval-store-undeclared")
	fragmentMust(t, err)
	undeclared, err := m.MkFuncDecl(undeclaredName, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	if status, err = evaluator.Query([]ast.ASTHandle{p, undeclared}); status != lbool.Undef || !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("earlier nonempty relation hid an undeclared query predicate", status, err)
	}
	status, err = evaluator.Query(nil)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("zero-relation query was not true", status)
	}
	formula, err := evaluator.RelationFormula(p)
	fragmentMust(t, err)
	if formula == (ast.ASTRef{}) {
		t.Fatal("nonempty relation produced no formula")
	}
	fragmentMust(t, m.DelRef(&formula))

	// The store now contains [1,5], whose convex hull also contains 3 even
	// though 3 was never inserted. This abstraction must not claim SAT.
	hullEvaluator, err := store.Evaluator(engine.set)
	fragmentMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	fragmentMust(t, err)
	hull, err := hullEvaluator.Relation(p)
	fragmentMust(t, err)
	contains, err = hull.ContainsFactExpressions(m, []ast.ASTHandle{three})
	fragmentMust(t, err)
	if !contains {
		t.Fatal("expected convex-hull false-positive witness")
	}
	status, err = hullEvaluator.Query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("convex-hull false positive produced a definite status", status)
	}
	fragmentMust(t, hullEvaluator.Close())

	fragmentMust(t, store.Reset())
	emptyEvaluator, err := store.Evaluator(engine.set)
	fragmentMust(t, err)
	status, err = emptyEvaluator.Query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("reset relation query was not false", status)
	}
	fragmentMust(t, emptyEvaluator.Close())
	fragmentMust(t, evaluator.Close())
	fragmentMust(t, store.Close())
	closedCount, err := m.GetRefCount(p)
	fragmentMust(t, err)
	if closedCount != before {
		t.Fatal("store declaration pin survived close", before, closedCount)
	}
	if err = store.AddFact(p, []ast.ASTHandle{one}); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("closed store accepted a fact", err)
	}
}

func TestIntervalDatalogStoreRejectsNonArithmeticSignature(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("interval-store-bool")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	store, err := NewIntervalDatalogStore(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, store.Close()) }()
	if err = store.Register(p); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("non-arithmetic predicate was accepted", err)
	}
}

func TestIntervalDatalogEvaluatorRejectsNonBooleanRange(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("interval-evaluator-non-predicate")
	fragmentMust(t, err)
	function, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	if _, err = NewIntervalDatalogEvaluator(m, engine.set, []ast.ASTHandle{function}, nil); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("non-Boolean declaration was accepted as a predicate", err)
	}
}
