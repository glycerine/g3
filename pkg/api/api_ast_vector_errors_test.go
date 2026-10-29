package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestASTVectorBoundsAndForeignHandles(t *testing.T) {
	f, a, _ := astVectorFixture(t)
	v, err := f.c.NewASTVectorValues(a)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, v.Close()) })
	if _, err = v.At(1); !errors.Is(err, arena.ErrBounds) {
		t.Fatalf("get bounds: %v", err)
	}
	if err = v.Set(1, a); !errors.Is(err, arena.ErrBounds) {
		t.Fatalf("set bounds: %v", err)
	}
	foreign := newSMTAPIFixture(t, "QF_UF", nil)
	foreignSort := foreign.pin(foreign.m.MkBoolSort())
	if err = v.Push(foreignSort); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("foreign value: %v", err)
	}
	base, err := f.m.GetRefCount(a)
	astTestMust(t, err)
	if _, err = f.c.NewASTVectorValues(a, foreignSort); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("foreign constructor value: %v", err)
	}
	after, err := f.m.GetRefCount(a)
	astTestMust(t, err)
	if after != base {
		t.Fatalf("failed constructor retained prefix: got %d want %d", after, base)
	}
}
