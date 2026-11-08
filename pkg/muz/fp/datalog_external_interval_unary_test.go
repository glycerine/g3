package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newExternalIntervalUnaryFixture(t *testing.T, kinds []string, tailVariables, headVariables []uint32, values []string) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	sorts := func(count int) []ast.ASTHandle {
		result := make([]ast.ASTHandle, count)
		for i := range result {
			result[i] = integer
		}
		return result
	}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-unary-p", sorts(len(tailVariables)), kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-unary-q", sorts(len(headVariables)), kinds)
	if values != nil {
		fact := make([]ast.ASTHandle, len(values))
		for i, value := range values {
			fact[i], err = manager.ArithmeticIntegerNumeralText(value)
			fragmentMust(t, err)
		}
		fragmentMust(t, engine.AddFact(p, fact))
	}
	application := func(predicate ast.ASTHandle, variables []uint32) ast.ASTHandle {
		arguments := make([]ast.ASTHandle, len(variables))
		for i, variable := range variables {
			arguments[i], err = manager.MkVar(variable, integer)
			fragmentMust(t, err)
		}
		result, cause := manager.MkApp(predicate, arguments)
		fragmentMust(t, cause)
		return result
	}
	body, head := application(p, tailVariables), application(q, headVariables)
	formula, err := manager.MkImplies(body, head)
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(formula, "external-interval-unary"))
	return manager, engine, callbacks, p, q
}

func resetExternalIntervalUnaryCallbacks(callbacks *externalIntervalCallbackState) {
	resetExternalIntervalCartesianCallbacks(callbacks)
	callbacks.filterColumns = nil
	callbacks.filterArguments = nil
}

func externalIntervalUnaryOperations(operations []int32) []int32 {
	var result []int32
	for _, operation := range operations {
		switch operation {
		case ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion:
			result = append(result, operation)
		}
	}
	return result
}

func TestExternalIntervalDatalogUnaryFilterProjectAndHeadOrder(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, test := range []struct {
			name       string
			tail, head []uint32
			values     []string
			filters    [][]uint32
			removed    []uint32
			renames    [][]uint32
			want       []string
		}{
			{name: "last-occurrence", tail: []uint32{0, 0}, head: []uint32{0}, values: []string{"7", "7"}, filters: [][]uint32{{0, 1}}, removed: []uint32{0}, want: []string{"7"}},
			{name: "three-occurrences", tail: []uint32{0, 0, 0}, head: []uint32{0}, values: []string{"7", "7", "7"}, filters: [][]uint32{{0, 1}, {0, 2}}, removed: []uint32{0, 1}, want: []string{"7"}},
			{name: "variable-order-before-column-order", tail: []uint32{1, 0, 1, 0}, head: []uint32{0, 1}, values: []string{"2", "7", "2", "7"}, filters: [][]uint32{{1, 3}, {0, 2}}, removed: []uint32{0, 1}, renames: [][]uint32{{0, 1}}, want: []string{"7", "2"}},
			{name: "project-swap", tail: []uint32{0, 1, 2}, head: []uint32{2, 0}, values: []string{"2", "7", "9"}, removed: []uint32{1}, renames: [][]uint32{{0, 1}}, want: []string{"9", "2"}},
			{name: "cycle", tail: []uint32{0, 1, 2}, head: []uint32{2, 0, 1}, values: []string{"2", "7", "9"}, renames: [][]uint32{{0, 2}, {2, 1}}, want: []string{"9", "2", "7"}},
			{name: "nullary-head", tail: []uint32{0, 1}, values: []string{"2", "7"}, removed: []uint32{0, 1}},
		} {
			t.Run(kinds[0]+"/"+test.name, func(t *testing.T) {
				_, engine, callbacks, p, q := newExternalIntervalUnaryFixture(t, kinds, test.tail, test.head, test.values)
				for run := 0; run < 2; run++ {
					resetExternalIntervalUnaryCallbacks(callbacks)
					status, err := engine.QueryPredicate(q)
					fragmentMust(t, err)
					if status != lbool.Undef {
						t.Fatal("nonempty unary query", status)
					}
					var projects [][]uint32
					if len(test.removed) != 0 {
						projects = [][]uint32{test.removed}
					}
					if !reflect.DeepEqual(callbacks.filterColumns, test.filters) || !reflect.DeepEqual(callbacks.projectParameters, projects) || !reflect.DeepEqual(callbacks.renameParameters, test.renames) {
						t.Fatal("unary source columns", callbacks.filterColumns, callbacks.projectParameters, callbacks.renameParameters)
					}
					want := []int32{ast.DatalogOpClone}
					if run != 0 {
						want = append(want, ast.DatalogOpClone)
					}
					if len(test.filters) != 0 {
						want = append(want, ast.DatalogOpClone)
						for range test.filters {
							want = append(want, ast.DatalogOpFilter)
						}
						mutation := callbacks.cloneOutputs[len(callbacks.cloneOutputs)-1]
						for _, argument := range callbacks.filterArguments {
							if argument != mutation {
								t.Fatal("FILTER did not use the single mutation clone", callbacks.cloneOutputs, callbacks.filterArguments)
							}
						}
					}
					if len(projects) != 0 {
						want = append(want, ast.DatalogOpProject)
					}
					for range test.renames {
						want = append(want, ast.DatalogOpRename)
					}
					want = append(want, ast.DatalogOpUnion)
					assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), want)
					for column, value := range test.want {
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, column, value, value)
					}
					for column, value := range test.values {
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
					}
				}
			})
		}
	}
}

func TestExternalIntervalDatalogUnaryDisjointColumnsStopBeforeProjection(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, test := range []struct {
			name       string
			tail, head []uint32
			values     []string
			filters    [][]uint32
		}{
			{name: "one-group", tail: []uint32{0, 0}, head: []uint32{0}, values: []string{"7", "9"}, filters: [][]uint32{{0, 1}}},
			{name: "stop-before-next-group", tail: []uint32{1, 0, 1, 0}, head: []uint32{0, 1}, values: []string{"7", "1", "7", "2"}, filters: [][]uint32{{1, 3}}},
		} {
			t.Run(kinds[0]+"/"+test.name, func(t *testing.T) {
				_, engine, callbacks, p, q := newExternalIntervalUnaryFixture(t, kinds, test.tail, test.head, test.values)
				resetExternalIntervalUnaryCallbacks(callbacks)
				status, err := engine.QueryPredicate(q)
				fragmentMust(t, err)
				if status != lbool.False || !reflect.DeepEqual(callbacks.filterColumns, test.filters) {
					t.Fatal("disjoint unary result", status, callbacks.filterColumns)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter})
				for column, value := range test.values {
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
				}
			})
		}
	}
}

func TestExternalIntervalDatalogUnaryCallbackRefusalRollsBack(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, operation := range []int32{ast.DatalogOpFilter, ast.DatalogOpProject} {
			t.Run(fmt.Sprintf("%s/operation-%d", kinds[0], operation), func(t *testing.T) {
				_, engine, callbacks, p, q := newExternalIntervalUnaryFixture(t, kinds, []uint32{0, 0}, []uint32{0}, []string{"7", "7"})
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ := activation.entries[p].product, activation.entries[q].product
				resetExternalIntervalUnaryCallbacks(callbacks)
				refusal := errors.New("unary callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = operation, 1, refusal
				status, err := engine.QueryPredicate(q)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("unary refusal changed persistent relations", status, err, callbacks.operations)
				}
				for column := 0; column < 2; column++ {
					assertExternalIntervalGroundProgramInterval(t, oldP, column, "7", "7")
				}
				callbacks.refusal = nil
				empty, err := oldQ.Empty()
				fragmentMust(t, err)
				if !empty {
					t.Fatal("unary refusal populated its target")
				}
				resetExternalIntervalUnaryCallbacks(callbacks)
				status, err = engine.QueryPredicate(q)
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("unary retry result", status)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
					ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
				})
				assertExternalIntervalGroundProgramInterval(t, activation.entries[q].product, 0, "7", "7")
			})
		}
	}
}

func TestExternalIntervalDatalogUnaryProgramComposition(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			manager, engine, callbacks, p, q := newExternalIntervalUnaryFixture(t, kinds, []uint32{0, 0}, []uint32{0}, []string{"7", "7"})
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-unary-r", []ast.ASTHandle{integer}, kinds)
			addExternalIntervalIdentityRule(t, manager, engine, q, r, []ast.ASTHandle{integer}, "external-interval-unary-consumer")
			_, err = engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			resetExternalIntervalUnaryCallbacks(callbacks)
			status, err := engine.QueryPredicate(r)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("unary program result", status)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion, ast.DatalogOpUnion,
			})
			for _, predicate := range []ast.ASTHandle{p, q, r} {
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[predicate].product, 0, "7", "7")
			}
		})
	}
}
