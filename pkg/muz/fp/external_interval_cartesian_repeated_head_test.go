package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExternalIntervalDatalogCartesianRepeatedHeadOriginalShape(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			// Exact former rejection: H(x,x) <- P(x),Q(x), with the
			// original MkAnd(P(x),Q(x)) construction and actual facts.
			_, engine, callbacks, p, q, h := newExternalIntervalPositiveFixture(t, kinds, []uint32{0}, []uint32{0}, []uint32{0, 0}, false, "")
			resetExternalIntervalUnaryGuardCallbacks(callbacks)
			status, err := engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef || !reflect.DeepEqual(callbacks.joinParameters, [][]uint32{{0, 0}}) || len(callbacks.projectParameters) != 0 || !reflect.DeepEqual(callbacks.renameParameters, [][]uint32{{0, 1}}) {
				t.Fatal("existing joined columns were not reused", status, callbacks.joinParameters, callbacks.projectParameters, callbacks.renameParameters)
			}
			if plan := engine.externalIntervalDatalog.cartesian; plan.left != q || plan.right != p {
				t.Fatal("former repeated-head fixture normalized tail order", plan)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion,
			})
			for column := 0; column < 2; column++ {
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, column, "1", "1")
			}
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, 0, "1", "1")
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, "1", "1")
		})
	}
}

func TestExternalIntervalDatalogCartesianRepeatedHeadAssemblyAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, real := range []bool{false, true} {
			for _, shape := range []string{"reuse", "reuse-guard", "self-join", "project-join", "reuse-then-duplicate", "two-new-copies", "constants-guard", "constants-guard-only"} {
				t.Run(fmt.Sprintf("%s/real=%t/%s", kinds[0], real, shape), func(t *testing.T) {
					x, y, z := "1", "5", "3"
					if real {
						x, y, z = "1/2", "5/2", "3/2"
					}
					left, right, head := []int{0}, []int{1}, []uint32{1, 0, 0}
					leftFacts, rightFacts := []string{x}, []string{y}
					joins := [][]uint32{{}, {0, 0}}
					projects, renames := [][]uint32{{1}}, [][]uint32{{0, 1}}
					values := []string{y, x, x}
					var guards []externalIntervalUnaryGuardSpec
					var conditions []string
					var constantColumns []uint32
					want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion}
					switch shape {
					case "reuse":
						right, rightFacts, head, values = []int{0}, []string{x}, []uint32{0, 0}, []string{x, x}
						joins, projects = [][]uint32{{0, 0}}, nil
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion}
					case "reuse-guard":
						right, rightFacts = []int{0, 1}, []string{x, y}
						joins, projects, renames = [][]uint32{{0, 0}}, nil, [][]uint32{{0, 2}}
						guards, conditions = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}}, []string{"(< v1 v2)"}
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpFilter, ast.DatalogOpRename, ast.DatalogOpUnion}
					case "self-join":
						head, values, renames = []uint32{0, 0}, []string{x, x}, nil
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpUnion}
					case "reuse-then-duplicate":
						right, rightFacts, head, values = []int{0}, []string{x}, []uint32{0, 0, 0}, []string{x, x, x}
						joins = [][]uint32{{0, 0}, {0, 0}}
					case "two-new-copies":
						head, values = []uint32{1, 0, 0, 0}, []string{y, x, x, x}
						joins, projects = [][]uint32{{}, {0, 0}, {0, 0}}, [][]uint32{{1}, {1, 2}}
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion}
					case "constants-guard":
						left, right = []int{-1, 0}, []int{1, -2}
						leftFacts, rightFacts = []string{"7", x}, []string{y, "9"}
						guards, conditions = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}}, []string{"(< v1 v2)"}
						constantColumns, projects = []uint32{0, 3}, [][]uint32{{0, 3}, {1}}
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpFilter, ast.DatalogOpFilter, ast.DatalogOpFilter,
							ast.DatalogOpProject, ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion}
					case "constants-guard-only":
						left, right = []int{-1, 0, 2}, []int{2, 3, 1, -2}
						leftFacts, rightFacts = []string{"7", x, "11"}, []string{"11", z, y, "9"}
						guards, conditions = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}}, []string{"(< v1 v2)"}
						joins, constantColumns, projects = [][]uint32{{2, 0}, {0, 0}}, []uint32{0, 4}, [][]uint32{{2, 3}, {2}, {0, 3}, {1}}
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpFilter, ast.DatalogOpFilter,
							ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpProject, ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion}
					}
					_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianConstantsFixture(t, kinds, left, right, head, leftFacts, rightFacts, real, guards)
					for run := 0; run < 2; run++ {
						resetExternalIntervalUnaryGuardCallbacks(callbacks)
						status, err := engine.QueryPredicate(h)
						fragmentMust(t, err)
						if status != lbool.Undef || !reflect.DeepEqual(callbacks.joinParameters, joins) || !reflect.DeepEqual(callbacks.projectParameters, projects) ||
							!reflect.DeepEqual(callbacks.renameParameters, renames) || !reflect.DeepEqual(callbacks.guardConditions, conditions) || !reflect.DeepEqual(callbacks.constantColumns, constantColumns) {
							t.Fatal("repeated-head source assembly", status, callbacks.joinParameters, callbacks.projectParameters, callbacks.renameParameters, callbacks.guardConditions, callbacks.constantColumns)
						}
						if len(joins) > 1 {
							self := callbacks.joinArguments[1][0] == callbacks.joinArguments[1][1]
							if self != (shape == "self-join") {
								t.Fatal("duplicate-column JOIN input ownership", callbacks.joinArguments)
							}
							joinIndex := 0
							for index, operation := range callbacks.operations {
								if operation != ast.DatalogOpJoin {
									continue
								}
								if joinIndex > 0 && (index+1 >= len(callbacks.operations) || callbacks.operations[index+1] != ast.DatalogOpIsEmpty) {
									t.Fatal("duplicate JOIN skipped source fast_empty before later assembly", callbacks.operations)
								}
								joinIndex++
							}
						}
						expected := append([]int32(nil), want...)
						if run > 0 {
							expected = append([]int32{ast.DatalogOpClone}, expected...)
						}
						assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), expected)
						for column, value := range values {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, column, value, value)
						}
						for column, value := range leftFacts {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
						}
						for column, value := range rightFacts {
							assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, column, value, value)
						}
					}
				})
			}
		}
	}
}

func TestExternalIntervalDatalogCartesianRepeatedHeadEmptyFrontiers(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, shape := range []string{"join", "guard"} {
			t.Run(kinds[0]+"/"+shape, func(t *testing.T) {
				left, right, head := []int{0}, []int{0}, []uint32{0, 0}
				leftFacts, rightFacts := []string{"1"}, []string{"2"}
				var guards []externalIntervalUnaryGuardSpec
				want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin}
				if shape == "guard" {
					right, rightFacts, head = []int{1}, []string{"0"}, []uint32{1, 0, 0}
					guards = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}}
					want = append(want, ast.DatalogOpFilter)
				}
				_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianConstantsFixture(t, kinds, left, right, head, leftFacts, rightFacts, false, guards)
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(h)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("empty rule reached repeated-head assembly", status)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), want)
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, 0, leftFacts[0], leftFacts[0])
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, rightFacts[0], rightFacts[0])
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianRepeatedHeadProgramRefusalAndRetry(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, stop := range []struct {
			name string
			op   int32
			at   int
		}{{"auxiliary-project", ast.DatalogOpProject, 1}, {"duplicate-join", ast.DatalogOpJoin, 2}} {
			t.Run(kinds[0]+"/"+stop.name, func(t *testing.T) {
				manager, engine, callbacks, p, q, h, sort := newExternalIntervalCartesianConstantsFixture(t, kinds,
					[]int{0}, []int{1}, []uint32{1, 0, 0}, []string{"1"}, []string{"5"}, false, nil)
				r := declareExternalIntervalPredicate(t, manager, engine, "cartesian-repeat-consumer", []ast.ASTHandle{sort, sort, sort}, kinds)
				addExternalIntervalIdentityRule(t, manager, engine, h, r, []ast.ASTHandle{sort, sort, sort}, "cartesian-repeat-consumer-rule")
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ, oldH, oldR := activation.entries[p].product, activation.entries[q].product, activation.entries[h].product, activation.entries[r].product
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				refusal := errors.New("repeated-head callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = stop.op, stop.at, refusal
				status, err := engine.QueryPredicate(r)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[h].product != oldH || activation.entries[r].product != oldR || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("duplicate-column refusal published working registers", status, err, callbacks.operations)
				}
				callbacks.refusal = nil
				for _, predicate := range []ast.ASTHandle{h, r} {
					empty, cause := activation.entries[predicate].product.Empty()
					fragmentMust(t, cause)
					if !empty {
						t.Fatal("failed repeated-head program populated derived relation")
					}
				}
				assertExternalIntervalGroundProgramInterval(t, oldP, 0, "1", "1")
				assertExternalIntervalGroundProgramInterval(t, oldQ, 0, "5", "5")
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err = engine.QueryPredicate(r)
				fragmentMust(t, err)
				if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpJoin) != 2 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 {
					t.Fatal("repeated-head program retry", status, callbacks.operations)
				}
				for column, value := range []string{"5", "1", "1"} {
					assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, column, value, value)
				}
			})
		}
	}
}
