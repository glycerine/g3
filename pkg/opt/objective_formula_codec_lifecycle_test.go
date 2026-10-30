package opt

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectiveFormulaSetOwnsMarkersAndClosesIdempotently(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	set, err := NewObjectiveFormulaSet(f.manager, []ast.ASTHandle{f.p}, []FormulaObjective{{Kind: FormulaMinimize, Term: f.q}})
	if err != nil {
		t.Fatal(err)
	}
	handles, err := set.Handles()
	if err != nil || len(handles) != 2 || handles[0] != f.p {
		t.Fatal(handles, err)
	}
	decoded, err := set.Decode()
	if err != nil || len(decoded.Hard) != 1 || len(decoded.Objectives) != 1 || !decoded.Objectives[0].Negate {
		t.Fatal(decoded, err)
	}
	if err = set.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = set.Handles(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if _, err = set.Decode(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if err = set.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestObjectiveFormulaCodecRejectsNilManager(t *testing.T) {
	if _, err := NewObjectiveFormulaCodec(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if _, err := NewObjectiveFormulaSet(nil, nil, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
}
