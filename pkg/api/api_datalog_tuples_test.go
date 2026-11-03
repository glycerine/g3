package api

// Source: datalog_frontend.cpp output_tuples and WPA finite-domain display.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIRunDatalogPathPrintsMappedOutputTuples(t *testing.T) {
	for _, tc := range []struct {
		name, constant string
		numeric        bool
	}{
		{name: "default-map-names", constant: "alpha"},
		{name: "disabled-map-names", constant: "7", numeric: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := NewSMTContext()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, context.Close()) })
			if tc.numeric {
				astTestMust(t, context.SetBoolOption("fixedpoint.datalog.use_map_names", false))
			}
			directory := t.TempDir()
			for _, file := range []struct{ name, text string }{
				{"V.map", "7 alpha\n"},
				{"program.rules", "P(x:V) outputtuples\nQ(y:V)\n"},
				{"P.rel", "7\n"},
				{"Q.rel", "7\n"},
			} {
				astTestMust(t, os.WriteFile(filepath.Join(directory, file.name), []byte(file.text), 0o600))
			}
			result, err := context.RunDatalogPath(directory)
			astTestMust(t, err)
			prefix := "Tuples in P: \n\t(x=" + tc.constant + "(0))\n"
			if !strings.HasPrefix(result.Output, prefix+"Time: ") || strings.Count(result.Output, "Tuples in ") != 1 || strings.Contains(result.Output, "Tuples in Q:") {
				t.Fatalf("marked tuple output did not precede statistics: %q", result.Output)
			}
			if len(result.Relations) != 2 {
				t.Fatalf("relation count = %d, want 2", len(result.Relations))
			}
			for i, name := range []string{"P", "Q"} {
				relation := result.Relations[i]
				if relation.Name != name || relation.Status != lbool.True || !reflect.DeepEqual(relation.Facts, [][]uint64{{0}}) {
					t.Fatalf("relation %s changed its internal domain index: %#v", name, relation)
				}
			}
			output := result.Output
			astTestMust(t, context.Close())
			if result.Output != output || !strings.HasPrefix(result.Output, prefix+"Time: ") || !reflect.DeepEqual(result.Relations[0].Facts, [][]uint64{{0}}) {
				t.Fatalf("copied result changed after context close: %#v", result)
			}
			result.Relations[0].Facts[0][0] = 99
			if !reflect.DeepEqual(result.Relations[1].Facts, [][]uint64{{0}}) || result.Output != output {
				t.Fatal("copied relation rows or output share mutable storage")
			}
		})
	}
}
