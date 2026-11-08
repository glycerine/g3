package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// smtDiseqFixture builds enodes for count constants plus assumption literals
// used to justify merges, and returns the expanded conflict antecedents helper.
func smtDiseqFixture(t *testing.T, prefix string, count, literals int) (*ast.ASTManager, *SMTInternalizerState, []*SMTEnode, []SMTLiteral) {
	t.Helper()
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	_, constants := smtUFConstants(t, m, prefix, count)
	nodes := make([]*SMTEnode, count)
	for i, constant := range constants {
		node, err := s.MkEnode(constant, false, false, true)
		astTestMust(t, err)
		nodes[i] = node
	}
	result := make([]SMTLiteral, literals)
	for i := range result {
		atom := smtRelevancyBool(t, m, prefix+"-just-"+string(rune('a'+i)))
		variable, err := s.MkBoolVar(atom)
		astTestMust(t, err)
		result[i] = NewSMTLiteral(variable, false)
	}
	return m, s, nodes, result
}
