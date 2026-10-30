package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITypedASTViewsRejectWrongKinds(t *testing.T) {
	f := newAPITypedASTFixture(t)
	for name, call := range map[string]func() error{
		"application": func() error { _, err := f.context.ToApplication(f.sort); return err },
		"sort":        func() error { _, err := f.context.ToSort(f.declaration); return err },
		"function":    func() error { _, err := f.context.ToFunctionDeclaration(f.application); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ast.ErrASTDomain) {
				t.Fatal(err)
			}
		})
	}
}

func TestAPITypedASTViewsRejectForeignAndNullHandles(t *testing.T) {
	f := newAPITypedASTFixture(t)
	other := newAPITypedASTFixture(t)
	if _, err := f.context.ToSort(other.sort); err == nil {
		t.Fatal("foreign sort accepted")
	}
	if _, err := f.context.ToSort(ast.ASTHandle{}); err == nil {
		t.Fatal("null sort accepted")
	}
}
