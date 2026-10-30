package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIOptimizeBoundGuards(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	index, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	if _, err = f.opt.Lower(index); err == nil {
		t.Fatal("bound available before check")
	}
	_, err = f.opt.Check()
	astTestMust(t, err)
	if _, err = f.opt.Upper(index + 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("out-of-range objective", err)
	}
	astTestMust(t, f.opt.Close())
	if _, err = f.opt.Lower(index); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed optimize", err)
	}
}
