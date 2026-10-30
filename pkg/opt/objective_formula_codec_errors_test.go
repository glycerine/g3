package opt

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectiveFormulaCodecRejectsMalformedObjectives(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	codec, err := NewObjectiveFormulaCodec(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := codec.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, objective := range []FormulaObjective{
		{},
		{Kind: FormulaMaximize},
		{Kind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{f.p}},
		{Kind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{f.p}, Weights: []*big.Rat{nil}},
	} {
		if _, err := codec.EncodeObjective(objective); err == nil {
			t.Fatal("accepted malformed objective", objective)
		}
	}
	if _, err := codec.Decode([]ast.ASTHandle{{}}); err == nil {
		t.Fatal("accepted null formula")
	}
	if err = codec.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = codec.EncodeObjective(FormulaObjective{Kind: FormulaMaximize, Term: f.p}); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
