package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeModelOwnsLatestOptimalModel(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	_, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	status, err := f.opt.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("optimization status", status)
	}
	model, err := f.opt.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(f.x)
	astTestMust(t, err)
	value, err := model.ConstantInterpretation(declaration)
	astTestMust(t, err)
	text, err := f.c.NumeralString(value)
	astTestMust(t, err)
	if text != "5" {
		t.Fatal("optimized model value", text)
	}
	astTestMust(t, model.Close())
}
