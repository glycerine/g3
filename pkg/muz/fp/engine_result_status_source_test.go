package fp

// Source: src/muz/base/dl_context.cpp context::query execution-status reset;
// backend query implementations only override it for relational approximation
// and Spacer bounded search.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestFixedpointUndefinedStatusMatchesSourceBackend(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	for _, backend := range []string{"clp", "tab", "bmc", "qbmc", "ddnf", "pdr", "qpdr", "spacer", "duality"} {
		t.Run(backend, func(t *testing.T) {
			e.backend = backend
			e.recordQueryStatus(lbool.Undef, nil)
			reason, err := e.ReasonUnknown()
			fragmentMust(t, err)
			if reason != "ok" {
				t.Fatalf("ordinary %s unknown status = %q, want ok", backend, reason)
			}
		})
	}

	e.backend = "datalog"
	e.recordQueryStatus(lbool.Undef, nil)
	reason, err := e.ReasonUnknown()
	fragmentMust(t, err)
	if reason != "approximated" {
		t.Fatalf("abstract Datalog status = %q, want approximated", reason)
	}

	e.backend = "pdr"
	e.recordQueryStatusWithApproximation(lbool.Undef, nil, true)
	reason, err = e.ReasonUnknown()
	fragmentMust(t, err)
	if reason != "approximated" {
		t.Fatalf("multi-relation extension status = %q, want approximated", reason)
	}
}

func TestFixedpointExplicitApproximationDoesNotInheritSpacerBound(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.spacer.max_level", "0"))
	fragmentMust(t, e.SelectEngine("spacer"))
	fragmentMust(t, e.addRuleEager(f.applications["p"], "bounded-fact"))

	status, err := e.QueryPredicate(f.declarations["p"])
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatalf("zero-level Spacer status = %v, want undefined", status)
	}
	reason, err := e.ReasonUnknown()
	fragmentMust(t, err)
	if reason != "bounded" {
		t.Fatalf("zero-level Spacer reason = %q, want bounded", reason)
	}

	status, err = e.QueryPredicates([]ast.ASTHandle{f.declarations["p"], f.declarations["q"]})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatalf("multi-relation Spacer status = %v, want undefined", status)
	}
	reason, err = e.ReasonUnknown()
	fragmentMust(t, err)
	if reason != "approximated" {
		t.Fatalf("multi-relation Spacer reason = %q, want approximated", reason)
	}
}
