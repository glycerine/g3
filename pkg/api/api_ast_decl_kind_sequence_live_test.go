package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPISequenceDeclarationKindLiveSequenceAndRegex(t *testing.T) {
	f := newAPITypedASTFixture(t)
	left, err := f.manager.MkString("a")
	if err != nil {
		t.Fatal(err)
	}
	right, err := f.manager.MkString("b")
	if err != nil {
		t.Fatal(err)
	}
	concat, err := f.manager.MkSequenceConcat(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.SequenceDeclarationKind(concat); err != nil || got != APIDeclarationSequenceConcat {
		t.Fatal("concat app", got, err)
	}
	declaration, err := f.manager.GetDecl(concat)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.SequenceDeclarationKind(declaration); err != nil || got != APIDeclarationSequenceConcat {
		t.Fatal("concat declaration", got, err)
	}
	regex, err := f.manager.MkSequenceToRegex(left)
	if err != nil {
		t.Fatal(err)
	}
	plus, err := f.manager.MkRegexPlus(regex)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.SequenceDeclarationKind(plus); err != nil || got != APIDeclarationRegexPlus {
		t.Fatal("regex plus", got, err)
	}
	if _, err := f.context.SequenceDeclarationKind(f.application); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uninterpreted family accepted", err)
	}
}
