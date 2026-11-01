package rel

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type externalRelationDisplayFailure struct{ err error }

func (w externalRelationDisplayFailure) Write([]byte) (int, error) { return 0, w.err }

func TestExternalRelationDisplayMatchesTupleDisplayAndPropagatesWriterErrors(t *testing.T) {
	f := newTableRelationFixture(t)
	relation, err := NewExternalRelation(f.relations, []ast.ASTHandle{f.boolean}, ExternalRelationCallbacks{})
	astTestMust(t, err)
	defer func() { astTestMust(t, relation.Close()) }()

	var display, tuples bytes.Buffer
	if err = relation.Display(&display); err != nil {
		t.Fatal(err)
	}
	if err = relation.DisplayTuples(f.boolean, &tuples); err != nil {
		t.Fatal(err)
	}
	if display.String() != tuples.String() || !strings.HasSuffix(display.String(), "\n") {
		t.Fatalf("display=%q tuples=%q", display.String(), tuples.String())
	}
	if strings.TrimSpace(display.String()) == "" {
		t.Fatal("empty relation expression was not displayed")
	}

	wantErr := errors.New("display writer failed")
	if err = relation.Display(externalRelationDisplayFailure{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("writer error = %v, want %v", err, wantErr)
	}
	if err = relation.Display(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("nil writer error = %v", err)
	}
	if err = relation.Close(); err != nil {
		t.Fatal(err)
	}
	if err = relation.Display(io.Discard); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("closed relation display error = %v", err)
	}
}
