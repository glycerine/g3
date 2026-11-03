package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Negative terms denote 7, 9, 11, and 1; nonnegative terms are variable IDs.
func newExternalIntervalHeadConstantsFixture(t *testing.T, kinds []string, left, right, head []int, leftFacts, rightFacts []string, real, originalOrder bool, guards []externalIntervalUnaryGuardSpec) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	sortKind := int32(1)
	if real {
		sortKind = 0
	}
	sort, err := manager.MkFamilySort(family, sortKind, nil)
	fragmentMust(t, err)
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			fragmentMust(t, manager.DelRef(&roots[i]))
		}
	}()
	own := func(value ast.ASTHandle, cause error) ast.ASTHandle {
		fragmentMust(t, cause)
		root, cause := manager.NewRef(value)
		fragmentMust(t, cause)
		roots = append(roots, root)
		return value
	}
	numeral := func(value string) ast.ASTHandle {
		if real {
			return own(manager.ArithmeticRealNumeralText(value))
		}
		return own(manager.ArithmeticIntegerNumeralText(value))
	}
	sorts := func(count int) []ast.ASTHandle {
		result := make([]ast.ASTHandle, count)
		for i := range result {
			result[i] = sort
		}
		return result
	}
	p := declareExternalIntervalPredicate(t, manager, engine, "cartesian-head-constants-p", sorts(len(left)), kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "cartesian-head-constants-q", sorts(len(right)), kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "cartesian-head-constants-h", sorts(len(head)), kinds)
	for _, fact := range []struct {
		predicate ast.ASTHandle
		values    []string
	}{{p, leftFacts}, {q, rightFacts}} {
		if fact.values != nil {
			values := make([]ast.ASTHandle, len(fact.values))
			for column, value := range fact.values {
				values[column] = numeral(value)
			}
			fragmentMust(t, engine.AddFact(fact.predicate, values))
		}
	}
	application := func(predicate ast.ASTHandle, terms []int) ast.ASTHandle {
		arguments := make([]ast.ASTHandle, len(terms))
		for i, term := range terms {
			if term < 0 {
				arguments[i] = numeral([]string{"7", "9", "11", "1"}[-term-1])
			} else {
				arguments[i] = own(manager.MkVar(uint32(term), sort))
			}
		}
		return own(manager.MkApp(predicate, arguments))
	}
	var conditions []ast.ASTHandle
	for _, guard := range guards {
		first := own(manager.MkVar(guard.left, sort))
		var second ast.ASTHandle
		if guard.bound != "" {
			second = numeral(guard.bound)
		} else {
			second = own(manager.MkVar(guard.right, sort))
		}
		switch guard.op {
		case "<":
			conditions = append(conditions, own(manager.MkArithmeticLT(first, second)))
		case ">=":
			conditions = append(conditions, own(manager.MkArithmeticGE(first, second)))
		default:
			t.Fatal("unknown test guard", guard.op)
		}
	}
	if originalOrder {
		conditions = append(conditions, application(p, left), application(q, right))
	} else {
		conditions = append(conditions, application(q, right), application(p, left))
	}
	body := own(manager.MkAndArgs(conditions))
	formula := own(manager.MkImplies(body, application(h, head)))
	fragmentMust(t, engine.AddRule(formula, "cartesian-head-constants-rule"))
	return manager, engine, callbacks, p, q, h, sort
}

func externalIntervalHeadConstantOperations(operations []int32) []int32 {
	var result []int32
	for _, operation := range operations {
		switch operation {
		case ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion, ast.DatalogOpStore, ast.DatalogOpComplement:
			result = append(result, operation)
		}
	}
	return result
}

func TestExternalIntervalDatalogCartesianHeadConstantsOriginalShape(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			// Exact old constant-head boundary: H(x,1) <- P(x),Q(1),
			// with MkAnd(P(x),Q(1)) and actual P(2),Q(1) facts.
			_, engine, callbacks, p, q, h, _ := newExternalIntervalHeadConstantsFixture(t, kinds,
				[]int{0}, []int{-4}, []int{0, -4}, []string{"2"}, []string{"1"}, false, true, nil)
			resetExternalIntervalUnaryGuardCallbacks(callbacks)
			status, err := engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef || !reflect.DeepEqual(callbacks.constantColumns, []uint32{0}) || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{0}}) || len(callbacks.renameParameters) != 0 || !reflect.DeepEqual(callbacks.joinParameters, [][]uint32{{}, {}}) {
				t.Fatal("original constant-head source columns", status, callbacks.constantColumns, callbacks.projectParameters, callbacks.renameParameters, callbacks.joinParameters)
			}
			if plan := engine.externalIntervalDatalog.cartesian; plan.left != q || plan.right != p {
				t.Fatal("original constant-head normalized order", plan)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalHeadConstantOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpFilter, ast.DatalogOpProject,
				ast.DatalogOpStore, ast.DatalogOpJoin, ast.DatalogOpUnion,
			})
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 0, "2", "2")
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 1, "1", "1")
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, 0, "2", "2")
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, "1", "1")
		})
	}
}

func TestExternalIntervalDatalogCartesianHeadConstantsAssemblyAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, real := range []bool{false, true} {
			for _, shape := range []string{"interleaved", "all-constant-head", "constants-guard-repeated"} {
				t.Run(fmt.Sprintf("%s/real=%t/%s", kinds[0], real, shape), func(t *testing.T) {
					x, y, z := "1", "5", "3"
					if real {
						x, y, z = "1/2", "5/2", "3/2"
					}
					left, right, head := []int{0}, []int{1}, []int{-1, 1, 0, -2, -1}
					leftFacts, rightFacts := []string{x}, []string{y}
					values := []string{"7", y, x, "9", "7"}
					var guards []externalIntervalUnaryGuardSpec
					var conditions []string
					var projects [][]uint32
					var columns []uint32
					joins := [][]uint32{{}, {}, {}, {}}
					renames := [][]uint32{{0, 2}}
					want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpStore, ast.DatalogOpJoin,
						ast.DatalogOpStore, ast.DatalogOpJoin, ast.DatalogOpJoin, ast.DatalogOpRename, ast.DatalogOpUnion}
					stores := 2
					switch shape {
					case "all-constant-head":
						head, values = []int{-1, -1, -2}, []string{"7", "7", "9"}
						projects, renames = [][]uint32{{0, 1}}, nil
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject,
							ast.DatalogOpStore, ast.DatalogOpJoin, ast.DatalogOpJoin, ast.DatalogOpStore, ast.DatalogOpJoin, ast.DatalogOpUnion}
					case "constants-guard-repeated":
						left, right, head = []int{-1, 0, 2}, []int{2, 3, 1, -2}, []int{-3, 1, 0, 0, -3}
						leftFacts, rightFacts = []string{"7", x, "13"}, []string{"13", z, y, "9"}
						values = []string{"11", y, x, x, "11"}
						guards, conditions = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}}, []string{"(< v1 v2)"}
						columns = []uint32{0, 4}
						projects = [][]uint32{{2, 3}, {2}, {0, 3}, {1, 2, 3}}
						joins, renames = [][]uint32{{2, 0}, {}, {}, {0, 0}}, [][]uint32{{0, 2}, {3, 4}}
						stores = 1
						want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpFilter, ast.DatalogOpFilter,
							ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpProject,
							ast.DatalogOpStore, ast.DatalogOpJoin, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpJoin,
							ast.DatalogOpRename, ast.DatalogOpRename, ast.DatalogOpUnion}
					}
					_, engine, callbacks, p, q, h, _ := newExternalIntervalHeadConstantsFixture(t, kinds, left, right, head, leftFacts, rightFacts, real, false, guards)
					for run := 0; run < 2; run++ {
						resetExternalIntervalUnaryGuardCallbacks(callbacks)
						status, err := engine.QueryPredicate(h)
						fragmentMust(t, err)
						if status != lbool.Undef || !reflect.DeepEqual(callbacks.joinParameters, joins) || !reflect.DeepEqual(callbacks.projectParameters, projects) ||
							!reflect.DeepEqual(callbacks.renameParameters, renames) || !reflect.DeepEqual(callbacks.guardConditions, conditions) || !reflect.DeepEqual(callbacks.constantColumns, columns) || operationCount(callbacks.operations, ast.DatalogOpStore) != stores {
							t.Fatal("head constant assembly columns", status, callbacks.joinParameters, callbacks.projectParameters, callbacks.renameParameters, callbacks.guardConditions, callbacks.constantColumns, callbacks.operations)
						}
						expected := append([]int32(nil), want...)
						if run > 0 {
							expected = append([]int32{ast.DatalogOpClone}, expected...)
						}
						assertExternalIntervalRegisterOperations(t, externalIntervalHeadConstantOperations(callbacks.operations), expected)
						first, same := 1, 3
						if shape != "interleaved" {
							same = 2
						}
						if callbacks.joinArguments[first][1] != callbacks.joinArguments[same][1] {
							t.Fatal("equal head numerals did not share the singleton", callbacks.joinArguments)
						}
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

func TestExternalIntervalDatalogCartesianHeadConstantsEmptyProducerStillInitializes(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, shape := range []string{"empty-source", "disjoint-join", "tail-constant", "guard"} {
			t.Run(kinds[0]+"/"+shape, func(t *testing.T) {
				left, right := []int{0}, []int{1}
				leftFacts, rightFacts := []string{"1"}, []string{"5"}
				var guards []externalIntervalUnaryGuardSpec
				want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject}
				switch shape {
				case "empty-source":
					leftFacts, want = nil, []int32{ast.DatalogOpClone}
				case "disjoint-join":
					right = []int{0}
				case "tail-constant":
					left, leftFacts = []int{-4, 0}, []string{"2", "1"}
					want = append(want, ast.DatalogOpFilter)
				case "guard":
					leftFacts, rightFacts = []string{"5"}, []string{"1"}
					guards = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 1}}
					want = []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject}
				}
				_, engine, callbacks, _, _, h, _ := newExternalIntervalHeadConstantsFixture(t, kinds, left, right, []int{-1, -1, -2}, leftFacts, rightFacts, false, false, guards)
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(h)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("empty producer acquired rows from a head singleton", status)
				}
				// Unary-singleton instructions do not depend on the empty source
				// register; only their subsequent append JOINs are skipped.
				want = append(want, ast.DatalogOpStore, ast.DatalogOpStore)
				assertExternalIntervalRegisterOperations(t, externalIntervalHeadConstantOperations(callbacks.operations), want)
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianHeadConstantsEmptyFirstUseSharedCache(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			manager, engine, callbacks, p, _, emptyHead, sort := newExternalIntervalHeadConstantsFixture(t, kinds,
				[]int{0}, []int{1}, []int{0, -1}, []string{"1"}, nil, false, false, nil)
			otherKinds := []string{kinds[1], kinds[0]}
			r := declareExternalIntervalPredicate(t, manager, engine, "head-cache-intermediate", []ast.ASTHandle{sort}, otherKinds)
			result := declareExternalIntervalPredicate(t, manager, engine, "head-cache-result", []ast.ASTHandle{sort, sort}, otherKinds)
			addExternalIntervalIdentityRule(t, manager, engine, p, r, []ast.ASTHandle{sort}, "head-cache-intermediate-rule")
			x, err := manager.MkVar(0, sort)
			fragmentMust(t, err)
			y, err := manager.MkVar(1, sort)
			fragmentMust(t, err)
			left, err := manager.MkApp(p, []ast.ASTHandle{x})
			fragmentMust(t, err)
			right, err := manager.MkApp(r, []ast.ASTHandle{y})
			fragmentMust(t, err)
			body, err := manager.MkAnd(right, left)
			fragmentMust(t, err)
			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			head, err := manager.MkApp(result, []ast.ASTHandle{x, seven})
			fragmentMust(t, err)
			formula, err := manager.MkImplies(body, head)
			fragmentMust(t, err)
			fragmentMust(t, engine.AddRule(formula, "head-cache-result-rule"))
			var createdKinds [][]string
			installed := callbacks.callbacks()
			assign := installed.ReduceAssign
			installed.ReduceAssign = func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
				operation, cause := manager.GetDeclKind(declaration)
				if cause != nil {
					return cause
				}
				if operation == ast.DatalogOpStore {
					registers := engine.externalIntervalDatalog.program.registers
					if registers == nil || registers.constants == nil || len(registers.constants.owned) == 0 {
						return fmt.Errorf("head singleton was not owned before STORE")
					}
					created := registers.constants.owned[len(registers.constants.owned)-1]
					createdKinds = append(createdKinds, created.Kinds())
				}
				return assign(declaration, arguments, outputs)
			}
			fragmentMust(t, engine.SetExternalRelationCallbacks(installed))
			for run := 0; run < 2; run++ {
				createdKinds = nil
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(result)
				fragmentMust(t, err)
				if status != lbool.Undef || !reflect.DeepEqual(createdKinds, [][]string{kinds}) || operationCount(callbacks.operations, ast.DatalogOpStore) != 1 || operationCount(callbacks.operations, ast.DatalogOpJoin) != 2 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 {
					t.Fatal("empty first head did not own the shared singleton cache", status, createdKinds, callbacks.operations)
				}
				storeAt, joinAt := -1, -1
				for index, operation := range callbacks.operations {
					if operation == ast.DatalogOpStore {
						storeAt = index
					}
					if operation == ast.DatalogOpJoin && joinAt < 0 {
						joinAt = index
					}
				}
				if storeAt < 0 || storeAt >= joinAt {
					t.Fatal("empty first rule deferred its singleton until the live consumer", callbacks.operations)
				}
				empty, err := engine.externalIntervalDatalog.entries[emptyHead].product.Empty()
				fragmentMust(t, err)
				if !empty {
					t.Fatal("empty first head acquired singleton rows")
				}
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[result].product, 0, "1", "1")
				assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[result].product, 1, "7", "7")
			}
		})
	}
}

func TestExternalIntervalDatalogCartesianHeadConstantsProgramRefusalAndRetry(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, stop := range []struct {
			name string
			op   int32
			at   int
		}{{"later-singleton", ast.DatalogOpStore, 2}, {"first-append", ast.DatalogOpJoin, 2}, {"cached-append", ast.DatalogOpJoin, 4}} {
			t.Run(kinds[0]+"/"+stop.name, func(t *testing.T) {
				manager, engine, callbacks, p, q, h, sort := newExternalIntervalHeadConstantsFixture(t, kinds,
					[]int{0}, []int{1}, []int{-1, 0, -2, -1}, []string{"1"}, []string{"5"}, false, false, nil)
				sorts := []ast.ASTHandle{sort, sort, sort, sort}
				r := declareExternalIntervalPredicate(t, manager, engine, "head-constants-consumer", sorts, kinds)
				addExternalIntervalIdentityRule(t, manager, engine, h, r, sorts, "head-constants-consumer-rule")
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ, oldH, oldR := activation.entries[p].product, activation.entries[q].product, activation.entries[h].product, activation.entries[r].product
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				refusal := errors.New("head constant callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = stop.op, stop.at, refusal
				status, err := engine.QueryPredicate(r)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[h].product != oldH || activation.entries[r].product != oldR || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("head constant failure published working registers", status, err, callbacks.operations)
				}
				callbacks.refusal = nil
				for _, predicate := range []ast.ASTHandle{h, r} {
					empty, cause := activation.entries[predicate].product.Empty()
					fragmentMust(t, cause)
					if !empty {
						t.Fatal("head constant failure populated a derived relation")
					}
				}
				assertExternalIntervalGroundProgramInterval(t, oldP, 0, "1", "1")
				assertExternalIntervalGroundProgramInterval(t, oldQ, 0, "5", "5")
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err = engine.QueryPredicate(r)
				fragmentMust(t, err)
				if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpStore) != 2 || operationCount(callbacks.operations, ast.DatalogOpJoin) != 4 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 {
					t.Fatal("head constant program retry", status, callbacks.operations)
				}
				for column, value := range []string{"7", "1", "9", "7"} {
					assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, column, value, value)
				}
			})
		}
	}
}
