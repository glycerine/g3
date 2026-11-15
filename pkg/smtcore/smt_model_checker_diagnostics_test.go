package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelCheckerDiagnosticOutcomesAndGuards(t *testing.T) {
	for _, scenario := range []string{"accepted", "two-failures", "unnamed", "verbose-failure", "iteration-limit", "quiet-limit", "restart", "closed-disabled", "closed-enabled", "empty-registry"} {
		t.Run(scenario, func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			checker, err := NewSMTModelChecker(solver.kernel.internalizer, 1)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, checker.Close()) })
			proto, err := protomodel.NewProtoModel(m, config)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, proto.Close()) })
			level := uint32(0)
			if scenario == "verbose-failure" {
				level = 5
			}
			if scenario == "iteration-limit" {
				level = 1
			}
			if scenario == "restart" {
				level = 100
			}
			out := smtAssertedVerboseFixture(t, level)
			checker.trace = scenario != "verbose-failure" && scenario != "closed-disabled"
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			variable, err := m.MkVar(0, boolean)
			astTestMust(t, err)
			makeQ := func(id symbol.Symbol, body ast.ASTHandle) ast.ASTHandle {
				q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "x")}, QID: id, Body: body})
				astTestMust(t, err)
				ref, err := m.NewRef(q)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
				if scenario != "empty-registry" {
					astTestMust(t, solver.kernel.internalizer.Internalize(q, true))
				}
				return q
			}
			id := astTestSymbol(t, m, "first")
			if scenario == "unnamed" {
				id = symbol.Symbol{}
			}
			body := variable
			if scenario == "accepted" || scenario == "closed-disabled" {
				body, err = m.MkTrue()
				astTestMust(t, err)
			}
			qs := []ast.ASTHandle{makeQ(id, body)}
			if scenario == "two-failures" {
				negative, err := m.MkNot(variable)
				astTestMust(t, err)
				qs = append(qs, makeQ(astTestSymbol(t, m, "second"), negative))
			}
			if scenario == "iteration-limit" || scenario == "quiet-limit" {
				astTestMust(t, checker.ConfigureLifecycle(1, 0))
			}
			if strings.HasPrefix(scenario, "closed-") {
				astTestMust(t, out.Close())
			}
			var accepted bool
			if scenario == "restart" {
				err = checker.Restart()
			} else {
				if scenario == "empty-registry" {
					qs = nil
				}
				accepted, err = checker.Check(proto, qs)
			}
			if scenario == "closed-enabled" {
				if err == nil {
					t.Fatal("enabled diagnostics ignored closed stream")
				}
				return
			}
			astTestMust(t, err)
			if scenario == "closed-disabled" {
				if !accepted {
					t.Fatal("disabled output changed accepted candidate")
				}
				return
			}
			got, err := out.String()
			astTestMust(t, err)
			expected := ""
			switch scenario {
			case "accepted":
				expected = "(smt.mbqi \"started\")\n(smt.mbqi :checking first)\n(smt.mbqi :succeeded true)\n"
				if !accepted {
					t.Fatal("accepted candidate rejected")
				}
			case "two-failures":
				expected = "(smt.mbqi \"started\")\n(smt.mbqi :checking first)\n(smt.mbqi :failed first)\n(smt.mbqi :checking second)\n(smt.mbqi :failed second)\n(smt.mbqi :num-failures 2)\n"
			case "unnamed":
				expected = "(smt.mbqi \"started\")\n(smt.mbqi :failed null)\n(smt.mbqi :num-failures 1)\n"
			case "verbose-failure":
				expected = "(smt.mbqi :failed first)\n"
			case "iteration-limit":
				expected = "(smt.mbqi \"max instantiations 0 reached\")\n"
			case "restart":
				expected = "(smt.mbqi \"instantiating new instances...\")\n"
			}
			if got != expected {
				t.Fatalf("diagnostic mismatch\ngot %q\nwant %q", got, expected)
			}
			// smtcore_05-F5: model_checker::check calls
			// set_reason_unknown("max mbqi instantiations reached").
			reason := solver.kernel.unknown
			if limited := scenario == "iteration-limit" || scenario == "quiet-limit"; limited != (reason == "max mbqi instantiations reached") {
				t.Fatalf("reason unknown %q after %s", reason, scenario)
			}
		})
	}
}

func TestSMTModelCheckerIncompleteQuantifierDoesNotCountLaterSuccessAsFailure(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	checker, err := NewSMTModelChecker(solver.kernel.internalizer, 1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, checker.Close()) })
	proto, err := protomodel.NewProtoModel(m, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	out := smtAssertedVerboseFixture(t, 0)
	checker.trace = true
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	inner, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "y")}, Body: x})
	astTestMust(t, err)
	outer, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "x")}, QID: astTestSymbol(t, m, "incomplete"), Body: inner})
	astTestMust(t, err)
	outerRef, err := m.NewRef(outer)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&outerRef)) })
	yes, err := m.MkTrue()
	astTestMust(t, err)
	valid, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "x")}, QID: astTestSymbol(t, m, "valid"), Body: yes})
	astTestMust(t, err)
	validRef, err := m.NewRef(valid)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&validRef)) })
	accepted, err := checker.Check(proto, []ast.ASTHandle{outer, valid})
	astTestMust(t, err)
	got, err := out.String()
	astTestMust(t, err)
	if accepted || !checker.Exhausted() || strings.Contains(got, ":failed valid") || !strings.HasSuffix(got, "(smt.mbqi :num-failures 1)\n") {
		t.Fatal("incomplete outcome contaminated later quantifier result", accepted, checker.Exhausted(), got)
	}
}
