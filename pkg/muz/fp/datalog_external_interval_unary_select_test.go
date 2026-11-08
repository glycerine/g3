package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Negative tail entries are arithmetic constants; other entries are variable
// indices. Real marks a column's arithmetic sort independently of its value.
func newExternalIntervalUnarySelectFixture(t *testing.T, kinds []string, tail []int, head []uint32, constants, facts []string, realColumns []bool) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	real, err := manager.MkFamilySort(family, 0, nil)
	fragmentMust(t, err)
	sorts := make([]ast.ASTHandle, len(tail))
	variableSorts := make(map[uint32]ast.ASTHandle)
	for column, variable := range tail {
		sorts[column] = integer
		if len(realColumns) > column && realColumns[column] {
			sorts[column] = real
		}
		if variable >= 0 {
			variableSorts[uint32(variable)] = sorts[column]
		}
	}
	headSorts := make([]ast.ASTHandle, len(head))
	for column, variable := range head {
		headSorts[column] = variableSorts[variable]
	}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-select-p", sorts, kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-select-q", headSorts, kinds)
	var numeralRoots []ast.ASTRef
	defer func() {
		for i := len(numeralRoots) - 1; i >= 0; i-- {
			fragmentMust(t, manager.DelRef(&numeralRoots[i]))
		}
	}()
	numeral := func(column int, text string) ast.ASTHandle {
		var result ast.ASTHandle
		var cause error
		if sorts[column] == real {
			result, cause = manager.ArithmeticRealNumeralText(text)
		} else {
			result, cause = manager.ArithmeticIntegerNumeralText(text)
		}
		fragmentMust(t, cause)
		root, cause := manager.NewRef(result)
		fragmentMust(t, cause)
		numeralRoots = append(numeralRoots, root)
		return result
	}
	values := make([]ast.ASTHandle, len(facts))
	for column, text := range facts {
		values[column] = numeral(column, text)
	}
	fragmentMust(t, engine.AddFact(p, values))
	arguments := make([]ast.ASTHandle, len(tail))
	for column, variable := range tail {
		if variable < 0 {
			arguments[column] = numeral(column, constants[column])
		} else {
			arguments[column], err = manager.MkVar(uint32(variable), sorts[column])
			fragmentMust(t, err)
		}
	}
	body, err := manager.MkApp(p, arguments)
	fragmentMust(t, err)
	headArguments := make([]ast.ASTHandle, len(head))
	for column, variable := range head {
		headArguments[column], err = manager.MkVar(variable, headSorts[column])
		fragmentMust(t, err)
	}
	conclusion, err := manager.MkApp(q, headArguments)
	fragmentMust(t, err)
	formula, err := manager.MkImplies(body, conclusion)
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(formula, "external-interval-select-rule"))
	return manager, engine, callbacks, p, q, headSorts
}

func resetExternalIntervalUnarySelectCallbacks(callbacks *externalIntervalCallbackState) {
	resetExternalIntervalUnaryCallbacks(callbacks)
	callbacks.constantColumns = nil
	callbacks.constantValues = nil
	callbacks.constantSorts = nil
}

func TestExternalIntervalDatalogUnarySelectConstant(t *testing.T) {
	_, engine, callbacks, _, q, _ := newExternalIntervalUnarySelectFixture(t,
		[]string{"external_relation", "interval_relation"}, []int{-1, 0}, []uint32{0},
		[]string{"7", ""}, []string{"7", "9"}, nil)
	resetExternalIntervalUnarySelectCallbacks(callbacks)
	status, err := engine.QueryPredicate(q)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("constant selection result", status)
	}
	assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
		ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
	})
	assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[q].product, 0, "9", "9")
}

func TestExternalIntervalDatalogUnarySelectShrinkingColumnsAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, test := range []struct {
			name          string
			tail          []int
			head          []uint32
			constants     []string
			facts         []string
			reals         []bool
			selected      []uint32
			selectedValue []string
			selectedReal  []bool
			identical     [][]uint32
			projects      [][]uint32
			renames       [][]uint32
			want          []string
		}{
			{name: "shrinking-columns", tail: []int{-1, 0, -1, 1, -1}, head: []uint32{1, 0}, constants: []string{"7", "", "9", "", "7"}, facts: []string{"7", "2", "9", "3", "7"}, selected: []uint32{0, 1, 2}, selectedValue: []string{"7", "9", "7"}, selectedReal: []bool{false, false, false}, projects: [][]uint32{{0}, {1}, {2}}, renames: [][]uint32{{0, 1}}, want: []string{"3", "2"}},
			{name: "repeated-after-selection", tail: []int{-1, 0, 0}, head: []uint32{0}, constants: []string{"7", "", ""}, facts: []string{"7", "9", "9"}, selected: []uint32{0}, selectedValue: []string{"7"}, selectedReal: []bool{false}, identical: [][]uint32{{0, 1}}, projects: [][]uint32{{0}, {0}}, want: []string{"9"}},
			{name: "mixed-sorts", tail: []int{-1, 0, -1, 1}, head: []uint32{1, 0}, constants: []string{"7", "", "7/2", ""}, facts: []string{"7", "2", "7/2", "9/2"}, reals: []bool{false, false, true, true}, selected: []uint32{0, 1}, selectedValue: []string{"7", "7/2"}, selectedReal: []bool{false, true}, projects: [][]uint32{{0}, {1}}, renames: [][]uint32{{0, 1}}, want: []string{"9/2", "2"}},
			{name: "all-constant-nullary-head", tail: []int{-1, -1}, constants: []string{"7", "9"}, facts: []string{"7", "9"}, selected: []uint32{0, 0}, selectedValue: []string{"7", "9"}, selectedReal: []bool{false, false}, projects: [][]uint32{{0}, {0}}},
		} {
			t.Run(kinds[0]+"/"+test.name, func(t *testing.T) {
				manager, engine, callbacks, p, q, headSorts := newExternalIntervalUnarySelectFixture(t, kinds, test.tail, test.head, test.constants, test.facts, test.reals)
				family, err := manager.RegisterArithmeticPlugin()
				fragmentMust(t, err)
				integer, err := manager.MkFamilySort(family, 1, nil)
				fragmentMust(t, err)
				real, err := manager.MkFamilySort(family, 0, nil)
				fragmentMust(t, err)
				for run := 0; run < 2; run++ {
					resetExternalIntervalUnarySelectCallbacks(callbacks)
					status, err := engine.QueryPredicate(q)
					fragmentMust(t, err)
					if status != lbool.Undef {
						t.Fatal("selection result", status)
					}
					if !reflect.DeepEqual(callbacks.constantColumns, test.selected) || !reflect.DeepEqual(callbacks.constantValues, test.selectedValue) ||
						!reflect.DeepEqual(callbacks.filterColumns, test.identical) || !reflect.DeepEqual(callbacks.projectParameters, test.projects) || !reflect.DeepEqual(callbacks.renameParameters, test.renames) {
						t.Fatal("selection source operands", callbacks.constantColumns, callbacks.constantValues, callbacks.filterColumns, callbacks.projectParameters, callbacks.renameParameters)
					}
					for column, isReal := range test.selectedReal {
						wantSort := integer
						if isReal {
							wantSort = real
						}
						if callbacks.constantSorts[column] != wantSort {
							t.Fatal("selection changed the numeral sort", column)
						}
					}
					wantOps := []int32{ast.DatalogOpClone}
					if run != 0 {
						wantOps = append(wantOps, ast.DatalogOpClone)
					}
					for range test.selected {
						wantOps = append(wantOps, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject)
					}
					for range test.identical {
						wantOps = append(wantOps, ast.DatalogOpFilter)
					}
					for range test.projects[len(test.selected):] {
						wantOps = append(wantOps, ast.DatalogOpProject)
					}
					for range test.renames {
						wantOps = append(wantOps, ast.DatalogOpRename)
					}
					wantOps = append(wantOps, ast.DatalogOpUnion)
					assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), wantOps)
					selectedProjects := 0
					for index, operation := range callbacks.operations {
						if operation == ast.DatalogOpProject && selectedProjects < len(test.selected) {
							if index+1 == len(callbacks.operations) || callbacks.operations[index+1] != ast.DatalogOpIsEmpty {
								t.Fatal("selection omitted its post-projection empty check", callbacks.operations)
							}
							selectedProjects++
						}
					}
					product := engine.externalIntervalDatalog.entries[q].product
					sorts, err := product.Sorts()
					fragmentMust(t, err)
					if len(sorts) != len(headSorts) {
						t.Fatal("selection output arity", sorts)
					}
					for column, value := range test.want {
						if sorts[column] != headSorts[column] {
							t.Fatal("selection changed output sort", column)
						}
						assertExternalIntervalGroundProgramInterval(t, product, column, value, value)
					}
					for column, value := range test.facts {
						assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
					}
				}
			})
		}
	}
}

func TestExternalIntervalDatalogUnarySelectContradictionStillProjects(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, later := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/later=%t", kinds[0], later), func(t *testing.T) {
				facts := []string{"8", "2", "9"}
				selections := 2
				if later {
					facts = []string{"7", "2", "10"}
				}
				_, engine, callbacks, p, q, _ := newExternalIntervalUnarySelectFixture(t, kinds, []int{-1, 0, -1}, []uint32{0}, []string{"7", "", "9"}, facts, nil)
				resetExternalIntervalUnarySelectCallbacks(callbacks)
				status, err := engine.QueryPredicate(q)
				fragmentMust(t, err)
				// The source interval projection drops the empty flag after the
				// contradictory selection, so the query result is imprecise.
				if status != lbool.Undef {
					t.Fatal("empty-source projection did not match C++ behavior", status)
				}
				projected := engine.externalIntervalDatalog.entries[q].product
				if empty, cause := projected.Empty(); cause != nil || empty {
					t.Fatal("contradictory projection did not preserve the source reset payload", empty, cause)
				}
				want := []int32{ast.DatalogOpClone}
				for range selections {
					want = append(want, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject)
				}
				want = append(want, ast.DatalogOpUnion)
				assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), want)
				for column, value := range facts {
					assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[p].product, column, value, value)
				}
			})
		}
	}
}

func TestExternalIntervalDatalogUnarySelectCallbackRefusalRollback(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, operation := range []int32{ast.DatalogOpFilter, ast.DatalogOpProject} {
			t.Run(fmt.Sprintf("%s/operation-%d", kinds[0], operation), func(t *testing.T) {
				_, engine, callbacks, p, q, _ := newExternalIntervalUnarySelectFixture(t, kinds, []int{-1, 0, -1}, []uint32{0}, []string{"7", "", "9"}, []string{"7", "2", "9"}, nil)
				_, err := engine.ensureExternalIntervalDatalog()
				fragmentMust(t, err)
				activation := engine.externalIntervalDatalog
				oldP, oldQ := activation.entries[p].product, activation.entries[q].product
				resetExternalIntervalUnarySelectCallbacks(callbacks)
				refusal := errors.New("second selection callback refused")
				callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = operation, 2, refusal
				status, err := engine.QueryPredicate(q)
				if status != lbool.Undef || !errors.Is(err, refusal) || activation.entries[p].product != oldP || activation.entries[q].product != oldQ || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
					t.Fatal("selection refusal published partial work", status, err, callbacks.operations)
				}
				for column, value := range []string{"7", "2", "9"} {
					assertExternalIntervalGroundProgramInterval(t, oldP, column, value, value)
				}
				callbacks.refusal = nil
				empty, err := oldQ.Empty()
				fragmentMust(t, err)
				if !empty {
					t.Fatal("selection refusal populated target")
				}
				resetExternalIntervalUnarySelectCallbacks(callbacks)
				status, err = engine.QueryPredicate(q)
				fragmentMust(t, err)
				if status != lbool.Undef {
					t.Fatal("selection retry result", status)
				}
				assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
					ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject,
					ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion,
				})
				assertExternalIntervalGroundProgramInterval(t, activation.entries[q].product, 0, "2", "2")
			})
		}
	}
}

func TestExternalIntervalDatalogUnarySelectProgramComposition(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			manager, engine, callbacks, _, q, sorts := newExternalIntervalUnarySelectFixture(t, kinds, []int{-1, 0}, []uint32{0}, []string{"7", ""}, []string{"7", "9"}, nil)
			r := declareExternalIntervalPredicate(t, manager, engine, "external-interval-select-r", sorts, kinds)
			addExternalIntervalIdentityRule(t, manager, engine, q, r, sorts, "external-interval-select-consumer")
			_, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			resetExternalIntervalUnarySelectCallbacks(callbacks)
			status, err := engine.QueryPredicate(r)
			fragmentMust(t, err)
			if status != lbool.Undef {
				t.Fatal("selection program result", status)
			}
			assertExternalIntervalRegisterOperations(t, externalIntervalUnaryOperations(callbacks.operations), []int32{
				ast.DatalogOpClone, ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpUnion, ast.DatalogOpUnion,
			})
			assertExternalIntervalGroundProgramInterval(t, engine.externalIntervalDatalog.entries[r].product, 0, "9", "9")
		})
	}
}
