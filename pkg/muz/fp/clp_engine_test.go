package fp_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCLPEngineResultSnapshotAndAnswer(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("clp"))
	declaration, application := routingPredicate(t, engine, manager, "clp")
	if _, err := engine.WriteCertificate(io.Discard, declaration, nil); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("certificate available before query", err)
	}
	applicationRef, err := manager.NewRef(application)
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&applicationRef)) }()
	status, err := engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("empty predicate", status)
	}
	var certificate bytes.Buffer
	if found, err := engine.WriteCertificate(&certificate, declaration, nil); err != nil || !found || certificate.String() != "true\n" {
		t.Fatal("unreachable CLP certificate", found, certificate.String(), err)
	}
	if found, err := engine.WriteCertificate(clpFailWriter{}, declaration, nil); !found || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("certificate writer error", found, err)
	}
	answer, err := engine.Answer(declaration)
	fpMust(t, err)
	value, err := manager.RefGet(answer)
	fpMust(t, err)
	if !manager.IsTrue(value) {
		t.Fatal("CLP answer", value)
	}
	fpMust(t, manager.DelRef(&answer))
	fpMust(t, engine.AddRule(application, "fact"))
	if _, err = engine.Answer(declaration); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale answer", err)
	}
	status, err = engine.QueryPredicate(declaration)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("fact query", status)
	}
	if last, err := engine.LastResult(); err != nil || last != lbool.True {
		t.Fatal("result snapshot", last, err)
	}
	answer, err = engine.Answer(declaration)
	fpMust(t, err)
	fpMust(t, manager.DelRef(&answer))
}

type clpFailWriter struct{}

func (clpFailWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCLPQueryWrapperDoesNotMutateProgram(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("clp"))
	p, pApp := routingPredicate(t, engine, manager, "clp-wrapper-p")
	q, qApp := routingPredicate(t, engine, manager, "clp-wrapper-q")
	fpMust(t, engine.AddRule(pApp, "fact"))
	edge, err := manager.MkImplies(pApp, qApp)
	fpMust(t, err)
	fpMust(t, engine.AddRule(edge, "edge"))
	for _, query := range []ast.ASTHandle{q, p, q} {
		status, err := engine.QueryPredicate(query)
		fpMust(t, err)
		if status != lbool.True {
			t.Fatal("query after wrapper creation", status)
		}
		answer, err := engine.Answer(query)
		fpMust(t, err)
		value, err := manager.RefGet(answer)
		fpMust(t, err)
		if !manager.IsTrue(value) {
			t.Fatal("CLP answer", value)
		}
		fpMust(t, manager.DelRef(&answer))
		predicates, err := engine.Predicates()
		fpMust(t, err)
		rules, err := engine.Rules()
		fpMust(t, err)
		if len(predicates) != 2 || len(rules) != 2 {
			t.Fatal("query mutated persistent program", len(predicates), len(rules))
		}
		for i := range rules {
			fpMust(t, manager.DelRef(&rules[i]))
		}
	}
}
