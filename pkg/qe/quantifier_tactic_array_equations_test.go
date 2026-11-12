package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQuantifierArrayEquationsProjection(t *testing.T) {
	for _, name := range []string{"array-equality", "single-select", "two-selects", "store", "store-with-disequality", "nested-stores", "three-stores", "same-index-stores", "disjunction", "guarded-disjunction"} {
		t.Run(name, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			family, err := m.RegisterArrayPlugin()
			fmMust(t, err)
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("qe-array-a", sort)
			b := f.constant("qe-array-b", sort)
			c := f.constant("qe-array-c", sort)
			i, j := f.constant("qe-array-i", f.integer), f.constant("qe-array-j", f.integer)
			v, w := f.constant("qe-array-v", f.integer), f.constant("qe-array-w", f.integer)
			k, x := f.constant("qe-array-k", f.integer), f.constant("qe-array-x", f.integer)
			selectAt := func(array, index ast.ASTHandle) ast.ASTHandle {
				return f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, index}))
			}
			storeAt := func(array, index, value ast.ASTHandle) ast.ASTHandle {
				return f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, index, value}))
			}
			first := f.own(m.MkEq(selectAt(a, i), v))
			var body, expected ast.ASTHandle
			if name == "array-equality" {
				body = f.own(m.MkAnd(f.own(m.MkEq(a, b)), first))
				expected = f.own(m.MkEq(selectAt(b, i), v))
			} else if name == "single-select" {
				body = first
				expected = f.own(m.MkTrue())
			} else if name == "two-selects" {
				body = f.own(m.MkAnd(first, f.own(m.MkEq(selectAt(a, j), w))))
				expected = f.own(m.MkOr(f.own(m.MkNot(f.own(m.MkEq(i, j)))), f.own(m.MkEq(v, w))))
			} else if name == "store" {
				body = f.own(m.MkEq(storeAt(a, i, v), b))
				expected = f.own(m.MkEq(selectAt(b, i), v))
			} else if name == "store-with-disequality" {
				body = f.own(m.MkAnd(f.own(m.MkEq(storeAt(a, i, v), b)), f.own(m.MkNot(f.own(m.MkEq(selectAt(a, i), w))))))
				expected = f.own(m.MkEq(selectAt(b, i), v))
			} else if name == "nested-stores" {
				body = f.own(m.MkEq(storeAt(storeAt(a, i, v), j, w), b))
				firstCondition := f.own(m.MkOr(f.own(m.MkEq(i, j)), f.own(m.MkEq(selectAt(b, i), v))))
				expected = f.own(m.MkAnd(firstCondition, f.own(m.MkEq(selectAt(b, j), w))))
			} else if name == "three-stores" {
				body = f.own(m.MkEq(storeAt(storeAt(storeAt(a, i, v), j, w), k, x), b))
				first := f.own(m.MkOr(f.own(m.MkEq(i, j)), f.own(m.MkOr(f.own(m.MkEq(i, k)), f.own(m.MkEq(selectAt(b, i), v))))))
				second := f.own(m.MkOr(f.own(m.MkEq(j, k)), f.own(m.MkEq(selectAt(b, j), w))))
				expected = f.own(m.MkAnd(f.own(m.MkAnd(first, second)), f.own(m.MkEq(selectAt(b, k), x))))
			} else if name == "same-index-stores" {
				body = f.own(m.MkEq(storeAt(storeAt(a, i, v), i, w), b))
				expected = f.own(m.MkEq(selectAt(b, i), w))
			} else {
				left := f.own(m.MkAnd(f.own(m.MkEq(a, b)), first))
				right := f.own(m.MkAnd(f.own(m.MkEq(a, c)), f.own(m.MkEq(selectAt(a, i), w))))
				body = f.own(m.MkOr(left, right))
				expected = f.own(m.MkOr(f.own(m.MkEq(selectAt(b, i), v)), f.own(m.MkEq(selectAt(c, i), w))))
				if name == "guarded-disjunction" {
					guard := f.constant("qe-array-guard", f.boolean)
					body = f.own(m.MkAnd(guard, body))
					expected = f.own(m.MkAnd(guard, expected))
				}
			}
			bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{a}, body)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&bound)) }()
			quantifier, err := m.RefGet(bound)
			fmMust(t, err)
			result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
			fmMust(t, err)
			defer func() { fmMust(t, m.DelRef(&result)) }()
			value, err := m.RefGet(result)
			fmMust(t, err)
			// z3 4.7.1 keeps these quantified: selects alone are not solved
			// (the array stays bound), and solve_store's fresh w values stay
			// bound over nested stores.
			retained := map[string]ast.ASTHandle{"single-select": sort, "two-selects": sort, "disjunction": sort, "guarded-disjunction": sort, "nested-stores": f.integer, "three-stores": f.integer}
			if boundSort, ok := retained[name]; ok {
				if complete {
					t.Fatal("array quantifier was eliminated where z3 keeps it")
				}
				qeTestRequireBoundSort(t, m, value, boundSort)
				return
			}
			if !complete {
				t.Fatal("array equations were not eliminated")
			}
			difference := f.own(m.MkNot(f.own(m.MkEq(value, expected))))
			kernel := qsatKernelOwner(t, f)
			solver, err := kernel.solver()
			fmMust(t, err)
			fmMust(t, solver.AssertFormula(difference))
			status, err := solver.CheckSat(nil)
			fmMust(t, err)
			if status != lbool.False {
				t.Fatal("projected array formula differs from expected", status)
			}
		})
	}
}

func TestQuantifierArrayMultidimensionalStoreProjection(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	family, err := m.RegisterArrayPlugin()
	fmMust(t, err)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer, f.integer}), f.integer))
	a, b := f.constant("multi-a", sort), f.constant("multi-b", sort)
	i, j := f.constant("multi-i", f.integer), f.constant("multi-j", f.integer)
	k, l := f.constant("multi-k", f.integer), f.constant("multi-l", f.integer)
	v, w := f.constant("multi-v", f.integer), f.constant("multi-w", f.integer)
	store := func(base, x, y, value ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, x, y, value}))
	}
	selectAt := func(base, x, y ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{base, x, y}))
	}
	body := f.own(m.MkEq(store(store(a, i, j, v), k, l, w), b))
	same := f.own(m.MkAnd(f.own(m.MkEq(i, k)), f.own(m.MkEq(j, l))))
	first := f.own(m.MkOr(same, f.own(m.MkEq(selectAt(b, i, j), v))))
	expected := f.own(m.MkAnd(first, f.own(m.MkEq(selectAt(b, k, l), w))))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{a}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	_ = expected
	result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	// z3 4.7.1 eliminates the array but keeps solve_store's fresh values
	// bound: (exists ((w!8 Int) (w!10 Int)) ...).
	if complete {
		t.Fatal("multidimensional store values were not kept bound")
	}
	value, err := m.RefGet(result)
	fmMust(t, err)
	qeTestRequireBoundSort(t, m, value, f.integer)
}

// qeTestRequireBoundSort requires an existential quantifier (possibly under
// the expression root) whose bound variables all have the given sort.
func qeTestRequireBoundSort(t *testing.T, m *ast.ASTManager, value, sort ast.ASTHandle) {
	t.Helper()
	kind, err := m.GetKind(value)
	fmMust(t, err)
	if kind != ast.ASTQuantifier {
		t.Fatal("expected an existential quantifier", kind)
	}
	forall, err := m.IsForall(value)
	fmMust(t, err)
	n, err := m.GetNumDeclarations(value)
	fmMust(t, err)
	if forall || n == 0 {
		t.Fatal("expected bound existential variables")
	}
	for i := uint32(0); i < n; i++ {
		got, err := m.GetDeclarationSort(value, i)
		fmMust(t, err)
		if got != sort {
			t.Fatal("bound variable sort", i)
		}
	}
}

func TestQuantifierArrayDisjunctionUnderNegationIsNotDistributed(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	_, err := m.RegisterArrayPlugin()
	fmMust(t, err)
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a, b, c := f.constant("neg-array-a", sort), f.constant("neg-array-b", sort), f.constant("neg-array-c", sort)
	body := f.own(m.MkNot(f.own(m.MkOr(f.own(m.MkEq(a, b)), f.own(m.MkEq(a, c))))))
	bound, err := bindExprQuantifierVariables(m, []ast.ASTHandle{a}, body)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&bound)) }()
	quantifier, err := m.RefGet(bound)
	fmMust(t, err)
	// z3 4.7.1 keeps the array bound over the NNF body:
	// (exists ((x!0 (Array Int Int))) (and (not (= x!0 b)) (not (= x!0 c)))).
	result, complete, err := qeTestEliminate(m, quantifier, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	if complete {
		t.Fatal("negative disjunction was incorrectly projected")
	}
	value, err := m.RefGet(result)
	fmMust(t, err)
	qeTestRequireBoundSort(t, m, value, sort)
}
