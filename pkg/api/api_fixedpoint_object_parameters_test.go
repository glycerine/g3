package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointParametersAreOwnedAndLocal(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	other, err := f.c.NewFixedpoint()
	astTestMust(t, err)
	defer func() { astTestMust(t, other.Close()) }()
	p, err := other.DeclareRelation("other-parameter-query", nil)
	astTestMust(t, err)
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "duality"))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	for _, query := range []func() (lbool.LBool, error){
		func() (lbool.LBool, error) { return f.fp.Query(f.p) },
		func() (lbool.LBool, error) { return f.fp.QueryExpression(f.pAtom) },
		func() (lbool.LBool, error) { return f.fp.QueryRelations([]ast.ASTHandle{f.p}) },
		func() (lbool.LBool, error) { return f.fp.QueryExpressionFromLevel(f.pAtom, 0) },
	} {
		if _, err = query(); err == nil || !strings.Contains(err.Error(), "duality is not ported") {
			t.Fatal("local engine not applied", err)
		}
	}
	if _, err = other.Query(p); err != nil {
		t.Fatal("another fixedpoint inherited local options", err)
	}
	params, err = f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, params.SetSymbol("engine", "pdr"))
	astTestMust(t, f.fp.SetParameters(params))
	astTestMust(t, params.Close())
	if _, err = f.fp.Query(f.p); err != nil {
		t.Fatal("replacement engine", err)
	}
}

func TestAPIFixedpointParametersValidateCatalog(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	params, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, params.Close()) }()
	astTestMust(t, params.SetBool("unknown_fixedpoint_setting", true))
	if err = f.fp.SetParameters(params); err == nil {
		t.Fatal("unknown fixedpoint option accepted")
	}
}
