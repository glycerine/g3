package cmd_context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCommandDatalogParserOptionalMapAndSourceSizedSymbol(t *testing.T) {
	for _, tc := range []struct {
		name, mapName, symbol string
		writeMap              bool
	}{
		{name: "missing", mapName: "absent.map", symbol: "dynamic"},
		{name: "large", mapName: "names.map", symbol: strings.Repeat("mapped-symbol-", 6000), writeMap: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			if tc.writeMap {
				contextMust(t, os.WriteFile(filepath.Join(directory, tc.mapName), []byte(tc.symbol+"\n"), 0o600))
			}
			main := filepath.Join(directory, "main.dl")
			program := "Color 2 " + tc.mapName + "\n\nP(x:Color)\nP(\"" + tc.symbol + "\").\n"
			contextMust(t, os.WriteFile(main, []byte(program), 0o600))
			context := contextFixture(t)
			contextMust(t, context.ParseDatalogFile(main))
			declaration, err := context.FindFunctionDeclaration("P", nil, nil, ast.ASTHandle{})
			contextMust(t, err)
			status, err := context.QueryFixedpoint(declaration)
			contextMust(t, err)
			if status != lbool.True {
				t.Fatal("mapped fact query", status)
			}
		})
	}
}
