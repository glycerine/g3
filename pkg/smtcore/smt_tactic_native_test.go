package smtcore

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestSMTTacticCPPApplyReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to compare native apply")
	}
	for _, name := range []string{"sat", "unsat", "conflict-limit", "candidate"} {
		t.Run(name, func(t *testing.T) {
			m, evaluator := smtModelGeneratorFixture(t)
			settings, err := evaluator.Context.NewValue()
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, evaluator.Context.Del(&settings)) })
			limit := name == "conflict-limit" || name == "candidate"
			operation := "smt"
			if limit {
				key := "max_conflicts"
				astTestMust(t, evaluator.Context.SetUIntName(&settings, &key, 0))
				operation = "(using-params smt :max_conflicts 0)"
				if name == "candidate" {
					key = "candidate_models"
					astTestMust(t, evaluator.Context.SetBoolName(&settings, &key, true))
					operation = "(using-params smt :max_conflicts 0 :candidate_models true)"
				}
			}
			created, err := NewSMTTacticUsing(m, &settings, "QF_UF", true, SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
			astTestMust(t, err)
			astTestMust(t, created.IncRef())
			t.Cleanup(func() { astTestMust(t, created.DecRef()) })
			goal, err := tactic.NewGoal(m, false, true, false)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, goal.Close()) })
			a := smtRelevancyBool(t, m, "a")
			b := smtRelevancyBool(t, m, "b")
			notA, err := m.MkNot(a)
			astTestMust(t, err)
			notB, err := m.MkNot(b)
			astTestMust(t, err)
			assertions := "(assert a)"
			if limit {
				assertions = "(assert (or a b))(assert (or (not a) b))(assert (or a (not b)))(assert (or (not a) (not b)))"
				for _, pair := range [][2]ast.ASTHandle{{a, b}, {notA, b}, {a, notB}, {notA, notB}} {
					clause, err := m.MkOr(pair[0], pair[1])
					astTestMust(t, err)
					astTestMust(t, goal.Assert(clause))
				}
			} else {
				astTestMust(t, goal.Assert(a))
				if name == "unsat" {
					assertions += "(assert (not a))"
					astTestMust(t, goal.Assert(notA))
				}
			}
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			err = created.Apply(goal, &result)
			// mk_proto_model builds no model after NUM_CONFLICTS, so the
			// candidate_models application has no model converter
			// (AUDIT.md smtcore_07-F1).
			if err == nil && name == "candidate" && result.ModelConverter != nil {
				t.Fatal("conflict-limit candidate application published a model")
			}
			if err == nil && name == "sat" {
				if result.ModelConverter == nil {
					t.Fatal("successful SMT application lost its model")
				}
				var reference tactic.ModelRef
				defer func() { astTestMust(t, reference.Close()) }()
				astTestMust(t, result.ModelConverter.ApplyUnindexed(&reference))
				model, e := reference.Get()
				astTestMust(t, e)
				if model == nil {
					t.Fatal("model converter returned no model")
				}
				declaration, e := m.GetDecl(a)
				astTestMust(t, e)
				value, e := model.ConstantInterpretation(declaration)
				astTestMust(t, e)
				if !m.IsTrue(value) {
					t.Fatal("wrong Boolean interpretation in SMT model", name, value)
				}
			}
			var output bytes.Buffer
			if err != nil {
				fmt.Fprintf(&output, "(error \"tactic failed: %s\")", err)
			} else {
				printer, e := ast.NewLegacySMTPrinter(m)
				astTestMust(t, e)
				output.WriteString("(goals (goal ")
				size, e := goal.Size()
				astTestMust(t, e)
				for i := uint32(0); i < size; i++ {
					formula, e := goal.Form(i)
					astTestMust(t, e)
					astTestMust(t, printer.DisplayExpr(&output, formula, nil))
					output.WriteByte(' ')
				}
				depth, e := goal.Depth()
				astTestMust(t, e)
				fmt.Fprintf(&output, ":precision precise :depth %d))", depth)
			}
			command := exec.Command(reference, "-in")
			command.Stdin = strings.NewReader("(set-logic QF_UF)(declare-const a Bool)(declare-const b Bool)" + assertions + "(apply " + operation + ")")
			native, e := command.CombinedOutput()
			if e != nil {
				t.Fatal(e, string(native))
			}
			tokens := func(s string) string {
				return strings.Join(strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(s)), " ")
			}
			if tokens(output.String()) != tokens(string(native)) {
				t.Fatalf("SMT tactic apply differs from C++\nGo: %s\nC++: %s", output.String(), native)
			}
		})
	}
}
