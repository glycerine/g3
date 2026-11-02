package fp_test

// Source: src/muz/base/dl_engine_base.h engine_base::query relation wrapper.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCLPRelationQueryChecksCardinalityBeforeDeclarations(t *testing.T) {
	engine, manager := newRoutingEngine(t)
	fpMust(t, engine.SelectEngine("clp"))
	p, _ := routingPredicate(t, engine, manager, "clp-cardinality-p")
	q, _ := routingPredicate(t, engine, manager, "clp-cardinality-q")

	for name, declarations := range map[string][]ast.ASTHandle{
		"empty":         nil,
		"two-valid":     {p, q},
		"two-with-null": {p, {}},
	} {
		t.Run(name, func(t *testing.T) {
			status, err := engine.QueryPredicates(declarations)
			if err != nil || status != lbool.Undef {
				t.Fatal("source cardinality result", status, err)
			}
		})
	}
	status, err := engine.QueryPredicates([]ast.ASTHandle{{}})
	if status != lbool.Undef || !errors.Is(err, fp.ErrPropositionalHorn) {
		t.Fatal("single invalid relation bypassed validation", status, err)
	}
}
