package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Negative terms denote the numerals 7,9,11; nonnegative terms are variables.
// The conjunction deliberately places q before p so normalized JOIN order is p,q.
func newExternalIntervalCartesianConstantsFixture(t *testing.T, kinds []string, left, right []int, head []uint32, leftFacts, rightFacts []string, real bool, guards []externalIntervalUnaryGuardSpec) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
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
	p := declareExternalIntervalPredicate(t, manager, engine, "cartesian-constants-p", sorts(len(left)), kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "cartesian-constants-q", sorts(len(right)), kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "cartesian-constants-h", sorts(len(head)), kinds)
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
				arguments[i] = numeral([]string{"7", "9", "11"}[-term-1])
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
	conditions = append(conditions, application(q, right), application(p, left))
	body := own(manager.MkAndArgs(conditions))
	headTerms := make([]int, len(head))
	for i, variable := range head {
		headTerms[i] = int(variable)
	}
	formula := own(manager.MkImplies(body, application(h, headTerms)))
	fragmentMust(t, engine.addRuleEager(formula, "cartesian-constants-rule"))
	return manager, engine, callbacks, p, q, h, sort
}

func TestExternalIntervalDatalogCartesianConstantsSharedKey(t *testing.T) {
	_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianConstantsFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{-1, 0, 1, -2, 4}, []int{1, -1, 3, -3, 2, -2}, []uint32{2, 0},
		[]string{"7", "1", "5", "9", "99"}, []string{"5", "7", "3", "11", "13", "9"}, false, nil)
	resetExternalIntervalUnaryGuardCallbacks(callbacks)
	status, err := engine.QueryPredicate(h)
	fragmentMust(t, err)
	if status != lbool.Undef || !reflect.DeepEqual(callbacks.joinParameters, [][]uint32{{2, 0}}) || !reflect.DeepEqual(callbacks.constantColumns, []uint32{0, 2, 3, 4, 6}) ||
		!reflect.DeepEqual(callbacks.constantValues, []string{"7", "9", "7", "11", "9"}) || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{2, 4, 5, 7}, {0, 2, 3, 4, 6}}) {
		t.Fatal("constant join or fixed filter columns", status, callbacks.joinParameters, callbacks.constantColumns, callbacks.constantValues, callbacks.projectParameters)
	}
	if plan := engine.externalIntervalDatalog.cartesian; plan.left != p || plan.right != q {
		t.Fatal("constant-tail normalized order", plan)
	}
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 0, "13", "13")
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 1, "1", "1")
}

func TestExternalIntervalDatalogCartesianConstantsGuardColumnsAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, real := range []bool{false, true} {
			for _, shape := range []string{"plain", "head-bound-guard", "guard-only-column"} {
				t.Run(fmt.Sprintf("%s/real=%t/%s", kinds[0], real, shape), func(t *testing.T) {
					x, z, y := "1", "3", "13"
					if real {
						x, z, y = "1/2", "3/2", "13/2"
					}
					var guards []externalIntervalUnaryGuardSpec
					var condition []string
					projects := [][]uint32{{2, 4, 5, 7}, {0, 2, 3, 4, 6}}
					constantColumns := []uint32{0, 2, 3, 4, 6}
					if shape == "head-bound-guard" {
						guards = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 2}}
						condition = []string{"(< v1 v5)"}
					} else if shape == "guard-only-column" {
						guards = []externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}}
						condition = []string{"(< v1 v4)"}
						projects = [][]uint32{{2, 4, 5}, {4}, {0, 2, 3, 4, 6}}
						constantColumns = []uint32{0, 2, 3, 5, 7}
					}
					leftFacts, rightFacts := []string{"7", x, "5", "9", "99"}, []string{"5", "7", z, "11", y, "9"}
					_, engine, callbacks, p, q, h, sort := newExternalIntervalCartesianConstantsFixture(t, kinds,
						[]int{-1, 0, 1, -2, 4}, []int{1, -1, 3, -3, 2, -2}, []uint32{2, 0}, leftFacts, rightFacts, real, guards)
					for run := 0; run < 2; run++ {
						resetExternalIntervalUnaryGuardCallbacks(callbacks)
						status, err := engine.QueryPredicate(h)
						fragmentMust(t, err)
						if status != lbool.Undef || !reflect.DeepEqual(callbacks.joinParameters, [][]uint32{{2, 0}}) || !reflect.DeepEqual(callbacks.constantColumns, constantColumns) ||
							!reflect.DeepEqual(callbacks.constantValues, []string{"7", "9", "7", "11", "9"}) || !reflect.DeepEqual(callbacks.guardConditions, condition) ||
							!reflect.DeepEqual(callbacks.projectParameters, projects) || !reflect.DeepEqual(callbacks.renameParameters, [][]uint32{{0, 1}}) {
							t.Fatal("constant/guard projected columns", status, callbacks.joinParameters, callbacks.constantColumns, callbacks.constantValues, callbacks.guardConditions, callbacks.projectParameters, callbacks.renameParameters)
						}
						for _, actualSort := range callbacks.constantSorts {
							if actualSort != sort {
								t.Fatal("constant lost its arithmetic sort", actualSort, sort)
							}
						}
						want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject}
						if run > 0 {
							want = append([]int32{ast.DatalogOpClone}, want...)
						}
						for range constantColumns {
							want = append(want, ast.DatalogOpFilter)
						}
						if shape == "guard-only-column" {
							want = append(want, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject)
						} else if shape == "head-bound-guard" {
							want = append(want, ast.DatalogOpFilter)
						}
						want = append(want, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion)
						assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), want)
						// Plain PROJECT has no source fast_empty check, unlike
						// JOIN_PROJECT and FILTER(_AND_PROJECT) instructions.
						lastProject := -1
						for i, operation := range callbacks.operations {
							if operation == ast.DatalogOpProject {
								lastProject = i
							}
						}
						if lastProject < 0 || lastProject+1 >= len(callbacks.operations) || callbacks.operations[lastProject+1] != ast.DatalogOpRename {
							t.Fatal("head PROJECT inserted an Empty callback", callbacks.operations)
						}
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 0, y, y)
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 1, x, x)
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

func TestExternalIntervalDatalogCartesianConstantsAreNotJoinKeys(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, nullary := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nullary=%t", kinds[0], nullary), func(t *testing.T) {
				left, right, head := []int{-1, 0}, []int{-1, 1}, []uint32{0, 1}
				leftFacts, rightFacts := []string{"7", "1"}, []string{"7", "3"}
				columns, values := []uint32{0, 2}, []string{"7", "7"}
				if nullary {
					left, right, head = []int{-1, -2}, []int{-1, -3}, nil
					leftFacts, rightFacts = []string{"7", "9"}, []string{"7", "11"}
					columns, values = []uint32{0, 1, 2, 3}, []string{"7", "9", "7", "11"}
				}
				_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianConstantsFixture(t, kinds, left, right, head, leftFacts, rightFacts, false, nil)
				for run := 0; run < 2; run++ {
					resetExternalIntervalUnaryGuardCallbacks(callbacks)
					status, err := engine.QueryPredicate(h)
					fragmentMust(t, err)
					if status != lbool.Undef || len(callbacks.joinParameters) != 1 || len(callbacks.joinParameters[0]) != 0 || !reflect.DeepEqual(callbacks.constantColumns, columns) ||
						!reflect.DeepEqual(callbacks.constantValues, values) || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{columns}) || len(callbacks.renameParameters) != 0 || operationCount(callbacks.operations, ast.DatalogOpComplement) != 0 {
						t.Fatal("shared constants became keys or nullary TOTAL", status, callbacks.joinParameters, callbacks.constantColumns, callbacks.constantValues, callbacks.projectParameters, callbacks.operations)
					}
					want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin}
					if run > 0 {
						want = append([]int32{ast.DatalogOpClone}, want...)
					}
					for range columns {
						want = append(want, ast.DatalogOpFilter)
					}
					want = append(want, ast.DatalogOpProject, ast.DatalogOpUnion)
					assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), want)
					sorts, err := engine.externalIntervalDatalog.entries[h].product.Sorts()
					fragmentMust(t, err)
					if len(sorts) != len(head) {
						t.Fatal("constant-tail head arity", sorts)
					}
					if !nullary {
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 0, "1", "1")
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[h].product, 1, "3", "3")
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

func TestExternalIntervalDatalogCartesianConstantsEmptyFrontiers(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, shape := range []string{"join", "first-constant", "middle-constant", "last-constant", "guard"} {
			t.Run(kinds[0]+"/"+shape, func(t *testing.T) {
				leftFacts, rightFacts := []string{"7", "1", "5", "9", "99"}, []string{"5", "7", "3", "11", "13", "9"}
				count := 0
				switch shape {
				case "join":
					rightFacts[0], count = "6", 5
				case "first-constant":
					leftFacts[0], count = "8", 1
				case "middle-constant":
					rightFacts[1], count = "8", 3
				case "last-constant":
					rightFacts[5], count = "8", 5
				case "guard":
					leftFacts[1], count = "4", 5
				}
				_, engine, callbacks, p, q, h, _ := newExternalIntervalCartesianConstantsFixture(t, kinds,
					[]int{-1, 0, 1, -2, 4}, []int{1, -1, 3, -3, 2, -2}, []uint32{2, 0}, leftFacts, rightFacts, false,
					[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}})
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err := engine.QueryPredicate(h)
				fragmentMust(t, err)
				// Source vector_relation::mk_project drops the empty flag when
				// constructing its full destination. The later constant filters
				// therefore still run, including constants after the contradictory
				// one. Preserve all filter callbacks as a regression of that order.
				wantConstantColumns := []uint32{0, 2, 3, 5, 7}[:count]
				wantConstantValues := []string{"7", "9", "7", "11", "9"}[:count]
				wantStatus := lbool.False
				if shape == "join" || shape == "guard" {
					wantStatus = lbool.Undef
				}
				if status != wantStatus || !reflect.DeepEqual(callbacks.constantColumns, wantConstantColumns) || !reflect.DeepEqual(callbacks.constantValues, wantConstantValues) {
					t.Fatal("projected empty source did not continue constant filtering", status, callbacks.constantColumns, callbacks.constantValues)
				}
				want := []int32{ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpJoin, ast.DatalogOpProject}
				for range count {
					want = append(want, ast.DatalogOpFilter)
				}
				projects := [][]uint32{{2, 4, 5}}
				if shape == "guard" {
					want = append(want, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion)
					projects = append(projects, []uint32{4}, []uint32{0, 2, 3, 4, 6})
				} else if shape == "join" {
					want = append(want, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpUnion)
					projects = append(projects, []uint32{4}, []uint32{0, 2, 3, 4, 6})
				} else if len(callbacks.guardConditions) != 0 {
					t.Fatal("constant contradiction reached interpreted guard", callbacks.guardConditions)
				}
				if !reflect.DeepEqual(callbacks.projectParameters, projects) {
					t.Fatal("constant/guard contradiction projection frontier", callbacks.projectParameters, projects)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalCartesianGuardOperations(callbacks.operations), want)
				for column, value := range leftFacts {
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
				}
				for column, value := range rightFacts {
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, column, value, value)
				}
			})
		}
	}
}

func TestExternalIntervalDatalogCartesianConstantsProgramRefusalAndRetry(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, stop := range []struct {
			name string
			op   int32
			at   int
		}{{"first-constant", ast.DatalogOpFilter, 1}, {"last-constant", ast.DatalogOpFilter, 5}, {"head-project", ast.DatalogOpProject, 3}} {
			t.Run(kinds[0]+"/"+stop.name, func(t *testing.T) {
				manager, engine, callbacks, p, q, h, sort := newExternalIntervalCartesianConstantsFixture(t, kinds,
					[]int{-1, 0, 1, -2, 4}, []int{1, -1, 3, -3, 2, -2}, []uint32{2, 0},
					[]string{"7", "1", "5", "9", "99"}, []string{"5", "7", "3", "11", "13", "9"}, false,
					[]externalIntervalUnaryGuardSpec{{op: "<", left: 0, right: 3}})
				r := declareExternalIntervalPredicate(t, manager, engine, "cartesian-constants-consumer", []ast.ASTHandle{sort, sort}, kinds)
				addExternalIntervalIdentityRule(t, manager, engine, h, r, []ast.ASTHandle{sort, sort}, "cartesian-constants-consumer-rule")
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ, oldH, oldR := activation.entries[p].product, activation.entries[q].product, activation.entries[h].product, activation.entries[r].product
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				refusal := errors.New("two-tail constant callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = stop.op, stop.at, refusal
				status, err := engine.QueryPredicate(r)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[h].product != oldH || activation.entries[r].product != oldR || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("constant callback failure published working registers", status, err, callbacks.operations)
				}
				callbacks.refusal = nil
				for _, predicate := range []ast.ASTHandle{h, r} {
					empty, cause := activation.entries[predicate].product.Empty()
					fragmentMust(t, cause)
					if !empty {
						t.Fatal("failed constant program populated a derived relation")
					}
				}
				assertExternalIntervalGroundProgramInterval(t, oldP, 1, "1", "1")
				assertExternalIntervalGroundProgramInterval(t, oldQ, 4, "13", "13")
				resetExternalIntervalUnaryGuardCallbacks(callbacks)
				status, err = engine.QueryPredicate(r)
				fragmentMust(t, err)
				if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 || !reflect.DeepEqual(callbacks.projectParameters, [][]uint32{{2, 4, 5}, {4}, {0, 2, 3, 4, 6}}) {
					t.Fatal("constant program retry", status, callbacks.operations, callbacks.projectParameters)
				}
				assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 0, "13", "13")
				assertExternalIntervalGroundProgramInterval(t, activation.entries[r].product, 1, "1", "1")
			})
		}
	}
}
