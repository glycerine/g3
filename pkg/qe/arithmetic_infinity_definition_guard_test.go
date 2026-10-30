package qe_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticInfinityDefinitionRequiresPreparedEntry(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	truth := f.pin(f.manager.MkTrue())
	if _, e := f.plugin.InfinityDefinition(f.contains, truth); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal(e)
	}
	if e := f.plugin.ValidateInfinityDefinition(nil, truth); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
}
