package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func TestSolverResetDeletesUnreferencedInequalityAndRootAtoms(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 2, 0}, evaluatorTerm{-2, 0, 0})
	ineq, err := s.MkInequalityAtom(AtomGT, []polynomial.Polynomial{p}, []bool{false})
	assignmentMust(t, err)
	root, err := s.MkRootAtom(AtomRootEQ, 0, 1, p)
	assignmentMust(t, err)
	if s.atoms.bools.atoms[ineq] == nil || s.atoms.bools.atoms[root] == nil {
		t.Fatal("fixture atoms")
	}
	assignmentMust(t, s.Reset())
	if !s.atoms.bools.dead[ineq] || !s.atoms.bools.dead[root] || s.atoms.bools.atoms[ineq] != nil || s.atoms.bools.atoms[root] != nil {
		t.Fatal("zero-reference atoms survived reset")
	}
	if len(s.atoms.ineq.owned) != 0 || len(s.atoms.ownedRoots) != 0 {
		t.Fatal("factory owners survived", len(s.atoms.ineq.owned), len(s.atoms.ownedRoots))
	}
}
