package api

// Source: src/api/api_ast.cpp MK_BINARY/MK_NARY construction macros.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIBooleanEqualityKeepsSourceEqualityDeclaration(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("bool-eq-x", boolean))
	y := f.pin(f.c.Constant("bool-eq-y", boolean))
	equality := f.pin(f.c.Equal(x, y))
	iff := f.pin(f.c.Iff(x, y))
	if equality == iff {
		t.Fatal("API equality collapsed into the AST convenience IFF")
	}
	if got, err := f.c.PublicDeclarationKind(equality); err != nil || got != APIDeclarationEqual {
		t.Fatal("API equality kind", got, err)
	}
	if got, err := f.c.PublicDeclarationKind(iff); err != nil || got != APIDeclarationIff {
		t.Fatal("API IFF kind", got, err)
	}
	for i, want := range []ast.ASTHandle{x, y} {
		got := f.pin(f.c.ApplicationArgument(equality, uint32(i)))
		if got != want {
			t.Fatal("equality argument order", i, got, want)
		}
	}
	interpolant := f.pin(f.c.Interpolant(x))
	if kind, err := f.c.PublicDeclarationKind(interpolant); err != nil || kind != APIDeclarationInterpolant {
		t.Fatal("interpolant marker declaration", kind, err)
	}
	if argument := f.pin(f.c.ApplicationArgument(interpolant, 0)); argument != x {
		t.Fatal("interpolant marker argument", argument, x)
	}
}

func TestAPIBooleanNaryConstructorsDoNotSimplifyArityOrDuplicates(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("nary-x", boolean))
	y := f.pin(f.c.Constant("nary-y", boolean))
	for _, args := range [][]ast.ASTHandle{nil, {x}, {y, x, y}} {
		for _, tc := range []struct {
			build func(...ast.ASTHandle) (ast.ASTHandle, error)
			want  APIDeclarationKind
		}{
			{f.c.And, APIDeclarationAnd}, {f.c.Or, APIDeclarationOr}, {f.c.Distinct, APIDeclarationDistinct},
		} {
			term := f.pin(tc.build(args...))
			if kind, err := f.c.PublicDeclarationKind(term); err != nil || kind != tc.want {
				t.Fatal("nary declaration", kind, tc.want, err)
			}
			if count, err := f.c.ApplicationArgumentCount(term); err != nil || int(count) != len(args) {
				t.Fatal("nary arity", count, len(args), err)
			}
			for i, want := range args {
				if got := f.pin(f.c.ApplicationArgument(term, uint32(i))); got != want {
					t.Fatal("nary argument order", i, got, want)
				}
			}
		}
	}
}
