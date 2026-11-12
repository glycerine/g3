package smt2

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMT2BMCEngineSearch(t *testing.T) {
	for _, tc := range []struct{ name, options, rules, want string }{
		{"linear", "", "(rule A)(rule (=> A G))", "sat\n"},
		{"bounded", "(set-option :fixedpoint.bmc.linear_unrolling_depth 0)", "(rule A)(rule (=> A G))", "unknown\n"},
		{"bounded-after-inlining", "(set-option :fixedpoint.bmc.linear_unrolling_depth 1)", "(rule A)(rule (=> A G))", "sat\n"},
		{"bounded-without-inlining", "(set-option :fixedpoint.bmc.linear_unrolling_depth 1)(set-option :fixedpoint.xform.inline_eager false)(set-option :fixedpoint.xform.inline_linear false)", "(rule A)(rule (=> A G))", "sat\n"},
		{"nonlinear", "", "(rule A)(rule B)(rule (=> (and A B) G))", "sat\n"},
		{"no-rules", "", "", "unsat\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := fmt.Sprintf("(set-option :fixedpoint.engine bmc)%s(set-logic HORN)(declare-rel A ())(declare-rel B ())(declare-rel G ())%s(query G)", tc.options, tc.rules)
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSMT2BMCCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine bmc)
(declare-rel A ())
(declare-rel G ())
(declare-rel U ())
(rule A)
(rule (=> A G))
(query G :print-certificate true)
(query U :print-certificate true)`
	var output bytes.Buffer
	if err := ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "sat\n(asserted query!0)\n\nunsat\nnull\n\n" {
		t.Fatalf("BMC certificates = %q", got)
	}
}

func TestSMT2BMCRecursiveCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine bmc)
(set-option :fixedpoint.bmc.linear_unrolling_depth 8)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-logic HORN)
(declare-rel P (Int))
(declare-rel G ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (< x 3)) (P (+ x 1))))
(rule (=> (P 3) G))
(query G :print-certificate true)`
	var output bytes.Buffer
	if err := ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.HasPrefix(got, "sat\n") || !strings.Contains(got, "hyper-res") {
		t.Fatalf("expected a reachable query with a hyper-resolution certificate: %s", got)
	}
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	answer, err := context.FixedpointExpressionAnswer()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := manager.DelRef(&answer); err != nil {
			t.Error(err)
		}
	}()
	proof, err := manager.RefGet(answer)
	if err != nil {
		t.Fatal(err)
	}
	conclusion, err := manager.GetProofFact(proof)
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := manager.GetDecl(conclusion)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.GetASTName(declaration)
	if err != nil {
		t.Fatal(err)
	}
	queryName, err := manager.Symbols.Display(name)
	if err != nil || !strings.HasPrefix(queryName, "query!") {
		t.Fatalf("proof does not conclude the synthetic query: %q, %v", queryName, err)
	}
	// The serialized output must contain exactly one complete S-expression,
	// including the checked proof's conclusion and original predicates. This
	// permits printer layout and let-binding changes without accepting a
	// truncated certificate.
	scanner, err := NewScanner(strings.NewReader(strings.TrimPrefix(output.String(), "sat\n")), false)
	if err != nil {
		t.Fatal(err)
	}
	depth, roots := 0, 0
	seen := make(map[string]bool)
	var printedTokens []string
	tokenText := func(scanner *Scanner, token Token) string {
		switch token {
		case SymbolToken, KeywordToken:
			return scanner.ID()
		case StringToken:
			return scanner.String()
		case IntegerToken, DecimalToken, BitVectorToken:
			return fmt.Sprintf("%s/%d", scanner.Number().RatString(), scanner.BitVectorSize())
		default:
			return ""
		}
	}
	for {
		token, err := scanner.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if token == EOFToken {
			break
		}
		printedTokens = append(printedTokens, fmt.Sprintf("%d:%s", token, tokenText(scanner, token)))
		switch token {
		case LeftParenToken:
			if depth == 0 {
				roots++
			}
			depth++
		case RightParenToken:
			depth--
			if depth < 0 {
				t.Fatal("unbalanced printed certificate")
			}
		case SymbolToken:
			if depth == 0 {
				t.Fatal("unexpected token outside printed certificate")
			}
			seen[scanner.ID()] = true
		default:
			if depth == 0 {
				t.Fatal("unexpected token outside printed certificate")
			}
		}
	}
	if depth != 0 || roots != 1 || !seen[queryName] || !seen["P"] || !seen["G"] || !seen["hyper-res"] {
		t.Fatalf("incomplete printed certificate: depth=%d roots=%d symbols=%v", depth, roots, seen)
	}
	var formatted bytes.Buffer
	if err = context.WriteExpression(&formatted, proof); err != nil {
		t.Fatal(err)
	}
	expectedScanner, err := NewScanner(strings.NewReader(formatted.String()), false)
	if err != nil {
		t.Fatal(err)
	}
	var proofTokens []string
	for {
		token, err := expectedScanner.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if token == EOFToken {
			break
		}
		proofTokens = append(proofTokens, fmt.Sprintf("%d:%s", token, tokenText(expectedScanner, token)))
	}
	if !reflect.DeepEqual(printedTokens, proofTokens) {
		t.Fatal("printed certificate differs from the checked proof")
	}
	checker, err := proofs.NewChecker(manager)
	if err != nil {
		t.Fatal(err)
	}
	valid, conditions, err := checker.Check(proof)
	defer func() {
		if err := checker.ReleaseSideConditions(conditions); err != nil {
			t.Error(err)
		}
	}()
	// Linear BMC records the source hyper-resolution positions (0, 1)
	// (dl_bmc_engine.cpp), which the proof checker's set_false rejects.
	if err == nil && valid {
		t.Fatal("source (0, 1) BMC positions passed structural checking")
	}
	if err := checker.ReleaseSideConditions(conditions); err != nil {
		t.Fatal(err)
	}
	conditions = nil
	solver, err := context.NewSimpleSolver()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := solver.Close(); err != nil {
			t.Error(err)
		}
	}()
	for i, condition := range conditions {
		if err := solver.Push(); err != nil {
			t.Fatal(err)
		}
		negated, err := manager.MkNot(condition)
		if err != nil {
			t.Fatal(err)
		}
		if err = solver.Assert(negated); err != nil {
			t.Fatal(err)
		}
		status, err := solver.CheckSat(nil)
		if err != nil || status != lbool.False {
			t.Fatalf("proof side condition %d is not valid: status=%v error=%v", i, status, err)
		}
		if err = solver.Pop(1); err != nil {
			t.Fatal(err)
		}
	}

}

func TestSMT2BMCNonlinearProofTrace(t *testing.T) {
	for _, engine := range []string{"bmc", "qbmc"} {
		t.Run(engine, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := `(set-option :fixedpoint.engine ` + engine + `)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(declare-rel A ())
(declare-rel B ())
(declare-rel Goal ())
(rule A)
(rule B)
(rule (=> (and A B) Goal))
(query Goal :print-certificate true)
`
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			text := output.String()
			if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "asserted") {
				t.Fatal("nonlinear certificate", text)
			}
		})
	}
}

func TestSMT2BMCNonlinearCertificateWithoutProofOption(t *testing.T) {
	for _, engine := range []string{"bmc", "qbmc"} {
		for _, slice := range []string{"true", "false"} {
			for _, outputKind := range []string{"print-certificate", "print-answer"} {
				t.Run(engine+"/slice="+slice+"/"+outputKind, func(t *testing.T) {
					context, err := cmd_context.NewStandaloneContext()
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := context.Close(); err != nil {
							t.Error(err)
						}
					}()
					script := fmt.Sprintf(`(set-option :fixedpoint.engine %s)
(set-option :fixedpoint.xform.slice %s)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (P x))
(rule (Q y))
(rule (=> (and (P x) (Q y)) Goal))
(query Goal :%s true)
`, engine, slice, outputKind)
					var output bytes.Buffer
					if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
						t.Fatal(err, output.String())
					}
					text := output.String()
					if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "(P ") || !strings.Contains(text, "(Q ") || strings.Contains(text, "!slice!") {
						t.Fatal("nonlinear certificate without global proof option", text)
					}
				})
			}
		}
	}
}
