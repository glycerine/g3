package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins14CheckRelationCheckedTables: datalog.check_relation
// (tr_check, tr_sparse, tr_hashtable, tr_lazy_sparse) with and without
// datalog.default_table_checked, over several default tables and
// checkers: check_relation_plugin is the favourite relation plugin over
// its base table_relation plugin (rel_context::updt_params), whose tables
// are check tables when the base is tr_check or the favourite table is
// checked. The answers (their row order included) are the fixed fork's
// (plugins14_check_<combination>.stdout).
func TestPlugins14CheckRelationCheckedTables(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("test_vectors", "plugins14_check_*.smt2"))
	if err != nil || len(names) == 0 {
		t.Fatal("no vectors", err)
	}
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			script, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(name, ".smt2") + ".stdout")
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if out.String() != string(want) {
				t.Fatalf("got\n%s\nwant\n%s", out.String(), want)
			}
		})
	}
}
