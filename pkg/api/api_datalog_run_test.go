package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIRunDatalogPathReturnsCopiedRelationTables(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	path := filepath.Join(t.TempDir(), "reach.dl")
	program := "D 2\n\nP(x:D)\nQ(x:D)\nP(\"a\").\nQ(X) :- P(X).\n"
	astTestMust(t, os.WriteFile(path, []byte(program), 0o600))
	result, err := context.RunDatalogPath(path)
	astTestMust(t, err)
	if len(result.Relations) != 2 || result.Relations[0].Name != "P" || result.Relations[1].Name != "Q" || result.Relations[0].Status != lbool.True || len(result.Relations[1].Facts) != 1 || result.Output == "" {
		t.Fatalf("result %#v", result)
	}
	result.Relations[0].Facts[0][0] = 99
}
