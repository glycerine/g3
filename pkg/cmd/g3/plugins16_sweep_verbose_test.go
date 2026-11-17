//go:build g3trace

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins16SweepVerbose: plugins16 sweep programs at -v:12 against the
// fixed fork's stderr (plugins16_sweep_<name>.v12.txt, times as 0s): check
// tables of check_relation placeholders print mk_empty when get_relation
// creates them; the compiler's m_pred_regs and the rule set's m_deps and
// m_head2rules keep their insert_if_not_there2 expansions and capacities
// (store order, loop order); mk_filter_rules' filter cache never hits;
// projections onto zero columns go through null_signature_table_project_fn
// (the favourite table plugin's mk_empty, "new type returned"); a lazy
// table's nullary forcing and a union into an empty register take the
// manager's nullary table; bit-vector tables of check tables pool their
// sparse results; and the AST-order fixes behind check_relation's solver
// runs (act_cache skolemizer, to_formula into m_fml).
func TestPlugins16SweepVerbose(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("test_vectors", "plugins16_sweep_*.smt2"))
	if err != nil || len(names) == 0 {
		t.Fatal("no vectors", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			want, err := os.ReadFile(strings.TrimSuffix(name, ".smt2") + ".v12.txt")
			if err != nil {
				t.Fatal(err)
			}
			got := runVerboseProgressChild(t, []string{"-v:12", filepath.Join(wd, name)})
			if got = plugins15VerboseTime.ReplaceAllString(got, "0s$1"); got != string(want) {
				t.Fatalf("%s -v:12\n%s\nwant\n%s", name, got, want)
			}
		})
	}
}
