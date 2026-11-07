package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
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
	if s.booleanData[nv].basicOp != ast.ASTOpNot || s.booleanData[nv].notChild != a {
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
	if bv != nv || !s.booleanData[bv].notChild.IsNull() || s.booleanData[bv].basicOp != -1 {
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
		if len(s.booleanData[pv].gateLiterals) != 2 || s.booleanData[pv].gateLiterals[0] != NewSMTLiteral(av, true) {
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
