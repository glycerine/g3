package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestModelFinderHintSatisfiedResidueSourceScopeTransitions(t *testing.T) {
	m := astTestManager(t)
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	qs := make([]ast.ASTHandle, 3)
	for i := range qs {
		body, e := m.MkVar(uint32(i), boolSort)
		astTestMust(t, e)
		qs[i], e = m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{boolSort},
			Names: []symbol.Symbol{symbol.Numerical(uint32(i))}, Body: body,
		})
		astTestMust(t, e)
		astTestMust(t, m.IncRef(qs[i]))
	}
	f := make([]ast.ASTHandle, 2)
	for i := range f {
		f[i], e = m.MkFuncDecl(symbol.Numerical(uint32(i+10)), nil, boolSort, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, m.IncRef(f[i]))
	}
	t.Cleanup(func() {
		for _, h := range f {
			astTestMust(t, m.DecRef(h))
		}
		for _, h := range qs {
			astTestMust(t, m.DecRef(h))
		}
	})

	s := newHintSearchState(m)
	d0, d1 := qs[0], qs[1]
	s.infos[qs[0]] = &hintQuantifierInfo{nonGround: []ast.ASTHandle{f[0]}, macros: []hintConditionalMacro{{function: f[0], definition: d0, satisfyAtom: true}}}
	s.infos[qs[1]] = &hintQuantifierInfo{nonGround: []ast.ASTHandle{f[0], f[1]}, macros: []hintConditionalMacro{{function: f[1], definition: d1, satisfyAtom: true}}}
	s.infos[qs[2]] = &hintQuantifierInfo{nonGround: []ast.ASTHandle{f[1]}}
	astTestMust(t, s.makeQuantifierIndexes(qs))
	if _, e := s.updateSatisfiedResidue(f[0], d0); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("source update requires greedy scope", e)
	}
	s.pushScope()
	useful, e := s.updateSatisfiedResidue(f[0], d0)
	astTestMust(t, e)
	if !useful || !s.satisfied.contains(qs[0]) || !s.residue.contains(qs[1]) || s.residue.contains(qs[0]) || s.infos[qs[0]].theOne != f[0] {
		t.Fatal("first source q_f_def then q_f transition")
	}
	s.pushScope()
	useful, e = s.updateSatisfiedResidue(f[1], d1)
	astTestMust(t, e)
	if !useful || !s.satisfied.contains(qs[1]) || s.residue.contains(qs[1]) || !s.residue.contains(qs[2]) || s.infos[qs[1]].theOne != f[1] {
		t.Fatal("nested source residue-to-satisfied transition")
	}
	useful, e = s.updateSatisfiedResidue(f[1], d1)
	astTestMust(t, e)
	if useful || len(s.satisfied.trail) != 2 || len(s.residue.trail) != 3 {
		t.Fatal("already satisfied q_f_def must return before q_f update")
	}
	astTestMust(t, s.popScope())
	if s.infos[qs[1]].theOne != (ast.ASTHandle{}) || s.satisfied.contains(qs[1]) || !s.residue.contains(qs[1]) || s.residue.contains(qs[2]) || s.infos[qs[0]].theOne != f[0] {
		t.Fatal("satisfied event and residue undo order")
	}
	astTestMust(t, s.popScope())
	if s.infos[qs[0]].theOne != (ast.ASTHandle{}) || !s.satisfied.empty() || !s.residue.empty() {
		t.Fatal("outer source scope restoration")
	}
}
