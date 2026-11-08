package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source: dl_context.cpp:922-932 keeps an existing engine on ensure_engine;
// repeated explicit selection is the Go dispatcher's corresponding reuse path.
func TestEngineRegistrationRepeatedSelectionKeepsDualityOwner(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("duality"))
	must(e.addRuleEager(f.applications["p"], "fact"))
	status, err := e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True || e.duality == nil {
		t.Fatalf("query did not instantiate Duality: status=%v owner=%v", status, e.duality)
	}
	owner := e.duality
	must(e.SelectEngine("duality"))
	if e.duality != owner {
		t.Fatal("repeated selection replaced the instantiated owner")
	}
	proof, err := e.Proof(f.declarations["p"])
	must(err)
	defer func() { must(m.DelRef(&proof)) }()
	handle, err := m.RefGet(proof)
	must(err)
	conclusion, err := m.GetProofFact(handle)
	must(err)
	if !m.IsFalse(conclusion) {
		t.Fatal("repeated selection lost the query proof")
	}
	status, err = e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True || e.duality != owner {
		t.Fatalf("subsequent query did not reuse Duality: status=%v", status)
	}
}
