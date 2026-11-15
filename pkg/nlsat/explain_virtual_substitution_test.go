package nlsat

import "testing"

func TestSignedProjectionLinearEqualityUsesVirtualSubstitution(t *testing.T) {
	for _, tc := range []struct {
		name       string
		equality   []evaluatorTerm
		other      []evaluatorTerm
		wantKind   AtomKind
		wantResult []evaluatorTerm
		x0, x1     int64
	}{
		{
			name:       "positive transformed value",
			equality:   []evaluatorTerm{{1, 0, 1}, {-1, 1, 0}},
			other:      []evaluatorTerm{{1, 0, 2}, {-1, 1, 0}},
			wantKind:   AtomGT,
			wantResult: []evaluatorTerm{{1, 2, 0}, {-1, 1, 0}},
			x0:         2,
			x1:         2,
		},
		{
			name:     "negative leading coefficient",
			equality: []evaluatorTerm{{-2, 0, 1}, {1, 1, 0}},
			other:    []evaluatorTerm{{1, 0, 1}, {1, 1, 0}},
			// addSimpleAssumption canonicalizes the negative leading sign,
			// so -3*x0 < 0 is stored as 3*x0 > 0.
			wantKind:   AtomGT,
			wantResult: []evaluatorTerm{{3, 1, 0}},
			x0:         2,
			x1:         1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, r, n, _ := projectionCoefficientFixture(t, true)
			evaluatorSetFraction(t, e, 0, tc.x0, 1)
			evaluatorSetFraction(t, e, 1, tc.x1, 1)
			equality := evaluatorPolynomial(t, e, tc.equality...)
			other := evaluatorPolynomial(t, e, tc.other...)
			ps := projectionOutput(t, e)
			assignmentMust(t, ps.Push(equality))
			assignmentMust(t, ps.Push(other))
			assignmentMust(t, n.collectSignedProjection(ps, 1))
			if len(n.output.result) != 1 {
				t.Fatalf("virtual substitution assumptions = %v", n.output.result)
			}
			atom, ok := r.store.atoms.bools.atoms[n.output.result[0].Variable()].(*InequalityAtom)
			if !ok || atom.kind != tc.wantKind || len(atom.factors) != 1 || atom.factors[0].Even {
				t.Fatalf("virtual substitution atom = %#v", atom)
			}
			want := evaluatorPolynomial(t, e, tc.wantResult...)
			equal, err := e.core.Equal(atom.factors[0].Polynomial, want)
			assignmentMust(t, err)
			if !equal {
				t.Fatal("virtual substitution polynomial differs")
			}
		})
	}
}

func TestSignedSolveLinearEqualityRejectsNonlinearSelection(t *testing.T) {
	e, _, n, _ := projectionCoefficientFixture(t, true)
	atomDebugFixture(t)
	evaluatorSetFraction(t, e, 0, 0, 1)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{-1, 0, 0})
	ps := projectionOutput(t, e)
	assignmentMust(t, ps.Push(p))
	if err := n.signedSolveLinearEquality(0, 0, ps); err == nil {
		t.Fatal("nonlinear equality accepted by linear substitution")
	}
	// The rejected path must not alter its borrowed input vector.
	values := projectionValues(t, ps)
	if len(values) != 1 || values[0] != p {
		t.Fatal("rejected substitution changed input", values)
	}
}

// z3 4.7.1 never resets solve_eq's table qs of powers of A, so for q2,
// whose degree is above q1's, it reads q1's block and computes the
// constant -2 in place of 1 - 3*x0^2, dropping its sign condition
// (nlsat_explain.cpp:1723-1761; fixed upstream in 19e2f8c9d; AUDIT.md
// nlsat_01-F3). g3 rebuilds the powers for each polynomial: the result
// holds the signs of A = x0, of x0^2 + 1 and of 3*x0^2 - 1.
func TestSignedSolveLinearEqualityPowerTableAcrossPolynomials(t *testing.T) {
	e, _, n, _ := projectionCoefficientFixture(t, true)
	evaluatorSetFraction(t, e, 0, 2, 1)
	evaluatorSetFraction(t, e, 1, 1, 2)
	equality := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 1}, evaluatorTerm{-1, 0, 0})
	q1 := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{1, 1, 0})
	q2 := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{-3, 0, 0})
	ps := projectionOutput(t, e)
	assignmentMust(t, ps.Push(equality))
	assignmentMust(t, ps.Push(q1))
	assignmentMust(t, ps.Push(q2))
	assignmentMust(t, n.collectSignedProjection(ps, 1))
	if len(n.output.result) != 3 {
		t.Fatal("virtual substitution lost a sign condition", n.output.result)
	}
}
