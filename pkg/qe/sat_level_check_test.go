package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSATLevelChecksOwnAssignmentsAndModel(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("level-check-p", f.boolean)
	notP := f.own(m.MkNot(p))
	truth := f.own(m.MkTrue())
	levels, err := newSATLevels(m, f.r.config, nil, 1, p, nil)
	fmMust(t, err)
	defer func() { fmMust(t, levels.Close()) }()
	result, err := levels.check(0, p)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	if !result.sat || result.model == nil {
		t.Fatal("satisfiable first level omitted model")
	}
	pDecl, err := m.GetDecl(p)
	fmMust(t, err)
	value, err := result.model.ConstantInterpretation(pDecl)
	fmMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("model did not preserve the checked context")
	}
	if len(result.assignments) == 0 {
		t.Fatal("no solver assignments captured")
	}
	unsat, err := levels.check(1, notP)
	fmMust(t, err)
	defer func() { fmMust(t, unsat.Close()) }()
	if unsat.sat || unsat.model != nil {
		t.Fatal("inconsistent last level returned a model")
	}
	again, err := levels.check(0, truth)
	fmMust(t, err)
	fmMust(t, again.Close())
	for _, ref := range result.assignments {
		h, e := m.RefGet(ref)
		fmMust(t, e)
		if h == (ast.ASTHandle{}) {
			t.Fatal("assignment lost ownership")
		}
	}
	value, err = result.model.ConstantInterpretation(pDecl)
	fmMust(t, err)
	if !m.IsTrue(value) {
		t.Fatal("model changed after later check")
	}
	if _, err = levels.check(2, truth); err == nil {
		t.Fatal("invalid level accepted")
	}
}
