package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestASTVectorTranslateCreatesIndependentTargetRoots(t *testing.T) {
	source, a, _ := astVectorFixture(t)
	target := newSMTAPIFixture(t, "QF_UF", nil)
	v, err := source.c.NewASTVectorValues(a)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, v.Close()) })
	translated, err := v.Translate(target.c)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, translated.Close()) })
	copy, err := translated.At(0)
	astTestMust(t, err)
	if copy.Origin() == a.Origin() {
		t.Fatal("translation retained source manager identity")
	}
	if kind, err := target.m.GetKind(copy); err != nil || kind != ast.ASTApp {
		t.Fatalf("translated value: %v %v", kind, err)
	}
	if _, err = v.Translate(source.c); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("same-context translation: %v", err)
	}
}
