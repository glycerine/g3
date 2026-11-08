package fp

// Source: pdr_dl_interface forwards pdr.validate_result into context::solve.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRValidateResultParameterChecksSATAndUNSATQueries(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		value := "false"
		if enabled {
			name, value = "enabled", "true"
		}
		t.Run(name, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, manager := f.engine, f.manager
			if err := e.globals.SourceSetName("fixedpoint.pdr.validate_result", value); err != nil {
				t.Fatal(err)
			}
			status, err := e.QueryPredicate(f.declarations["q"])
			if err != nil || status != lbool.True {
				t.Fatal("validated reachable query", status, err)
			}
			falsity, err := manager.MkFalse()
			if err != nil {
				t.Fatal(err)
			}
			unreachable := f.applications["u"]
			impossible, err := manager.MkImplies(falsity, unreachable)
			if err != nil {
				t.Fatal(err)
			}
			if err = e.addRuleEager(impossible, "validated-impossible-rule"); err != nil {
				t.Fatal(err)
			}
			status, err = e.QueryPredicate(f.declarations["u"])
			if err != nil || status != lbool.False {
				t.Fatal("validated unreachable query", status, err)
			}
		})
	}
}
