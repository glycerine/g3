package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestIntervalDatalogActivationLifecycleAndZeroArityQuery(t *testing.T) {
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
	declare := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("interval-activation-p", []ast.ASTHandle{integer})
	activation, selected, err := newIntervalDatalogActivation(m, engine.set, []ast.ASTHandle{p})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("all-arithmetic program was not selected")
	}
	defer func() { fragmentMust(t, activation.close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, activation.addFact(p, []ast.ASTHandle{one}))

	// Direct facts may instantiate the store before later arithmetic
	// predicates are registered.
	q := declare("interval-activation-late-q", []ast.ASTHandle{integer})
	fragmentMust(t, activation.syncPredicates([]ast.ASTHandle{p, q}))
	seed, err := activation.store.Relation(p)
	fragmentMust(t, err)
	found, err := seed.ContainsFactExpressions(m, []ast.ASTHandle{one})
	fragmentMust(t, err)
	if !found {
		t.Fatal("predicate synchronization discarded an existing seed")
	}

	// Expression-query cloning registers its synthetic nullary head after an
	// activation may already own source facts.
	query := declare("interval-activation-query", nil)
	fragmentMust(t, activation.syncPredicates([]ast.ASTHandle{p, q, query}))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	fragmentMust(t, err)
	queryApp, err := m.MkApp(query, nil)
	fragmentMust(t, err)
	rule, err := m.MkImplies(px, queryApp)
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "synthetic zero-arity query"))
	clone, err := activation.clone(engine.set)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, clone.close()) }()
	status, err := activation.query([]ast.ASTHandle{query}, true)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty imprecise zero-arity query was not unknown", status)
	}
	result, err := activation.relation(p)
	fragmentMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	fragmentMust(t, err)
	fragmentMust(t, result.FilterEqualExpression(m, 0, two))
	if !result.Empty() {
		t.Fatal("caller result mutation did not change the snapshot")
	}
	unchanged, err := activation.relation(p)
	fragmentMust(t, err)
	found, err = unchanged.ContainsFactExpressions(m, []ast.ASTHandle{one})
	fragmentMust(t, err)
	if !found {
		t.Fatal("caller mutation changed activation result")
	}
	formula, err := activation.relationFormula(query)
	fragmentMust(t, err)
	fragmentMust(t, m.DelRef(&formula))
	fragmentMust(t, activation.reset())
	cloneStatus, err := clone.query([]ast.ASTHandle{p}, true)
	fragmentMust(t, err)
	if cloneStatus != lbool.Undef {
		t.Fatal("activation clone did not preserve independent seed facts", cloneStatus)
	}
	status, err = activation.query([]ast.ASTHandle{p, query}, true)
	fragmentMust(t, err)
	// The source vector_relation projection constructs a full destination and
	// does not copy m_empty. The empty unary predicate therefore becomes a
	// full nullary relation when the query rule projects away its argument.
	if status != lbool.Undef {
		t.Fatal("empty-source projection did not match C++ interval behavior", status)
	}
	resetPredicate, err := activation.relation(p)
	fragmentMust(t, err)
	if !resetPredicate.Empty() {
		t.Fatal("reset changed the original predicate seed")
	}
	projectedQuery, err := activation.relation(query)
	fragmentMust(t, err)
	if projectedQuery.Empty() || !projectedQuery.IsFull() {
		t.Fatal("empty nullary projection did not become full", projectedQuery.Empty(), projectedQuery.IsFull())
	}
}

func TestIntervalDatalogActivationDeclinesMixedSignaturesAndRejectsUnsupportedRules(t *testing.T) {
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
	mkDecl := func(text string, domain ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(name, []ast.ASTHandle{domain}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		return declaration
	}
	p, q := mkDecl("interval-activation-unsupported-p", integer), mkDecl("interval-activation-unsupported-q", integer)
	mixed := mkDecl("interval-activation-finite", boolean)
	if eligible, cause := intervalDatalogEligible(m, []ast.ASTHandle{p, mixed}); cause != nil || eligible {
		t.Fatal("mixed finite/arithmetic signatures selected interval-only activation", eligible, cause)
	}
	nullaryName, err := m.Symbols.Intern("interval-activation-nullary-only")
	fragmentMust(t, err)
	nullary, err := m.MkFuncDecl(nullaryName, nil, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	if eligible, cause := intervalDatalogEligible(m, []ast.ASTHandle{nullary}); cause != nil || !eligible {
		t.Fatal("explicit all-nullary interval signature was rejected", eligible, cause)
	}
	nullaryActivation, selected, err := newIntervalDatalogActivation(m, engine.set, []ast.ASTHandle{nullary})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("explicit all-nullary interval activation was declined")
	}
	fragmentMust(t, nullaryActivation.addFact(nullary, nil))
	status, err := nullaryActivation.query([]ast.ASTHandle{nullary}, true)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty imprecise nullary interval relation was not unknown", status)
	}
	fragmentMust(t, nullaryActivation.close())
	fragmentMust(t, engine.RegisterPredicate(p))
	fragmentMust(t, engine.RegisterPredicate(q))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	fragmentMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	notP, err := m.MkNot(px)
	fragmentMust(t, err)
	qApp, err := m.MkApp(q, []ast.ASTHandle{x})
	fragmentMust(t, err)
	rule, err := m.MkImplies(notP, qApp)
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "unsupported negative tail"))
	activation, selected, err := newIntervalDatalogActivation(m, engine.set, []ast.ASTHandle{p, q})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("arithmetic signatures declined before rule validation")
	}
	defer func() { fragmentMust(t, activation.close()) }()
	fragmentMust(t, activation.addFact(p, []ast.ASTHandle{one}))
	if cause := activation.syncPredicates([]ast.ASTHandle{p, q, mixed}); !errors.Is(cause, ErrIntervalDatalogFragment) {
		t.Fatal("mixed signature after interval selection did not fail", cause)
	}
	seed, err := activation.store.Relation(p)
	fragmentMust(t, err)
	found, err := seed.ContainsFactExpressions(m, []ast.ASTHandle{one})
	fragmentMust(t, err)
	if !found {
		t.Fatal("failed mixed synchronization discarded interval seeds")
	}
	if status, cause := activation.query([]ast.ASTHandle{q}, true); status != lbool.Undef || !errors.Is(cause, ErrIntervalDatalogFragment) {
		t.Fatal("selected unsupported fragment silently produced a result", status, cause)
	}
	if _, cause := activation.relation(q); !errors.Is(cause, ErrIntervalDatalogFragment) {
		t.Fatal("failed query published a stale relation", cause)
	}
}
