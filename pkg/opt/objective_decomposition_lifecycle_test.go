package opt

import (
	"errors"
	"math/big"
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
	if _, err := (*DecomposedObjective)(nil).RestoreValue(big.NewRat(1, 1)); !errors.Is(err, ErrInvalidOptimizationValue) {
		t.Fatal(err)
	}
	objective := DecomposedObjective{Offset: *big.NewRat(7, 3), Negate: true}
	if _, err := objective.RestoreValue(nil); !errors.Is(err, ErrInvalidOptimizationValue) {
		t.Fatal(err)
	}
	penalty := big.NewRat(5, 2)
	value, err := objective.RestoreValue(penalty)
	if err != nil || value.RatString() != "-1/6" || penalty.RatString() != "5/2" {
		t.Fatal(value, penalty, err)
	}
}
