package fp_test

// Source: src/muz/base/dl_engine_base.h,
// src/muz/fp/dl_register_engine.{h,cpp},
// src/muz/pdr/pdr_dl_interface.{h,cpp}, and
// src/muz/spacer/spacer_dl_interface.{h,cpp}.

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func newRoutingEngine(t *testing.T) (*fp.PropositionalEngine, *ast.ASTManager) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := context.Manager()
	fpMust(t, err)
	globals, err := context.GlobalParameters()
	fpMust(t, err)
	engine, err := fp.NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fpMust(t, err)
	t.Cleanup(func() {
		fpMust(t, engine.Close())
		fpMust(t, context.Close())
	})
	return engine, manager
}

func TestFixedpointAutoConfigQuantifiedRule(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	_, head := routingPredicate(t, engine, manager, "auto-quantified")
	rule := autoQuantifiedRule(t, manager, head)
	fpMust(t, engine.AddRule(rule, "quantified"))
	fpMust(t, engine.SelectEngine("auto-config"))
	kind, err := engine.SelectedEngine()
	fpMust(t, err)
	if kind != fp.EngineQPDR {
		t.Fatal("quantified rule selected", kind)
	}
}

func autoQuantifiedRule(t *testing.T, manager *ast.ASTManager, head ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	family, err := manager.RegisterArithmeticPlugin()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fpMust(t, err)
	x, err := manager.MkVar(0, integer)
	fpMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	positive, err := manager.MkArithmeticGT(x, zero)
	fpMust(t, err)
	quantified, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: positive})
	fpMust(t, err)
	rule, err := manager.MkImplies(quantified, head)
	fpMust(t, err)
	return rule
}

func TestFixedpointAutoConfigRestoresRuleClassification(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	_, head := routingPredicate(t, engine, manager, "auto-scope")
	fpMust(t, engine.AddRule(head, "fact"))
	check := func(want fp.EngineKind) {
		t.Helper()
		fpMust(t, engine.SelectEngine("auto-config"))
		kind, err := engine.SelectedEngine()
		fpMust(t, err)
		if kind != want {
			t.Fatal("selected engine", kind, "want", want)
		}
	}
	check(fp.EngineDatalog)
	fpMust(t, engine.Push())
	fpMust(t, engine.UpdateRule(autoQuantifiedRule(t, manager, head), "fact"))
	check(fp.EngineQPDR)
	fpMust(t, engine.Pop())
	check(fp.EngineDatalog)
}

func TestFixedpointAutoConfigStopsAtFirstNonDatalogRule(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	family, err := manager.RegisterArithmeticPlugin()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fpMust(t, err)
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("auto-order-integer")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	fpMust(t, err)
	fact, err := manager.MkApp(declaration, []ast.ASTHandle{zero})
	fpMust(t, err)
	fpMust(t, engine.AddRule(fact, "integer-fact"))
	_, head := routingPredicate(t, engine, manager, "auto-order-quantified")
	fpMust(t, engine.AddRule(autoQuantifiedRule(t, manager, head), "quantified"))
	fpMust(t, engine.SelectEngine("auto-config"))
	kind, err := engine.SelectedEngine()
	fpMust(t, err)
	if kind != fp.EnginePDR {
		t.Fatal("later rule changed first non-Datalog selection", kind)
	}
}

func TestFixedpointAutoEmptyInfiniteRelationAnswer(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	family, err := manager.RegisterArithmeticPlugin()
	fpMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fpMust(t, err)
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("auto-empty-infinite")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	finite, _ := routingPredicate(t, engine, manager, "auto-empty-finite")
	fpMust(t, engine.SelectEngine("auto-config"))
	kind, err := engine.SelectedEngine()
	fpMust(t, err)
	if kind != fp.EngineDatalog {
		t.Fatal("empty program engine", kind)
	}
	status, err := engine.QueryPredicates([]ast.ASTHandle{declaration, finite})
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("empty relations result", status)
	}
	answer, err := engine.Answer(declaration)
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&answer)) }()
	value, err := manager.RefGet(answer)
	fpMust(t, err)
	if !manager.IsFalse(value) {
		t.Fatal("empty infinite relation answer", value)
	}
}

func TestFixedpointAutoIgnoresOuterRuleBinders(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	bitVector, err := manager.BitVectorSort(4)
	fpMust(t, err)
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("auto-bound-bitvector")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{bitVector}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	x, err := manager.MkVar(0, bitVector)
	fpMust(t, err)
	head, err := manager.MkApp(declaration, []ast.ASTHandle{x})
	fpMust(t, err)
	rule, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{bitVector}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: head})
	fpMust(t, err)
	fpMust(t, engine.AddRule(rule, "bound-fact"))
	fpMust(t, engine.SelectEngine("auto-config"))
	kind, err := engine.SelectedEngine()
	fpMust(t, err)
	if kind != fp.EngineDatalog {
		t.Fatal("outer rule binder selected", kind)
	}
}

func TestDDNFQueryDumpsCompiledRulesAndReturnsUnknown(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	var dump bytes.Buffer
	fpMust(t, engine.SetDDNFOutput(&dump))
	fpMust(t, engine.SelectEngine("ddnf"))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("ddnf-routing-predicate")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	fpMust(t, err)
	fpMust(t, engine.AddRule(fact, "ddnf-fact"))
	status, err := engine.QueryPredicate(predicate)
	fpMust(t, err)
	if status != lbool.Undef {
		t.Fatal("disabled DDNF executor returned a decided result", status)
	}
	if !bytes.Contains(dump.Bytes(), []byte("(set-logic HORN)")) || !bytes.Contains(dump.Bytes(), []byte("ddnf-fact")) || !bytes.Contains(dump.Bytes(), []byte("ddnf-routing-predicate")) || bytes.Count(dump.Bytes(), []byte("(assert ")) != 2 {
		t.Fatalf("compiled DDNF SMT2 dump is incomplete: %s", dump.String())
	}
	last, err := engine.LastResult()
	fpMust(t, err)
	if last != lbool.Undef {
		t.Fatal("DDNF result snapshot", last)
	}
	// Source ddnf::imp::collect_statistics and reset_statistics are empty.
	// Preserve caller-owned contributions across both no-op operations.
	stats := statistics.New()
	stats.UpdateUint("seed", 7)
	fpMust(t, engine.CollectStatistics(stats))
	if stats.Size() != 1 || stats.Key(0) != "seed" || !stats.IsUint(0) || stats.UintValue(0) != 7 {
		t.Fatal("DDNF collection changed source-empty statistics", stats.Size())
	}
	fpMust(t, engine.ResetStatistics())
	fpMust(t, engine.CollectStatistics(stats))
	if stats.Size() != 1 || stats.Key(0) != "seed" || !stats.IsUint(0) || stats.UintValue(0) != 7 {
		t.Fatal("DDNF reset changed source-empty statistics", stats.Size())
	}
	answer, answerErr := engine.Answer(predicate)
	if answer != (ast.ASTRef{}) {
		fpMust(t, manager.DelRef(&answer))
	}
	if !errors.Is(answerErr, fp.ErrPropositionalEngine) {
		t.Fatal("disabled DDNF executor exposed a fabricated answer", answerErr)
	}
	if model, modelErr := engine.Model(predicate); !errors.Is(modelErr, fp.ErrPropositionalEngine) {
		if model != nil {
			fpMust(t, model.Close())
		}
		t.Fatal("disabled DDNF executor exposed a fabricated model", modelErr)
	}
	var certificate bytes.Buffer
	if found, certErr := engine.WriteCertificate(&certificate, predicate, func(io.Writer, ast.ASTHandle) error { return nil }); found || !errors.Is(certErr, fp.ErrPropositionalEngine) || certificate.Len() != 0 {
		t.Fatal("unreachable DDNF certificate path was hidden", found, certErr, certificate.String())
	}
}

func TestDDNFRejectedRuleIsPrintedBeforeUnknown(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	var output bytes.Buffer
	fpMust(t, engine.SetDDNFOutput(&output))
	fpMust(t, engine.SelectEngine("ddnf"))
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	bitVector, err := manager.BitVectorSort(4)
	fpMust(t, err)
	name, err := manager.Symbols.Intern("ddnf-rejected-rule")
	fpMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{bitVector}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(predicate))
	variable, err := manager.MkVar(0, bitVector)
	fpMust(t, err)
	numerals := manager.NumeralService()
	value, err := numerals.NewValue()
	fpMust(t, err)
	fpMust(t, numerals.SetUint64(&value, 3))
	constant, err := manager.BitVectorNumeral(&value, 4)
	fpMust(t, err)
	fpMust(t, numerals.Del(&value))
	comparison, err := manager.MkBitVectorULE(variable, constant)
	fpMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	fpMust(t, err)
	rule, err := manager.MkImplies(comparison, head)
	fpMust(t, err)
	fpMust(t, engine.AddRule(rule, "rejected-bound"))
	status, err := engine.QueryPredicate(predicate)
	fpMust(t, err)
	if status != lbool.Undef || !bytes.Contains(output.Bytes(), []byte("rejected-bound:")) || bytes.Contains(output.Bytes(), []byte("(set-logic HORN)")) {
		t.Fatalf("source rejected-rule output/status: %v, %v, %s", status, err, output.String())
	}
}

func routingPredicate(t *testing.T, engine *fp.PropositionalEngine, manager *ast.ASTManager, suffix string) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("engine-routing-" + suffix)
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	application, err := manager.MkApp(declaration, nil)
	fpMust(t, err)
	fpMust(t, engine.RegisterPredicate(declaration))
	return declaration, application
}

func TestRetainedEngineKindSelectionAndDiagnostics(t *testing.T) {
	engine, _ := newRoutingEngine(t)
	for _, test := range []struct {
		name string
		kind fp.EngineKind
	}{
		{"datalog", fp.EngineDatalog},
		{"pdr", fp.EnginePDR},
		{"qpdr", fp.EngineQPDR},
		{"spacer", fp.EngineSpacer},
		{"bmc", fp.EngineBMC},
		{"qbmc", fp.EngineQBMC},
		{"tab", fp.EngineTab},
		{"clp", fp.EngineCLP},
		{"duality", fp.EngineDuality},
		{"ddnf", fp.EngineDDNF},
	} {
		fpMust(t, engine.SelectEngine(test.name))
		kind, err := engine.SelectedEngine()
		fpMust(t, err)
		if kind != test.kind || kind.String() != test.name || !kind.Retained() {
			t.Fatal("selected engine", kind, kind.String(), kind.Retained())
		}
	}
	if err := engine.SelectEngine("not-an-engine"); err == nil {
		t.Fatal("unknown engine spelling accepted")
	}
}

func TestDatalogResultSnapshotRoutesAndInvalidates(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	declaration, application := routingPredicate(t, engine, manager, "datalog")
	if _, err := engine.Answer(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("answer available before query", err)
	}
	if _, err := engine.Model(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("model available before query", err)
	}
	fpMust(t, engine.AddRule(application, "fact"))
	status, err := engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("fact query", status)
	}
	last, err := engine.LastResult()
	fpMust(t, err)
	if last != lbool.True {
		t.Fatal("last result", last)
	}
	answer, err := engine.Answer(declaration)
	fpMust(t, err)
	fpMust(t, manager.DelRef(&answer))
	resultModel, err := engine.Model(declaration)
	fpMust(t, err)
	fpMust(t, resultModel.Close())
	if _, err = engine.Proof(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("Datalog exposed PDR proof", err)
	}
	var output bytes.Buffer
	found, err := engine.WriteCertificate(&output, declaration, func(io.Writer, ast.ASTHandle) error { return nil })
	if err != nil || found || output.Len() != 0 {
		t.Fatal("Datalog certificate route", found, output.String(), err)
	}

	secondDeclaration, secondApplication := routingPredicate(t, engine, manager, "second")
	fpMust(t, engine.AddRule(secondApplication, "second-fact"))
	for operation, err := range map[string]error{
		"answer": func() error { _, e := engine.Answer(declaration); return e }(),
		"model":  func() error { _, e := engine.Model(declaration); return e }(),
		"last":   func() error { _, e := engine.LastResult(); return e }(),
	} {
		if !errors.Is(err, fp.ErrPropositionalEngine) {
			t.Fatal(operation, "accepted stale result", err)
		}
	}
	_ = secondDeclaration
}

func TestPDRAndSpacerPreQueryResultGuards(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			engine, manager := newRoutingEngine(t)
			declaration, _ := routingPredicate(t, engine, manager, backend)
			fpMust(t, engine.SelectEngine(backend))
			assertOwner := func(name string, err error) {
				t.Helper()
				if !errors.Is(err, fp.ErrPropositionalEngine) {
					t.Fatal(name, "accepted before query", err)
				}
			}
			_, err := engine.Model(declaration)
			assertOwner("model", err)
			_, err = engine.LastResult()
			assertOwner("last result", err)
			var output bytes.Buffer
			_, err = engine.WriteCertificate(&output, declaration, func(io.Writer, ast.ASTHandle) error { return nil })
			assertOwner("certificate", err)
			if backend == "pdr" || backend == "qpdr" {
				_, err = engine.Proof(declaration)
				assertOwner("proof", err)
			} else {
				_, err = engine.GroundSatAnswer(declaration)
				assertOwner("ground answer", err)
				_, _, err = engine.RulesAlongTrace()
				assertOwner("trace", err)
				_, err = engine.Reachable(declaration)
				assertOwner("reachable", err)
			}
		})
	}
}

func TestPDRAndSpacerResultSnapshotsInvalidateAfterParameterRevision(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			engine, manager := newRoutingEngine(t)
			declaration, application := routingPredicate(t, engine, manager, backend+"-revision")
			fpMust(t, engine.AddRule(application, "reachable-fact"))
			fpMust(t, engine.SelectEngine(backend))
			status, err := engine.QueryPredicate(declaration)
			fpMust(t, err)
			if status != lbool.True {
				t.Fatal("reachable fact result", status)
			}
			last, err := engine.LastResult()
			fpMust(t, err)
			if last != status {
				t.Fatal("published result differs from query", last, status)
			}
			if backend == "qpdr" {
				_, err := engine.NumLevels(declaration)
				fpMust(t, err)
			}
			answer, err := engine.Answer(declaration)
			fpMust(t, err)
			fpMust(t, manager.DelRef(&answer))
			var certificate bytes.Buffer
			calls := 0
			found, err := engine.WriteCertificate(&certificate, declaration, func(io.Writer, ast.ASTHandle) error {
				calls++
				return nil
			})
			fpMust(t, err)
			if (backend == "pdr" || backend == "qpdr") && (!found || calls != 1) {
				t.Fatal("PDR reachable certificate route", found, calls)
			}
			if backend == "spacer" && (found || calls != 0 || certificate.Len() != 0) {
				t.Fatal("Spacer empty certificate presentation", found, calls, certificate.String())
			}
			if backend == "spacer" {
				ground, err := engine.GroundSatAnswer(declaration)
				fpMust(t, err)
				fpMust(t, manager.DelRef(&ground))
				trace, _, err := engine.RulesAlongTrace()
				fpMust(t, err)
				for i := len(trace) - 1; i >= 0; i-- {
					fpMust(t, manager.DelRef(&trace[i]))
				}
			}

			fpMust(t, engine.ParametersUpdated())
			for operation, err := range map[string]error{
				"answer":      func() error { _, e := engine.Answer(declaration); return e }(),
				"model":       func() error { _, e := engine.Model(declaration); return e }(),
				"last-result": func() error { _, e := engine.LastResult(); return e }(),
				"certificate": func() error {
					_, e := engine.WriteCertificate(io.Discard, declaration, func(io.Writer, ast.ASTHandle) error { return nil })
					return e
				}(),
			} {
				if !errors.Is(err, fp.ErrPropositionalEngine) {
					t.Fatal(operation, "accepted stale", backend, "snapshot", err)
				}
			}
		})
	}
}

func TestEngineSwitchDoesNotRepublishPriorBackendResult(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	declaration, application := routingPredicate(t, engine, manager, "switch")
	fpMust(t, engine.AddRule(application, "switch-fact"))
	status, err := engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("Datalog switch fixture", status)
	}
	fpMust(t, engine.SelectEngine("pdr"))
	if _, err = engine.LastResult(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("Datalog result republished as PDR result", err)
	}
	if _, err = engine.Answer(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("Datalog answer republished as PDR answer", err)
	}
	fpMust(t, engine.SelectEngine("datalog"))
	if _, err = engine.LastResult(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("old snapshot was restored after engine round trip", err)
	}
}

func TestPDRAndQPDRSwitchInvalidatesResult(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	declaration, fact := routingPredicate(t, engine, manager, "qpdr-switch")
	fpMust(t, engine.AddRule(fact, "fact"))
	for _, backend := range []string{"qpdr", "pdr", "qpdr"} {
		fpMust(t, engine.SelectEngine(backend))
		if _, err := engine.LastResult(); !errors.Is(err, fp.ErrPropositionalEngine) {
			t.Fatal("engine switch exposed stale result", backend, err)
		}
		status, err := engine.QueryPredicate(declaration)
		fpMust(t, err)
		if status != lbool.True {
			t.Fatal("fact query after engine switch", backend, status)
		}
		answer, err := engine.Answer(declaration)
		fpMust(t, err)
		fpMust(t, manager.DelRef(&answer))
	}
}

func TestExpressionQueryLastResultAndSpacerTraceInvalidation(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("spacer"))
	declaration, application := routingPredicate(t, engine, manager, "expression-snapshot")
	fpMust(t, engine.AddRule(application, "expression-fact"))
	status, err := engine.QueryExpression(application)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("query", status)
	}
	result, err := engine.LastResult()
	fpMust(t, err)
	if result != lbool.True {
		t.Fatal("last result", result)
	}
	trace, _, err := engine.RulesAlongTrace()
	fpMust(t, err)
	for i := range trace {
		fpMust(t, manager.DelRef(&trace[i]))
	}
	fpMust(t, engine.AddFact(declaration, nil))
	if _, err = engine.LastResult(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale result", err)
	}
	if _, _, err = engine.RulesAlongTrace(); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale trace", err)
	}
}

func TestFailedExpressionQueryInvalidatesPreviousResult(t *testing.T) {
	for _, backend := range []string{"datalog", "pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			engine, manager := newRoutingEngine(t)
			declaration, application := routingPredicate(t, engine, manager, "failed-expression-"+backend)
			fpMust(t, engine.AddRule(application, "fact"))
			fpMust(t, engine.SelectEngine(backend))
			status, err := engine.QueryPredicate(declaration)
			fpMust(t, err)
			if status != lbool.True {
				t.Fatal("initial result", status)
			}
			ownedAnswer, err := engine.Answer(declaration)
			fpMust(t, err)
			defer func() { fpMust(t, manager.DelRef(&ownedAnswer)) }()
			if _, err = engine.QueryExpression(ast.ASTHandle{}); err == nil {
				t.Fatal("invalid expression query succeeded")
			}
			if _, err = manager.RefGet(ownedAnswer); err != nil {
				t.Fatal("failed query invalidated an already owned answer", err)
			}
			reason, err := engine.ReasonUnknown()
			fpMust(t, err)
			if reason != "input error" {
				t.Fatal("failed query status", reason)
			}
			if _, err = engine.LastResult(); !errors.Is(err, fp.ErrPropositionalEngine) {
				t.Fatal("failed expression exposed an earlier result", err)
			}
			if _, err = engine.Answer(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
				t.Fatal("failed expression exposed an earlier answer", err)
			}
			status, err = engine.QueryExpression(application)
			fpMust(t, err)
			if status != lbool.True {
				t.Fatal("query did not recover", status)
			}
		})
	}
}
