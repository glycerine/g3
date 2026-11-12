package smtcore

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSMTRelevancyAssignmentWaitsForBooleanPropagation(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "deferred-assignment")
	target := smtRelevancyBool(t, m, "deferred-target")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, p.AddWatchTarget(a, true, target))
	literal, err := s.Literal(a)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	if p.IsRelevant(target) {
		t.Fatal("assignment delivered relevance before Boolean propagation")
	}
	consistent, err := s.PropagateCore()
	astTestMust(t, err)
	if !consistent || !p.IsRelevant(target) {
		t.Fatal("completed propagation did not deliver queued relevance")
	}
}

func TestSMTRelevancyCallbacksObserveCompleteBooleanBatch(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	seed := smtRelevancyBool(t, m, "batch-seed")
	a := smtRelevancyBool(t, m, "batch-first-argument")
	b := smtRelevancyBool(t, m, "batch-second-argument")
	parent, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(seed, true))
	astTestMust(t, s.Internalize(parent, true))
	seedLit, err := s.Literal(seed)
	astTestMust(t, err)
	aLit, err := s.Literal(a)
	astTestMust(t, err)
	bLit, err := s.Literal(b)
	astTestMust(t, err)
	// BCP assigns b before a. C++ delivers the b callback only after both are
	// assigned, so propagate_relevant_or chooses a in source argument order.
	astTestMust(t, s.addGateClause([]SMTLiteral{seedLit.Negated(), bLit}))
	astTestMust(t, s.addGateClause([]SMTLiteral{seedLit.Negated(), aLit}))
	astTestMust(t, p.MarkRelevant(parent))
	astTestMust(t, p.Propagate())
	if p.IsRelevant(a) || p.IsRelevant(b) {
		t.Fatal("unassigned OR selected an argument")
	}
	astTestMust(t, s.AssignLiteral(seedLit, NewSMTBooleanAxiom()))
	consistent, err := s.PropagateCore()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("Boolean batch was inconsistent")
	}
	for _, literal := range []SMTLiteral{aLit, bLit} {
		v, err := s.Assignment(literal)
		astTestMust(t, err)
		if v != lbool.True {
			t.Fatal("BCP did not assign both operands")
		}
	}
	if !p.IsRelevant(a) || p.IsRelevant(b) {
		t.Fatal("callback observed a partial Boolean batch instead of selecting the first true argument")
	}
}

func TestSMTRelevancyBooleanConflictSuppressesAssignmentCallbacks(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "conflicting-assignment")
	b := smtRelevancyBool(t, m, "conflicting-implied")
	target := smtRelevancyBool(t, m, "conflict-must-not-deliver")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, s.Internalize(b, true))
	al, err := s.Literal(a)
	astTestMust(t, err)
	bl, err := s.Literal(b)
	astTestMust(t, err)
	astTestMust(t, s.addGateClause([]SMTLiteral{al.Negated(), bl}))
	astTestMust(t, s.addGateClause([]SMTLiteral{al.Negated(), bl.Negated()}))
	astTestMust(t, p.AddWatchTarget(a, true, target))
	astTestMust(t, s.AssignLiteral(al, NewSMTBooleanAxiom()))
	consistent, err := s.PropagateCore()
	astTestMust(t, err)
	if consistent {
		t.Fatal("missing Boolean conflict")
	}
	if p.IsRelevant(target) {
		t.Fatal("failed BCP batch delivered assignment callbacks")
	}
}

func TestSMTRelevancyBatchScopesRestorePendingAndDeliveredAssignments(t *testing.T) {
	for _, deliver := range []bool{false, true} {
		m := astTestManager(t)
		_, p, s := smtInternalizerFixture(t, m, nil)
		a := smtRelevancyPinnedBool(t, m, "scope-deferred-assignment")
		target := smtRelevancyPinnedBool(t, m, "scope-deferred-target")
		astTestMust(t, s.Internalize(a, true))
		astTestMust(t, p.AddWatchTarget(a, true, target))
		literal, err := s.Literal(a)
		astTestMust(t, err)
		astTestMust(t, s.PushScope())
		astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		if deliver {
			ok, err := s.PropagateCore()
			astTestMust(t, err)
			if !ok || !p.IsRelevant(target) {
				t.Fatal("scoped callback was not delivered")
			}
		}
		astTestMust(t, s.PopScopes(1))
		ok, err := s.PropagateCore()
		astTestMust(t, err)
		if !ok || p.IsRelevant(target) || s.relevancyBatch.active {
			t.Fatal("popped relevance batch survived")
		}
		astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		if p.IsRelevant(target) {
			t.Fatal("reassignment delivered relevance early")
		}
		ok, err = s.PropagateCore()
		astTestMust(t, err)
		if !ok || !p.IsRelevant(target) {
			t.Fatal("restored queue missed the reassignment")
		}
	}
}

func TestSMTRelevancyBatchRetryAfterBooleanQueueAdvanced(t *testing.T) {
	f := newSMTNativeCleanupFixture(t)
	s, m := f.state, f.manager
	p, err := NewSMTRelevancyPropagator(s.context, true)
	astTestMust(t, err)
	astTestMust(t, s.SetRelevancy(p))
	a := smtRelevancyBool(t, m, "retry-deferred-assignment")
	target := smtRelevancyBool(t, m, "retry-deferred-target")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, p.AddWatchTarget(a, true, target))
	literal, err := s.Literal(a)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	var cause error
	f.refuse(t, func() { _, cause = s.PropagateCore() })
	if !errors.Is(cause, ast.ErrASTPending) || s.propagationHead != len(s.assignedLiterals) || !s.relevancyBatch.active || s.relevancyBatch.head != 0 {
		t.Fatal("refused callback lost its completed BCP batch", cause, s.propagationHead, s.relevancyBatch)
	}
	if p.IsRelevant(target) {
		t.Fatal("refused callback marked its target")
	}
	ok, err := s.PropagateCore()
	astTestMust(t, err)
	if !ok || !p.IsRelevant(target) || s.relevancyBatch.active {
		t.Fatal("retry did not deliver the saved batch exactly once")
	}
	ok, err = s.PropagateCore()
	astTestMust(t, err)
	if !ok || s.relevancyBatch.active {
		t.Fatal("completed batch remained queued")
	}
}

func TestSMTRelevancyRestoredLearnedAssignmentIsDeferred(t *testing.T) {
	f := newSMTNativeCleanupFixture(t)
	s, m := f.state, f.manager
	p, err := NewSMTRelevancyPropagator(s.context, true)
	astTestMust(t, err)
	astTestMust(t, s.SetRelevancy(p))
	a := smtRelevancyBool(t, m, "restored-deferred-assignment")
	target := smtRelevancyBool(t, m, "restored-deferred-target")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, p.AddWatchTarget(a, true, target))
	literal, err := s.Literal(a)
	astTestMust(t, err)
	before, assigned := s.assignment[literal.Index()], len(s.assignedLiterals)
	justification := NewSMTBooleanAxiom()
	var cause error
	f.refuse(t, func() { cause = s.AssignLiteral(literal, justification) })
	if !errors.Is(cause, ast.ErrASTPending) {
		t.Fatal("missing context-publication refusal", cause)
	}
	pending := s.learnedAssignmentRemainder(literal, before, assigned, justification)
	astTestMust(t, s.finishLearnedAssignment(&pending))
	astTestMust(t, s.finishLearnedAssignment(&pending))
	if p.IsRelevant(target) || len(s.assignedLiterals) != assigned+1 {
		t.Fatal("learned assignment continuation delivered relevance early or duplicated the queue entry")
	}
	ok, err := s.PropagateCore()
	astTestMust(t, err)
	if !ok || !p.IsRelevant(target) {
		t.Fatal("restored learned assignment was not delivered after BCP")
	}
}

type smtRelevancyBatchObserver struct {
	*smtInternalizerTestTheory
	onRelevant func(ast.ASTHandle) error
}

func (o *smtRelevancyBatchObserver) Relevant(expression ast.ASTHandle) error {
	return o.onRelevant(expression)
}

func TestSMTRelevancyCallbackAssignmentsWaitForNextBooleanBatch(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	a := smtRelevancyBool(t, m, "snapshot-assignment-a")
	b := smtRelevancyBool(t, m, "snapshot-assignment-b")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, s.Internalize(b, true))
	al, err := s.Literal(a)
	astTestMust(t, err)
	bl, err := s.Literal(b)
	astTestMust(t, err)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	var targets []ast.ASTHandle
	for _, name := range []string{"snapshot-target-a", "snapshot-target-b"} {
		info, err := m.NewFuncDeclInfo(family, 9003, nil, ast.ASTFuncFlags{}, false)
		astTestMust(t, err)
		decl, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, name), nil, boolean, info)
		astTestMust(t, err)
		astTestMust(t, m.DelFuncDeclInfo(&info))
		expression, err := m.MkApp(decl, nil)
		astTestMust(t, err)
		targets = append(targets, expression)
	}
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	var events []ast.ASTHandle
	observer := &smtRelevancyBatchObserver{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}, onRelevant: func(expression ast.ASTHandle) error {
		events = append(events, expression)
		if expression == targets[0] {
			if s.propagationHead != len(s.assignedLiterals) {
				t.Fatal("first callback ran before BCP completed")
			}
			if err := s.AssignLiteral(bl, NewSMTBooleanAxiom()); err != nil {
				return err
			}
			if p.IsRelevant(targets[1]) {
				t.Fatal("callback assignment was delivered recursively")
			}
		} else if expression == targets[1] && s.propagationHead != len(s.assignedLiterals) {
			t.Fatal("new assignment callback ran before the next BCP batch")
		}
		return nil
	}}
	astTestMust(t, s.RegisterTheory(observer))
	astTestMust(t, p.AddWatchTarget(a, true, targets[0]))
	astTestMust(t, p.AddWatchTarget(b, true, targets[1]))
	astTestMust(t, s.AssignLiteral(al, NewSMTBooleanAxiom()))
	ok, err := s.PropagateCore()
	astTestMust(t, err)
	if !ok || len(events) != 2 || events[0] != targets[0] || events[1] != targets[1] {
		t.Fatal("relevance batches lost FIFO order or repeated callbacks", events)
	}
}

func TestSMTRelevancyPendingBatchDiscardedByPop(t *testing.T) {
	f := newSMTNativeCleanupFixture(t)
	s, m := f.state, f.manager
	p, err := NewSMTRelevancyPropagator(s.context, true)
	astTestMust(t, err)
	astTestMust(t, s.SetRelevancy(p))
	a := smtRelevancyBool(t, m, "discard-pending-assignment")
	target := smtRelevancyBool(t, m, "discard-pending-target")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, p.AddWatchTarget(a, true, target))
	literal, err := s.Literal(a)
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	var cause error
	f.refuse(t, func() { _, cause = s.PropagateCore() })
	if !errors.Is(cause, ast.ErrASTPending) || !s.relevancyBatch.active {
		t.Fatal("did not interrupt a live callback batch", cause)
	}
	astTestMust(t, s.PopScopes(1))
	ok, err := s.PropagateCore()
	astTestMust(t, err)
	if !ok || s.relevancyBatch.active || p.IsRelevant(target) {
		t.Fatal("pop replayed a retired callback batch")
	}
}
