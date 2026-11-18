package opt

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectiveDecomposerLifecycleAndInvalidKinds(t *testing.T) {
	if _, err := NewObjectiveDecomposer(nil, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	f := newObjectiveFormulaFixture(t)
	decomposer, err := NewObjectiveDecomposer(f.manager, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = decomposer.Decompose(DecodedFormulaSet{Objectives: []DecodedFormulaObjective{{Kind: 99, Term: f.p}}}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if err = decomposer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err = decomposer.Decompose(DecodedFormulaSet{}); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if err = decomposer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDecomposedObjectiveRestoreValueGuardsAndOrder(t *testing.T) {
	q := newObjectiveFormulaFixture(t).manager.NumeralService().Manager
	out := testRat(0, 1)
	if err := (*DecomposedObjective)(nil).RestoreValue(q, testRatPtr(1, 1), &out); !errors.Is(err, ErrInvalidOptimizationValue) {
		t.Fatal(err)
	}
	objective := DecomposedObjective{Offset: testRat(7, 3), Negate: true}
	if err := objective.RestoreValue(q, nil, &out); !errors.Is(err, ErrInvalidOptimizationValue) {
		t.Fatal(err)
	}
	penalty := testRat(5, 2)
	err := objective.RestoreValue(q, &penalty, &out)
	if err != nil || ratString(&out) != "-1/6" || ratString(&penalty) != "5/2" {
		t.Fatal(ratString(&out), ratString(&penalty), err)
	}
}

func restoreValueT(t *testing.T, o *DecomposedObjective, penalty *numeral.Rational) (numeral.Rational, error) {
	t.Helper()
	q := newObjectiveFormulaFixture(t).manager.NumeralService().Manager
	out := testRat(0, 1)
	return out, o.RestoreValue(q, penalty, &out)
}
