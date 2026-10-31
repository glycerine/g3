package api

// Source: src/api/api_ast.cpp:880-1210, complete family dispatch.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPublicDeclarationKindAllFamiliesAndFallbacks(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	for _, tc := range []struct {
		family    string
		operation int32
		want      APIDeclarationKind
	}{
		{"", 6, APIDeclarationUninterpreted},
		{"basic", ast.ASTOpAnd, APIDeclarationAnd},
		{"basic", ast.ASTPrHypothesis, APIDeclarationProofHypothesis},
		{"arith", 6, APIDeclarationAdd},
		{"array", ast.ASTArrayOpStore, APIDeclarationArrayStore},
		{"bv", 4, APIDeclarationBitVectorAdd},
		{"datatype", ast.ASTDatatypeConstructor, APIDeclarationDatatypeConstructor},
		{"datalog_relation", 3, APIDeclarationRelationJoin},
		{"seq", 2, APIDeclarationSequenceConcat},
		{"fpa", ast.FPAOpRMNearestEven, APIDeclarationFPARMNearestTiesToEven},
		{"label", 0, APIDeclarationLabel},
		{"label", 1, APIDeclarationLabelLiteral},
		{"pb", 0, APIDeclarationPBAtMost},
		{"api-custom-family", ast.ASTOpAnd, APIDeclarationUninterpreted},
	} {
		t.Run(tc.family, func(t *testing.T) {
			family := ast.NullFamilyID
			var err error
			if tc.family != "" {
				family, err = f.m.EnsureFamilyID(tc.family)
				astTestMust(t, err)
			}
			for _, unknown := range []bool{false, true} {
				operation, want := tc.operation, tc.want
				if unknown {
					operation = 2000000000
					if tc.family != "" && tc.family != "api-custom-family" {
						want = APIDeclarationInternal
					}
				}
				info, err := f.m.NewFuncDeclInfo(family, operation, nil, ast.ASTFuncFlags{}, false)
				astTestMust(t, err)
				name, err := f.m.Symbols.Intern("public-kind-" + tc.family)
				astTestMust(t, err)
				declaration := f.pin(f.m.MkFuncDeclWithInfo(name, nil, boolean, info))
				astTestMust(t, f.m.DelFuncDeclInfo(&info))
				application := f.pin(f.m.MkApp(declaration, nil))
				for _, value := range []ast.ASTHandle{declaration, application} {
					if got, err := f.c.PublicDeclarationKind(value); err != nil || got != want {
						t.Fatal("public family dispatch", tc.family, operation, got, want, err)
					}
				}
			}
		})
	}
}

func TestAPIPublicDeclarationKindRejectsInvalidInputs(t *testing.T) {
	f := newAPITypedASTFixture(t)
	for _, value := range []ast.ASTHandle{f.sort, {}} {
		if _, err := f.context.PublicDeclarationKind(value); err == nil {
			t.Fatal("invalid declaration accepted", value)
		}
	}
	other := newAPITypedASTFixture(t)
	if _, err := f.context.PublicDeclarationKind(other.declaration); err == nil {
		t.Fatal("foreign declaration accepted")
	}
	context, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, context.Close())
	if _, err := context.PublicDeclarationKind(f.declaration); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed context", err)
	}
}
