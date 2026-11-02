package fp_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type tabCertificateRefusingWriter struct{}

func (tabCertificateRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestTabEngineWriteCertificateLiveAnswers(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("tab"))
	_, fact := routingPredicate(t, engine, manager, "tab-certificate-fact")
	goalDeclaration, goal := routingPredicate(t, engine, manager, "tab-certificate-goal")
	missingDeclaration, _ := routingPredicate(t, engine, manager, "tab-certificate-missing")
	fpMust(t, engine.AddRule(fact, "fact"))
	step, err := manager.MkImplies(fact, goal)
	fpMust(t, err)
	fpMust(t, engine.AddRule(step, "step"))
	var output bytes.Buffer
	if found, err := engine.WriteCertificate(&output, goalDeclaration, nil); found || !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("certificate before query", found, err)
	}
	status, err := engine.QueryPredicate(goalDeclaration)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable query", status)
	}
	found, err := engine.WriteCertificate(&output, goalDeclaration, nil)
	fpMust(t, err)
	if !found || output.Len() == 0 || !strings.HasSuffix(output.String(), "\n") || output.String() == "true\n" {
		t.Fatal("reachable tab certificate", found, output.String())
	}
	answer, err := engine.Answer(goalDeclaration)
	fpMust(t, err)
	proof, err := manager.RefGet(answer)
	fpMust(t, err)
	valid, err := manager.IsProof(proof)
	fpMust(t, err)
	if !valid {
		t.Fatal("certificate display invalidated answer")
	}
	fpMust(t, manager.DelRef(&answer))
	if found, err = engine.WriteCertificate(tabCertificateRefusingWriter{}, goalDeclaration, nil); !found || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("certificate writer refusal", found, err)
	}
	status, err = engine.QueryPredicate(missingDeclaration)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("unreachable query", status)
	}
	output.Reset()
	found, err = engine.WriteCertificate(&output, missingDeclaration, nil)
	fpMust(t, err)
	if !found || output.String() != "true\n" {
		t.Fatal("unreachable tab certificate", found, output.String())
	}
	fpMust(t, engine.AddRule(goal, "mutation"))
	if found, err = engine.WriteCertificate(io.Discard, missingDeclaration, nil); found || !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("stale certificate", found, err)
	}
}
