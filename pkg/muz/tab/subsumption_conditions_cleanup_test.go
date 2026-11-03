package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSubsumptionConditionCleanupAdvancesConsumedFrontier(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	f, err := newTabTestContext()
	must(err)
	defer func() { must(f.Close()) }()
	truth, err := f.manager.MkTrue()
	must(err)
	owned, err := f.manager.NewRef(truth)
	must(err)
	search := &subsumptionSearch{manager: f.manager, conditions: []ast.ASTRef{{}, owned}}
	if err = search.truncateConditions(0); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("consumed condition did not report its original cleanup error", err)
	}
	if len(search.conditions) != 0 {
		t.Fatal("consumed condition remained in cleanup frontier", search.conditions)
	}
	if err = search.truncateConditions(0); err != nil {
		t.Fatal("condition cleanup retry was not idempotent", err)
	}
	if err = search.truncateConditions(1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid cleanup frontier accepted", err)
	}
}
