package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTBitVectorMKBVBooleanArgumentsEnodesAndScopes(t *testing.T) {
	for _, reflect := range []bool{false, true} {
		t.Run(fmt.Sprint(reflect), func(t *testing.T) {
			m, _ := smtModelGeneratorFixture(t)
			_, _, s := smtInternalizerFixture(t, m, nil)
			name, err := m.Symbols.Intern("bv")
			astTestMust(t, err)
			family, err := m.FamilyIDForSymbol(name)
			astTestMust(t, err)
			theory, err := NewSMTBitVectorTheory(family)
			astTestMust(t, err)
			astTestMust(t, s.RegisterTheory(theory))
			config := theory.config
			config.Reflect = reflect
			astTestMust(t, theory.ConfigureBitVector(config))
			p := smtRelevancyBool(t, m, "mkbv-p")
			q := smtRelevancyBool(t, m, "mkbv-q")
			notQ, err := m.MkNot(q)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			falsity, err := m.MkFalse()
			astTestMust(t, err)
			args := []ast.ASTHandle{p, notQ, truth, falsity}
			term, err := m.MkBitVectorFromBits(args)
			astTestMust(t, err)
			root, err := m.NewRef(term)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
			atom, err := m.MkBitVectorBit2Bool(0, term)
			astTestMust(t, err)
			atomRoot, err := m.NewRef(atom)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&atomRoot)) })
			astTestMust(t, s.PushScope())
			astTestMust(t, s.Internalize(atom, true))
			node := s.context.enodes.get(term)
			if node == nil || node.suppressArguments == reflect {
				t.Fatal("MKBV enode flags", node)
			}
			for _, arg := range args {
				if s.context.enodes.get(arg) == nil {
					t.Fatal("Boolean argument missing source term-context enode", arg)
				}
			}
			bits, err := theory.bitsFor(term)
			astTestMust(t, err)
			if len(bits) != len(args) {
				t.Fatal("bit width", len(bits))
			}
			for i := range bits {
				if bits[i] != args[i] {
					t.Fatal("MKBV bit identity", i, bits[i], args[i])
				}
			}
			astTestMust(t, s.PopScopes(1))
			if s.context.enodes.get(term) != nil {
				t.Fatal("scoped MKBV enode survived pop")
			}
		})
	}
}
