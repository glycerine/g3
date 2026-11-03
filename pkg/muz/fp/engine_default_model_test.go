package fp

// Source: src/muz/base/dl_engine_base.h engine_base::get_model. BMC, QBMC,
// TAB, CLP and DDNF inherit its unconditional fresh empty-model default.

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func checkEmptyEngineDefaultModel(t *testing.T, md *model.Model) {
	t.Helper()
	declarations, err := md.Core().NumDeclarations()
	fragmentMust(t, err)
	sorts, err := md.NumUninterpretedSorts()
	fragmentMust(t, err)
	if declarations != 0 || sorts != 0 {
		t.Fatal("default model contains interpretations", declarations, sorts)
	}
}

func TestEngineDefaultModelStatusAndIndependentOwners(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc", "tab", "clp", "ddnf"} {
		for _, reachable := range []bool{false, true} {
			name := backend + "/false"
			if reachable {
				name = backend + "/true"
			}
			t.Run(name, func(t *testing.T) {
				f := newPDRLifecycleFixture(t)
				e, m := f.engine, f.manager
				fragmentMust(t, e.SelectEngine(backend))
				fragmentMust(t, e.SetDDNFOutput(io.Discard))
				query := f.declarations["u"]
				want := lbool.False
				if reachable {
					query, want = f.declarations["p"], lbool.True
				}
				if backend == "ddnf" {
					want = lbool.Undef
				}
				checkInvalid := func(label string) {
					t.Helper()
					m.Budget().FailAfter(0)
					md, err := e.Model(query)
					m.Budget().FailAfter(-1)
					if md != nil || !errors.Is(err, ErrPropositionalEngine) {
						t.Fatal(label, md, err)
					}
				}
				checkInvalid("pre-query model reached allocation")
				run := func() {
					t.Helper()
					status, err := e.QueryPredicate(query)
					fragmentMust(t, err)
					if status != want {
						t.Fatal("query result", status, want)
					}
				}
				run()
				first, err := e.Model(query)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, first.Close()) }()
				second, err := e.Model(query)
				fragmentMust(t, err)
				defer func() { fragmentMust(t, second.Close()) }()
				checkEmptyEngineDefaultModel(t, first)
				checkEmptyEngineDefaultModel(t, second)
				if first == second || first.Core() == second.Core() {
					t.Fatal("default models share an owner")
				}
				truth, err := m.MkTrue()
				fragmentMust(t, err)
				fragmentMust(t, first.Core().RegisterConstant(f.declarations["p"], truth))
				checkEmptyEngineDefaultModel(t, second)
				run()
				third, err := e.Model(query)
				fragmentMust(t, err)
				checkEmptyEngineDefaultModel(t, third)
				fragmentMust(t, third.Close())
				if _, err = e.QueryPredicate(ast.ASTHandle{}); err == nil {
					t.Fatal("invalid query unexpectedly succeeded")
				}
				checkInvalid("failed-query model reached allocation")
				run()
				fragmentMust(t, e.AddRule(f.applications["p"], "invalidate-default-model"))
				checkInvalid("stale model reached allocation")
				run()
				fragmentMust(t, e.Close())
				checkInvalid("closed engine model reached allocation")
				value, err := first.Core().ConstantInterpretation(f.declarations["p"])
				fragmentMust(t, err)
				if !m.IsTrue(value) {
					t.Fatal("caller model lost its independent interpretation")
				}
				checkEmptyEngineDefaultModel(t, second)
			})
		}
	}
}

func TestEngineDefaultModelConstructionFailureAndRetry(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc", "tab", "clp", "ddnf"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, m := f.engine, f.manager
			fragmentMust(t, e.SelectEngine(backend))
			fragmentMust(t, e.SetDDNFOutput(io.Discard))
			_, err := e.QueryPredicate(f.declarations["u"])
			fragmentMust(t, err)
			failed, completed := false, false
			for allowance := int64(0); allowance < 8 && !completed; allowance++ {
				before, refs, nodes := m.Budget().Used(), m.Stats().LiveReferences, m.Stats().LiveNodes
				m.Budget().FailAfter(allowance)
				md, err := e.Model(f.declarations["u"])
				m.Budget().FailAfter(-1)
				if err != nil {
					failed = true
					if md != nil || !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal("failed model publication", allowance, md, err)
					}
				} else {
					completed = true
					checkEmptyEngineDefaultModel(t, md)
					fragmentMust(t, md.Close())
				}
				if m.Budget().Used() != before || m.Stats().LiveReferences != refs || m.Stats().LiveNodes != nodes {
					t.Fatal("model construction leaked its owner", allowance, before, m.Budget().Used())
				}
				retry, err := e.Model(f.declarations["u"])
				fragmentMust(t, err)
				checkEmptyEngineDefaultModel(t, retry)
				fragmentMust(t, retry.Close())
				if m.Budget().Used() != before || m.Stats().LiveReferences != refs || m.Stats().LiveNodes != nodes {
					t.Fatal("model retry leaked its owner")
				}
			}
			if !failed || !completed {
				t.Fatal("missing actual failure and completion", failed, completed)
			}
		})
	}
}
