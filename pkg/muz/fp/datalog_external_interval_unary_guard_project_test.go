package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExternalIntervalDatalogUnaryGuardProjectBorrowedInput(t *testing.T) {
	_, engine, callbacks, _, q, _ := newExternalIntervalUnaryGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{0, 1}, []uint32{0}, []string{"1", "3"}, false, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
	resetExternalIntervalUnaryGuardCallbacks(callbacks)
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{"(< v0 v1)"}) || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{1}}) {
		t.Fatal("fused guard projection", status, callbacks.guardConditions, callbacks.projectParameters)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
		ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
	})
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, "1", "1")
}

func TestExternalIntervalDatalogUnaryGuardProjectOwnedInputsAndHeadColumns(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, real := range []bool{false, true} {
			for _, shape := range []string{"repeated-kept-and-removed", "selected-owned", "nullary-head"} {
				t.Run(fmt.Sprintf("%s/real=%t/%s", kinds[0], real, shape), func(t *testing.T) {
					x, y, z := "1", "2", "3"
					if real {
						x, y, z = "1/2", "3/2", "5/2"
					}
					tail, head := []int{1, 0, 2, 1, 0, 2}, []uint32{0, 1}
					facts := []string{y, x, z, y, x, z}
					guard := externalIntervalUnaryGuardSpec{op: "<", left: 0, right: 2}
					condition := "(< v4 v5)"
					identical := [][]uint32{{1, 4}, {0, 3}, {2, 5}}
					projects, renames := [][]uint32{{2, 5}, {0, 1}}, [][]uint32{{0, 1}}
					wantValues := []string{x, y}
					wantOps := []int32{
						ast.DatalogOpClone, ast.DatalogOpClone,
						ast.DatalogOpFilter, ast.DatalogOpFilter, ast.DatalogOpFilter,
						ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject,
						ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion,
					}
					switch shape {
					case "selected-owned":
						tail, head, facts = []int{-1, 1, 0}, []uint32{0}, []string{"7", y, x}
						guard.right, condition = 1, "(< v1 v0)"
						identical, projects, renames = nil, [][]uint32{{0}, {0}}, nil
						wantValues = []string{x}
						wantOps = []int32{
							ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject,
							ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
						}
					case "nullary-head":
						tail, head, facts = []int{0}, nil, []string{x}
						guard.bound, condition = "3", "(< v0 3)"
						identical, projects, renames = nil, [][]uint32{{0}}, nil
						wantValues = nil
						wantOps = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion}
					}
					_, engine, callbacks, p, q, _ := newExternalIntervalUnaryGuardFixture(t, kinds, tail, head, facts, real, false, []externalIntervalUnaryGuardSpec{guard})
					for run := 0; run < 2; run++ {
						resetExternalIntervalUnaryGuardCallbacks(callbacks)
						status, err := engine.QueryPredicate(q)
						fragmentMust(t, err)
						if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{condition}) || !reflect.DeepEqual(callbacks.filterColumns, identical) ||
							!reflect.DeepEqual(callbacks.projectParameters, projects) || !reflect.DeepEqual(callbacks.renameParameters, renames) {
							t.Fatal("fused guard or shifted head columns", status, callbacks.guardConditions, callbacks.filterColumns, callbacks.projectParameters, callbacks.renameParameters)
						}
						expected := append([]int32(nil), wantOps...)
						if run != 0 {
							expected = append([]int32{ast.DatalogOpClone}, expected...)
						}
						assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), expected)
						product := engine.externalIntervalDatalog.entries[q].product
						sorts, err := product.Sorts()
						fragmentMust(t, err)
						if len(sorts) != len(head) || operationCount(callbacks.operations, ast.DatalogOpComplement) != 0 {
							t.Fatal("fused projection changed arity or constructed a TOTAL", sorts, callbacks.operations)
						}
						for column, value := range wantValues {
							assertExternalIntervalGroundProgramInterval(t, product, column, value, value)
						}
						for column, value := range facts {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
						}
					}
				})
			}
		}
	}
}

func TestExternalIntervalDatalogUnaryGuardProjectContradictionStillProjects(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			_, engine, callbacks, p, q, _ := newExternalIntervalUnaryGuardFixture(t, kinds,
				[]int{0, 1}, []uint32{0}, []string{"3", "1"}, false, false,
				[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
			resetExternalIntervalUnaryGuardCallbacks(callbacks)
			status, err := engine.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.Undef || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{1}}) {
				t.Fatal("empty-source projection did not match C++ behavior", status, callbacks.projectParameters)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
			})
			if empty, cause := engine.externalIntervalDatalog.entries[q].product.Empty(); cause != nil || empty {
				t.Fatal("empty-source projection did not publish the reset payload", empty, cause)
			}
			for column, value := range []string{"3", "1"} {
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
			}
		})
	}
}

func TestExternalIntervalDatalogUnaryGuardProjectConjunctionStaysOneFilter(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, firstFalse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/first-false=%t", kinds[0], firstFalse), func(t *testing.T) {
				facts := []string{"0", "1"}
				if firstFalse {
					facts = []string{"3", "2"}
				}
				_, engine, callbacks, _, q, _ := newExternalIntervalUnaryGuardFixture(t, kinds,
					[]int{0, 1}, []uint32{0}, facts, false, false,
					[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}, {op: ">=", left: 1, bound: "2"}})
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(q)
				fragmentMust(t, err)
				// Either conjunct is false, but source interval filtering ignores
				// their conjunction and the external callback remains approximate.
				if status != lbool.Undef || !reflect.DeepEqual(callbacks.guardConditions, []string{"(and (< v0 v1) (>= v1 2))"}) || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{1}}) {
					t.Fatal("fused operation split or reordered the conjunction", status, callbacks.guardConditions, callbacks.projectParameters)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
					ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
				})
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, facts[0], facts[0])
			})
		}
	}
}

func TestExternalIntervalDatalogUnaryGuardProjectEmptyInputSkipsCallbacks(t *testing.T) {
	_, engine, callbacks, _, q, _ := newExternalIntervalUnaryGuardFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{0, 1}, []uint32{0}, nil, false, false,
		[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}})
	resetExternalIntervalUnaryGuardCallbacks(callbacks)
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.False || len(externalIntervalUnaryOperations(callbacks.operations)) != 0 {
		t.Fatal("empty source executed fused callback operations", status, callbacks.operations)
	}
}

func TestExternalIntervalDatalogUnaryGuardProjectProgramRefusalAndRetry(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, stop := range []struct {
			name string
			op   int32
			at   int
		}{{"guard", ast.DatalogOpFilter, 4}, {"fused-project", ast.DatalogOpProject, 1}, {"head-project", ast.DatalogOpProject, 2}} {
			t.Run(kinds[0]+"/"+stop.name, func(t *testing.T) {
				facts := []string{"2", "1", "3", "2", "1", "3"}
				manager, engine, callbacks, p, q, sort := newExternalIntervalUnaryGuardFixture(t, kinds,
					[]int{1, 0, 2, 1, 0, 2}, []uint32{0, 1}, facts, false, false,
					[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 2}})
				r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-fused-consumer", []ast.ASTHandle{sort, sort}, kinds)
				addExternalIntervalIdentityRule(t, manager, engine, q, r, []ast.ASTHandle{sort, sort}, "external-interval-fused-consumer-rule")
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ, oldR := activation.entries[p].product, activation.entries[q].product, activation.entries[r].product
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				refusal := errors.New("fused program callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = stop.op, stop.at, refusal
				status, err := engine.QueryPredicate(r)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[r].product != oldR || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("fused refusal published partial program work", status, err, callbacks.operations)
				}
				callbacks.refusal = nil
				for _, predicate := range []ast.ASTHandle{q, r} {
					empty, cause := activation.entries[predicate].product.Empty()
					fragmentMust(t, cause)
					if !empty {
						t.Fatal("fused refusal populated a derived relation")
					}
				}
				for column, value := range facts {
					assertExternalIntervalGroundProgramInterval(t, oldP, column, value, value)
				}
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err = engine.QueryPredicate(r)
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("fused program retry", status)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
					ast.DatalogOpClone, ast.DatalogOpClone,
					ast.DatalogOpFilter, ast.DatalogOpFilter, ast.DatalogOpFilter,
					ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject,
					ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion, ast.DatalogOpUnion,
				})
				assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 0, "1", "1")
				assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 1, "2", "2")
			})
		}
	}
}
