package main

// Source: dl_product_relation.cpp joins existing DOC and finite components
// independently; acyclic compiler strata resolve both derived operands first.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestDatalogUDocJoinProductsParsedDerivedOperands(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			for _, test := range []struct {
				name, facts           string
				rightKey              uint64
				leftBools, rightBools uint8 // bit 0 is false, bit 1 is true.
			}{
				{"matching-components", "(rule (T true #b01))\n(rule (U true #b01))", 1, 2, 2},
				{"source-correlation-loss", "(rule (T true #b01))\n(rule (T false #b10))\n(rule (U false #b01))", 1, 3, 1},
				{"DOC-key-contradiction", "(rule (T true #b01))\n(rule (U true #b10))", 2, 2, 2},
				{"finite-Bool-contradiction", "(rule (T true #b01))\n(rule (U false #b01))", 1, 2, 1},
			} {
				t.Run(fmt.Sprintf("%s/reverse=%t/%s", kind, reverse, test.name), func(t *testing.T) {
					context, err := cmd_context.NewStandaloneContext()
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := context.Close(); err != nil {
							t.Error(err)
						}
					})
					body := "(and (H1 x b) (H2 x b))"
					if reverse {
						body = "(and (H2 x b) (H1 x b))"
					}
					// K is declared and defined before both dependencies. All
					// predicates are queried by name; no expression-query
					// rewriting can bypass the concrete paired product join.
					script := fmt.Sprintf(`
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.datalog.default_table %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel K ((_ BitVec 2) Bool))
(declare-rel H1 ((_ BitVec 2) Bool))
(declare-rel H2 ((_ BitVec 2) Bool))
(declare-rel D ((_ BitVec 2)))
(declare-rel E ((_ BitVec 2)))
(declare-rel T (Bool (_ BitVec 2)))
(declare-rel U (Bool (_ BitVec 2)))
(declare-var x (_ BitVec 2))
(declare-var b Bool)
(rule (=> %s (K x b)))
(rule (=> (and (U b x) (E x)) (H2 x b)))
(rule (=> (and (D x) (T b x)) (H1 x b)))
%s
(rule (D #b01))
(rule (E (_ bv%d 2)))
(query K)
`, kind, body, test.facts, test.rightKey)
					parse := func(input, want string) {
						t.Helper()
						var output bytes.Buffer
						if err := smt2.ParseShellCommands(context, strings.NewReader(input), &output); err != nil {
							t.Fatal(err, output.String())
						}
						if got := strings.Join(strings.Fields(output.String()), " "); got != want {
							t.Fatalf("parsed paired-product result = %q, want %q", got, want)
						}
					}
					joinedBools := test.leftBools & test.rightBools
					if test.rightKey != 1 {
						joinedBools = 0
					}
					status := "unsat"
					if joinedBools != 0 {
						status = "sat"
					}
					parse(script, status)
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
					for _, name := range []string{"K", "H1", "H2"} {
						if declarations[name].IsNull() {
							t.Fatal("parser lost paired-product declaration", name)
						}
					}
					check := func(name string, key uint64, booleans uint8) {
						t.Helper()
						checkParsedUDocProductAnswer(t, context, declarations[name], false, func(number uint64, boolean bool) bool {
							mask := uint8(1)
							if boolean {
								mask = 2
							}
							return number == key && booleans&mask != 0
						})
					}
					check("K", 1, joinedBools)
					parse("(query H1)", "sat")
					check("H1", 1, test.leftBools)
					parse("(query H2)", "sat")
					check("H2", test.rightKey, test.rightBools)
					parse("(query K)", status)
					check("K", 1, joinedBools)
				})
			}
		}
	}
}
