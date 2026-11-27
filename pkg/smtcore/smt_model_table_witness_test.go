package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTUnaryWitnessUsesFreshFalsifyingInverse(t *testing.T) {
	m := astTestManager(t)
	no, err := m.MkFalse()
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	first, err := m.MkConst(astTestSymbol(t, m, "first"), boolean)
	astTestMust(t, err)
	next, err := m.MkConst(astTestSymbol(t, m, "next"), boolean)
	astTestMust(t, err)
	set, err := newSMTModelFinderInstantiationSet(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, set.Close()) })
	_, err = set.Insert(first, 1)
	astTestMust(t, err)
	_, err = set.Insert(next, 0)
	astTestMust(t, err)
	astTestMust(t, set.MakeInverse(func(term ast.ASTHandle) (ast.ASTHandle, error) {
		if term == first {
			return no, nil
		}
		return yes, nil
	}))
	repeated := []smtModelCheckerInstance{{bindings: []ast.ASTHandle{first}}}
	both := []smtModelCheckerInstance{repeated[0], {bindings: []ast.ASTHandle{next}}}
	oldBinding := make([]smtModelCheckerInstance, 33)
	oldBinding[0] = repeated[0]
	pair := []smtModelCheckerInstance{{bindings: []ast.ASTHandle{first, next}}}
	domain := []int{tableFalse, tableTrue}
	for _, tc := range []struct {
		name           string
		pending        []smtModelCheckerInstance
		domain         []int
		root, budget   int
		original, want ast.ASTHandle
	}{
		{"alternative also falsifies", repeated, domain, 0, 8, no, yes},
		{"alternative satisfies", repeated, domain, 1, 8, no, no},
		{"no previous binding", nil, domain, 0, 8, no, no},
		{"previous binary instance", pair, domain, 0, 8, no, no},
		{"outside lookback", oldBinding, domain, 0, 8, no, no},
		{"already queued both", both, domain, 0, 8, no, no},
		{"alternative outside domain", repeated, domain[:1], 0, 8, no, no},
		{"generation zero", both[1:], domain, 0, 8, yes, yes},
		{"budget exhausted", repeated, domain, 0, 1, no, no},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tableModel{
				valueIDs: map[ast.ASTHandle]int{no: tableFalse, yes: tableTrue},
				sorts:    []ast.ASTHandle{{}, boolean, boolean},
				terms:    []tableTerm{{op: tableConstant, index: tableFalse}, {op: tableVariable, sort: boolean}},
				work:     tc.budget,
			}
			got, err := g.unaryWitness(tc.root, tc.domain, set, tc.pending, tc.original)
			astTestMust(t, err)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
