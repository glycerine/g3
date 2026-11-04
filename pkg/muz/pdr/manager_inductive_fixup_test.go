package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestInductiveClauseFixupSourceRewritesAndFreshParameters(t *testing.T) {
	m, c, _ := classifierContextFixture(t)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	p, err := m.MkFreshConst("fixup-p", boolean)
	closureMust(t, err)
	pRef, err := m.NewRef(p)
	closureMust(t, err)
	q, err := m.MkFreshConst("fixup-q", boolean)
	closureMust(t, err)
	qRef, err := m.NewRef(q)
	closureMust(t, err)
	notP, err := m.MkNot(p)
	closureMust(t, err)
	notRef, err := m.NewRef(notP)
	closureMust(t, err)
	defer func() {
		closureMust(t, m.DelRef(&notRef))
		closureMust(t, m.DelRef(&qRef))
		closureMust(t, m.DelRef(&pRef))
	}()
	duplicate, err := m.MkOr(p, p)
	closureMust(t, err)
	duplicateRef, err := m.NewRef(duplicate)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&duplicateRef)) }()
	tautology, err := m.MkOr(p, notP)
	closureMust(t, err)
	tautologyRef, err := m.NewRef(tautology)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&tautologyRef)) }()
	conjunction, err := m.MkAnd(duplicate, tautology)
	closureMust(t, err)
	conjunctionRef, err := m.NewRef(conjunction)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&conjunctionRef)) }()
	before := m.Stats().LiveReferences
	result, err := c.fixupInductiveClausesRef(conjunction)
	closureMust(t, err)
	got, err := m.RefGet(result)
	closureMust(t, err)
	if got != p {
		t.Fatal("duplicate/tautological clauses did not reduce", got, p)
	}
	closureMust(t, m.DelRef(&result))
	if n := m.Stats().LiveReferences; n != before {
		t.Fatal("fixup leaked roots", n, before)
	}
	// Source constructs a fresh bool_rewriter for fixup, unlike the older
	// manager-owned rewriter; new global settings must therefore be observed.
	closureMust(t, c.owner.contexts.config.Globals.SourceSetName("rewriter.elim_and", "true"))
	raw, err := m.MkAnd(p, q)
	closureMust(t, err)
	rawRef, err := m.NewRef(raw)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&rawRef)) }()
	result, err = c.fixupInductiveClausesRef(raw)
	closureMust(t, err)
	got, err = m.RefGet(result)
	closureMust(t, err)
	isNot, _, err := m.IsNot(got)
	closureMust(t, err)
	if !isNot {
		t.Fatal("fixup did not use the fresh elim_and setting", got)
	}
	closureMust(t, m.DelRef(&result))
}

func TestInductiveClauseFixupFailureCleanupAndRetry(t *testing.T) {
	m, c, _ := classifierContextFixture(t)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	p, err := m.MkFreshConst("fixup-failure", boolean)
	closureMust(t, err)
	root, err := m.NewRef(p)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&root)) }()
	before := m.Stats().LiveReferences
	failures, success := 0, false
	for n := int64(0); n < 300; n++ {
		m.Budget().FailAfter(n)
		result, x := c.fixupInductiveClausesRef(p)
		m.Budget().FailAfter(-1)
		if x == nil {
			closureMust(t, m.DelRef(&result))
			success = true
			break
		}
		failures++
		var pending memory.ConstructionFailure
		if errors.As(x, &pending) {
			closureMust(t, pending.Cleanup())
		}
		if !errors.Is(x, arena.ErrInjectedFailure) {
			t.Fatalf("failure point %d: %v", n, x)
		}
		if result != (ast.ASTRef{}) {
			t.Fatal("failed fixup published a result")
		}
		if got := m.Stats().LiveReferences; got != before {
			t.Fatalf("failure point %d leaked references: %d != %d", n, got, before)
		}
	}
	if failures == 0 || !success {
		t.Fatal("missing failure/success frontiers", failures, success)
	}
}

func TestInductiveClauseFixupReachesModelAnswerAndValidation(t *testing.T) {
	m, c, _ := classifierContextFixture(t)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	declare := func(name string) ast.ASTHandle {
		sym, x := m.Symbols.Intern(name)
		closureMust(t, x)
		d, x := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
		closureMust(t, x)
		return d
	}
	pred, query, aux := declare("fixup-model-pred"), declare("fixup-model-query"), declare("fixup-model-aux")
	predTransformer, err := NewPredicateTransformer(c.owner, pred, ReachableNoCache)
	closureMust(t, err)
	queryTransformer, err := NewPredicateTransformer(c.owner, query, ReachableNoCache)
	closureMust(t, err)
	c.transformers = make(map[ast.ASTHandle]*PredicateTransformer)
	c.transformers[pred] = predTransformer
	c.transformers[query] = queryTransformer
	c.order = []ast.ASTHandle{pred, query}
	c.queryDecl = query
	c.inductive = 0
	atom, err := m.MkApp(aux, nil)
	closureMust(t, err)
	clause, err := m.MkOr(atom, atom)
	closureMust(t, err)
	_, err = predTransformer.AddProperty(clause, InfinityLevel)
	closureMust(t, err)
	md, err := c.finiteFrameModel(0, query)
	closureMust(t, err)
	value, err := md.Core().ConstantInterpretation(pred)
	closureMust(t, err)
	if value != atom {
		t.Fatal("model did not normalize duplicate clause", value, atom)
	}
	closureMust(t, md.Close())
	answer, err := c.unsatAnswer(query)
	closureMust(t, err)
	formula, err := m.RefGet(answer)
	closureMust(t, err)
	rhs, err := m.GetArg(formula, 1)
	closureMust(t, err)
	if rhs != atom {
		t.Fatal("direct answer did not normalize duplicate clause", rhs, atom)
	}
	closureMust(t, m.DelRef(&answer))
	closureMust(t, c.validateUNSATResult())
}
