package smt2_test

// Source: src/muz/fp/dl_cmds.cpp declare-rel, rule, and query commands.

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

type fixedpointCertificateRefusingWriter struct{}

func (fixedpointCertificateRefusingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func fixedpointReference(t *testing.T, script string) string {
	t.Helper()
	path := os.Getenv("G3_CPP_REFERENCE")
	if path == "" {
		var err error
		path, err = filepath.Abs("../../../build/z3")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("C++ reference executable unavailable:", err)
	}
	command := exec.Command(path, "-in")
	command.Stdin = strings.NewReader(script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	return string(output)
}

func fixedpointSpacerReference(t *testing.T, script string) string {
	t.Helper()
	script = strings.Replace(script, "(set-option :fixedpoint.engine pdr)", "(set-option :fixedpoint.engine spacer)", 1)
	return fixedpointReference(t, script)
}

func TestSMT2FixedpointPropositionalQueriesMatchCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-rel P ())
(declare-rel Q ())
(declare-rel R ())
(declare-rel U ())
(rule P)
(rule (=> P Q))
(rule (=> Q R))
(query R :print-answer true :print-statistics false :print-certificate false)
(query U :print-answer true)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\ntrue\nunsat\n" {
		t.Fatal("fixedpoint SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointRuleNameAndBoundMatchCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-rel P ())
(rule P named-rule 0)
(query P)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\n" {
		t.Fatal("fixedpoint rule bound differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointRuleScopesMatchCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-rel P ())
(declare-rel Q ())
(rule P)
(fixedpoint-push)
(rule (=> P Q))
(fixedpoint-pop)
(query Q :print-answer true)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "unsat\n" {
		t.Fatal("fixedpoint rule-scope differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointFiniteRelationsMatchCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-rel P (Bool))
(declare-rel Q (Bool))
(declare-rel U (Bool))
(declare-var x Bool)
(rule (P true))
(rule (=> (P x) (Q x)))
(query Q)
(query U)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\nunsat\n" {
		t.Fatal("finite fixedpoint SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointTwoTailJoinMatchesCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-rel P (Bool Bool))
(declare-rel Q (Bool Bool))
(declare-rel R (Bool Bool))
(declare-rel U (Bool Bool))
(declare-var x Bool)
(declare-var y Bool)
(declare-var z Bool)
(rule (P true false))
(rule (P false true))
(rule (Q false true))
(rule (Q true false))
(rule (=> (and (P x y) (Q y z)) (R z x)))
(query R)
(query U)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\nunsat\n" {
		t.Fatal("two-tail fixedpoint SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointSimpleJoinNormalizationMatchesCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
	(declare-rel A (Bool Bool))
(declare-rel B (Bool Bool))
(declare-rel C (Bool Bool))
(declare-rel Goal (Bool Bool))
(declare-var w Bool)
(declare-var x Bool)
(declare-var y Bool)
(declare-var z Bool)
(rule (A true false))
(rule (B false true))
(rule (C true false))
(rule (=> (and (A x y) (B y z) (C z w)) (Goal x w)))
(query Goal)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\n" {
		t.Fatal("simple-join fixedpoint SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointRecursiveClosureMatchesCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-rel Edge (Bool Bool))
(declare-rel Path (Bool Bool))
(declare-rel Goal ())
(declare-var x Bool)
(declare-var y Bool)
(declare-var z Bool)
(rule (Edge true false))
(rule (Edge false true))
(rule (=> (Edge x y) (Path x y)))
(rule (=> (and (Path x y) (Edge y z)) (Path x z)))
(rule (=> (Path true true) Goal))
(query Goal)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\n" {
		t.Fatal("recursive fixedpoint SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointEmptyProgramQueryMatchesCPP(t *testing.T) {
	script := "(declare-rel P ())\n(query P)\n"
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "unsat\n" {
		t.Fatal("empty fixedpoint program differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointQueryRefusesUnknownOutputParameters(t *testing.T) {
	for _, parameter := range []string{":print-proof true"} {
		t.Run(parameter, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			script := "(declare-rel P ())\n(query P " + parameter + ")\n"
			err = smt2.ParseCommands(context, strings.NewReader(script), &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "unsupported fixedpoint query parameter") {
				t.Fatal("unavailable query output accepted", err)
			}
		})
	}
}

func TestSMT2FixedpointQueryLocalParametersMatchCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine duality)
(declare-rel P ())
(rule P)
(query P :engine datalog :datalog.subsumption false :timeout 0 :print-answer true)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\ntrue\n" {
		t.Fatal("query-local fixedpoint parameter differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointPDRStatistics(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine pdr)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-var x Int)
(rule (P 0))
(rule (=> (P x) (Q x)))
(query Q :print-answer false :print-statistics true)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{"sat\n(", ":PDR-num-unfoldings", ":PDR-max-depth", ":time"} {
		if !strings.Contains(text, expected) {
			t.Fatal("missing PDR statistic", expected, text)
		}
	}
	if cpp := fixedpointSpacerReference(t, script); !strings.Contains(cpp, "\nsat\n") || !strings.Contains(cpp, ":time") {
		t.Fatal("C++ PDR statistics result", cpp)
	}
}

func TestSMT2FixedpointTupleAnswerMatchesCPP(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := "(declare-rel P (Bool))\n(rule (P true))\n(rule (P false))\n(query P :print-answer true)\n"
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\n(or (= (:var 0) true) (= (:var 0) false))\n" {
		t.Fatal("tuple answer differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointFiniteVariableExpansionMatchesCPP(t *testing.T) {
	script := `(set-option :fixedpoint.engine datalog)
(declare-var x Bool)
(declare-rel All (Bool))
(rule (All x))
(query All :print-answer true)
(declare-rel Blocked (Bool))
(declare-rel Q (Bool))
(declare-rel Goal ())
(rule (Blocked true))
(rule (=> (not (Blocked x)) (Q x)))
(rule (=> (Q false) Goal))
(query Goal)
`
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	want := "sat\n(or (= (:var 0) false) (= (:var 0) true))\nsat\n"
	if output.String() != cpp || cpp != want {
		t.Fatal("finite variable expansion differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointRejectsUnstratifiedNegation(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	script := `(declare-rel P ())
(declare-rel Q ())
(rule (=> (not P) Q))
(rule (=> (not Q) P))
(query P)
`
	err = smt2.ParseCommands(context, strings.NewReader(script), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "negation is not stratified") {
		t.Fatal("unstratified negation accepted", err)
	}
}

func TestSMT2FixedpointSpacerIntegerQueriesMatchCPP(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine spacer)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel U (Int))
(declare-var x Int)
(rule (P 0))
(rule (=> (P x) (Q x)))
(query Q :print-answer true :print-certificate true)
(query U :print-answer false)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\ntrue\n\nunsat\n" {
		t.Fatal("spacer integer SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointPDRPrintCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine pdr)
(declare-rel P (Int))
(declare-rel U (Int))
(declare-var x Int)
(rule (P 0))
(query U :print-answer false :print-certificate true)
(query P :print-answer false :print-certificate true)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unsat\n(define-fun P ((x!0 Int)) Bool\n  (= x!0 0))\n\nsat\ntrue\n"; got != want {
		t.Fatalf("PDR certificate = %q, want %q", got, want)
	}
	integer, err := context.LookupSort("Int")
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := context.LookupSort("Bool")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := context.FindFunctionDeclaration("P", nil, []ast.ASTHandle{integer}, boolean)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := context.WriteFixedpointCertificate(fixedpointCertificateRefusingWriter{}, predicate); !found || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("refused PDR certificate = %v, %v", found, err)
	}
	var retry bytes.Buffer
	if found, err := context.WriteFixedpointCertificate(&retry, predicate); err != nil || !found || retry.String() != "true" {
		t.Fatalf("retried PDR certificate = %q, %v, %v", retry.String(), found, err)
	}
}

func TestSMT2FixedpointSpacerRebuildsAfterBoundedRecursiveSearch(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine spacer)
(set-option :fixedpoint.spacer.max_level 4)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-rel U ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (< x 2)) (P (+ x 1))))
(rule (=> (P 2) Goal))
(rule (=> U U))
(query Goal :print-answer false)
(query U :print-answer false)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if text := output.String(); text != "sat\nunsat\n" {
		t.Fatal("bounded Spacer query lifecycle", text)
	}
}

func TestSMT2FixedpointSpacerRecursiveTraceAnswerMatchesCPP(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine spacer)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (< x 2)) (P (+ x 1))))
(rule (=> (P 2) Goal))
(query Goal :print-answer true)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	cpp := fixedpointReference(t, script)
	if want := "sat\n(and (P 1) (P 0))\n"; output.String() != cpp || cpp != want {
		t.Fatal("Spacer recursive trace differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointPDRBackgroundAssertionsMatchCPP(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine pdr)
(declare-rel P (Int))
(rule (P 0))
(query P :print-answer false)
(assert false)
(query P :print-answer false)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointSpacerReference(t, script)
	if output.String() != cpp || cpp != "sat\nunsat\n" {
		t.Fatal("PDR background SMT-LIB differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointPDRRecursiveTraceAnswerMatchesCPP(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine pdr)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-rel U ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (< x 2)) (P (+ x 1))))
(rule (=> (P 2) Goal))
(query Goal :print-answer true)
(query U :print-answer false)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	want := "sat\n(and (P 2) (P 1) (P 0))\nunsat\n"
	if output.String() != cpp || cpp != want {
		t.Fatal("PDR recursive trace differential", output.String(), cpp)
	}
}

func TestSMT2FixedpointPDRTraceReductionMatchesCPP(t *testing.T) {
	tests := []struct {
		name, script, want string
	}{
		{
			"acyclic-branch",
			`(set-option :fixedpoint.engine pdr)
(declare-rel A (Int))
(declare-rel B (Int))
(declare-rel C (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (A 0))
(rule (B 0))
(rule (=> (and (A x) (B x)) (C x)))
(rule (=> (C 0) Goal))
(query Goal :print-answer true)
`,
			"sat\ntrue\n",
		},
		{
			"mutual-recursion",
			`(set-option :fixedpoint.engine pdr)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (=> (P x) (Q (+ x 1))))
(rule (=> (Q x) (P (+ x 1))))
(rule (=> (P 2) Goal))
(query Goal :print-answer true)
`,
			"sat\n(and (P 2) (P 0))\n",
		},
		{
			"nonlinear-recursion",
			`(set-option :fixedpoint.engine pdr)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (Q 0))
(rule (Q 1))
(rule (=> (and (P x) (Q x) (< x 2)) (P (+ x 1))))
(rule (=> (P 2) Goal))
(query Goal :print-answer true)
`,
			"sat\ntrue\n",
		},
		{
			"multiple-initial-facts",
			`(set-option :fixedpoint.engine pdr)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (P 1))
(rule (=> (and (P x) (< x 2)) (P (+ x 1))))
(rule (=> (P 2) Goal))
(query Goal :print-answer true)
`,
			"sat\n(and (P 2) (P 1))\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			context.PrintSuccess = false
			var output bytes.Buffer
			if err = smt2.ParseCommands(context, strings.NewReader(test.script), &output); err != nil {
				t.Fatal(err)
			}
			cpp := fixedpointReference(t, test.script)
			if output.String() != cpp || cpp != test.want {
				t.Fatal("PDR trace reduction differential", output.String(), cpp)
			}
		})
	}
}

func TestSMT2FixedpointPDRRejectsUninterpretedRuleSymbolsLikeCPP(t *testing.T) {
	tests := []struct {
		name, declaration, constraint, symbol string
	}{
		{"function", "(declare-fun f (Int) Int)", "(= (f x) 1)", "f"},
		{"constant", "(declare-const c Int)", "(= x c)", "c"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			context.PrintSuccess = false
			script := `(set-option :fixedpoint.engine pdr)
(set-option :print-success false)
` + test.declaration + `
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) ` + test.constraint + ` (< x 1)) (P (+ x 1))))
(rule (=> (P 1) Goal))
(query Goal :print-answer true)
`
			var output bytes.Buffer
			err = smt2.ParseCommands(context, strings.NewReader(script), &output)
			if err == nil || !strings.Contains(err.Error(), "uninterpreted symbols in PDR rules are unsupported") || output.Len() != 0 {
				t.Fatal("Go PDR uninterpreted-rule guard", err, output.String())
			}
			cpp := fixedpointSpacerReference(t, script)
			if !strings.Contains(cpp, "Uninterpreted '"+test.symbol+"'") || !strings.HasSuffix(cpp, "unknown\n") {
				t.Fatal("C++ PDR uninterpreted-rule guard", cpp)
			}
		})
	}
}

func TestSMT2FixedpointAutoConfigIntegerQueriesMatchCPP(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	context.PrintSuccess = false
	script := `(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel U (Int))
(declare-var x Int)
(rule (P 0))
(rule (=> (P x) (Q x)))
(query Q :print-answer false)
(query U :print-answer false)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	cpp := fixedpointReference(t, script)
	if output.String() != cpp || cpp != "sat\nunsat\n" {
		t.Fatal("auto-config integer SMT-LIB differential", output.String(), cpp)
	}
}
