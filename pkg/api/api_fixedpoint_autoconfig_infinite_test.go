package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextFixedpointAutoConfigSelectsPDRForInfiniteDomain(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	references := make([]ast.ASTRef, 0, 3)
	defer func() {
		for i := len(references) - 1; i >= 0; i-- {
			astTestMust(t, manager.DelRef(&references[i]))
		}
	}()
	pin := func(value ast.ASTHandle, cause error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, cause)
		ref, cause := manager.NewRef(value)
		astTestMust(t, cause)
		references = append(references, ref)
		return value
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	astTestMust(t, err)
	integer := pin(manager.MkFamilySort(arithmetic, 1, nil))
	zero := pin(manager.ArithmeticIntegerNumeralText("0"))
	predicate, err := context.DeclareRelation("api-auto-infinite-P", []ast.ASTHandle{integer})
	astTestMust(t, err)
	fact := pin(manager.MkApp(predicate, []ast.ASTHandle{zero}))
	astTestMust(t, context.AddFixedpointRule(fact, "api-auto-infinite-fact"))

	status, err := context.QueryFixedpoint(predicate)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable integer-domain fixedpoint query", status)
	}
	levels, err := context.FixedpointNumLevels(predicate)
	astTestMust(t, err)
	if levels == 0 {
		t.Fatal("auto-config did not publish PDR levels for the infinite-domain predicate")
	}
}
