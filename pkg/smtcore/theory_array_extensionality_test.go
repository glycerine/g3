package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTArrayExtensionalityDisequalityQueueAndRollback(t *testing.T) {
	for _, simple := range []bool{false, true} {
		t.Run(fmt.Sprint(simple), func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			var plugin SMTTheoryInternalizer
			if simple {
				plugin, err = NewSMTSimpleArrayTheory(family)
			} else {
				plugin, err = NewSMTArrayTheory(family)
			}
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(plugin))
			theory := smtArrayCore(plugin)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
			astTestMust(t, err)
			a, err := m.MkConst(astTestSymbol(t, m, "queued-array-a"), sort)
			astTestMust(t, err)
			b, err := m.MkConst(astTestSymbol(t, m, "queued-array-b"), sort)
			astTestMust(t, err)
			literal, err := theory.MkEqualityLiteral(a, b, true)
			astTestMust(t, err)
			if theory.CanPropagate() || len(theory.selectTrail) != 0 || theory.statistics.extensionality != 0 {
				t.Fatal("merely internalizing equality generated extensionality")
			}
			for attempt := 0; attempt < 3; attempt++ {
				astTestMust(t, state.PushScope())
				if attempt == 0 {
					astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
					_, err := state.PropagateCore()
					astTestMust(t, err)
					if theory.CanPropagate() || len(theory.selectTrail) != 0 {
						t.Fatal("positive equality generated a witness")
					}
				} else {
					astTestMust(t, state.AssignLiteral(literal.Negated(), NewSMTBooleanAxiom()))
					astTestMust(t, state.PropagateTheoryAssignments())
					astTestMust(t, state.PropagateTheoryEvents())
					if !theory.CanPropagate() || len(theory.extensionalityTodo) != 1 || len(theory.selectTrail) != 0 {
						t.Fatal("disequality did not defer its witness axiom")
					}
					left := state.context.enodes.get(a).TheoryVar(family)
					right := state.context.enodes.get(b).TheoryVar(family)
					astTestMust(t, theory.NewDisequality(right, left))
					if len(theory.extensionalityTodo) != 1 {
						t.Fatal("reversed duplicate witness was queued")
					}
					if attempt == 1 {
						astTestMust(t, theory.Propagate())
						if theory.CanPropagate() || len(theory.selectTrail) != 2 {
							t.Fatal("extensionality queue did not drain")
						}
					}
				}
				astTestMust(t, state.PopScopes(1))
				if theory.CanPropagate() || len(theory.extensionalityTrail) != 0 || len(theory.extensionalityFingerprints) != 0 || len(theory.selectTrail) != 0 {
					t.Fatal("popped witness work or fingerprint survived")
				}
			}
		})
	}
}

func TestSMTArrayExtensionalitySkipsKnownDisequalReads(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "different-read-array-a"), sort)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "different-read-array-b"), sort)
	astTestMust(t, err)
	index, err := m.MkTrue()
	astTestMust(t, err)
	left, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index})
	astTestMust(t, err)
	right, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, index})
	astTestMust(t, err)
	readEquality, err := theory.MkEqualityLiteral(left, right, true)
	astTestMust(t, err)
	for _, term := range []ast.ASTHandle{left, right, state.boolExpr[readEquality.Var()]} {
		astTestMust(t, state.markRelevant(term))
	}
	astTestMust(t, state.AssignLiteral(readEquality.Negated(), NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	arrayEquality, err := theory.MkEqualityLiteral(a, b, true)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(arrayEquality.Negated(), NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if theory.statistics.extensionality != 0 || theory.CanPropagate() || len(theory.selectTrail) != 2 || len(theory.extensionalityTrail) != 1 {
		t.Fatal("known read disequality did not suppress redundant extensionality")
	}
}
