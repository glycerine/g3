package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTBitVectorCarryXor3SourceClausesAndScopes(t *testing.T) {
	for _, op := range []int32{59, 60} {
		t.Run(fmt.Sprint(op), func(t *testing.T) {
			m, _ := smtModelGeneratorFixture(t)
			_, _, s := smtInternalizerFixture(t, m, nil)
			name, err := m.Symbols.Intern("bv")
			astTestMust(t, err)
			family, err := m.FamilyIDForSymbol(name)
			astTestMust(t, err)
			theory, err := NewSMTBitVectorTheory(family)
			astTestMust(t, err)
			astTestMust(t, s.RegisterTheory(theory))
			args := []ast.ASTHandle{smtRelevancyBool(t, m, "gate-a"), smtRelevancyBool(t, m, "gate-b"), smtRelevancyBool(t, m, "gate-c")}
			gate, err := m.MkFamilyApp(family, op, nil, args)
			astTestMust(t, err)
			astTestMust(t, s.Internalize(gate, true))
			want := 6
			if op == 60 {
				want = 8
			}
			if len(s.testClauses()) != want {
				t.Fatal("source gate clause count", len(s.testClauses()), want)
			}
			v, found := s.BoolVar(gate)
			if !found {
				t.Fatal("no gate variable")
			}
			data, err := s.BooleanData(v)
			astTestMust(t, err)
			if data.IsTheoryAtom() {
				t.Fatal("carry/XOR3 must use Boolean gate clauses")
			}
			for mask := 0; mask < 8; mask++ {
				count := 0
				astTestMust(t, s.PushScope())
				for i, arg := range args {
					lit, e := s.Literal(arg)
					astTestMust(t, e)
					if mask&(1<<i) == 0 {
						lit = lit.Negated()
					} else {
						count++
					}
					astTestMust(t, s.AssignLiteral(lit, NewSMTBooleanAxiom()))
				}
				truth := count >= 2
				if op == 60 {
					truth = count%2 != 0
				}
				wrong := NewSMTLiteral(v, truth)
				astTestMust(t, s.AssignLiteral(wrong, NewSMTBooleanAxiom()))
				consistent, e := s.Propagate()
				astTestMust(t, e)
				if consistent {
					t.Fatal("gate accepted wrong output", mask, truth)
				}
				astTestMust(t, s.PopScopes(1))
			}
			astTestMust(t, s.PushScope())
			astTestMust(t, s.Internalize(gate, false))
			data, err = s.BooleanData(v)
			astTestMust(t, err)
			if !data.IsENode() {
				t.Fatal("term-context enode flag")
			}
			astTestMust(t, s.PopScopes(1))
			data, err = s.BooleanData(v)
			astTestMust(t, err)
			if data.IsENode() {
				t.Fatal("enode flag survived pop")
			}
		})
	}
}
