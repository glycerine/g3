package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDatalogParserDeclarationMetadata(t *testing.T) {
	context := contextFixture(t)
	contextMust(t, context.ParseDatalogString("Node 2\n\nEdge(source:Node,target:Node) printtuples\nArc(node:Node) outputtuples\n"))
	declaration, err := context.FindFunctionDeclaration("Edge", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	engine, err := context.fixedpointContext()
	contextMust(t, err)
	manager, err := context.Manager()
	contextMust(t, err)

	for index, want := range []string{"source", "target"} {
		name, nameErr := engine.ArgumentName(declaration, uint32(index))
		contextMust(t, nameErr)
		got, nameErr := manager.Symbols.String(name)
		contextMust(t, nameErr)
		if got != want {
			t.Fatalf("argument %d name = %q, want %q", index, got, want)
		}
	}
	output, err := engine.IsOutputPredicate(declaration)
	contextMust(t, err)
	if !output {
		t.Fatal("printtuples declaration was not marked as output")
	}
	arc, err := context.FindFunctionDeclaration("Arc", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	output, err = engine.IsOutputPredicate(arc)
	contextMust(t, err)
	if !output {
		t.Fatal("outputtuples declaration was not marked as output")
	}
}
