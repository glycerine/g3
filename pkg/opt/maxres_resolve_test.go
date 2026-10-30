package opt

// Source: src/opt/maxres.cpp max_resolve cardinality-preserving clauses.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResResolvePreservesSatisfiedCountBelowFullCore(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	core := []ast.ASTHandle{maxResTestBool(t, manager, "maxres-a"), maxResTestBool(t, manager, "maxres-b"), maxResTestBool(t, manager, "maxres-c")}
	owner := maxSMTASTOwner{manager: manager}
	defer owner.close()
	clauses, err := maxResResolutionClauses(&owner, core)
	if err != nil {
		t.Fatal(err)
	}
	if len(clauses) != len(core)-1 {
		t.Fatal("resolved assumptions", len(clauses))
	}
	for mask := 0; mask < 1<<len(core)-1; mask++ {
		values := make(map[ast.ASTHandle]bool, len(core))
		original := 0
		for i, expression := range core {
			values[expression] = mask&(1<<i) != 0
			if values[expression] {
				original++
			}
		}
		resolved := 0
		for _, formula := range clauses {
			if evalMaxResFormula(t, manager, formula, values) {
				resolved++
			}
		}
		if resolved != original {
			t.Fatal("MaxRes count", mask, resolved, original)
		}
	}
}
