package rel

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type intervalSieveDisplayRefusal struct{ err error }

func (w intervalSieveDisplayRefusal) Write([]byte) (int, error) { return 0, w.err }

func TestIntervalSieveDisplaySourceMaskAndInnerState(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relation, err := NewIntervalSieveFull(f.ast, []ast.ASTHandle{f.boolean, integer, f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	if err = relation.FilterEqual(1, numeral("4")); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = relation.Display(&output); err != nil {
		t.Fatal(err)
	}
	const want = "Sieve relation (0,1,0)\n0 in [4, 4]\n\n"
	if output.String() != want {
		t.Fatalf("display: got %q, want %q", output.String(), want)
	}
	found, err := relation.ContainsFact([]ast.ASTHandle{f.falsehood, numeral("4"), f.truth})
	if err != nil || !found {
		t.Fatal("display mutated relation", found, err)
	}
}

func TestIntervalSieveDisplayEmptyNullaryAndWriterFailure(t *testing.T) {
	f, _, _ := intervalSieveFixture(t)
	nullary, err := NewIntervalSieveFull(f.ast, nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = nullary.Display(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "Sieve relation ()\n\n" {
		t.Fatalf("nullary display: %q", output.String())
	}

	empty, err := NewIntervalSieveEmpty(f.ast, nil)
	if err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err = empty.Display(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "Sieve relation ()\nempty\n" {
		t.Fatalf("empty display: %q", output.String())
	}

	refusal := errors.New("refused")
	if err = nullary.Display(intervalSieveDisplayRefusal{err: refusal}); !errors.Is(err, refusal) {
		t.Fatal("writer refusal", err)
	}
	if err = nullary.Display(nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("nil writer", err)
	}
	var nilRelation *IntervalSieve
	if err = nilRelation.Display(&output); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("nil relation", err)
	}
}
