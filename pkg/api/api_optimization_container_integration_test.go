package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOptimizationBoundStoredInASTContainers(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	index, err := f.opt.Maximize(f.x)
	astTestMust(t, err)
	status, err := f.opt.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatalf("optimization status: %v", status)
	}
	bound, err := f.opt.Upper(index)
	astTestMust(t, err)
	value, err := f.m.RefGet(bound)
	astTestMust(t, err)
	mapping, err := f.c.NewASTMap()
	astTestMust(t, err)
	defer func() { astTestMust(t, mapping.Close()) }()
	vector, err := f.c.NewASTVector()
	astTestMust(t, err)
	defer func() { astTestMust(t, vector.Close()) }()
	astTestMust(t, mapping.Insert(f.x, value))
	astTestMust(t, vector.Push(value))
	astTestMust(t, f.m.DelRef(&bound))
	astTestMust(t, f.opt.Close())
	fromMap, err := mapping.Find(f.x)
	astTestMust(t, err)
	fromVector, err := vector.At(0)
	astTestMust(t, err)
	if fromMap != fromVector {
		t.Fatal("containers changed the objective bound")
	}
	text, err := f.c.NumeralString(fromMap)
	astTestMust(t, err)
	if text != "5" {
		t.Fatalf("objective bound: %s", text)
	}
}
