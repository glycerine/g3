package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func smtClosestVariableFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *smtInternalizerTestTheory, []*SMTEnode) {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	theory := &smtInternalizerTestTheory{SMTTheoryBase: base}
	_, _, s := smtInternalizerFixture(t, m, nil)
	astTestMust(t, s.RegisterTheory(theory))
	sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "closest-sort"), nil)
	astTestMust(t, err)
	var nodes []*SMTEnode
	for i := 0; i < 6; i++ {
		x, err := m.MkConst(astTestSymbol(t, m, fmt.Sprintf("closest-%d", i)), sort)
		astTestMust(t, err)
		astTestMust(t, s.Internalize(x, false))
		nodes = append(nodes, s.context.enodes.get(x))
	}
	return m, s, theory, nodes
}

func TestSMTClosestTheoryVariableEqualityDisequalityAndRollback(t *testing.T) {
	for _, closest := range []bool{false, true} {
		t.Run(fmt.Sprint(closest), func(t *testing.T) {
			_, s, theory, nodes := smtClosestVariableFixture(t)
			s.newCore2thEq = closest
			for _, n := range nodes {
				_, err := theory.CreateVariable(n)
				astTestMust(t, err)
			}
			astTestMust(t, s.addEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
			astTestMust(t, s.addEquality(nodes[2], nodes[1], NewSMTEqualityAxiom()))
			astTestMust(t, s.PropagateTheoryEvents())
			astTestMust(t, s.PushScope())
			astTestMust(t, s.addEquality(nodes[2], nodes[3], NewSMTEqualityAxiom()))
			want := SMTTheoryVar(1)
			if closest {
				want = 2
			}
			events := s.PendingTheoryEqualities()
			if len(events) != 1 || events[0].Left != want || events[0].Right != 3 {
				t.Fatal("equality did not follow source proof branch selection", events, want)
			}
			astTestMust(t, s.PropagateTheoryEvents())
			astTestMust(t, s.PopScopes(1))
			if nodes[3].root != nodes[3] || nodes[2].root != nodes[1] {
				t.Fatal("scope did not restore equality proof branch")
			}
			eq, err := s.manager.MkEq(nodes[2].owner, nodes[4].owner)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(eq, true))
			literal, err := s.Literal(eq)
			astTestMust(t, err)
			astTestMust(t, s.addDisequality(nodes[2], nodes[4], literal.Negated()))
			diseqs := s.PendingTheoryDisequalities()
			if len(diseqs) != 1 || diseqs[0].Left != want || diseqs[0].Right != 4 {
				t.Fatal("disequality used class representatives instead of source selection", diseqs, want)
			}
		})
	}
}

func TestSMTClosestTheoryVariableWalksUnattachedProofNodes(t *testing.T) {
	_, s, theory, nodes := smtClosestVariableFixture(t)
	_, err := theory.CreateVariable(nodes[1])
	astTestMust(t, err)
	astTestMust(t, s.addEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
	if nodes[0].TheoryVar(theory.FamilyID()) != SMTNullTheoryVar || s.theoryVariableNear(nodes[0], theory.FamilyID()) != 0 {
		t.Fatal("closest variable did not follow unattached proof node")
	}
	if s.theoryVariableNear(nodes[0], SMTNullTheoryID) != SMTNullTheoryVar || s.theoryVariableNear(nodes[2], theory.FamilyID()) != SMTNullTheoryVar {
		t.Fatal("closest lookup invented an attachment")
	}
}

func TestSMTTheoryEqualityOriginFilteringAndMultiTheoryOrder(t *testing.T) {
	for _, returnToOwner := range []bool{false, true} {
		t.Run(fmt.Sprint(returnToOwner), func(t *testing.T) {
			m, s, first, nodes := smtClosestVariableFixture(t)
			family, err := m.RegisterDatatypePlugin()
			astTestMust(t, err)
			base, err := NewSMTTheoryBase(family)
			astTestMust(t, err)
			second := &smtInternalizerTestTheory{SMTTheoryBase: base}
			astTestMust(t, s.RegisterTheory(second))
			for _, theory := range []*smtInternalizerTestTheory{first, second} {
				for _, n := range nodes[:4] {
					_, err := theory.CreateVariable(n)
					astTestMust(t, err)
				}
			}
			astTestMust(t, s.addEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
			astTestMust(t, s.addEquality(nodes[2], nodes[1], NewSMTEqualityAxiom()))
			astTestMust(t, s.PropagateTheoryEvents())
			owner, err := NewSMTExtTheoryEqualityPropagationJustification(m, first.FamilyID(), nil, nil, nodes[2], nodes[3], nil)
			astTestMust(t, err)
			owner.returnToOwner = returnToOwner
			s.ownedJustifications = append(s.ownedJustifications, owner)
			astTestMust(t, s.addEquality(nodes[2], nodes[3], NewSMTEqualityJustification(owner)))
			events := s.PendingTheoryEqualities()
			want := []SMTTheoryID{second.FamilyID()}
			if returnToOwner {
				want = []SMTTheoryID{first.FamilyID(), second.FamilyID()}
			}
			if len(events) != len(want) {
				t.Fatal("origin callback filtering", events, want)
			}
			for i, event := range events {
				if event.Theory != want[i] || event.Left != 2 || event.Right != 3 {
					t.Fatal("multi-theory source order/nearby endpoints", events, want)
				}
			}
		})
	}
}

func TestSMTTheoryMergeExposesExistingDisequalities(t *testing.T) {
	for _, bothFamilies := range []bool{false, true} {
		t.Run(fmt.Sprint(bothFamilies), func(t *testing.T) {
			m, s, first, nodes := smtClosestVariableFixture(t)
			_, err := first.CreateVariable(nodes[0])
			astTestMust(t, err)
			_, err = first.CreateVariable(nodes[2])
			astTestMust(t, err)
			var second *smtInternalizerTestTheory
			if bothFamilies {
				family, err := m.RegisterDatatypePlugin()
				astTestMust(t, err)
				base, err := NewSMTTheoryBase(family)
				astTestMust(t, err)
				second = &smtInternalizerTestTheory{SMTTheoryBase: base}
				astTestMust(t, s.RegisterTheory(second))
				_, err = second.CreateVariable(nodes[1])
				astTestMust(t, err)
				_, err = second.CreateVariable(nodes[3])
				astTestMust(t, err)
			}
			negativeEquality := func(left, right *SMTEnode) {
				eq, err := m.MkEq(left.owner, right.owner)
				astTestMust(t, err)
				astTestMust(t, s.Internalize(eq, true))
				literal, err := s.Literal(eq)
				astTestMust(t, err)
				astTestMust(t, s.AssignLiteral(literal.Negated(), NewSMTBooleanAxiom()))
			}
			negativeEquality(nodes[1], nodes[2])
			if bothFamilies {
				negativeEquality(nodes[0], nodes[3])
			}
			_, err = s.PropagateCore()
			astTestMust(t, err)
			if len(s.PendingTheoryDisequalities()) != 0 || len(first.newDisequalities) != 0 || second != nil && len(second.newDisequalities) != 0 {
				t.Fatal("disequality was already visible to a shared theory")
			}
			astTestMust(t, s.PushScope())
			astTestMust(t, s.addEquality(nodes[0], nodes[1], NewSMTEqualityAxiom()))
			events := s.PendingTheoryDisequalities()
			want := []SMTTheoryID{first.FamilyID()}
			if second != nil {
				want = append(want, second.FamilyID())
			}
			if len(events) != len(want) {
				t.Fatal("merge lost newly visible disequalities", events, want)
			}
			for i, e := range events {
				if e.Theory != want[i] || e.Left != 0 || e.Right != 1 {
					t.Fatal("newly visible disequality source order/variables", events, want)
				}
			}
			astTestMust(t, s.PropagateTheoryEvents())
			astTestMust(t, s.PopScopes(1))
			if nodes[1].TheoryVar(first.FamilyID()) != SMTNullTheoryVar || nodes[0].root != nodes[0] {
				t.Fatal("newly attached root theory variable escaped pop")
			}
		})
	}
}
