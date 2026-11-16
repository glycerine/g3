package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func datalogFrontendContext(t *testing.T) *cmd_context.Context {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return context
}

// read_datalog saturates the parsed program with the parser's outputs
// (printtuples/outputtuples) alone: Q is no output, so mk_coi_filter drops
// its rule and Q stays empty, as in z3 4.7.1.
func TestDatalogFrontendFileDispatchSaturatesOutputPredicates(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "reach.dl")
	program := "D 2\n\nP(x:D)\nQ(x:D)\nP(\"a\").\nQ(X) :- P(X).\n"
	if err := os.WriteFile(path, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(path); err != nil {
		t.Fatal(err)
	}
	results, err := frontend.Results()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Name != "P" || results[1].Name != "Q" || results[0].Status != lbool.True || results[1].Status != lbool.False {
		t.Fatal("frontend results", results)
	}
	if text := output.String(); !strings.HasPrefix(text, "Time: ") || !strings.Contains(text, "\nParsing: ") {
		t.Fatal("frontend statistics", text)
	}
}
