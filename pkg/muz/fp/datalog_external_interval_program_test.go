package fp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newExternalIntervalProgramFixture(t *testing.T, kinds []string) (*PropositionalEngine, *externalIntervalCallbackState, []ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	unary := []ast.ASTHandle{integer}
	predicates := make([]ast.ASTHandle, 6)
	for index, name := range []string{"P", "T", "S", "Q", "R", "H"} {
		sorts := unary
		if name == "H" {
			sorts = []ast.ASTHandle{integer, integer}
		}
		predicates[index] = declareExternalIntervalPredicate(t, manager, engine, "external-interval-program-"+name, sorts, kinds)
	}
	for index, value := range []string{"2", "5", "9"} {
		numeral, cause := manager.ArithmeticIntegerNumeralText(value)
		fragmentMust(t, cause)
		fragmentMust(t, engine.AddFact(predicates[index], []ast.ASTHandle{numeral}))
	}
	p, other, side, q, r, head := predicates[0], predicates[1], predicates[2], predicates[3], predicates[4], predicates[5]
	// Add consumers before their producers. Both rules for Q must finish
	// before R reads its interval, and the join must see R's complete bounds.
	addExternalIntervalCartesianRule(t, manager, engine, r, side, head, integer, integer, "program-join-consumer")
	addExternalIntervalIdentityRule(t, manager, engine, q, r, unary, "program-chain-consumer")
	addExternalIntervalIdentityRule(t, manager, engine, p, q, unary, "program-first-seed")
	addExternalIntervalIdentityRule(t, manager, engine, other, q, unary, "program-second-seed")
	return engine, callbacks, predicates
}

func assertExternalIntervalProgramBounds(t *testing.T, engine *PropositionalEngine, predicates []ast.ASTHandle) {
	t.Helper()
	for index, predicate := range predicates[3:] {
		var display bytes.Buffer
		fragmentMust(t, engine.externalIntervalDatalog.entries[predicate].product.Display(&display))
		if !strings.Contains(display.String(), "0 in [2, 5]") {
			t.Fatalf("derived predicate %d missed one seed: %s", index, display.String())
		}
		if index == 2 && !strings.Contains(display.String(), "1 in [9, 9]") {
			t.Fatal("join lost the side input interval", display.String())
		}
	}
}

func assertExternalIntervalProgramCallbacks(t *testing.T, callbacks *externalIntervalCallbackState, clones int) {
	t.Helper()
	if operationCount(callbacks.operations, ast.DatalogOpClone) != clones ||
		operationCount(callbacks.operations, ast.DatalogOpUnion) != 4 ||
		operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 ||
		operationCount(callbacks.operations, ast.DatalogOpStore) != 0 {
		t.Fatal("program did not share one LOAD bank and final STORE", clones, callbacks.operations)
	}
	started := false
	var evaluations []int32
	for _, operation := range callbacks.operations {
		if operation == ast.DatalogOpClone && started {
			t.Fatal("program cloned an intermediate predicate between rules", callbacks.operations)
		}
		if operation == ast.DatalogOpUnion || operation == ast.DatalogOpJoin {
			started = true
			evaluations = append(evaluations, operation)
		}
	}
	assertExternalIntervalRegisterOperations(t, evaluations, []int32{
		ast.DatalogOpUnion, ast.DatalogOpUnion, ast.DatalogOpUnion,
		ast.DatalogOpJoin, ast.DatalogOpUnion,
	})
}

func TestExternalIntervalDatalogProgramDependencyOrderAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			engine, callbacks, predicates := newExternalIntervalProgramFixture(t, kinds)
			for run := 0; run < 2; run++ {
				resetExternalIntervalCartesianCallbacks(callbacks)
				status, err := engine.QueryPredicate(predicates[5])
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("nonempty approximate program result", run, status)
				}
				clones := 3
				if run != 0 {
					clones = 6
				}
				assertExternalIntervalProgramCallbacks(t, callbacks, clones)
				assertExternalIntervalProgramBounds(t, engine, predicates)
			}
		})
	}
}

func TestExternalIntervalDatalogProgramMiddleUnionRollbackAndRetry(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			engine, callbacks, predicates := newExternalIntervalProgramFixture(t, kinds)
			selected, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			if !selected {
				t.Fatal("program products were not selected")
			}
			activation := engine.externalIntervalDatalog
			before := make(map[ast.ASTHandle]*rel.ExternalIntervalProduct, len(predicates))
			for _, predicate := range predicates {
				before[predicate] = activation.entries[predicate].product
			}
			resetExternalIntervalCartesianCallbacks(callbacks)
			refusal := errors.New("third program UNION refused")
			callbacks.refusalOperation = ast.DatalogOpUnion
			callbacks.refusalAt = 3
			callbacks.refusal = refusal
			status, err := engine.QueryPredicate(predicates[5])
			if status != lbool.Undef || !errors.Is(err, refusal) ||
				operationCount(callbacks.operations, ast.DatalogOpUnion) != 3 ||
				operationCount(callbacks.operations, ast.DatalogOpJoin) != 0 ||
				operationCount(callbacks.operations, ast.DatalogOpClone) != 3 {
				t.Fatal("middle program refusal", status, err, callbacks.operations)
			}
			callbacks.refusal = nil
			for index, predicate := range predicates {
				if activation.entries[predicate].product != before[predicate] {
					t.Fatal("failed program published a partial STORE", index)
				}
				empty, cause := before[predicate].Empty()
				fragmentMust(t, cause)
				if empty != (index >= 3) {
					t.Fatal("failed program changed persistent predicate contents", index, empty)
				}
			}
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err = engine.QueryPredicate(predicates[5])
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("program retry result", status)
			}
			assertExternalIntervalProgramCallbacks(t, callbacks, 3)
			assertExternalIntervalProgramBounds(t, engine, predicates)
		})
	}
}
