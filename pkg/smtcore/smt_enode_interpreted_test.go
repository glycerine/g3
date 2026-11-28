package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTSuppressedEnodeInterpretedUsesOwnerArity(t *testing.T) {
	m, _, solver := smtDatatypeSolver(t)
	s := solver.kernel.internalizer
	constructors, err := m.DatatypeConstructors(smtDatatypePolicySort(t, m, true))
	astTestMust(t, err)
	base, err := m.MkApp(constructors[0], nil)
	astTestMust(t, err)
	next, err := m.MkApp(constructors[1], []ast.ASTHandle{base})
	astTestMust(t, err)
	astTestPin(t, m, next)
	for _, tc := range []struct {
		name        string
		owner       ast.ASTHandle
		interpreted bool
	}{
		{"base", base, true},
		{"next(base)", next, false},
	} {
		unique, err := m.IsUniqueValue(tc.owner)
		astTestMust(t, err)
		if !unique {
			t.Fatalf("%s must be a unique value for this test", tc.name)
		}
		node, err := s.MkEnode(tc.owner, true, false, false)
		astTestMust(t, err)
		if node.NumArgs() != 0 || node.IsInterpreted() != tc.interpreted {
			t.Errorf("%s with suppressed arguments: arity=%d interpreted=%v, want 0 and %v", tc.name, node.NumArgs(), node.IsInterpreted(), tc.interpreted)
		}
	}
}
