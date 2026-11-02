package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestSMT2DualityRecursiveIntegerQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine duality)
(declare-rel Reach (Int))
(declare-rel Missing ())
(declare-rel GoalTwo ())
(declare-rel GoalOne ())
(declare-var x Int)
(declare-var y Int)
(rule (Reach 0))
(rule (=> (and (Reach x) (= y (+ x 1))) (Reach y)))
(rule (=> (Reach 2) GoalTwo))
(rule (=> (Reach 1) GoalOne))
(query GoalTwo)
(query Missing)
(query GoalOne)
`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\nunsat\nsat\n" {
		t.Fatalf("Duality query results: %q", output.String())
	}
}

func TestSMT2DualitySafetyCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(declare-rel P ())
(declare-rel Q ())
(rule P)
(query Q :print-answer false :print-certificate true)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "unsat\n(fixedpoint \n") || !strings.Contains(output.String(), "define-fun Q") {
		t.Fatalf("safety certificate: %q", output.String())
	}
}

func TestSMT2DualityReachableCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(declare-rel Reach (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (Reach 0))
(rule (=> (and (Reach x) (= y (+ x 1))) (Reach y)))
(rule (=> (Reach 2) Goal))
(query Goal :print-answer false :print-certificate true)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n(derivation\n") || !strings.Contains(text, "(Reach 2)") || !strings.Contains(text, "(model \n\"") {
		t.Fatal("reachable certificate", text)
	}
}

func TestSMT2DualityArrayQuantificationQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel P ((Array Int Int)))
(declare-rel Goal ())
(declare-rel Missing ())
(declare-var a (Array Int Int))
(rule (=> (= (select a 0) 7) (P a)))
(rule (=> (and (P a) (= (select a 0) 7)) Goal))
(query Goal)
(query Missing)
(query Goal)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\nunsat\nsat\n" {
		t.Fatalf("array abstraction results %q", output.String())
	}
}

func TestSMT2DualityArrayValuedOutputQuantification(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel P ((Array Int Int)))
(declare-rel Missing ((Array Int Int)))
(declare-var a (Array Int Int))
(rule (P a))
(query P)
(query Missing)
(query P)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\nunsat\nsat\n" {
		t.Fatalf("array output results %q", output.String())
	}
}

func TestSMT2DualityQuantifiedArraySafeModelCertificate(t *testing.T) {
	t.Run("safe-model", func(t *testing.T) {
		context, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := context.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		context.PrintSuccess = false
		var output bytes.Buffer
		script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel P ((Array Int Int)))
(declare-rel Missing ())
(declare-var a (Array Int Int))
(rule (=> (= (select a 0) 7) (P a)))
(query Missing :print-answer false :print-certificate true)
`
		if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(output.String(), "unsat\n(fixedpoint \n") || !strings.Contains(output.String(), "define-fun query!") {
			t.Fatal("quantified-array safety certificate", output.String())
		}
		m, err := context.Manager()
		if err != nil {
			t.Fatal(err)
		}
		boolean, err := m.MkBoolSort()
		if err != nil {
			t.Fatal(err)
		}
		missing, err := context.FindFunctionDeclaration("Missing", nil, nil, boolean)
		if err != nil {
			t.Fatal(err)
		}
		md, err := context.FixedpointModel(missing)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := md.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		value, err := md.Core().ConstantInterpretation(missing)
		if err != nil {
			t.Fatal(err)
		}
		if !value.IsNull() {
			t.Fatal("source transformed model unexpectedly reconstructed Missing", value)
		}
		queryFalse := false
		for _, declaration := range md.Core().ConstantDeclarations {
			name, err := m.GetASTName(declaration)
			if err != nil {
				t.Fatal(err)
			}
			text, _, err := m.Symbols.BareString(name)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(text, "query!") {
				interpretation, err := md.Core().ConstantInterpretation(declaration)
				if err != nil {
					t.Fatal(err)
				}
				queryFalse = m.IsFalse(interpretation)
			}
		}
		if !queryFalse {
			t.Fatal("quantified-array transformed model did not refute synthetic query")
		}
	})
}

func TestSMT2DualityBackgroundCertificateWitness(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(declare-fun background-k () Int)
(assert (= background-k 7))
(declare-rel Goal ())
(rule (forall ((w Int)) (=> (= w background-k) Goal)))
(query Goal :print-answer false :print-certificate true)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n(derivation\n") || !strings.Contains(text, "(= w 7)") || !strings.Contains(text, "background-k -> 7") {
		t.Fatal("background witness certificate", text)
	}
}

func TestSMT2DualityRecursiveIntegerUnderapproximation(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.duality.use_underapprox true)
(declare-rel Reach (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (Reach 0))
(rule (=> (and (Reach x) (= y (+ x 1))) (Reach y)))
(rule (=> (Reach 3) Goal))
(query Goal :print-answer false :print-certificate true)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n(derivation\n") || !strings.Contains(text, "(Reach 1)") || !strings.Contains(text, "(Reach 2)") || !strings.Contains(text, "(Reach 3)") {
		t.Fatal("incomplete recursive certificate", text)
	}
}

func TestSMT2DualityMutualRecursionIntegerUnderapproximation(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.duality.use_underapprox true)
(declare-rel EvenStep (Int))
(declare-rel OddStep (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (EvenStep 0))
(rule (=> (and (EvenStep x) (= y (+ x 1))) (OddStep y)))
(rule (=> (and (OddStep x) (= y (+ x 1))) (EvenStep y)))
(rule (=> (OddStep 3) Goal))
(query Goal :print-answer false :print-certificate true)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n(derivation\n") {
		t.Fatal("mutual recursion result", text)
	}
	for _, instance := range []string{"(EvenStep 0)", "(OddStep 1)", "(EvenStep 2)", "(OddStep 3)"} {
		if !strings.Contains(text, instance) {
			t.Fatal("missing mutual recursion certificate instance", instance, text)
		}
	}
}

func TestSMT2FixedpointMutualRecursionUnreachableParity(t *testing.T) {
	for _, engine := range []string{"pdr", "spacer", "duality"} {
		for _, under := range []string{"true", "false"} {
			t.Run(engine+"/underapprox="+under, func(t *testing.T) {
				context, err := cmd_context.NewStandaloneContext()
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := context.Close(); err != nil {
						t.Fatal(err)
					}
				}()
				context.PrintSuccess = false
				var output bytes.Buffer
				script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.duality.use_underapprox true)
(declare-rel EvenStep (Int))
(declare-rel OddStep (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (EvenStep 0))
(rule (=> (and (EvenStep x) (< x 4) (= y (+ x 1))) (OddStep y)))
(rule (=> (and (OddStep x) (< x 4) (= y (+ x 1))) (EvenStep y)))
(rule (=> (EvenStep 3) Goal))
(query Goal :print-answer false)
`
				script = strings.Replace(script, ":fixedpoint.engine duality", ":fixedpoint.engine "+engine, 1)
				script = strings.Replace(script, ":fixedpoint.duality.use_underapprox true", ":fixedpoint.duality.use_underapprox "+under, 1)
				if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
					t.Fatal(err)
				}
				if output.String() != "unsat\n" {
					t.Fatal("unreachable parity query", output.String())
				}
			})
		}
	}
}
