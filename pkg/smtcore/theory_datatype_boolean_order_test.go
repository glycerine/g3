package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTDatatypeBooleanTermsUseSourceArgumentOrderAndEnodeFlags(t *testing.T) {
	m, dt, _, _, _ := smtDatatypeSplitFixture(t)
	s, err := dt.State()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: astTestSymbol(t, m, "boolean-order-sort"),
		Constructors: []ast.DatatypeConstructorDefinition{
			{Name: astTestSymbol(t, m, "boolean-order-box"), Recognizer: astTestSymbol(t, m, "boolean-order-is-box"), Accessors: []ast.DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "boolean-order-field"), Range: boolean}}},
			{Name: astTestSymbol(t, m, "boolean-order-empty"), Recognizer: astTestSymbol(t, m, "boolean-order-is-empty")},
		},
	}})
	astTestMust(t, err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	p := smtRelevancyBool(t, m, "boolean-order-p")
	box, err := m.MkApp(constructors[0], []ast.ASTHandle{p})
	astTestMust(t, err)
	recognizer, err := m.DatatypeConstructorRecognizer(constructors[0])
	astTestMust(t, err)
	atom, err := m.MkApp(recognizer, []ast.ASTHandle{box})
	astTestMust(t, err)
	accessors, err := m.DatatypeConstructorAccessors(constructors[0])
	astTestMust(t, err)
	selected, err := m.MkApp(accessors[0], []ast.ASTHandle{box})
	astTestMust(t, err)
	astTestPin(t, m, atom)
	astTestPin(t, m, selected)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, s.PushScope())
		astTestMust(t, s.Internalize(atom, true))
		pv, pok := s.BoolVar(p)
		sv, sok := s.BoolVar(selected)
		rv, rok := s.BoolVar(atom)
		if !pok || !sok || !rok || !(pv < sv && sv < rv) {
			t.Fatal("source argument/accessor/recognizer Boolean creation order", pv, sv, rv)
		}
		for _, v := range []int32{sv, rv} {
			data, err := s.BooleanData(v)
			astTestMust(t, err)
			if !data.IsENode() || data.Theory() != dt.FamilyID() {
				t.Fatal("datatype Boolean term lost source enode flag or theory", v, data.IsENode(), data.Theory())
			}
		}
		_, err = s.PropagateCore()
		astTestMust(t, err)
		astTestMust(t, s.PopScopes(1))
		if _, found := s.BoolVar(atom); found || s.context.enodes.get(atom) != nil || s.context.enodes.get(selected) != nil {
			t.Fatal("scoped Boolean datatype terms survived pop")
		}
	}
}
