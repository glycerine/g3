package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPredicateMetadataOwnsNamesAndMarksOutput(t *testing.T) {
	f := newDirectFactFixture(t)
	e, manager := f.engine, f.manager
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	declarationName, err := manager.Symbols.Intern("metadata-edge")
	fragmentMust(t, err)
	declaration, err := manager.MkFuncDecl(declarationName, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, e.RegisterPredicate(declaration))
	left, err := manager.Symbols.Intern("left")
	fragmentMust(t, err)
	right, err := manager.Symbols.Intern("right")
	fragmentMust(t, err)
	names := []symbol.Symbol{left, right}
	fragmentMust(t, e.SetArgumentNames(declaration, names))
	names[0] = right
	got, err := e.ArgumentName(declaration, 0)
	fragmentMust(t, err)
	if got != left {
		t.Fatal("engine borrowed argument-name slice")
	}
	if err = e.SetArgumentNames(declaration, []symbol.Symbol{left, right}); err == nil {
		t.Fatal("duplicate argument-name assignment accepted")
	}
	fragmentMust(t, e.SetOutputPredicate(declaration))
	output, err := e.IsOutputPredicate(declaration)
	fragmentMust(t, err)
	if !output {
		t.Fatal("output predicate mark unavailable")
	}

	clone, err := NewPropositionalEngine(manager)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, clone.Close()) })
	fragmentMust(t, e.copyQueryPredicates(clone))
	clonedName, err := clone.ArgumentName(declaration, 1)
	fragmentMust(t, err)
	if clonedName != right {
		t.Fatal("query clone lost argument names")
	}
	clonedOutput, err := clone.IsOutputPredicate(declaration)
	fragmentMust(t, err)
	if !clonedOutput {
		t.Fatal("query clone lost output predicate mark")
	}
}
