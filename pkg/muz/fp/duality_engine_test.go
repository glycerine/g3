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

func TestDualityFixedpointDispatchProofModelScopesAndSwitching(t *testing.T) {
	e, m := newRoutingEngine(t)
	fpMust(t, e.SelectEngine("duality"))
	pd, p := routingPredicate(t, e, m, "duality-dispatch-P")
	qd, q := routingPredicate(t, e, m, "duality-dispatch-Q")
	fpMust(t, e.AddRule(p, "fact"))
	status, err := e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable relation", status)
	}
	proof, err := e.Proof(pd)
	fpMust(t, err)
	defer func() { fpMust(t, m.DelRef(&proof)) }()
	h, err := m.RefGet(proof)
	fpMust(t, err)
	conclusion, err := m.GetProofFact(h)
	fpMust(t, err)
	if !m.IsFalse(conclusion) {
		t.Fatal("query proof must conclude false")
	}
	if _, err := e.Answer(pd); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("unsupported source get_answer", err)
	}
	status, err = e.QueryExpression(q)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("unreachable expression", status)
	}
	md, err := e.Model(qd)
	fpMust(t, err)
	defer func() { fpMust(t, md.Close()) }()
	fpMust(t, e.Push())
	fpMust(t, e.AddRule(q, "scoped-fact"))
	status, err = e.QueryPredicate(qd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("scoped fact", status)
	}
	fpMust(t, e.Pop())
	if _, err := e.Proof(qd); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("pop left stale proof", err)
	}
	status, err = e.QueryPredicate(qd)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("popped fact survived", status)
	}
	fpMust(t, e.SelectEngine("pdr"))
	if _, err := e.Model(qd); !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("switch left stale model", err)
	}
	fpMust(t, e.SelectEngine("duality"))
	status, err = e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("reselected backend", status)
	}
	fpMust(t, e.Close())
	if _, err := m.RefGet(proof); err != nil {
		t.Fatal("owned proof expired after switching/closure", err)
	}
}

func TestDualityFixedpointBackgroundAssertionsAndPop(t *testing.T) {
	e, m := newRoutingEngine(t)
	fpMust(t, e.SelectEngine("duality"))
	pd, p := routingPredicate(t, e, m, "duality-background-P")
	fpMust(t, e.AddRule(p, "fact"))
	fpMust(t, e.Push())
	falsity, err := m.MkFalse()
	fpMust(t, err)
	fpMust(t, e.AssertBackground(falsity))
	status, err := e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal("inconsistent background must block reachability", status)
	}
	fpMust(t, e.Pop())
	status, err = e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal("popped background still asserted", status)
	}
}

// Source: dl_interface::display_certificate_non_const, StatusModel branch.
func TestDualityFixedpointSafetyCertificate(t *testing.T) {
	e, m := newRoutingEngine(t)
	fpMust(t, e.SelectEngine("duality"))
	pd, p := routingPredicate(t, e, m, "duality-certificate-P")
	qd, _ := routingPredicate(t, e, m, "duality-certificate-Q")
	var output bytes.Buffer
	if found, err := e.WriteCertificate(&output, qd, nil); found || !errors.Is(err, fp.ErrPropositionalEngine) {
		t.Fatal("certificate before query", found, err)
	}
	fpMust(t, e.AddRule(p, "fact"))
	status, err := e.QueryPredicate(qd)
	fpMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	found, err := e.WriteCertificate(&output, qd, nil)
	fpMust(t, err)
	if !found || !strings.HasPrefix(output.String(), "(fixedpoint \n") || !strings.Contains(output.String(), "define-fun") || !strings.HasSuffix(output.String(), ")\n") {
		t.Fatal("safety certificate", found, output.String())
	}
	found, err = e.WriteCertificate(dualityRefusingWriter{}, qd, nil)
	if !found || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer failure", found, err)
	}
	status, err = e.QueryPredicate(pd)
	fpMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	output.Reset()
	if found, err := e.WriteCertificate(&output, pd, nil); found || !errors.Is(err, fp.ErrPropositionalEngine) || output.Len() != 0 {
		t.Fatal("unimplemented derivation certificate", found, err, output.String())
	}
}

type dualityRefusingWriter struct{}

func (dualityRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
