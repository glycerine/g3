package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestASTVectorMutationAndNullResizeSlots(t *testing.T) {
	f, a, b := astVectorFixture(t)
	v, err := f.c.NewASTVectorValues(a)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, v.Close()) })
	astTestMust(t, v.Resize(3))
	astTestMust(t, v.Set(1, b))
	values, err := v.Values()
	astTestMust(t, err)
	if len(values) != 3 || values[0] != a || values[1] != b || values[2] != (ast.ASTHandle{}) {
		t.Fatalf("resized values: %v", values)
	}
	astTestMust(t, v.Push(a))
	if size, err := v.Len(); err != nil || size != 4 {
		t.Fatalf("push size: %d %v", size, err)
	}
	astTestMust(t, v.Resize(1))
	if got, err := v.At(0); err != nil || got != a {
		t.Fatalf("shrink result: %v %v", got, err)
	}
}
