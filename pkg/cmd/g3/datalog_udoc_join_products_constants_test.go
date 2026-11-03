package main

// Source: dl_compiler.cpp joins positive tails, filters their constant
// columns, then projects and permutes the columns needed by the head.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestDatalogUDocJoinProductsParsedConstantTails(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			for _, boolOnLeft := range []bool{false, true} {
				for _, contradiction := range []string{"none", "Bool", "BV"} {
					name := fmt.Sprintf("%s/reverse=%t/bool-on-left=%t/contradiction=%s", kind, reverse, boolOnLeft, contradiction)
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
						leftKey, rightKey := uint64(1), uint64(2)
						leftBools, rightBools := uint8(3), uint8(3)
						left, right := "(H1 (_ bv1 2) b)", "(H2 x false)"
						if boolOnLeft {
							left, right = "(H1 x true)", "(H2 (_ bv2 2) b)"
						}
						if contradiction == "Bool" {
							if boolOnLeft {
								leftBools = 1 // Only false, while H1 requires true.
							} else {
								rightBools = 2 // Only true, while H2 requires false.
							}
						} else if contradiction == "BV" {
							if boolOnLeft {
								rightKey = 3
							} else {
								leftKey = 3
							}
						}
						if reverse {
							left, right = right, left
						}
						var facts strings.Builder
						for _, input := range []struct {
							name  string
							key   uint64
							bools uint8
						}{{"T", leftKey, leftBools}, {"U", rightKey, rightBools}} {
							for index, boolean := range []string{"false", "true"} {
								if input.bools&(1<<index) != 0 {
									fmt.Fprintf(&facts, "(rule (%s %s (_ bv%d 2)))\n", input.name, boolean, input.key)
								}
							}
						}
						// The constant-bearing consumer precedes its producers.
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
(rule (=> (and %s %s) (K x b)))
(rule (=> (and (D x) (T b x)) (H1 x b)))
(rule (=> (and (U b x) (E x)) (H2 x b)))
%s
(rule (D (_ bv%d 2)))
(rule (E (_ bv%d 2)))
(query K)
`, kind, left, right, facts.String(), leftKey, rightKey)
						parse := func(input, want string) {
							t.Helper()
							var output bytes.Buffer
							if err := smt2.ParseShellCommands(context, strings.NewReader(input), &output); err != nil {
								t.Fatal(err, output.String())
							}
							if got := strings.Join(strings.Fields(output.String()), " "); got != want {
								t.Fatalf("parsed constant product result = %q, want %q", got, want)
							}
						}
						status, outputBools := "sat", uint8(3)
						if contradiction != "none" {
							status, outputBools = "unsat", 0
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
						check := func(name string, key uint64, booleans uint8) {
							t.Helper()
							if declarations[name].IsNull() {
								t.Fatal("parser lost product declaration", name)
							}
							checkParsedUDocProductAnswer(t, context, declarations[name], false, func(number uint64, boolean bool) bool {
								mask := uint8(1)
								if boolean {
									mask = 2
								}
								return number == key && booleans&mask != 0
							})
						}
						outputKey := rightKey
						if boolOnLeft {
							outputKey = leftKey
						}
						check("K", outputKey, outputBools)
						// Constant filters mutate only the joined temporary.
						parse("(query H1)", "sat")
						check("H1", leftKey, leftBools)
						parse("(query H2)", "sat")
						check("H2", rightKey, rightBools)
						parse("(query K)", status)
						check("K", outputKey, outputBools)
					})
				}
			}
		}
	}
}
