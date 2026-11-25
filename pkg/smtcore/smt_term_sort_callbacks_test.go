package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtCallbackSortTheory struct {
	*smtInternalizerTestTheory
	apply func(*SMTEnode, ast.ASTHandle) error
}

func (t *smtCallbackSortTheory) ApplySortConstraint(n *SMTEnode, s ast.ASTHandle) error {
	return t.apply(n, s)
}

func TestSMTUninterpretedTermRechecksSortTheoryAfterCallback(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	_, _, s := smtInternalizerFixture(t, m, nil)
	base, e := NewSMTTheoryBase(family)
	astTestMust(t, e)
	first := &smtCallbackSortTheory{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}}
	second := &smtCallbackSortTheory{smtInternalizerTestTheory: first.smtInternalizerTestTheory}
	astTestMust(t, s.RegisterTheory(first))
	integer, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	term, e := m.MkConst(astTestSymbol(t, m, "sort-callback-term"), integer)
	astTestMust(t, e)
	astTestPin(t, m, term)
	refusal := errors.New("second constraint refused")
	calls := 0
	first.apply = func(n *SMTEnode, h ast.ASTHandle) error {
		if n.Owner() != term || h != integer || calls != 0 {
			t.Fatal("wrong first constraint")
		}
		calls++
		s.theories[family] = second
		return nil
	}
	second.apply = func(n *SMTEnode, h ast.ASTHandle) error {
		if n.Owner() != term || h != integer || calls != 1 {
			t.Fatal("wrong second constraint")
		}
		calls++
		return refusal
	}
	err := s.Internalize(term, false)
	s.theories[family] = first
	if !errors.Is(err, refusal) || calls != 2 || s.context.enodes.get(term) == nil {
		t.Fatalf("sort callback sequence: calls %d, error %v", calls, err)
	}
}

type smtCallbackTermTheory struct {
	*smtInternalizerTestTheory
	term func(ast.ASTHandle) (bool, error)
}

func (t *smtCallbackTermTheory) InternalizeTerm(h ast.ASTHandle) (bool, error) { return t.term(h) }

func TestSMTBinaryTermInvokesUnattachedChildTheory(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	info, err := m.NewFuncDeclInfo(family, 9001, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	decl, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "theory-child"), nil, integer, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	child, err := m.MkApp(decl, nil)
	astTestMust(t, err)
	plain, err := m.MkConst(astTestSymbol(t, m, "ordinary-child"), integer)
	astTestMust(t, err)
	pair, err := m.MkFuncDecl(astTestSymbol(t, m, "mixed-pair"), []ast.ASTHandle{integer, integer}, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	parent, err := m.MkApp(pair, []ast.ASTHandle{plain, child})
	astTestMust(t, err)
	astTestPin(t, m, parent)
	_, _, s := smtInternalizerFixture(t, m, nil)
	astTestMust(t, s.Internalize(plain, false))
	node, err := s.MkEnode(child, true, false, false)
	astTestMust(t, err)
	base, err := NewSMTTheoryBase(family)
	astTestMust(t, err)
	refusal := errors.New("unattached child theory")
	calls := 0
	theory := &smtCallbackTermTheory{&smtInternalizerTestTheory{SMTTheoryBase: base}, func(h ast.ASTHandle) (bool, error) {
		if h != child {
			t.Fatalf("unexpected theory term %v", h)
		}
		calls++
		return false, refusal
	}}
	astTestMust(t, s.RegisterTheory(theory))
	if err := s.Internalize(parent, false); !errors.Is(err, refusal) || calls != 1 || node.NumParents() != 0 || s.context.enodes.get(parent) != nil {
		t.Fatalf("child callback: calls %d, parents %d, error %v", calls, node.NumParents(), err)
	}
}
