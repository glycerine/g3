package main

// Source: dl_compiler.cpp constructs mixed ground tuples from singleton
// columns; dl_product_relation.cpp unions their independent components.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func checkParsedUDocProductAnswer(t *testing.T, context *cmd_context.Context, declaration ast.ASTHandle, boolFirst bool, want func(uint64, bool) bool) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := context.Manager()
	must(err)
	answer, err := context.FixedpointAnswer(declaration)
	must(err)
	defer func() { must(manager.DelRef(&answer)) }()
	formula, err := manager.RefGet(answer)
	must(err)
	substitution, err := rewriter.NewVarSubstitution(manager, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	ground, err := manager.NewRef(ast.ASTHandle{})
	must(err)
	defer func() { must(manager.DelRef(&ground)) }()
	for number := uint64(0); number < 4; number++ {
		for _, boolean := range []bool{false, true} {
			func() {
				value, err := manager.BitVectorNumeralUint64(number, 2)
				must(err)
				valueRef, err := manager.NewRef(value)
				must(err)
				defer func() { must(manager.DelRef(&valueRef)) }()
				bit, err := manager.MkFalse()
				must(err)
				if boolean {
					bit, err = manager.MkTrue()
					must(err)
				}
				values := []ast.ASTHandle{value, bit}
				if boolFirst {
					values = []ast.ASTHandle{bit, value}
				}
				must(substitution.Substitute(formula, values, ground))
				term, err := manager.RefGet(ground)
				must(err)
				reduced, err := context.Simplify(term, nil)
				must(err)
				defer func() { must(manager.DelRef(&reduced)) }()
				truth, err := manager.RefGet(reduced)
				must(err)
				if manager.IsTrue(truth) != want(number, boolean) || !manager.IsTrue(truth) && !manager.IsFalse(truth) {
					t.Fatalf("parsed product answer at (%d,%t) disagrees with source components; want %t", number, boolean, want(number, boolean))
				}
			}()
		}
	}
}

func TestDatalogUDocMixedGroundProductsParsedSemantics(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, boolFirst := range []bool{true, false} {
			for _, docFirst := range []bool{true, false} {
				for _, correlationLoss := range []bool{true, false} {
					name := fmt.Sprintf("%s/bool-first=%t/doc-first=%t/correlation-loss=%t", tableKind, boolFirst, docFirst, correlationLoss)
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
						sorts, tail := "Bool (_ BitVec 2)", "(T b x)"
						facts := "(rule (T true #b10))"
						if correlationLoss {
							facts = "(rule (T true #b01))\n(rule (T false #b10))\n(rule (T true #b01))"
						}
						if !boolFirst {
							sorts, tail = "(_ BitVec 2) Bool", "(T x b)"
							facts = "(rule (T #b10 true))"
							if correlationLoss {
								facts = "(rule (T #b01 true))\n(rule (T #b10 false))\n(rule (T #b01 true))"
							}
						}
						body := "(and (D x) " + tail + ")"
						if !docFirst {
							body = "(and " + tail + " (D x))"
						}
						// Put the consumer before its ground dependencies to
						// exercise compilation strata, not input ordering.
						script := fmt.Sprintf(`
(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.datalog.default_table %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel D ((_ BitVec 2)))
(declare-rel T (%s))
(declare-rel H ((_ BitVec 2) Bool))
(declare-var x (_ BitVec 2))
(declare-var b Bool)
(rule (=> %s (H x b)))
%s
(rule (D #b01))
(query H)
`, tableKind, sorts, body, facts)
						var output bytes.Buffer
						parse := func(text, expected string) {
							t.Helper()
							output.Reset()
							if err := smt2.ParseShellCommands(context, strings.NewReader(text), &output); err != nil {
								t.Fatal(err, output.String())
							}
							if got := strings.Join(strings.Fields(output.String()), " "); got != expected {
								t.Fatalf("parsed query result = %q, want %q", got, expected)
							}
						}
						status := "unsat"
						if correlationLoss {
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
						if declarations["H"].IsNull() || declarations["T"].IsNull() {
							t.Fatal("parser lost mixed product declarations")
						}
						checkHead := func(number uint64, _ bool) bool { return correlationLoss && number == 1 }
						checkParsedUDocProductAnswer(t, context, declarations["H"], false, checkHead)
						parse("(query T)", "sat")
						checkParsedUDocProductAnswer(t, context, declarations["T"], boolFirst, func(number uint64, boolean bool) bool {
							if correlationLoss {
								return number == 1 || number == 2
							}
							return number == 2 && boolean
						})
						parse("(query H)", status)
						checkParsedUDocProductAnswer(t, context, declarations["H"], false, checkHead)
					})
				}
			}
		}
	}
}
