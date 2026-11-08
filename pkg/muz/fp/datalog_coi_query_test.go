package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogConeOfInfluenceScopesDisconnectedRuleFacts(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.addRuleEager(f.applications["q"], "coi-q-fact"))
	must(e.addRuleEager(f.applications["p"], "coi-p-fact"))

	status, err := e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal("q query", status)
	}
	snapshots, err := e.DatalogRelationSnapshots([]ast.ASTHandle{f.declarations["p"]})
	must(err)
	if len(snapshots) != 1 || len(snapshots[0].Facts) != 0 {
		t.Fatalf("disconnected p survived q cone: %#v", snapshots)
	}

	status, err = e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("p query", status)
	}
	snapshots, err = e.DatalogRelationSnapshots([]ast.ASTHandle{f.declarations["p"]})
	must(err)
	if len(snapshots) != 1 || len(snapshots[0].Facts) != 1 || len(snapshots[0].Facts[0]) != 0 {
		t.Fatalf("p source rule was not restored for its query: %#v", snapshots)
	}
}
