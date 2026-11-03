package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
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
	} {
		status, err := query()
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("empty local Duality query", status)
		}
		if answer, err := f.fp.Answer(f.p); answer != (ast.ASTRef{}) || !errors.Is(err, fp.ErrPropositionalEngine) {
			t.Fatal("local Duality get_answer guard", answer, err)
		}
	}
	if _, err := f.fp.QueryExpressionFromLevel(f.pAtom, 0); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("Duality accepted Spacer level query", err)
	}
	if _, err = other.Query(p); err != nil {
		t.Fatal("another fixedpoint inherited local options", err)
	}
	answer, err := other.Answer(p)
	astTestMust(t, err)
	astTestMust(t, f.m.DelRef(&answer))
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

func TestAPIFixedpointParametersReplaceWrapperAndAccumulateEngine(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.c.SetUIntOption("timeout", 77))
	astTestMust(t, f.fp.AddFact(f.p, nil))

	plain, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, plain.SetBool("print_fixedpoint_extensions", false))
	astTestMust(t, plain.SetUInt("timeout", 42))
	astTestMust(t, f.fp.SetParameters(plain))
	// The fixedpoint owns an independent copy of the supplied set.
	astTestMust(t, plain.SetBool("print_fixedpoint_extensions", true))
	astTestMust(t, plain.SetUInt("timeout", 9))
	astTestMust(t, plain.Close())
	if value, e := f.fp.effectiveQueryTimeout(); e != nil || value != 42 {
		t.Fatal("copied wrapper timeout", value, e)
	}

	text, err := f.fp.SMT2(f.pAtom)
	astTestMust(t, err)
	for _, fragment := range []string{"(set-logic HORN)", "(declare-fun api-object-fp-p () Bool)", "(check-sat)"} {
		if !strings.Contains(text, fragment) {
			t.Fatal("plain fixedpoint profile missing", fragment, text)
		}
	}
	if strings.Contains(text, "(declare-rel api-object-fp-p ())") {
		t.Fatal("plain fixedpoint profile used extension syntax", text)
	}

	status, err := f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("plain fixedpoint query", status)
	}

	empty, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, f.fp.SetParameters(empty))
	astTestMust(t, empty.Close())
	if answer, answerErr := f.fp.LastAnswer(); answer != (ast.ASTRef{}) || answerErr == nil {
		t.Fatal("parameter replacement exposed stale query result", answer, answerErr)
	}
	if value, e := f.fp.effectiveQueryTimeout(); e != nil || value != 77 {
		t.Fatal("empty wrapper replacement did not restore parent timeout", value, e)
	}
	for _, name := range []string{"print_fixedpoint_extensions", "timeout"} {
		present, e := f.fp.params.params.ContainsName(&f.fp.params.value, &name)
		if e != nil || present {
			t.Fatal("latest wrapper snapshot kept prior parameter", name, present, e)
		}
	}

	text, err = f.fp.SMT2(f.pAtom)
	astTestMust(t, err)
	// datalog::context::updt_params copies updates into its existing params_ref,
	// so replacing the wrapper with an empty set does not clear engine options.
	for _, fragment := range []string{"(set-logic HORN)", "(declare-fun api-object-fp-p () Bool)", "(check-sat)"} {
		if !strings.Contains(text, fragment) {
			t.Fatal("empty update cleared cumulative engine profile", fragment, text)
		}
	}
	if strings.Contains(text, "(declare-rel api-object-fp-p ())") {
		t.Fatal("empty update restored extension syntax", text)
	}

	status, err = f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("fixedpoint query after parameter replacement", status)
	}

	extensions, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, extensions.SetBool("print_fixedpoint_extensions", true))
	astTestMust(t, f.fp.SetParameters(extensions))
	astTestMust(t, extensions.Close())
	if value, e := f.fp.effectiveQueryTimeout(); e != nil || value != 77 {
		t.Fatal("later wrapper update inherited prior timeout", value, e)
	}
	text, err = f.fp.SMT2(f.pAtom)
	astTestMust(t, err)
	for _, fragment := range []string{"(declare-rel api-object-fp-p ())", "(query api-object-fp-p)"} {
		if !strings.Contains(text, fragment) {
			t.Fatal("later engine update did not override cumulative profile", fragment, text)
		}
	}
	if strings.Contains(text, "(set-logic HORN)") || strings.Contains(text, "(check-sat)") {
		t.Fatal("later engine update kept plain fixedpoint profile", text)
	}
}
