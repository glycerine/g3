package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTRelevancyBooleanMetadataFollowsVariableLifetime(t *testing.T) {
	m := astTestManager(t)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	s, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	p, err := NewSMTRelevancyPropagator(context, true)
	astTestMust(t, err)
	astTestMust(t, s.SetRelevancy(p))
	a := smtRelevancyBool(t, m, "metadata-a")
	av, err := s.MkBoolVar(a)
	astTestMust(t, err)
	astTestMust(t, s.SetAssignment(av, lbool.True))
	astTestMust(t, s.PushScope())
	neg, err := m.MkNot(a)
	astTestMust(t, err)
	nv, err := s.MkBoolVar(neg)
	astTestMust(t, err)
	if s.notChild(nv) != a {
		t.Fatal("NOT metadata was not recorded")
	}
	// A nongate NOT has its own event assignment, but find_assignment still
	// normalizes it to the child's value, as in C++.
	astTestMust(t, s.SetAssignment(nv, lbool.True))
	if p.assignment(neg) != lbool.False {
		t.Fatal("NOT find_assignment used its independent variable")
	}
	astTestMust(t, p.assignmentVariableEvent(nv, true))
	astTestMust(t, s.PopScopes(1))
	b := smtRelevancyBool(t, m, "metadata-b")
	bv, err := s.MkBoolVar(b)
	astTestMust(t, err)
	if bv != nv || !s.notChild(bv).IsNull() {
		t.Fatal("reused variable kept stale NOT metadata")
	}
	astTestMust(t, s.SetAssignment(bv, lbool.True))
	if p.assignment(b) != lbool.True {
		t.Fatal("cached assignment disagrees after variable reuse")
	}
	astTestMust(t, p.assignmentVariableEvent(bv, true))
}

func TestSMTRelevancyCachedGateArgumentsPreserveNegationAndOrder(t *testing.T) {
	for _, conjunction := range []bool{false, true} {
		m := astTestManager(t)
		context, err := NewSMTCheckerContext(m)
		astTestMust(t, err)
		s, err := NewSMTInternalizerState(m, context, nil)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, s.Close()) })
		p, err := NewSMTRelevancyPropagator(context, true)
		astTestMust(t, err)
		astTestMust(t, s.SetRelevancy(p))
		a := smtRelevancyBool(t, m, "cached-gate-a")
		b := smtRelevancyBool(t, m, "cached-gate-b")
		neg, err := m.MkNot(a)
		astTestMust(t, err)
		parent, err := m.MkOr(neg, b)
		if conjunction {
			parent, err = m.MkAnd(neg, b)
		}
		astTestMust(t, err)
		astTestMust(t, s.Internalize(parent, true))
		av, bv, pv := s.boolVar.get(a), s.boolVar.get(b), s.boolVar.get(parent)
		astTestMust(t, s.SetAssignment(av, lbool.FromBool(conjunction)))
		astTestMust(t, s.SetAssignment(bv, lbool.FromBool(!conjunction)))
		astTestMust(t, s.SetAssignment(pv, lbool.FromBool(!conjunction)))
		if gate := s.boolGate(pv); gate == nil || len(gate.literals) != 2 || gate.literals[0] != NewSMTLiteral(av, true) {
			t.Fatal("cached gate did not normalize its NOT operand")
		}
		astTestMust(t, p.MarkRelevant(parent))
		astTestMust(t, p.assignmentVariableEvent(pv, !conjunction))
		astTestMust(t, p.Propagate())
		if !p.IsRelevant(a) || !p.IsRelevant(neg) || p.IsRelevant(b) {
			t.Fatal("cached gate lost signed child selection or source argument order")
		}
	}
}

func TestSMTGatePageReuseAtBoundary(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	for i := 0; i < smtBoolGatesPerPage-2; i++ {
		_, err := s.MkBoolVar(smtRelevancyBool(t, m, fmt.Sprintf("page-leaf-%d", i)))
		astTestMust(t, err)
	}
	if len(s.gatePages) != 0 {
		t.Fatal("leaf-only variables allocated gate pages")
	}
	a, b := smtRelevancyPinnedBool(t, m, "page-a"), smtRelevancyPinnedBool(t, m, "page-b")
	for _, expected := range []int32{ast.ASTOpAnd, ast.ASTOpOr} {
		astTestMust(t, s.PushScope())
		neg, err := m.MkNot(a)
		astTestMust(t, err)
		n, err := s.MkBoolVar(neg)
		astTestMust(t, err)
		var e ast.ASTHandle
		if expected == ast.ASTOpAnd {
			e, err = m.MkAnd(a, b)
		} else {
			e, err = m.MkOr(a, b)
		}
		astTestMust(t, err)
		v, err := s.MkBoolVar(e)
		astTestMust(t, err)
		notPage, gatePage := s.gatePage(n), s.gatePage(v)
		if notPage == nil || gatePage == nil || notPage.compound != nil || gatePage.notChildren != nil {
			t.Fatal("allocated unneeded gate metadata on a homogeneous page")
		}
		if n != smtBoolGatesPerPage-1 || v != smtBoolGatesPerPage || s.notChild(n) != a || s.boolGateOp(v) != expected {
			t.Fatal("failed to cache adjacent gates across a page boundary", n, v)
		}
		astTestMust(t, s.PopScopes(1))
		if !s.notChild(n).IsNull() || s.boolGate(v) != nil || s.boolGateOp(v) != -1 {
			t.Fatal("retired gate metadata remained reachable")
		}
	}
}
