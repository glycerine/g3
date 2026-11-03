package fp

// Source: the inliner runs before bit blasting, so original-domain relation
// reconstruction must precede replay of the inliner's removed-rule models.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogBitBlastComposesInliningModelsAndExpressionQueries(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(globals.SourceSetName("fixedpoint.xform.bit_blast", "true"))
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	must(err)
	t.Cleanup(func() { must(e.Close()) })
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	bv, err := m.BitVectorSort(2)
	must(err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, err := m.Symbols.Intern("bit-composition-" + name)
		must(err)
		d, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	seed := declare("S", []ast.ASTHandle{bv})
	middle := declare("M", []ast.ASTHandle{bv})
	result := declare("R", []ast.ASTHandle{bv})
	goal := declare("Goal", nil)
	originals := map[ast.ASTHandle]bool{seed: true, middle: true, result: true, goal: true}
	must(e.AddTableFact(seed, []uint32{1}))
	x, err := m.MkVar(0, bv)
	must(err)
	app := func(d ast.ASTHandle, values ...ast.ASTHandle) ast.ASTHandle {
		a, err := m.MkApp(d, values)
		must(err)
		return a
	}
	for _, pair := range [][2]ast.ASTHandle{{seed, middle}, {middle, result}} {
		rule, err := m.MkImplies(app(pair[0], x), app(pair[1], x))
		must(err)
		must(e.AddRule(rule, ""))
	}
	three, err := m.BitVectorNumeralUint64(3, 2)
	must(err)
	rule, err := m.MkImplies(app(result, three), app(goal))
	must(err)
	must(e.AddRule(rule, "unreachable-composed-goal"))

	// A semantic model check alone could also pass if the bit-blast option
	// were ignored. Inspect the mapped seeded relation while its scope lives.
	func() {
		scope, err := e.beginDatalogQueryScope([]ast.ASTHandle{goal})
		must(err)
		original := e.set
		e.set = scope.set
		defer func() { e.set = original; must(scope.Close()) }()
		valid, err := e.set.Finalize()
		must(err)
		if !valid {
			t.Fatal("composition fixture is not stratified")
		}
		blast := &datalogBitBlastQuery{engine: e}
		must(e.normalizeSimpleJoins(blast))
		if len(blast.columns) != 1 || blast.columns[0].original != seed {
			t.Fatal("inlining did not precede actual bit-blast expansion", blast.columns)
		}
		mapped, err := e.set.DerivedPredicate(seed)
		must(err)
		if mapped != blast.columns[0].derived || mapped == seed {
			t.Fatal("seed predicate mapping was not installed")
		}
		if present, err := e.predicates[mapped].table.Contains(rel.Fact{1, 0}); err != nil || !present {
			t.Fatal("composed pipeline lost low-bit-first stored fact", present, err)
		}
	}()

	status, err := e.QueryPredicate(goal)
	must(err)
	if status != lbool.False {
		t.Fatal("unreachable composed query", status)
	}
	for _, d := range []ast.ASTHandle{middle, result} {
		removed, err := e.hornInlineEliminated(e.datalogConversions, d)
		must(err)
		if !removed {
			t.Fatal("default eager inlining did not remove the intermediate definition", d)
		}
	}
	func() {
		inductive, err := e.Model(goal)
		must(err)
		defer func() { must(inductive.Close()) }()
		count, err := inductive.Core().NumDeclarations()
		must(err)
		if count != 3 {
			t.Fatal("original model declaration count", count)
		}
		for i := uint32(0); i < count; i++ {
			d, err := inductive.Core().Declaration(i)
			must(err)
			if !originals[d] || d == goal {
				t.Fatal("generated predicate escaped the original model", d)
			}
		}
		cooperation, err := cooperate.NewCooperation(m.Budget())
		must(err)
		defer func() { must(cooperation.Close()) }()
		evaluator, err := model.NewModelEvaluator(inductive.Core(), model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
		must(err)
		defer func() { must(evaluator.Close()) }()
		for _, d := range []ast.ASTHandle{seed, middle, result} {
			for value := uint64(0); value < 4; value++ {
				constant, err := m.BitVectorNumeralUint64(value, 2)
				must(err)
				evaluated, err := evaluator.EvaluateValue(app(d, constant))
				must(err)
				truth, err := m.RefGet(evaluated)
				must(err)
				if m.IsTrue(truth) != (value == 1) || !m.IsTrue(truth) && !m.IsFalse(truth) {
					t.Fatal("bit-blast/inliner model replay order", d, value, truth)
				}
				must(m.DelRef(&evaluated))
			}
		}
	}()

	for _, value := range []uint64{1, 2, 1} {
		constant, err := m.BitVectorNumeralUint64(value, 2)
		must(err)
		query, err := m.NewRef(app(result, constant))
		must(err)
		expression, err := m.RefGet(query)
		must(err)
		status, err = e.QueryExpression(expression)
		must(err)
		must(m.DelRef(&query))
		if status != lbool.FromBool(value == 1) || e.queryClone == nil {
			t.Fatal("original-domain expression query", value, status)
		}
		for d := range e.queryClone.predicates {
			if !originals[d] && d != e.queryClonePredicate {
				t.Fatal("bit-blasted predicate escaped the expression scope", d)
			}
		}
		rows, err := e.predicates[seed].table.Facts()
		must(err)
		if len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != 1 {
			t.Fatal("expression query changed the original stored fact", rows)
		}
		count, err := e.set.Len()
		must(err)
		if count != 3 || len(e.predicates) != 4 || len(e.auxiliary) != 0 {
			t.Fatal("composed query changed persistent rules or predicates", count, len(e.predicates), len(e.auxiliary))
		}
	}
}
