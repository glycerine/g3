package opt

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectivePurifierLifecycleAndModelGuards(t *testing.T) {
	if _, err := NewObjectivePurifier(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	f := newObjectiveFormulaFixture(t)
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = purifier.FilterModel(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err = purifier.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = purifier.Statistics(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if _, err = purifier.Purify(nil, nil); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if _, err = purifier.FilterModel(nil); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	if err = purifier.Close(); err != nil {
		t.Fatal(err)
	}
}
