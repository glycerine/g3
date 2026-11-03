package main

// Source: dl_compiler.cpp assembles ground tuples from singleton columns;
// dl_product_relation.cpp unions and joins DOC/table components independently.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestDatalogUDocGroundProductsParsedWithoutRawAnchors(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, boolFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bool-first=%t", kind, boolFirst), func(t *testing.T) {
				context, err := cmd_context.NewStandaloneContext()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := context.Close(); err != nil {
						t.Error(err)
					}
				})
				sorts, facts := "(_ BitVec 2) Bool", "(rule (T #b01 true))\n(rule (T #b10 false))\n(rule (T #b01 true))"
				if boolFirst {
					sorts, facts = "Bool (_ BitVec 2)", "(rule (T true #b01))\n(rule (T false #b10))\n(rule (T true #b01))"
				}
				script := fmt.Sprintf(`
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.datalog.default_table %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel T (%s))
%s
(query T)
`, kind, sorts, facts)
				parse := parsedUDocGroundProductCommands(t, context)
				parse(script, "sat")
				declarations := parsedUDocGroundProductDeclarations(t, context)
				if len(declarations) != 1 || declarations["T"].IsNull() {
					t.Fatal("standalone ground fixture acquired an unrelated predicate", declarations)
				}
				for iteration := 0; iteration < 2; iteration++ {
					checkParsedUDocProductAnswer(t, context, declarations["T"], boolFirst, func(number uint64, _ bool) bool {
						// Independent component union admits both crossed tuples.
						return number == 1 || number == 2
					})
					if iteration == 0 {
						parse("(query T)", "sat")
					}
				}
			})
		}
	}
}

func TestDatalogUDocGroundProductsParsedJoinWithoutRawAnchors(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, boolFirst := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				for _, test := range []struct {
					name, keys              string
					leftBits, rightBits     uint8
					leftBools, rightBools   uint8
					outputBits, outputBools uint8
				}{
					{"matching", "both", 1 << 1, 1 << 1, 2, 2, 1 << 1, 2},
					{"correlation-loss", "both", 1<<1 | 1<<2, 1 << 1, 3, 1, 1 << 1, 1},
					{"BV-key-contradiction", "both", 1 << 1, 1 << 2, 2, 2, 0, 0},
					{"Bool-key-contradiction", "both", 1 << 1, 1 << 1, 2, 1, 0, 0},
					{"BV-key-only", "BV", 1 << 1, 1 << 1, 2, 1, 1 << 1, 1},
					{"Bool-key-only", "Bool", 1 << 1, 1 << 2, 2, 2, 1 << 1, 2},
				} {
					name := fmt.Sprintf("%s/bool-first=%t/reverse=%t/%s", kind, boolFirst, reverse, test.name)
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
						atom := func(predicate, bit, boolean string) string {
							if boolFirst {
								return fmt.Sprintf("(%s %s %s)", predicate, boolean, bit)
							}
							return fmt.Sprintf("(%s %s %s)", predicate, bit, boolean)
						}
						left, right := atom("T", "x", "b"), atom("U", "x", "b")
						if test.keys == "BV" {
							left = atom("T", "x", "c")
						} else if test.keys == "Bool" {
							right = atom("U", "y", "b")
						}
						if reverse {
							left, right = right, left
						}
						var facts strings.Builder
						for _, input := range []struct {
							name        string
							bits, bools uint8
						}{{"T", test.leftBits, test.leftBools}, {"U", test.rightBits, test.rightBools}} {
							// Emit just two correlated rows when both components
							// vary, so a finite tuple set would fail the oracle.
							for number := uint64(0); number < 4; number++ {
								if input.bits&(1<<number) == 0 {
									continue
								}
								for index, boolean := range []string{"false", "true"} {
									if input.bools&(1<<index) == 0 || input.bools == 3 && (number == 1) != (index == 1) {
										continue
									}
									fmt.Fprintf(&facts, "(rule %s)\n", atom(input.name, fmt.Sprintf("(_ bv%d 2)", number), boolean))
								}
							}
						}
						sorts := "(_ BitVec 2) Bool"
						if boolFirst {
							sorts = "Bool (_ BitVec 2)"
						}
						script := fmt.Sprintf(`
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.datalog.default_table %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel K ((_ BitVec 2) Bool))
(declare-rel T (%s))
(declare-rel U (%s))
(declare-var x (_ BitVec 2))
(declare-var y (_ BitVec 2))
(declare-var b Bool)
(declare-var c Bool)
(rule (=> (and %s %s) (K x b)))
%s
(query K)
`, kind, sorts, sorts, left, right, facts.String())
						parse := parsedUDocGroundProductCommands(t, context)
						status := "unsat"
						if test.outputBits != 0 && test.outputBools != 0 {
							status = "sat"
						}
						parse(script, status)
						declarations := parsedUDocGroundProductDeclarations(t, context)
						if len(declarations) != 3 {
							t.Fatal("ground pair fixture acquired an unrelated predicate", declarations)
						}
						check := func(name string, bits, bools uint8) {
							t.Helper()
							if declarations[name].IsNull() {
								t.Fatal("parser lost ground product declaration", name)
							}
							checkParsedUDocProductAnswer(t, context, declarations[name], name != "K" && boolFirst, func(number uint64, boolean bool) bool {
								mask := uint8(1)
								if boolean {
									mask = 2
								}
								return bits&(1<<number) != 0 && bools&mask != 0
							})
						}
						check("K", test.outputBits, test.outputBools)
						parse("(query T)", "sat")
						check("T", test.leftBits, test.leftBools)
						parse("(query U)", "sat")
						check("U", test.rightBits, test.rightBools)
						parse("(query K)", status)
						check("K", test.outputBits, test.outputBools)
					})
				}
			}
		}
	}
}

func parsedUDocGroundProductCommands(t *testing.T, context *cmd_context.Context) func(string, string) {
	t.Helper()
	return func(input, want string) {
		t.Helper()
		var output bytes.Buffer
		if err := smt2.ParseShellCommands(context, strings.NewReader(input), &output); err != nil {
			t.Fatal(err, output.String())
		}
		if got := strings.Join(strings.Fields(output.String()), " "); got != want {
			t.Fatalf("parsed standalone ground-product result = %q, want %q", got, want)
		}
	}
}

func parsedUDocGroundProductDeclarations(t *testing.T, context *cmd_context.Context) map[string]ast.ASTHandle {
	t.Helper()
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
	return declarations
}
