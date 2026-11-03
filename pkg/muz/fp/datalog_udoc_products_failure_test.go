package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func newPublicUDocProductFailureFixture(t *testing.T) *publicUDocProductFixture {
	return newPublicUDocProductFailureFixtureMode(t, false)
}

func newPublicUDocProductFailureFixtureMode(t *testing.T, groundRules bool) *publicUDocProductFixture {
	t.Helper()
	var budget *arena.Budget
	// Registered before fixture cleanup so this runs after engine, AST and
	// numeral/parameter services have released their actual native storage.
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("public DOC product fixture leaked native budget", budget.Used())
		}
	})
	f := newPublicUDocProductFixture(t, "hashtable", false, groundRules)
	budget = f.m.Budget()
	if groundRules {
		prepareUDocProductGroundRules(f)
		f.addGroundRule(f.doc, "ground-doc", f.values[1])
		f.addGroundRule(f.finite, "ground-finite", f.yes)
	} else {
		fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
		fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1}))
		fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{0, 2}))
	}
	f.addRule(false)
	if f.e.udocDatalog != nil {
		t.Fatal("failure fixture adopted DOC before a public query")
	}
	// DOC has its own native allocation profile. Small pages connected to the
	// same budget make the campaign reach real migration/join/project storage,
	// rather than relying on an unrelated AST allocation happening by chance.
	factory, err := rel.NewUDocRelationFactory(f.m, arena.VectorConfig{
		PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: budget},
	})
	fragmentMust(t, err)
	f.e.udocDatalog = &datalogUDocContext{manager: f.m, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}
	return f
}

func publicUDocProductLiveCounts(stats ast.ASTManagerStats) [4]uint64 {
	return [4]uint64{uint64(stats.LiveNodes), uint64(stats.LiveInfos), uint64(stats.LiveParameters), uint64(stats.LiveReferences)}
}

func TestDatalogUDocProductsPublicAllocationFailureRetryAndClose(t *testing.T) {
	checkPublicUDocProductAllocationFailureRetryAndClose(t, newPublicUDocProductFailureFixture)
}

func checkPublicUDocProductAllocationFailureRetryAndClose(t *testing.T, fixture func(*testing.T) *publicUDocProductFixture) {
	t.Helper()
	var expected [4]uint64
	t.Run("uninterrupted-baseline", func(t *testing.T) {
		f := fixture(t)
		f.checkRows(lbool.True, [2]uint32{1, 1})
		f.checkRows(lbool.True, [2]uint32{1, 1})
		fragmentMust(t, f.e.udocDatalog.discardResult())
		expected = publicUDocProductLiveCounts(f.m.Stats())
		fragmentMust(t, f.e.Close())
	})
	if t.Failed() {
		return
	}
	failures, completed := 0, 0
	for allowance := int64(0); allowance <= 1024; allowance++ {
		t.Run(fmt.Sprintf("reservations-%d", allowance), func(t *testing.T) {
			f := fixture(t)
			f.m.Budget().FailAfter(allowance)
			status, cause := f.e.QueryPredicate(f.head)
			f.m.Budget().FailAfter(-1)
			if cause != nil {
				failures++
				var failure memory.Failure
				if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected product query allocation failure", allowance, status, cause)
				}
				answer, answerErr := f.e.Answer(f.head)
				if answer != (ast.ASTRef{}) || answerErr == nil {
					if answer != (ast.ASTRef{}) {
						_ = f.m.DelRef(&answer)
					}
					t.Fatal("failed public product query exposed an answer", allowance, answerErr)
				}
			} else {
				completed++
				if status != lbool.True {
					t.Fatal("completed product query changed result", status)
				}
			}
			f.checkRows(lbool.True, [2]uint32{1, 1})
			f.checkRows(lbool.True, [2]uint32{1, 1})
			if f.e.udocDatalog.products == nil || f.e.udocDatalog.products.result[f.head] == nil || len(f.e.udocDatalog.products.pending) != 0 {
				t.Fatal("retried query lost actual product or pending cleanup frontier")
			}
			fragmentMust(t, f.e.udocDatalog.discardResult())
			if actual := publicUDocProductLiveCounts(f.m.Stats()); actual != expected {
				t.Fatal("failed public product query leaked AST ownership", allowance, expected, actual, cause)
			}
			fragmentMust(t, f.e.Close())
			fragmentMust(t, f.e.Close())
		})
		if t.Failed() {
			return
		}
		if completed != 0 {
			break
		}
	}
	if failures == 0 || completed == 0 {
		t.Fatal("public product campaign missed failure or completion", failures, completed)
	}
}

func TestDatalogUDocProductsPublicCancellationInvalidatesAndRetries(t *testing.T) {
	f := newPublicUDocProductFailureFixture(t)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	held, err := f.e.Answer(f.head)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&held)) }()
	f.m.Limit.Cancel()
	_, cause := f.e.QueryPredicate(f.head)
	f.m.Limit.ResetCancel()
	if cause == nil {
		t.Fatal("public product query ignored cancellation")
	}
	answer, err := f.e.Answer(f.head)
	if err == nil || answer != (ast.ASTRef{}) {
		if answer != (ast.ASTRef{}) {
			_ = f.m.DelRef(&answer)
		}
		t.Fatal("cancelled product query exposed stale answer", err)
	}
	f.checkFormula(held, []ast.ASTHandle{f.values[1], f.yes}, true)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog.products.result[f.head] == nil || len(f.e.udocDatalog.products.pending) != 0 {
		t.Fatal("cancelled product query did not recover actual product execution")
	}
	fragmentMust(t, f.e.Close())
}
