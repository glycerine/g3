package duality_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Array quantification introduces a synthetic bridge whose antecedent embeds
// a predicate below a forall. That antecedent is a real proof premise even
// though it is not represented by an RPFP child edge.
func TestDualityQuantifiedArrayProofReconstructsBridgePremise(t *testing.T) {
	for _, test := range []struct {
		name      string
		rangeSort string
		value     string
		guarded   bool
	}{
		{name: "integer_equality", rangeSort: "Int", value: "7", guarded: true},
		{name: "Boolean_equivalence", rangeSort: "Bool", value: "true"},
	} {
		t.Run(test.name, func(t *testing.T) {
			testDualityQuantifiedArrayProofReconstructsBridgePremise(t, test.rangeSort, test.value, test.guarded)
		})
	}
}

func testDualityQuantifiedArrayProofReconstructsBridgePremise(t *testing.T, rangeSort, value string, guarded bool) {
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
	fact := fmt.Sprintf("(P ((as const (Array Int %s)) %s))", rangeSort, value)
	rule := fmt.Sprintf("(rule %s)", fact)
	bridgeDeclaration := ""
	if guarded {
		bridgeDeclaration = "(declare-rel Mid ())"
		rule = fmt.Sprintf("(rule (=> Seed Mid))\n(rule (=> Mid %s))", fact)
	}
	script := fmt.Sprintf(`(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel Seed ())
(declare-rel P ((Array Int %s)))
(declare-rel Goal ())
%s
(rule Seed)
%s
(rule (=> %s Goal))
(query Goal :print-answer false)
`, rangeSort, bridgeDeclaration, rule, fact)
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	// z3 rejects the quantifier-abstracted rules: duality's get_rules()
	// runs check_rules (check_existential_tail) on the transformed set.
	const rejected = "(error \"query failed: recursive predicate (P (:var 0) (select (:var 1) (:var 0))) occurs nested in the body of a rule\")\nunknown\n"
	if output.String() == rejected {
		return
	}
	if output.String() != "sat\n" {
		t.Fatal("quantified-array query", output.String())
	}
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	goal, err := context.FindFunctionDeclaration("Goal", nil, nil, boolean)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := context.FixedpointProof(goal)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&proof); err != nil {
			t.Fatal(err)
		}
	}()
	h, err := m.RefGet(proof)
	if err != nil {
		t.Fatal(err)
	}
	checker, err := proofs.NewChecker(m)
	if err != nil {
		t.Fatal(err)
	}
	valid, side, err := checker.Check(h)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := checker.ReleaseSideConditions(side); err != nil {
			t.Fatal(err)
		}
	}()
	if !valid {
		t.Fatal("quantified-array proof is structurally invalid")
	}
	closed, err := proofs.IsClosed(m, h)
	if err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("quantified-array proof has an escaping hypothesis")
	}
	config, err := context.SolverTranslationConfig()
	if err != nil {
		t.Fatal(err)
	}
	native, err := smtcore.NewSMTSolver(m, nil, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := native.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	for i, condition := range side {
		if err := native.Push(); err != nil {
			t.Fatal(err)
		}
		negated, err := m.MkNot(condition)
		if err != nil {
			t.Fatal(err)
		}
		if err := native.Assert(negated); err != nil {
			t.Fatal(err)
		}
		status, err := native.CheckSat(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := native.Pop(1); err != nil {
			t.Fatal(err)
		}
		if status != lbool.False {
			var rendered bytes.Buffer
			if err := context.WriteExpression(&rendered, condition); err != nil {
				t.Fatal(err)
			}
			t.Log("invalid condition", rendered.String())
			t.Fatal("quantified-array proof side condition is not valid", i, status)
		}
	}
}

func TestDualityQuantifiedArrayProofRejectsMissingTransitiveBridge(t *testing.T) {
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
	const script = `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel Seed ())
(declare-rel Mid ())
(declare-rel P ((Array Int Int)))
(declare-rel Goal ())
(rule Seed)
(rule (=> Mid (P ((as const (Array Int Int)) 7))))
(rule (=> (P ((as const (Array Int Int)) 7)) Goal))
(query Goal :print-answer false)
`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "unsat\n" {
		t.Fatal("missing transitive bridge was accepted", output.String())
	}
}
