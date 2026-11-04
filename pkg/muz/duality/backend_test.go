package duality_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	graph "github.com/glycerine/g3/pkg/duality"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityOwnedBackendIntegerQueriesAndBorrowedServiceLifetime(t *testing.T) {
	for _, mbqi := range []bool{false, true} {
		name := "without-mbqi"
		if mbqi {
			name = "with-mbqi"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			owner, err := cmd_context.NewStandaloneContext()
			must(err)
			defer owner.Close()
			m, err := owner.Manager()
			must(err)
			config, err := owner.SolverTranslationConfig()
			must(err)
			initial, err := m.ProofMode()
			must(err)
			backend, err := duality.NewBackend(m, config, mbqi)
			must(err)
			defer backend.Close()
			if md, err := backend.GetModel(); md != nil || !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("model requested before a completed query", md, err)
			}
			copied := *backend
			if err := copied.Close(); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("copied backend accepted", err)
			}
			var refs []ast.ASTRef
			defer func() {
				for i := range refs {
					must(m.DelRef(&refs[i]))
				}
			}()
			pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				must(err)
				ref, err := m.NewRef(h)
				must(err)
				refs = append(refs, ref)
				return h
			}
			zero := pin(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
			one := pin(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
			integer, err := m.GetSort(zero)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			symbol, err := m.Symbols.Intern("backend-integer-P")
			must(err)
			predicate := pin(m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
			fact := pin(m.MkApp(predicate, []ast.ASTHandle{zero}))
			queryOne := pin(m.MkApp(predicate, []ast.ASTHandle{one}))
			var previous *graph.Counterexample
			var proofSnapshot ast.ASTRef
			defer func() { must(m.DelRef(&proofSnapshot)) }()
			var snapshot *model.Model
			defer func() {
				if snapshot != nil {
					must(snapshot.Close())
				}
			}()
			checkModel := func(md *model.Model) {
				t.Helper()
				ev, err := model.NewModelEvaluator(md.Core(), config.ModelEvaluator)
				must(err)
				defer func() { must(ev.Close()) }()
				for _, tc := range []struct {
					term ast.ASTHandle
					want bool
				}{{fact, true}, {queryOne, false}} {
					value, err := ev.EvaluateValue(tc.term)
					must(err)
					h, err := m.RefGet(value)
					must(err)
					expected, err := m.MkBoolVal(tc.want)
					must(err)
					matches := h == expected
					must(m.DelRef(&value))
					if !matches {
						t.Fatal("annotation model violates fact or excluded query", tc.want)
					}
				}
			}
			var profileOutput bytes.Buffer
			must(backend.SetProfileOutput(&profileOutput))
			var reportOutput bytes.Buffer
			must(backend.SetReportOutput(&reportOutput))
			for round, query := range []ast.ASTHandle{queryOne, fact, queryOne} {
				reportOutput.Reset()
				profileOutput.Reset()
				options := duality.DefaultQueryOptions()
				options.Report = true
				options.Profile = true
				options.ConjectureFile = filepath.Join(t.TempDir(), "conjectures.smt2")
				if round == 1 {
					options.StratifiedInlining = true
				}
				must(backend.SetOptions(options))
				prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{fact}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{predicate}, math.MaxUint32)
				must(err)
				status, err := backend.Query(prepared)
				must(err)
				must(prepared.Close())
				expected := lbool.False
				if round == 1 {
					expected = lbool.True
				}
				if status != expected {
					t.Fatalf("integer query %d: %v want %v", round, status, expected)
				}
				if round == 0 && (!strings.Contains(reportOutput.String(), "]node ") || !strings.Contains(reportOutput.String(), "]update ") || !strings.Contains(reportOutput.String(), "]check ") || !strings.Contains(reportOutput.String(), "]expand ")) {
					t.Fatal("missing live search events", reportOutput.String())
				}
				if round == 0 && (!strings.Contains(profileOutput.String(), "CandidateFeasible") || !strings.Contains(profileOutput.String(), "ProduceCandidatesForExtension") || !strings.Contains(profileOutput.String(), "Extend")) {
					t.Fatal("missing feasibility timer", profileOutput.String())
				}
				if round == 0 {
					for _, phase := range []string{"Derive", "Build", "Pop", "ExpandSomeNodes", "ExpansionChoices", "Check", "SetEdgeMaps", "Localize", "RedVars"} {
						if !strings.Contains(profileOutput.String(), phase) {
							t.Fatal("missing derivation timer", phase, profileOutput.String())
						}
					}
				}
				if round == 1 && (strings.Contains(profileOutput.String(), "StratifiedInlining") || !strings.Contains(profileOutput.String(), "\n    Extend")) {
					t.Fatal("incorrect stratified timer nesting", profileOutput.String())
				}
				report, err := os.ReadFile(options.ConjectureFile)
				must(err)
				if round == 0 && !strings.Contains(string(report), "(define-fun ") {
					t.Fatal("missing learned annotation report", string(report))
				}
				if round == 0 {
					snapshot, err = backend.GetModel()
					must(err)
				}
				if round == 2 {
					latest, err := backend.GetModel()
					must(err)
					checkModel(latest)
					must(latest.Close())
				}
				checkModel(snapshot)
				proof, err := backend.GetProof()
				must(err)
				if round == 1 {
					proofSnapshot = proof
					validateDualityProof(t, m, config, proofSnapshot)
				} else {
					h, err := m.RefGet(proof)
					must(err)
					if !h.IsNull() {
						t.Fatal("safe query returned a counterexample proof")
					}
					must(m.DelRef(&proof))
				}
				if previous != nil && previous.Root() != nil {
					t.Fatal("old backend counterexample still owned")
				}
				previous = backend.Counterexample()
				if round == 1 && (previous == nil || previous.Root() == nil) {
					t.Fatal("missing integer counterexample")
				}
				if backend.IsResultRecursionBounded() {
					t.Fatal("unbounded integer query marked bounded")
				}
				mode, err := m.ProofMode()
				must(err)
				if mode != initial {
					t.Fatal("backend leaked proof mode")
				}

				// Failed attempts hide every previously published result but keep
				// the previous search available for learning on a successful retry.
				retryQuery, err := duality.PrepareQuery(m, []ast.ASTHandle{fact}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{predicate}, math.MaxUint32)
				must(err)
				for _, refusedAllocation := range []bool{false, true} {
					var status lbool.LBool
					var failure error
					if refusedAllocation {
						m.Budget().FailAfter(0)
						status, failure = backend.Query(retryQuery)
						m.Budget().FailAfter(-1)
					} else {
						status, failure = backend.Query(nil)
					}
					if failure == nil || status != lbool.Undef {
						t.Fatal("failed query result", status, failure)
					}
					if md, err := backend.GetModel(); md != nil || !errors.Is(err, graph.ErrWrapperOwner) {
						t.Fatal("stale model", md, err)
					}
					if proof, err := backend.GetProof(); proof != (ast.ASTRef{}) || !errors.Is(err, graph.ErrWrapperOwner) {
						t.Fatal("stale proof", proof, err)
					}
					if backend.Counterexample() != nil || backend.IsResultRecursionBounded() {
						t.Fatal("stale query metadata")
					}
					if err := backend.WriteCertificate(io.Discard, owner.WriteExpression); !errors.Is(err, graph.ErrWrapperOwner) {
						t.Fatal("stale certificate", err)
					}
					status, err = backend.Query(retryQuery)
					must(err)
					if status != expected {
						t.Fatal("retry changed query result", status, expected)
					}
					checkModel(snapshot)
				}
				must(retryQuery.Close())
			}
			must(backend.Close())
			must(backend.Close())
			checkModel(snapshot)
			validateDualityProof(t, m, config, proofSnapshot)
			if md, err := backend.GetModel(); md != nil || !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closed backend accepted model request", md, err)
			}
			if _, err := backend.Query(nil); !errors.Is(err, graph.ErrWrapperOwner) {
				t.Fatal("closed backend accepted query", err)
			}
			// Closing the backend must leave the command owner's services usable.
			_, err = config.ModelEvaluator.Cooperation.Enabled()
			must(err)
			_, err = config.ModelEvaluator.Memory.AllocationSize()
			must(err)
			p := config.Params.EmptyValue()
			must(config.Params.Del(&p))
			m.Budget().FailAfter(0)
			failed, allocationErr := duality.NewBackend(m, config, mbqi)
			m.Budget().FailAfter(-1)
			if failed != nil || allocationErr == nil {
				t.Fatal("refused backend allocation unexpectedly succeeded", failed, allocationErr)
			}
			mode, err := m.ProofMode()
			must(err)
			if mode != initial {
				t.Fatal("failed backend construction leaked proof mode")
			}
			retry, err := duality.NewBackend(m, config, mbqi)
			must(err)
			must(retry.Close())
			broken := config
			broken.ModelEvaluator.Memory = nil
			if failed, err := duality.NewBackend(m, broken, mbqi); err == nil || failed != nil {
				t.Fatal("missing borrowed service accepted", failed, err)
			}
		})
	}
}

func TestDualityBackendCancellationAndRetry(t *testing.T) {
	owner, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	m, err := owner.Manager()
	if err != nil {
		t.Fatal(err)
	}
	config, err := owner.SolverTranslationConfig()
	if err != nil {
		t.Fatal(err)
	}
	backend, err := duality.NewBackend(m, config, true)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	query, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := duality.PrepareQuery(m, nil, nil, query, nil, math.MaxUint32)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	initial, err := m.ProofMode()
	if err != nil {
		t.Fatal(err)
	}
	status, err := backend.Query(prepared)
	if err != nil || status != lbool.False {
		t.Fatal(status, err)
	}
	m.ResourceLimit().Cancel()
	status, err = backend.Query(prepared)
	m.ResourceLimit().ResetCancel()
	if status != lbool.Undef || err == nil {
		t.Fatal("canceled Duality query", status, err)
	}
	if md, err := backend.GetModel(); md != nil || !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("canceled model publication", md, err)
	}
	mode, err := m.ProofMode()
	if err != nil || mode != initial {
		t.Fatal("cancellation leaked proof mode", mode, err)
	}
	status, err = backend.Query(prepared)
	if err != nil || status != lbool.False {
		t.Fatal("retry after cancellation", status, err)
	}
}

func TestDualityBackendProfileOutputAndWriterRetry(t *testing.T) {
	owner, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	m, err := owner.Manager()
	if err != nil {
		t.Fatal(err)
	}
	config, err := owner.SolverTranslationConfig()
	if err != nil {
		t.Fatal(err)
	}
	backend, err := duality.NewBackend(m, config, true)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	query, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := duality.PrepareQuery(m, nil, nil, query, nil, math.MaxUint32)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	var output bytes.Buffer
	if err := backend.SetProfileOutput(&output); err != nil {
		t.Fatal(err)
	}
	status, err := backend.Query(prepared)
	if err != nil || status != lbool.False || output.Len() != 0 {
		t.Fatal("disabled profiling", status, err, output.String())
	}
	options := duality.DefaultQueryOptions()
	options.ConjectureFile = filepath.Join(t.TempDir(), "missing", "conjectures.smt2")
	if err := backend.SetOptions(options); err != nil {
		t.Fatal(err)
	}
	status, err = backend.Query(prepared)
	if status != lbool.Undef || err == nil {
		t.Fatal("conjecture file open failure", status, err)
	}
	options.ConjectureFile = filepath.Join(t.TempDir(), "conjectures.smt2")
	options.Profile = true
	if err := backend.SetOptions(options); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		output.Reset()
		status, err = backend.Query(prepared)
		if err != nil || status != lbool.False {
			t.Fatal(status, err)
		}
		if !strings.Contains(output.String(), "SolveMain") || !strings.Contains(output.String(), "TOTALS:") {
			t.Fatal("profile report", output.String())
		}
	}
	if err := backend.SetProfileOutput(dualityProfileRefusingWriter{}); err != nil {
		t.Fatal(err)
	}
	status, err = backend.Query(prepared)
	if status != lbool.Undef || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("profile writer failure", status, err)
	}
	if md, err := backend.GetModel(); md != nil || !errors.Is(err, graph.ErrWrapperOwner) {
		t.Fatal("profile failure published model", md, err)
	}
	if err := backend.SetProfileOutput(&output); err != nil {
		t.Fatal(err)
	}
	status, err = backend.Query(prepared)
	if err != nil || status != lbool.False {
		t.Fatal("profile writer retry", status, err)
	}
}

type dualityProfileRefusingWriter struct{}

func (dualityProfileRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDualityBackendProfilesSurviveBackendClose(t *testing.T) {
	profile, interpolation := graph.ProcessProfilers()
	run := func(first bool) string {
		t.Helper()
		owner, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		m, err := owner.Manager()
		if err != nil {
			t.Fatal(err)
		}
		config, err := owner.SolverTranslationConfig()
		if err != nil {
			t.Fatal(err)
		}
		backend, err := duality.NewBackend(m, config, true)
		if err != nil {
			t.Fatal(err)
		}
		query, err := m.MkFalse()
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := duality.PrepareQuery(m, nil, nil, query, nil, math.MaxUint32)
		if err != nil {
			t.Fatal(err)
		}
		if first {
			if err := profile.TimerStart("cross-backend-graph"); err != nil {
				t.Fatal(err)
			}
			if err := profile.TimerStop("cross-backend-graph"); err != nil {
				t.Fatal(err)
			}
			if err := interpolation.TimerStart("cross-backend-interpolation"); err != nil {
				t.Fatal(err)
			}
			if err := interpolation.TimerStop("cross-backend-interpolation"); err != nil {
				t.Fatal(err)
			}
		}
		options := duality.DefaultQueryOptions()
		options.Profile = true
		if err := backend.SetOptions(options); err != nil {
			t.Fatal(err)
		}
		if !first {
			if err := backend.SetProfileOutput(dualityProfileRefusingWriter{}); err != nil {
				t.Fatal(err)
			}
			status, err := backend.Query(prepared)
			if status != lbool.Undef || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(status, err)
			}
		}
		var output bytes.Buffer
		if err := backend.SetProfileOutput(&output); err != nil {
			t.Fatal(err)
		}
		status, err := backend.Query(prepared)
		if err != nil || status != lbool.False {
			t.Fatal(status, err)
		}
		if err := prepared.Close(); err != nil {
			t.Fatal(err)
		}
		if err := backend.Close(); err != nil {
			t.Fatal(err)
		}
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	run(true)
	output := run(false)
	for _, label := range []string{"cross-backend-graph", "cross-backend-interpolation", "SolveMain"} {
		if !strings.Contains(output, label) {
			t.Fatalf("second backend profile missing %q:\n%s", label, output)
		}
	}
}
