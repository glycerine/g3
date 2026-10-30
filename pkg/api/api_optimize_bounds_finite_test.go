package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func optimizeBoundNumeral(t *testing.T, f *apiOptimizeFixture, ref *ast.ASTRef) string {
	t.Helper()
	handle, err := f.m.RefGet(*ref)
	astTestMust(t, err)
	text, _, matched, err := f.m.ArithmeticNumeralText(handle)
	astTestMust(t, err)
	if !matched {
		t.Fatal("bound is not a numeral", handle)
	}
	return text
}

func TestAPIOptimizeFiniteLowerUpper(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	index, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	_, err = f.opt.Check()
	astTestMust(t, err)
	for _, get := range []func(uint32) (ast.ASTRef, error){f.opt.Lower, f.opt.Upper} {
		ref, getErr := get(index)
		astTestMust(t, getErr)
		if text := optimizeBoundNumeral(t, f, &ref); text != "5" {
			t.Fatal("bound", text)
		}
		astTestMust(t, f.m.DelRef(&ref))
	}
}
