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
		{"spacer", fp.EngineSpacer},
	} {
		fpMust(t, engine.SelectEngine(test.name))
		kind, err := engine.SelectedEngine()
		fpMust(t, err)
		if kind != test.kind || kind.String() != test.name || !kind.Retained() {
			t.Fatal("selected engine", kind, kind.String(), kind.Retained())
		}
	}
	for _, name := range []string{"qpdr", "bmc", "qbmc", "tab", "clp", "duality", "ddnf"} {
		if err := engine.SelectEngine(name); err == nil || errors.Is(err, fp.ErrPropositionalEngine) {
			t.Fatal("known unported engine diagnostic", name, err)
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
	for _, backend := range []string{"pdr", "spacer"} {
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
			if backend == "pdr" {
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
	for _, backend := range []string{"pdr", "spacer"} {
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
			if backend == "pdr" && (!found || calls != 1) {
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
