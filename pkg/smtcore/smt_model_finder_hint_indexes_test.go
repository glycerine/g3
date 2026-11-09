package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestModelFinderHintPreprocessIndexesAndResidueCandidates(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	qs := make([]ast.ASTHandle, 5)
	for i := range qs {
		body, e := m.MkVar(uint32(i), b)
		astTestMust(t, e)
		qs[i], e = m.MkQuantifier(ast.ASTQuantifierSpec{
			Forall: true, Sorts: []ast.ASTHandle{b},
			Names: []symbol.Symbol{symbol.Numerical(uint32(i))}, Body: body,
		})
		astTestMust(t, e)
		astTestMust(t, m.IncRef(qs[i]))
	}
	fs := make([]ast.ASTHandle, 4)
	for i := range fs {
		fs[i], e = m.MkFuncDecl(symbol.Numerical(uint32(40+i)), nil, b, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, m.IncRef(fs[i]))
	}
	t.Cleanup(func() {
		for _, f := range fs {
			astTestMust(t, m.DecRef(f))
		}
		for _, q := range qs {
			astTestMust(t, m.DecRef(q))
		}
	})
	s := newHintSearchState(m)
	s.infos[qs[0]] = &hintQuantifierInfo{macros: []hintConditionalMacro{{function: fs[0], definition: qs[0], satisfyAtom: true}}}
	s.infos[qs[1]] = &hintQuantifierInfo{nonGround: []ast.ASTHandle{fs[0]}}
	s.infos[qs[2]] = &hintQuantifierInfo{nonGround: []ast.ASTHandle{fs[1]}, macros: []hintConditionalMacro{{function: fs[0], definition: qs[2], satisfyAtom: true}}}
	s.infos[qs[3]] = &hintQuantifierInfo{macros: []hintConditionalMacro{{function: fs[1], definition: qs[3], satisfyAtom: true}}}
	s.infos[qs[4]] = &hintQuantifierInfo{
		nonGround: []ast.ASTHandle{fs[2]},
		macros: []hintConditionalMacro{
			{function: fs[2], definition: qs[4], satisfyAtom: true},
			{function: fs[3], definition: qs[4], satisfyAtom: false},
		},
	}
	kept, rejected, e := s.preprocess(qs)
	astTestMust(t, e)
	if !equalHintHandles(kept, qs[4:]) || !equalHintHandles(rejected, []ast.ASTHandle{qs[1], qs[2], qs[3], qs[0]}) {
		t.Fatal("source iterative candidate filtering and forbidden publication", kept, rejected)
	}
	if s.forbidden.size() != 2 {
		t.Fatal("forbidden declarations", s.forbidden)
	}
	astTestMust(t, s.makeQuantifierIndexes(kept))
	key := hintFunctionDefinition{function: fs[2], definition: qs[4]}
	if s.qByFunction[fs[2]].size() != 1 || s.qByDefinition[key].size() != 1 || s.defsByFunction[fs[2]].size() != 1 || s.candidates.size() != 1 {
		t.Fatal("Q_f, Q_f_def, f2defs and initial candidate indexes")
	}
	s.residue.insert(qs[4])
	fromResidue, e := s.candidatesFromResidue()
	astTestMust(t, e)
	if fromResidue.size() != 1 {
		t.Fatal("residue candidate filter", fromResidue)
	}
	if !fromResidue.has(fs[2]) {
		t.Fatal("allowed satisfying macro absent")
	}
	astTestMust(t, s.selectDefinition(fs[2], qs[4]))
	fromResidue, e = s.candidatesFromResidue()
	astTestMust(t, e)
	if fromResidue.size() != 0 {
		t.Fatal("already-selected macro head remained a residue candidate", fromResidue)
	}
	s.forgetDefinition(fs[2])
	s.residue.reset()
	s.pushScope()
	astTestMust(t, s.selectDefinition(fs[2], qs[4]))
	useful, e := s.updateSatisfiedResidue(fs[2], qs[4])
	astTestMust(t, e)
	if !useful || !s.satisfied.contains(qs[4]) || !s.residue.empty() || s.infos[qs[4]].theOne != fs[2] {
		t.Fatal("indexed update_satisfied_residue")
	}
	cyclic, e := s.isCyclic(m)
	astTestMust(t, e)
	if cyclic {
		t.Fatal("selected definition with no selected function occurrence")
	}
	astTestMust(t, s.popScope())
	s.forgetDefinition(fs[2])
	if s.infos[qs[4]].theOne != (ast.ASTHandle{}) || !s.satisfied.empty() {
		t.Fatal("indexed hint state rollback")
	}
}

func equalHintHandles(a, b []ast.ASTHandle) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
