package fp

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newExternalIntervalPositiveFixture(t *testing.T, kinds []string, leftVariables, rightVariables, headVariables []uint32, mixed bool, rightKey string) (*ast.ASTManager, *PropositionalEngine, *externalIntervalCallbackState, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	real, err := manager.MkFamilySort(family, 0, nil)
	fragmentMust(t, err)
	variableSorts := []ast.ASTHandle{integer, integer, integer, integer}
	if mixed {
		variableSorts[2] = real
	}
	selectSorts := func(variables []uint32) []ast.ASTHandle {
		result := make([]ast.ASTHandle, len(variables))
		for i, variable := range variables {
			result[i] = variableSorts[variable]
		}
		return result
	}
	p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-positive-p", selectSorts(leftVariables), kinds)
	q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-positive-q", selectSorts(rightVariables), kinds)
	h := declareExternalIntervalPredicate(t, manager, engine, "external-interval-positive-h", selectSorts(headVariables), kinds)
	addFact := func(predicate ast.ASTHandle, variables []uint32, right bool) {
		values := make([]ast.ASTHandle, len(variables))
		texts := []string{"1", "7", "9", "11"}
		for i, variable := range variables {
			value := texts[variable]
			if right && variable == 1 && rightKey != "" {
				value = rightKey
			}
			if variableSorts[variable] == real {
				values[i], err = manager.ArithmeticRealNumeralText(value)
			} else {
				values[i], err = manager.ArithmeticIntegerNumeralText(value)
			}
			fragmentMust(t, err)
		}
		fragmentMust(t, engine.AddFact(predicate, values))
	}
	addFact(p, leftVariables, false)
	addFact(q, rightVariables, true)
	application := func(predicate ast.ASTHandle, variables []uint32) ast.ASTHandle {
		arguments := make([]ast.ASTHandle, len(variables))
		for i, variable := range variables {
			arguments[i], err = manager.MkVar(variable, variableSorts[variable])
			fragmentMust(t, err)
		}
		result, cause := manager.MkApp(predicate, arguments)
		fragmentMust(t, cause)
		return result
	}
	left, right, head := application(p, leftVariables), application(q, rightVariables), application(h, headVariables)
	body, err := manager.MkAnd(left, right)
	fragmentMust(t, err)
	formula, err := manager.MkImplies(body, head)
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(formula, "external-interval-positive"))
	return manager, engine, callbacks, p, q, h
}

func TestExternalIntervalDatalogPositiveJoinProjectAndHeadAssembly(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, test := range []struct {
			name              string
			left, right, head []uint32
			join, removed     []uint32
			renames           [][]uint32
			mixed             bool
		}{
			{name: "shared-key-project-swap", left: []uint32{0, 1}, right: []uint32{1, 2}, head: []uint32{0, 2}, join: []uint32{0, 1}, removed: []uint32{0, 3}, renames: [][]uint32{{0, 1}}},
			{name: "shared-key-last-copy-mixed-cycle", left: []uint32{0, 1}, right: []uint32{1, 2}, head: []uint32{0, 1, 2}, join: []uint32{0, 1}, removed: []uint32{0}, renames: [][]uint32{{0, 1}, {1, 2}}, mixed: true},
			{name: "shared-key-last-copy-integer-cycle", left: []uint32{0, 1}, right: []uint32{1, 2}, head: []uint32{0, 1, 2}, join: []uint32{0, 1}, removed: []uint32{0}, renames: [][]uint32{{0, 1}, {1, 2}}},
			{name: "four-column-cycle", left: []uint32{0, 1}, right: []uint32{2, 3}, head: []uint32{3, 0, 1, 2}, renames: [][]uint32{{0, 1}, {1, 2}, {2, 3}}},
			{name: "two-keys", left: []uint32{0, 1, 2}, right: []uint32{2, 0, 3}, head: []uint32{3, 1, 0}, join: []uint32{0, 2, 1, 0}, removed: []uint32{0, 1, 5}, renames: [][]uint32{{1, 2}}},
			{name: "reordered-cartesian", left: []uint32{0}, right: []uint32{1}, head: []uint32{1, 0}},
			{name: "unary-head", left: []uint32{0}, right: []uint32{1, 2}, head: []uint32{0}, removed: []uint32{0, 1}},
			{name: "nullary-head", left: []uint32{0, 1}, right: []uint32{1, 2}, head: nil, join: []uint32{0, 1}, removed: []uint32{0, 1, 2, 3}},
		} {
			t.Run(kinds[0]+"/"+test.name, func(t *testing.T) {
				_, engine, callbacks, p, q, h := newExternalIntervalPositiveFixture(t, kinds, test.left, test.right, test.head, test.mixed, "")
				for run := 0; run < 2; run++ {
					resetExternalIntervalCartesianCallbacks(callbacks)
					status, err := engine.QueryPredicate(h)
					if err != nil {
						t.Fatal("positive query", err, callbacks.operations, callbacks.joinParameters, callbacks.projectParameters, callbacks.renameParameters)
					}
					if status != lbool.Undef {
						t.Fatal("positive rule result", status)
					}
					plan := engine.externalIntervalDatalog.cartesian
					if plan.left != q || plan.right != p {
						t.Fatal("JOIN changed normalized source tail order", plan)
					}
					if len(callbacks.joinParameters) != 1 || !externalIntervalColumnsEqual(callbacks.joinParameters[0], test.join) {
						t.Fatal("JOIN key pairs", callbacks.joinParameters, test.join)
					}
					var projects [][]uint32
					if len(test.removed) > 0 {
						projects = [][]uint32{test.removed}
					}
					if !reflect.DeepEqual(callbacks.projectParameters, projects) || !reflect.DeepEqual(callbacks.renameParameters, test.renames) {
						t.Fatal("source projection/rename columns", callbacks.projectParameters, callbacks.renameParameters, projects, test.renames)
					}
					want := []int32{}
					if run > 0 || kinds[0] == "external_relation" {
						want = append(want, ast.DatalogOpIsEmpty)
					}
					if run > 0 {
						want = append(want, ast.DatalogOpClone)
					}
					want = append(want, ast.DatalogOpIsEmpty, ast.DatalogOpClone, ast.DatalogOpIsEmpty, ast.DatalogOpClone, ast.DatalogOpJoin)
					if len(test.removed) > 0 {
						want = append(want, ast.DatalogOpProject)
					}
					want = append(want, ast.DatalogOpIsEmpty)
					for range test.renames {
						want = append(want, ast.DatalogOpRename)
					}
					if run == 0 {
						want = append(want, ast.DatalogOpEmpty)
					}
					want = append(want, ast.DatalogOpUnion, ast.DatalogOpIsEmpty)
					assertExternalIntervalRegisterOperations(t, callbacks.operations, want)
					var display bytes.Buffer
					fragmentMust(t, engine.externalIntervalDatalog.entries[h].product.Display(&display))
					values := []string{"1", "7", "9", "11"}
					for column, variable := range test.head {
						interval := fmt.Sprintf("%d in [%s, %s]", column, values[variable], values[variable])
						if !strings.Contains(display.String(), interval) {
							t.Fatal("head column value", interval, display.String())
						}
					}
				}
			})
		}
	}
}

func TestExternalIntervalDatalogPositiveDisjointKeysThenUpdate(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		t.Run(kinds[0], func(t *testing.T) {
			manager, engine, callbacks, _, q, h := newExternalIntervalPositiveFixture(t, kinds, []uint32{0, 1}, []uint32{1, 2}, []uint32{0, 2}, false, "8")
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err := engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.False || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpProject) != 1 || operationCount(callbacks.operations, ast.DatalogOpRename) != 0 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
				t.Fatal("disjoint shared key reached head assembly", status, callbacks.operations)
			}
			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			nine, err := manager.ArithmeticIntegerNumeralText("9")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{seven, nine}))
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err = engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("new matching key did not reach head", status, callbacks.operations)
			}
		})
	}
}

func TestExternalIntervalDatalogPositiveTransformRefusalRollback(t *testing.T) {
	for _, operation := range []int32{ast.DatalogOpProject, ast.DatalogOpRename} {
		t.Run(fmt.Sprint(operation), func(t *testing.T) {
			_, engine, callbacks, p, q, h := newExternalIntervalPositiveFixture(t, []string{"external_relation", "interval_relation"}, []uint32{0, 1}, []uint32{1, 2}, []uint32{0, 2}, false, "")
			selected, err := engine.ensureExternalIntervalDatalog()
			fragmentMust(t, err)
			if !selected {
				t.Fatal("positive rule was not selected")
			}
			activation := engine.externalIntervalDatalog
			oldP, oldQ, oldH := activation.entries[p].product, activation.entries[q].product, activation.entries[h].product
			resetExternalIntervalCartesianCallbacks(callbacks)
			refusal := errors.New("positive transformation refused")
			callbacks.refusalOperation, callbacks.refusalAt, callbacks.refusal = operation, 1, refusal
			status, queryErr := engine.QueryPredicate(h)
			if status != lbool.Undef || !errors.Is(queryErr, refusal) {
				t.Fatal("transformation refusal", status, queryErr)
			}
			if activation.entries[p].product != oldP || activation.entries[q].product != oldQ || activation.entries[h].product != oldH || operationCount(callbacks.operations, ast.DatalogOpUnion) != 0 {
				t.Fatal("transformation refusal published registers", callbacks.operations)
			}
			callbacks.refusal = nil
			resetExternalIntervalCartesianCallbacks(callbacks)
			status, err = engine.QueryPredicate(h)
			fragmentMust(t, err)
			if status != lbool.Undef || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpProject) != 1 || operationCount(callbacks.operations, ast.DatalogOpRename) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
				t.Fatal("transformation retry", status, callbacks.operations)
			}
		})
	}
}
