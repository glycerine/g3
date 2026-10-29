package cmd_context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestWPADirectoryMapsRulesRelationsAndSaturation(t *testing.T) {
	directory := t.TempDir()
	contextMust(t, os.WriteFile(filepath.Join(directory, "V.map"), []byte("10 alpha\n20 beta\n"), 0600))
	rules := "Edge(x:V,y:V)\nReach(x:V,y:V)\nReach(X,Y) :- Edge(X,Y).\n"
	contextMust(t, os.WriteFile(filepath.Join(directory, "program.rules"), []byte(rules), 0600))
	contextMust(t, os.WriteFile(filepath.Join(directory, "Edge.rel"), []byte("10 20\n# ignored\n"), 0600))
	contextMust(t, os.WriteFile(filepath.Join(directory, "DirectCall.rel"), []byte("bad input intentionally skipped\n"), 0600))
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	reach, err := context.FindFunctionDeclaration("Reach", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	answer, err := context.QueryFixedpoint(reach)
	contextMust(t, err)
	if answer != lbool.True {
		t.Fatal("WPA reachability", answer)
	}
}

func TestWPADirectoryRejectsUnknownTuplePredicate(t *testing.T) {
	directory := t.TempDir()
	contextMust(t, os.WriteFile(filepath.Join(directory, "V.map"), []byte("1 one\n"), 0600))
	contextMust(t, os.WriteFile(filepath.Join(directory, "program.rules"), []byte("Known(x:V)\n"), 0600))
	contextMust(t, os.WriteFile(filepath.Join(directory, "Unknown.rel"), []byte("1\n"), 0600))
	context := contextFixture(t)
	if err := context.ParseWPADirectory(directory); err == nil {
		t.Fatal("accepted tuple file for unknown predicate")
	}
}
