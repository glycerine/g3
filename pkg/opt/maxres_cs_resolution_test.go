package opt

// Source: src/opt/maxres.cpp cs_max_resolve satisfied-count duality.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxResCorrectionClausesRemoveExactlyOneSatisfiedMember(t *testing.T) {
	manager := newWeightedSoftFixture(t).manager
	correction := []ast.ASTHandle{maxResTestBool(t, manager, "maxres-cs-a"), maxResTestBool(t, manager, "maxres-cs-b"), maxResTestBool(t, manager, "maxres-cs-c")}
	owner := maxSMTASTOwner{manager: manager}
	defer owner.close()
	clauses, err := maxResCorrectionClauses(&owner, correction)
	if err != nil || len(clauses) != len(correction)-1 {
		t.Fatal(clauses, err)
	}
	for mask := 1; mask < 1<<len(correction); mask++ {
		values := make(map[ast.ASTHandle]bool, len(correction))
		original := 0
		for i, expression := range correction {
			values[expression] = mask&(1<<i) != 0
			if values[expression] {
				original++
			}
		}
		resolved := 0
		for _, clause := range clauses {
			if evalMaxResFormula(t, manager, clause, values) {
				resolved++
			}
		}
		if resolved != original-1 {
			t.Fatal("correction duality", mask, resolved, original)
		}
	}
}
