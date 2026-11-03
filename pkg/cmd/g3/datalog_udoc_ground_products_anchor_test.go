package main

// Source: src/muz/rel/dl_compiler.cpp compiles ground-rule strata before
// dependent joins, independently of their order in the SMT-LIB input.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestDatalogUDocGroundProductsParsedRules(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, tails := range []struct {
			name, body string
		}{
			{"doc-first", "(and (D x) (T b))"},
			{"table-first", "(and (T b) (D x))"},
		} {
			for _, facts := range []struct {
				name, rules, want string
			}{
				{
					"duplicate-ground-rules",
					"(rule (D #b01))\n(rule (T true))\n(rule (D #b01))\n(rule (T true))",
					"sat sat sat sat",
				},
				{
					"empty-doc-input",
					"(rule (T true))\n(rule (T true))",
					"unsat unsat sat unsat",
				},
				{
					"empty-table-input",
					"(rule (D #b01))\n(rule (D #b01))",
					"unsat sat unsat unsat",
				},
			} {
				t.Run(tableKind+"/"+tails.name+"/"+facts.name, func(t *testing.T) {
					// The mixed rule deliberately precedes every ground input rule.
					// Disable inlining so parsing exercises their execution strata.
					script := fmt.Sprintf(`
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.datalog.default_table %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel D ((_ BitVec 2)))
(declare-rel T (Bool))
(declare-rel H ((_ BitVec 2) Bool))
(declare-var x (_ BitVec 2))
(declare-var b Bool)
(rule (=> %s (H x b)))
%s
(query H)
(query D)
(query T)
(query H)
`, tableKind, tails.body, facts.rules)
					var output bytes.Buffer
					if err := run(strings.NewReader(script), &output); err != nil {
						t.Fatal(err, output.String())
					}
					if got := strings.Join(strings.Fields(output.String()), " "); got != facts.want {
						t.Fatalf("parsed ground DOC/table results = %q, want %q", got, facts.want)
					}
				})
			}
		}
	}
}
