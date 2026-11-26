package shell_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/shell"
)

func TestDIMACSFrontendCoreOption(t *testing.T) {
	for _, tc := range []struct {
		name string
		core bool
	}{
		{name: "default"},
		{name: "enabled after construction", core: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			})
			globals, err := context.GlobalParameters()
			if err != nil {
				t.Fatal(err)
			}
			const cnf = "p cnf 2 4\n1 2 0\n-1 2 0\n1 -2 0\n-1 -2 0\n"
			var out bytes.Buffer
			frontend, err := shell.NewDIMACSFrontend(shell.DIMACSFrontendConfig{
				Globals: globals, Input: strings.NewReader(cnf), Writer: &out,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := frontend.Close(); err != nil {
					t.Error(err)
				}
			})
			modules := len(globals.Current.Descriptors)
			if tc.core {
				if err := globals.SourceSetName("sat.dimacs.core", "true"); err != nil {
					t.Fatal(err)
				}
			}
			if err := frontend.Run(); err != nil {
				t.Fatal(err)
			}
			if tc.core {
				if !strings.HasPrefix(out.String(), "unsat\ncore\n") {
					t.Fatalf("expected a tracked core, got %q", out.String())
				}
			} else if out.String() != "unsat\n" || len(globals.Current.Descriptors) != modules {
				t.Fatalf("default run emitted %q and loaded %d additional parameter modules", out.String(), len(globals.Current.Descriptors)-modules)
			}
		})
	}
}
