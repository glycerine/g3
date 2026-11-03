package fp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityDisplayCancellationAndWriterFailurePreserveCompletedResult(t *testing.T) {
	f := newDualityDisplayResultFixture(t)
	e, m := f.e, f.m
	status, err := e.QueryExpression(f.application)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("open raw rule query", status)
	}
	proof, err := e.Proof(f.predicate)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, m.DelRef(&proof)) })
	proofHandle, err := m.RefGet(proof)
	fragmentMust(t, err)
	set, backend, revision := e.set, e.duality, e.revision
	raw := append([]rawRuleFormula(nil), e.rawRules...)
	if len(raw) != 1 || !raw[0].pending {
		t.Fatal("fixture did not leave an open raw input")
	}
	checkResult := func() {
		t.Helper()
		last, err := e.LastResult()
		fragmentMust(t, err)
		if last != status || e.set != set || e.duality != backend || e.queryClone != nil {
			t.Fatal("display changed completed source result or owner", last)
		}
		current, err := e.Proof(f.predicate)
		fragmentMust(t, err)
		handle, err := m.RefGet(current)
		fragmentMust(t, err)
		fragmentMust(t, m.DelRef(&current))
		if handle != proofHandle {
			t.Fatal("display replaced the completed source proof")
		}
	}
	limit := m.ResourceLimit()
	limit.IncCancel()
	var canceled bytes.Buffer
	failure := e.WriteContextSMT2(&canceled, nil, nil)
	message := limit.CancelMessage()
	limit.DecCancel()
	if failure == nil || !strings.Contains(failure.Error(), message) || canceled.Len() != 0 {
		t.Fatal("canceled promotion did not fail before output", failure, canceled.String())
	}
	if e.revision != revision || len(e.rawRules) != len(raw) || e.rawRules[0] != raw[0] {
		t.Fatal("canceled display advanced pending source phase")
	}
	checkResult()
	writerFailure := errors.New("Duality display writer refused output")
	failure = e.WriteContextSMT2(contextDisplayErrorWriter{writerFailure}, nil, nil)
	if !errors.Is(failure, writerFailure) {
		t.Fatal("display lost writer failure", failure)
	}
	if e.revision == revision || e.queryRevision != e.revision || len(e.rawRules) != 0 {
		t.Fatal("writer refusal did not follow completed source promotion")
	}
	checkResult()
	var completed bytes.Buffer
	fragmentMust(t, e.WriteContextSMT2(&completed, nil, nil))
	if !strings.Contains(completed.String(), "(rule ") {
		t.Fatal("display retry did not emit normalized source rule", completed.String())
	}
	checkResult()
	fragmentMust(t, e.Close())
	if h, err := m.RefGet(proof); err != nil || h != proofHandle || h == (ast.ASTHandle{}) {
		t.Fatal("held proof expired after engine close", h, err)
	}
}
