package arith

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func fmInputTags(t *testing.T, fm *FMTactic, out *bytes.Buffer) *diagnostic.TraceTags {
	t.Helper()
	tags := diagnostic.NewTraceTagsWithWriter(out)
	for _, tag := range []string{"is_occ_bug", "to_var_bug", "mk_constraint_bug", "add_constraint_bug", "fm_bug"} {
		tags.Enable(tag)
	}
	equationMust(t, fm.BindTrace(tags))
	return tags
}
func TestFMInputTraceSourceRecognitionCreationAndCopyOrder(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	var out bytes.Buffer
	tags := fmInputTags(t, fm, &out)
	name := "fm_limit"
	equationMust(t, fm.context.SetUIntName(&fm.params, &name, 0))
	equationMust(t, fm.UpdateParams(&fm.params))
	x := fixDLConst(t, m, false, "x")
	lower := fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "0"))
	upper := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))
	goal := fixDLGoal(t, m, false, false, false, lower, upper)
	equationMust(t, fm.Apply(goal, fmResult(t)))
	record := fmConstraintRecord
	want := record("is_occ_bug", "is_linear_ineq", 481, "(>= x 0.0)\n") +
		record("is_occ_bug", "is_linear_ineq", 481, "(<= x 4.0)\n") +
		record("is_occ_bug", "is_linear_ineq", 481, "(>= x 0.0)\n") +
		record("to_var_bug", "to_var", 971, "x --> 0\n") +
		record("to_var_bug", "add_constraint", 1070, "before mk_constraint:  0\n") +
		record("mk_constraint_bug", "mk_constraint", 557, "xs[0]: 0\n") +
		record("to_var_bug", "add_constraint", 1081, "add_constraint: (-1*x <= 0)\n") +
		record("is_occ_bug", "is_linear_ineq", 481, "(<= x 4.0)\n") +
		record("to_var_bug", "to_var", 971, "x --> 0\n") +
		record("to_var_bug", "add_constraint", 1070, "before mk_constraint:  0\n") +
		record("mk_constraint_bug", "mk_constraint", 557, "xs[0]: 0\n") +
		record("to_var_bug", "add_constraint", 1081, "add_constraint: (x <= 4)\n") +
		record("fm_bug", "copy_remaining", 1530, "asserting...\n(<= x 4.0)\nnew_dep: 0\n") +
		record("fm_bug", "copy_remaining", 1530, "asserting...\n(<= (* (- 1.0) x) 0.0)\nnew_dep: 0\n")
	if tags.ShouldTrace("is_occ_bug") {
		if out.String() != want {
			t.Fatalf("source parser records\ngot %q\nwant %q", out.String(), want)
		}
	} else if out.Len() != 0 {
		t.Fatal("erased parser traces", out.String())
	}
}
func TestFMInputTracePureBooleanClauseRecognitionModes(t *testing.T) {
	for _, clauses := range []bool{false, true} {
		t.Run(fmt.Sprint(clauses), func(t *testing.T) {
			fm, m := fmTacticFixture(t, false)
			name := "fm_occ"
			equationMust(t, fm.context.SetBoolName(&fm.params, &name, clauses))
			equationMust(t, fm.UpdateParams(&fm.params))
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			tags.Enable("is_occ_bug")
			equationMust(t, fm.BindTrace(tags))
			p, q := fmBoolConst(t, m, "p"), fmBoolConst(t, m, "q")
			disjunction, err := m.MkOrArgs([]ast.ASTHandle{p, q})
			equationMust(t, err)
			fixDLPin(t, m, disjunction)
			goal := fixDLGoal(t, m, false, false, false, disjunction)
			result := fmResult(t)
			equationMust(t, fm.Apply(goal, result))
			want := fmConstraintRecord("is_occ_bug", "init_forbidden_set", 820, "not OCC:\n(or p q)\n")
			if !clauses {
				rec := fmConstraintRecord("is_occ_bug", "is_linear_ineq", 481, "(or p q)\n")
				want = rec + want + rec
			}
			if tags.ShouldTrace("is_occ_bug") && out.String() != want {
				t.Fatalf("clause=%v got %q want %q", clauses, out.String(), want)
			}
			size, err := result.Goals[0].Size()
			equationMust(t, err)
			if size != 1 {
				t.Fatal("non-OCC formula changed", size)
			}
		})
	}
}
func TestFMInputTraceNegationAndDeferredRegistration(t *testing.T) {
	t.Run("stripped negation", func(t *testing.T) {
		fm, m := fmTacticFixture(t, false)
		var out bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&out)
		tags.Enable("is_occ_bug")
		equationMust(t, fm.BindTrace(tags))
		x := fixDLConst(t, m, false, "x")
		bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "1"))
		negated, err := m.MkNot(bound)
		equationMust(t, err)
		fixDLPin(t, m, negated)
		goal := fixDLGoal(t, m, false, false, false, negated)
		equationMust(t, fm.Apply(goal, fmResult(t)))
		want := strings.Repeat(fmConstraintRecord("is_occ_bug", "is_linear_ineq", 481, "(<= x 1.0)\n"), 2)
		if tags.ShouldTrace("is_occ_bug") && out.String() != want {
			t.Fatal(out.String(), want)
		}
	})
	t.Run("deferred resolvent", func(t *testing.T) {
		fm, m := fmTacticFixture(t, false)
		var out bytes.Buffer
		tags := diagnostic.NewTraceTagsWithWriter(&out)
		tags.Enable("add_constraint_bug")
		tags.Enable("fm_bug")
		equationMust(t, fm.BindTrace(tags))
		x, y := fixDLConst(t, m, false, "x"), fixDLConst(t, m, false, "y")
		goal := fixDLGoal(t, m, false, false, false, fmPredicate(t, m, y), fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "0")), fixDLApp(t, m, 2, fixDLApp(t, m, 6, x, y), fixDLNumber(t, m, false, "3")))
		equationMust(t, fm.Apply(goal, fmResult(t)))
		if tags.ShouldTrace("add_constraint_bug") {
			want := fmConstraintRecord("add_constraint_bug", "register_constraint", 1117, "all variables are forbidden (y <= 3)\n")
			if !strings.Contains(out.String(), want) || strings.Contains(out.String(), "copy_remaining") {
				t.Fatal("deferred source assertion route", out.String())
			}
		}
	})
}
func TestFMInputTraceAllocationFrontiersAndStreamFailure(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 1024; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			fm, m := fmTacticFixture(t, false)
			var out bytes.Buffer
			fmInputTags(t, fm, &out)
			name := "fm_limit"
			equationMust(t, fm.context.SetUIntName(&fm.params, &name, 0))
			equationMust(t, fm.UpdateParams(&fm.params))
			x := fixDLConst(t, m, false, "x")
			goal := fixDLGoal(t, m, false, true, true, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "0")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")))
			result := fmResult(t)
			m.Budget().FailAfter(stop)
			err := fm.Apply(goal, result)
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
			equationMust(t, fm.Cleanup())
			refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
			for _, tag := range []string{"is_occ_bug", "to_var_bug", "mk_constraint_bug", "add_constraint_bug", "fm_bug"} {
				refused.Enable(tag)
			}
			equationMust(t, fm.BindTrace(refused))
			equationMust(t, fm.Apply(goal, result))
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("never reached successful traced parsing")
	}
}

func TestFMInputTracePlainLiteralStillVisitsInequalityRecognizer(t *testing.T) {
	for _, clauses := range []bool{false, true} {
		t.Run(fmt.Sprint(clauses), func(t *testing.T) {
			fm, m := fmTacticFixture(t, false)
			name := "fm_occ"
			equationMust(t, fm.context.SetBoolName(&fm.params, &name, clauses))
			equationMust(t, fm.UpdateParams(&fm.params))
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			tags.Enable("is_occ_bug")
			equationMust(t, fm.BindTrace(tags))
			goal := fixDLGoal(t, m, false, false, false, fmBoolConst(t, m, "p"))
			equationMust(t, fm.Apply(goal, fmResult(t)))
			recognition := fmConstraintRecord("is_occ_bug", "is_linear_ineq", 481, "p\n")
			want := recognition + fmConstraintRecord("is_occ_bug", "init_forbidden_set", 820, "not OCC:\np\n") + recognition
			if tags.ShouldTrace("is_occ_bug") && out.String() != want {
				t.Fatal(out.String(), want)
			}
		})
	}
}
