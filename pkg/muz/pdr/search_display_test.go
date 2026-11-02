package pdr

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"io"
	"testing"
)

func TestModelSearchAndContextWriteDisplayTreeAndGoals(t *testing.T) {
	m, owner, transformer, _, closeBase := inductionTestTransformer(t, inductionTestEmpty)
	defer inductionCloseFixture(t, owner, transformer, closeBase)
	search := NewModelSearch(true)
	defer func() { closureMust(t, search.Close()) }()
	var out bytes.Buffer
	closureMust(t, search.WriteDisplay(&out))
	if out.String() != "goals 0\n" {
		t.Fatal(out.String())
	}
	truth, err := m.MkTrue()
	closureMust(t, err)
	falsity, err := m.MkFalse()
	closureMust(t, err)
	root, err := NewModelNode(nil, truth, transformer, 2)
	closureMust(t, err)
	closureMust(t, search.SetRoot(root))
	_, _, err = search.Next()
	closureMust(t, err)
	child, err := NewModelNode(root, falsity, transformer, 1)
	closureMust(t, err)
	closureMust(t, search.AddLeaf(child))
	second, err := NewModelNode(root, truth, transformer, 1)
	closureMust(t, err)
	closureMust(t, search.AddLeaf(second))
	truthID, err := m.GetASTID(truth)
	closureMust(t, err)
	falseID, err := m.GetASTID(falsity)
	closureMust(t, err)
	want := fmt.Sprintf("2 closure_predicate open\n  true %d\n 1 closure_predicate open\n   false %d\n 1 closure_predicate open\n   true %d\ngoals 2\n 1 closure_predicate open\n   false %d\n 1 closure_predicate open\n   true %d\n", truthID, falseID, truthID, falseID, truthID)
	out.Reset()
	closureMust(t, search.WriteDisplay(&out))
	if out.String() != want {
		t.Fatalf("got %q want %q", out.String(), want)
	}
	c := &Context{manager: m, owner: owner, search: search, order: []ast.ASTHandle{transformer.head}, transformers: map[ast.ASTHandle]*PredicateTransformer{transformer.head: transformer}}
	out.Reset()
	closureMust(t, c.WriteDisplay(&out))
	if out.String() != "transition\nfalse\n"+want {
		t.Fatalf("context %q", out.String())
	}
	for _, limit := range []int{0, 10, len(want) - 1} {
		if err := search.WriteDisplay(&pdrDisplayFailWriter{remaining: limit}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	}
	out.Reset()
	closureMust(t, search.WriteDisplay(&out))
	if out.String() != want {
		t.Fatal("display altered frontier")
	}
	closureMust(t, child.SetPreClosed())
	out.Reset()
	closureMust(t, child.WriteDisplay(&out, 2))
	if out.String() != fmt.Sprintf("  1 closure_predicate closed\n    false %d\n", falseID) {
		t.Fatalf("closed node %q", out.String())
	}
	if err := c.WriteDisplay(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	closureMust(t, search.Close())
	if err := search.WriteDisplay(&out); !errors.Is(err, ErrModelSearchOwner) {
		t.Fatal(err)
	}
	if err := root.WriteDisplay(&out, 0); !errors.Is(err, ErrModelSearchOwner) {
		t.Fatal(err)
	}
}
