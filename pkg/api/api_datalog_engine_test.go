package api

// Source: src/shell/datalog_frontend.cpp selects Datalog before parsing.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIRunDatalogPathSelectsDatalogOverConfiguredSpacer(t *testing.T) {
	for _, tc := range []struct {
		name, constant string
		numeric        bool
	}{
		{name: "mapped-names", constant: "alpha"},
		{name: "numeric-names", constant: "7", numeric: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := NewSMTContext()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, context.Close()) })
			astTestMust(t, context.SetSymbolOption("fixedpoint.engine", "spacer"))
			if tc.numeric {
				astTestMust(t, context.SetBoolOption("fixedpoint.datalog.use_map_names", false))
			}
			directory := t.TempDir()
			for _, file := range []struct{ name, text string }{
				{"V.map", "7 alpha\n"},
				{"program.rules", "P(x:V) outputtuples\n"},
				{"P.rel", "7\n"},
			} {
				astTestMust(t, os.WriteFile(filepath.Join(directory, file.name), []byte(file.text), 0o600))
			}
			result, err := context.RunDatalogPath(directory)
			astTestMust(t, err)
			prefix := "Tuples in P: \n\t(x=" + tc.constant + "(0))\nTime: "
			if !strings.HasPrefix(result.Output, prefix) {
				t.Fatalf("Datalog frontend did not preserve tuple naming: %q", result.Output)
			}
			if len(result.Relations) != 1 || result.Relations[0].Name != "P" || result.Relations[0].Status != lbool.True || !reflect.DeepEqual(result.Relations[0].Facts, [][]uint64{{0}}) {
				t.Fatalf("configured Spacer changed Datalog results: %#v", result.Relations)
			}
		})
	}
}
