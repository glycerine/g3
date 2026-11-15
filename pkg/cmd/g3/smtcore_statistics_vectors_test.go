package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// smtcoreStatisticsVector pins selected `-st` counters of a test vector.
// The values are what the C++ reference (build/z3) prints for the same
// arguments; set G3_CPP_REFERENCE to recheck them against it.
type smtcoreStatisticsVector struct {
	args []string
	// want maps a statistics key (without the leading colon) to its value.
	// An empty value means the key must be absent.
	want map[string]string
}

var smtcoreStatisticsVectors = map[string]smtcoreStatisticsVector{
	// AUDIT.md smtcore_10-F6: theory::assume_eqs assumes every interface
	// equality in one final check.
	"dense_assume_eqs_all.smt2": {
		want: map[string]string{"final-checks": "2", "mk-clause": "3", "mk-bool-var": "28", "binary-propagations": "4"},
	},
	// AUDIT.md smtcore_10-F5: the dense theory builds its equality axioms
	// with arith_eq_adapter::mk_axioms.
	"dense_equality_axiom_order.smt2": {
		want: map[string]string{"dd-assertions": "8", "mk-bool-var": "16", "arith-eq-adapter": "2"},
	},
	// AUDIT.md smtcore_12-F5: theory_pb::collect_statistics has no
	// "pb clauses" key.
	"pb_statistics_no_clause_key.smt2": {
		want: map[string]string{"pb-clauses": "", "pb-conflicts": "1", "pb-predicates": "3"},
	},
	// AUDIT.md smtcore_10-F4: final_check_eh keeps splitting the remaining
	// roots after a split conflict.
	"datatype_final_check_split_conflict.smt2": {
		want: map[string]string{"datatype-occurs-check": "9", "datatype-splits": "7", "mk-bool-var": "21"},
	},
}

// smtcoreStatistics parses the ` :key value` lines that -st prints.
func smtcoreStatistics(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "("))
		line = strings.TrimSuffix(line, ")")
		if !strings.HasPrefix(line, ":") {
			continue
		}
		fields := strings.Fields(line[1:])
		if len(fields) == 2 {
			result[fields[0]] = fields[1]
		}
	}
	return result
}

func TestSMTCoreStatisticsVectors(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	for name, vector := range smtcoreStatisticsVectors {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("test_vectors", name)
			args := append(append([]string{"-st"}, vector.args...), path)
			var out bytes.Buffer
			if err := runArguments(args, nil, &out); err != nil {
				t.Fatal(err, out.String())
			}
			check := func(who, output string) {
				t.Helper()
				got := smtcoreStatistics(output)
				for key, want := range vector.want {
					value, found := got[key]
					if want == "" && found {
						t.Fatalf("%s: unexpected :%s %s\n%s", who, key, value, output)
					}
					if want != "" && value != want {
						t.Fatalf("%s: :%s = %q, want %q\n%s", who, key, value, want, output)
					}
				}
			}
			check("g3", out.String())
			if reference != "" {
				native, err := exec.Command(reference, args...).Output()
				if err != nil {
					t.Fatal("C++ reference", err, string(native))
				}
				check("C++", string(native))
			}
		})
	}
}
