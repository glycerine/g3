package nlsat

import "testing"

func TestSignedProjectionMovesLowerStagePolynomialsToAssumptions(t *testing.T) {
	e, r, n, _ := projectionCoefficientFixture(t, true)
	evaluatorSetFraction(t, e, 0, 2, 1)
	evaluatorSetFraction(t, e, 1, 0, 1)
	lower := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 0}, evaluatorTerm{-1, 0, 0})
	upper := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{1, 0, 0})
	ps := projectionOutput(t, e)
	assignmentMust(t, ps.Push(lower))
	assignmentMust(t, ps.Push(upper))
	assignmentMust(t, n.collectSignedProjection(ps, 1))

	values := projectionValues(t, ps)
	if len(values) != 1 || values[0] != upper {
		t.Fatal("signed projection did not compact lower-stage input", values)
	}
	if len(n.output.result) != 1 {
		t.Fatal("missing lower-stage sign assumption", n.output.result)
	}
	atom, ok := r.store.atoms.bools.atoms[n.output.result[0].Variable()].(*InequalityAtom)
	if !ok || atom.kind != AtomGT || atom.maxVar != 0 {
		t.Fatal("wrong lower-stage sign assumption", atom)
	}
}

func TestSignedProjectionInfinityUsesLeadingCoefficientSigns(t *testing.T) {
	for _, tc := range []struct {
		name string
		x    int64
	}{
		{name: "plus", x: 4},
		{name: "minus", x: -4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, r, n, _ := projectionCoefficientFixture(t, true)
			evaluatorSetFraction(t, e, 0, 2, 1)
			evaluatorSetFraction(t, e, 1, tc.x, 1)
			// Both roots lie on the same side of the model value. Their
			// nonconstant positive leading coefficients must be retained.
			p := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 1}, evaluatorTerm{1, 0, 0})
			q := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 1}, evaluatorTerm{3, 0, 0})
			ps := projectionOutput(t, e)
			assignmentMust(t, ps.Push(p))
			assignmentMust(t, ps.Push(q))
			assignmentMust(t, n.collectSignedProjection(ps, 1))
			if len(n.output.result) != 1 {
				t.Fatal("leading coefficient assumptions should canonicalize", n.output.result)
			}
			atom, ok := r.store.atoms.bools.atoms[n.output.result[0].Variable()].(*InequalityAtom)
			if !ok || atom.kind != AtomGT || atom.maxVar != 0 {
				t.Fatal("wrong infinity leading-coefficient assumption", atom)
			}
		})
	}
}

func TestSignedProjectionEqualityUsesGeneralPairProjection(t *testing.T) {
	e, _, n, _ := projectionCoefficientFixture(t, true)
	evaluatorSetFraction(t, e, 0, 1, 1)
	evaluatorSetFraction(t, e, 1, 0, 1)
	equality := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
	other := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1}, evaluatorTerm{1, 1, 0})
	ps := projectionOutput(t, e)
	assignmentMust(t, ps.Push(equality))
	assignmentMust(t, ps.Push(other))
	assignmentMust(t, n.collectSignedProjection(ps, 1))
	if len(n.output.result) == 0 {
		t.Fatal("equality pair projection omitted its lower-stage cell")
	}
}
