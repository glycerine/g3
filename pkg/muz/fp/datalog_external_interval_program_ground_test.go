package fp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newExternalIntervalGroundProgramFixture(t *testing.T, pKinds []string, qValue string) (*PropositionalEngine, *externalIntervalCallbackState, []ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	unary := []ast.ASTHandle{integer}
	qKinds := []string{pKinds[1], pKinds[0]}
	p := declareExternalIntervalPredicate(t, manager, engine, "ground-program-p", unary, pKinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "ground-program-q", unary, qKinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "ground-program-h", []ast.ASTHandle{integer, integer}, qKinds)
	seven, err := manager.ArithmeticIntegerNumeralText("7")
	fragmentMust(t, err)
	other, err := manager.ArithmeticIntegerNumeralText(qValue)
	fragmentMust(t, err)
	// Q's ground rule is inserted first, but P is the dependency-first
	// singleton consumer and determines its requested representation.
	addExternalIntervalCartesianRule(t, manager, engine, p, q, h, integer, integer, "ground-program-join")
	addExternalIntervalGroundRule(t, manager, engine, q, other, "ground-program-q-seed")
	addExternalIntervalIdentityRule(t, manager, engine, p, q, unary, "ground-program-identity")
	addExternalIntervalGroundRule(t, manager, engine, p, seven, "ground-program-p-seed")
	selected, err := engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	if !selected {
		t.Fatal("ground program was not selected")
	}
	return engine, callbacks, []ast.ASTHandle{p, q, h}
}

func externalIntervalGroundProgramOperations(operations []int32) []int32 {
	var result []int32
	for _, operation := range operations {
		switch operation {
		case ast.DatalogOpStore, ast.DatalogOpUnion, ast.DatalogOpJoin, ast.DatalogOpComplement:
			result = append(result, operation)
		}
	}
	return result
}

func assertExternalIntervalGroundProgramInterval(t *testing.T, product *rel.ExternalIntervalProduct, column int, lower, upper string) {
	t.Helper()
	var display bytes.Buffer
	fragmentMust(t, product.Display(&display))
	want := fmt.Sprintf("%d in [%s, %s]", column, lower, upper)
	if !strings.Contains(display.String(), want) {
		t.Fatal("ground program interval", want, display.String())
	}
}

func TestExternalIntervalDatalogProgramGroundCacheFirstUseAndRepeat(t *testing.T) {
	for _, pKinds := range [][]string{{"interval_relation", "external_relation"}, {"external_relation", "interval_relation"}} {
		t.Run(pKinds[0]+"-first", func(t *testing.T) {
			engine, callbacks, predicates := newExternalIntervalGroundProgramFixture(t, pKinds, "7")
			for run := 0; run < 2; run++ {
				resetExternalIntervalCartesianCallbacks(callbacks)
				status, err := engine.QueryPredicate(predicates[2])
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("ground program result", status)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalGroundProgramOperations(callbacks.operations), []int32{
					ast.DatalogOpStore, ast.DatalogOpUnion, ast.DatalogOpUnion,
					ast.DatalogOpUnion, ast.DatalogOpJoin, ast.DatalogOpUnion,
				})
				if operationCount(callbacks.operations, ast.DatalogOpClone) != 3*run {
					t.Fatal("ground program did not share its LOAD bank", run, callbacks.operations)
				}
				started := false
				for _, operation := range callbacks.operations {
					if operation == ast.DatalogOpStore {
						started = true
					}
					if started && operation == ast.DatalogOpClone {
						t.Fatal("LOAD clone ran after singleton construction", callbacks.operations)
					}
				}
				if got := engine.externalIntervalDatalog.entries[predicates[0]].product.Kinds(); !externalIntervalKindsEqual(got, pKinds) {
					t.Fatal("singleton used insertion order instead of first scheduled head", got, pKinds)
				}
				for _, predicate := range predicates {
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[predicate].product, 0, "7", "7")
				}
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[predicates[2]].product, 1, "7", "7")
			}
		})
	}
}

func TestExternalIntervalDatalogProgramGroundUnaryTupleCacheAndSorts(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed=%t", mixed), func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			rightSort, rightValue := integer, seven
			if mixed {
				rightSort, err = manager.MkFamilySort(family, 0, nil)
				fragmentMust(t, err)
				rightValue, err = manager.ArithmeticRealNumeralText("7")
				fragmentMust(t, err)
			}
			kinds := []string{"external_relation", "interval_relation"}
			p := declareExternalIntervalPredicate(t, manager, engine, "ground-program-tuple-p", []ast.ASTHandle{integer}, kinds)
			s := declareExternalIntervalPredicate(t, manager, engine, "ground-program-tuple-s", []ast.ASTHandle{rightSort}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "ground-program-tuple-q", []ast.ASTHandle{integer, rightSort}, kinds)
			addExternalIntervalNaryGroundRule(t, manager, engine, q, []ast.ASTHandle{seven, rightValue})
			addExternalIntervalCartesianRule(t, manager, engine, p, s, q, integer, rightSort, "ground-program-tuple-join")
			addExternalIntervalGroundRule(t, manager, engine, p, seven, "ground-program-tuple-p-seed")
			addExternalIntervalGroundRule(t, manager, engine, s, rightValue, "ground-program-tuple-s-seed")
			_, err = engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err := engine.QueryPredicate(q)
			fragmentMust(t, err)
			stores := 1
			if mixed {
				stores = 2
			}
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpStore) != stores ||
				operationCount(callbacks.operations, ast.DatalogOpUnion) != 4 || len(callbacks.joinArguments) != 2 {
				t.Fatal("unary and tuple rules did not share the sort-sensitive cache", status, callbacks.operations)
			}
			first := callbacks.joinArguments[0]
			if len(first) != 2 || (first[0] == first[1]) == mixed {
				t.Fatal("tuple did not reuse identical cached unary products", mixed, first)
			}
			product := engine.externalIntervalDatalog.entries[q].product
			sorts, err := product.Sorts()
			fragmentMust(t, err)
			if len(sorts) != 2 || sorts[0] != integer || sorts[1] != rightSort {
				t.Fatal("tuple cache changed column sorts", sorts)
			}
			assertExternalIntervalGroundProgramInterval(t, product, 0, "7", "7")
			assertExternalIntervalGroundProgramInterval(t, product, 1, "7", "7")
		})
	}
}

func TestExternalIntervalDatalogProgramGroundNullaryTotalsPerHead(t *testing.T) {
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	pKinds := []string{"interval_relation", "external_relation"}
	qKinds := []string{"external_relation", "interval_relation"}
	p := declareExternalIntervalPredicate(t, manager, engine, "ground-program-nullary-p", nil, pKinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "ground-program-nullary-q", nil, qKinds)
	addExternalIntervalNullaryGroundRule(t, manager, engine, p, "ground-program-nullary-p-rule")
	addExternalIntervalNullaryGroundRule(t, manager, engine, q, "ground-program-nullary-q-rule")
	_, err := engine.ensureExternalIntervalDatalog()
	fragmentMust(t, err)
	for run := 0; run < 2; run++ {
		resetExternalIntervalCartesianCallbacks(callbacks)
		status, cause := engine.QueryPredicate(q)
		fragmentMust(t, cause)
		if status != lbool.Undef {
			t.Fatal("nullary program result", status)
		}
		assertExternalIntervalRegisterOperations(t, externalIntervalGroundProgramOperations(callbacks.operations), []int32{
			ast.DatalogOpComplement, ast.DatalogOpUnion, ast.DatalogOpComplement, ast.DatalogOpUnion,
		})
		for index, predicate := range []ast.ASTHandle{p, q} {
			product := engine.externalIntervalDatalog.entries[predicate].product
			empty, cause := product.Empty()
			fragmentMust(t, cause)
			if empty || !externalIntervalKindsEqual(product.Kinds(), [][]string{pKinds, qKinds}[index]) {
				t.Fatal("nullary TOTAL was shared between different heads", index, empty, product.Kinds())
			}
		}
	}
}

func TestExternalIntervalDatalogProgramGroundLaterSingletonRefusalRollback(t *testing.T) {
	engine, callbacks, predicates := newExternalIntervalGroundProgramFixture(t, []string{"interval_relation", "external_relation"}, "9")
	before := make(map[ast.ASTHandle]*rel.ExternalIntervalProduct, len(predicates))
	for _, predicate := range predicates {
		before[predicate] = engine.externalIntervalDatalog.entries[predicate].product
	}
	resetExternalIntervalCartesianCallbacks(callbacks)
	refusal := errors.New("second program singleton STORE refused")
	callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = ast.DatalogOpStore, 2, refusal
	status, err := engine.QueryPredicate(predicates[2])
	if status != lbool.Undef || !errors.Is(err, refusal) {
		t.Fatal("later singleton failure", status, err)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalGroundProgramOperations(callbacks.operations), []int32{
		ast.DatalogOpStore, ast.DatalogOpUnion, ast.DatalogOpStore,
	})
	callbacks.refusal = nil
	for _, predicate := range predicates {
		if engine.externalIntervalDatalog.entries[predicate].product != before[predicate] {
			t.Fatal("later singleton failure published earlier rule work")
		}
		empty, cause := before[predicate].Empty()
		fragmentMust(t, cause)
		if !empty {
			t.Fatal("later singleton failure changed persistent facts")
		}
	}
	resetExternalIntervalCartesianCallbacks(callbacks)
	status, err = engine.QueryPredicate(predicates[2])
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("ground program retry result", status)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalGroundProgramOperations(callbacks.operations), []int32{
		ast.DatalogOpStore, ast.DatalogOpUnion, ast.DatalogOpStore,
		ast.DatalogOpUnion, ast.DatalogOpUnion, ast.DatalogOpJoin, ast.DatalogOpUnion,
	})
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[predicates[0]].product, 0, "7", "7")
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[predicates[1]].product, 0, "7", "9")
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[predicates[2]].product, 1, "7", "9")
}
