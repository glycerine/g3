package nlsat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSolverExactInequalityAtomAndEmptyLiteralFactories(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	for _, tc := range []struct {
		kind AtomKind
		want sat.Literal
	}{{AtomLT, FalseLiteral}, {AtomEQ, FalseLiteral}, {AtomGT, TrueLiteral}} {
		got, err := s.MkInequalityLiteral(tc.kind, nil, nil)
		assignmentMust(t, err)
		if got != tc.want {
			t.Fatal("empty product literal", tc.kind, got, tc.want)
		}
	}
	constant := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{2, 0, 0})
	variable, err := s.MkInequalityAtom(AtomGT, []polynomial.Polynomial{constant}, []bool{false})
	assignmentMust(t, err)
	if variable == TrueBooleanVariable || s.atoms.bools.atoms[variable] == nil {
		t.Fatal("constant atom was folded", variable)
	}
	literal, err := s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{constant}, []bool{false})
	assignmentMust(t, err)
	if literal.Variable() != variable || literal.Sign() {
		t.Fatal("literal factory missed canonical constant atom", literal, variable)
	}
	folded, err := s.MkInequality(AtomGT, []polynomial.Polynomial{constant}, []bool{false}, false)
	assignmentMust(t, err)
	if folded != TrueLiteral {
		t.Fatal("convenience constant fold changed", folded)
	}

	if _, err = s.MkInequalityAtom(AtomGT, nil, nil); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("empty atom accepted", err)
	}
	if _, err = s.MkInequalityLiteral(AtomRootEQ, nil, nil); !errors.Is(err, arena.ErrConfig) {
		t.Fatal("root kind accepted by inequality factory", err)
	}
	if _, err = s.MkInequalityLiteral(AtomGT, []polynomial.Polynomial{constant}, nil); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("parity mismatch accepted", err)
	}
}

func TestSolverExactAtomFactoriesNormalizeAndCanonicalize(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	negative := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{-1, 1, 0})
	b, err := s.MkInequalityAtom(AtomLT, []polynomial.Polynomial{negative}, []bool{false})
	assignmentMust(t, err)
	atom, found, err := s.AtomForBooleanVariable(b)
	assignmentMust(t, err)
	if !found || arithmeticAtomHeader(atom).Kind() != AtomGT {
		t.Fatal("leading-sign normalization", atom)
	}
	literal, err := s.MkInequalityLiteral(AtomLT, []polynomial.Polynomial{negative}, []bool{false})
	assignmentMust(t, err)
	if literal.Variable() != b {
		t.Fatal("inequality canonical identity", literal, b)
	}
	rootVariable, err := s.MkRootAtom(AtomRootEQ, 0, 1, negative)
	assignmentMust(t, err)
	rootLiteral, err := s.MkRoot(AtomRootEQ, 0, 1, negative, true)
	assignmentMust(t, err)
	if rootLiteral.Variable() != rootVariable || !rootLiteral.Sign() {
		t.Fatal("root canonical identity", rootLiteral, rootVariable)
	}
	atomDebugFixture(t)
	want := error(arena.ErrConfig)
	if diagnostic.DebugBuild {
		want = diagnostic.ErrDebugAssertion
	}
	if _, err = s.MkRootAtom(AtomRootEQ, 0, 0, negative); !errors.Is(err, want) {
		t.Fatal("zero root index accepted", err)
	}
}
