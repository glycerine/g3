package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITacticResultConvertModelGuardsAndIndependentLifetime(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	constant := f.pin(f.c.Constant("api-result-guards", boolean))
	declaration, err := f.m.GetDecl(constant)
	astTestMust(t, err)
	yes := f.pin(f.c.Bool(true))
	result := apiTacticModelResult(t, f, declaration, yes)
	source, err := f.c.NewAPIModel()
	astTestMust(t, err)
	if _, err = result.ConvertModel(1, source); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("subgoal bounds", err)
	}
	foreign := newSMTAPIFixture(t, "ALL", nil)
	foreignModel, err := foreign.c.NewAPIModel()
	astTestMust(t, err)
	if _, err = result.ConvertModel(0, foreignModel); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign model", err)
	}
	converted, err := result.ConvertModel(0, source)
	astTestMust(t, err)
	astTestMust(t, result.Close())
	if value, e := converted.ConstantInterpretation(declaration); e != nil || value != yes {
		t.Fatal("converted model did not outlive result", value, e)
	}
	if _, err = result.ConvertModel(0, source); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed result", err)
	}
	astTestMust(t, converted.Close())
	astTestMust(t, source.Close())
	astTestMust(t, foreignModel.Close())
}
