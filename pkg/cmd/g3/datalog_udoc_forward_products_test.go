package main

// Source: acyclic compiler strata forward concrete product relations through
// ordinary heads while preserving componentwise union and column permutation.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestDatalogUDocForwardProductsParsedChains(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, consumersFirst := range []bool{false, true} {
			for _, nonempty := range []bool{false, true} {
				name := fmt.Sprintf("%s/consumers-first=%t/nonempty=%t", kind, consumersFirst, nonempty)
				t.Run(name, func(t *testing.T) {
					context, err := cmd_context.NewStandaloneContext()
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := context.Close(); err != nil {
							t.Error(err)
						}
					})
					rules := []string{
						"(rule (=> (and (D x) (T b x)) (H x b)))",
						"(rule (=> (H x b) (R b x)))",
						"(rule (=> (R b x) (S x b)))",
					}
					if consumersFirst {
						rules[0], rules[2] = rules[2], rules[0]
					}
					facts := "(rule (T true #b10))"
					status := "unsat"
					if nonempty {
						facts = "(rule (T true #b01))\n(rule (T false #b10))"
						status = "sat"
					}
					script := fmt.Sprintf(`
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.datalog.default_table %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel D ((_ BitVec 2)))
(declare-rel T (Bool (_ BitVec 2)))
(declare-rel H ((_ BitVec 2) Bool))
(declare-rel R (Bool (_ BitVec 2)))
(declare-rel S ((_ BitVec 2) Bool))
(declare-var x (_ BitVec 2))
(declare-var b Bool)
%s
%s
(rule (D #b01))
(query R)
`, kind, strings.Join(rules, "\n"), facts)
					parse := func(input string) {
						t.Helper()
						var output bytes.Buffer
						if err := smt2.ParseShellCommands(context, strings.NewReader(input), &output); err != nil {
							t.Fatal(err, output.String())
						}
						if got := strings.Join(strings.Fields(output.String()), " "); got != status {
							t.Fatalf("parsed forwarding result = %q, want %q", got, status)
						}
					}
					parse(script)
					manager, err := context.Manager()
					if err != nil {
						t.Fatal(err)
					}
					predicates, err := context.FixedpointPredicates()
					if err != nil {
						t.Fatal(err)
					}
					declarations := make(map[string]ast.ASTHandle)
					for _, declaration := range predicates {
						symbol, err := manager.GetASTName(declaration)
						if err != nil {
							t.Fatal(err)
						}
						name, err := manager.Symbols.String(symbol)
						if err != nil {
							t.Fatal(err)
						}
						declarations[name] = declaration
					}
					if declarations["R"].IsNull() || declarations["S"].IsNull() {
						t.Fatal("parser lost ordinary forwarding heads")
					}
					// T's independent DOC and Bool unions permit both Bool
					// values at key 1. Both ordinary heads must preserve this
					// source behavior through two column permutations.
					want := func(number uint64, _ bool) bool { return nonempty && number == 1 }
					checkParsedUDocProductAnswer(t, context, declarations["R"], true, want)
					parse("(query S)")
					checkParsedUDocProductAnswer(t, context, declarations["S"], false, want)
					parse("(query R)")
					checkParsedUDocProductAnswer(t, context, declarations["R"], true, want)
				})
			}
		}
	}
}
