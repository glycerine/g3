package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTModelFinderAUFOffsetPopulationAndSameClassCutoff(t *testing.T) {
	for _, sameClass := range []bool{false, true} {
		name := "separate-classes"
		if sameClass {
			name = "same-class"
		}
		t.Run(name, func(t *testing.T) {
			m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			two, err := m.ArithmeticIntegerNumeralText("2")
			astTestMust(t, err)
			eight, err := m.ArithmeticIntegerNumeralText("8")
			astTestMust(t, err)
			ten, err := m.ArithmeticIntegerNumeralText("10")
			astTestMust(t, err)
			integer, err := m.GetSort(two)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			predicate := smtQuickPredicate(t, m, "auf-offset-p", integer, boolean)
			x, err := m.MkVar(0, integer)
			astTestMust(t, err)
			xPlusTwo, err := m.MkArithmeticAdd(x, two)
			astTestMust(t, err)
			body, err := m.MkApp(predicate, []ast.ASTHandle{xPlusTwo})
			astTestMust(t, err)
			if sameClass {
				direct, err := m.MkApp(predicate, []ast.ASTHandle{x})
				astTestMust(t, err)
				body, err = m.MkAnd(direct, body)
				astTestMust(t, err)
			}
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, "auf-offset-x")}, Body: body,
			})
			astTestMust(t, err)
			tenNode, err := NewSMTEnode(m, ten, state.context.enodes.get, SMTEnodeOptions{Generation: 6})
			astTestMust(t, err)
			astTestMust(t, state.context.RegisterEnode(tenNode, true))
			smtQuickApplicationNode(t, m, state.context, predicate, tenNode, true)
			graph := newSMTModelFinderAUFGraph(manager.modelFinder)
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
			variable := graph.uvars[smtModelFinderAUFKey{q, 0}].root()
			argument := graph.arguments[smtModelFinderAUFKey{predicate, 0}].root()
			if (variable == argument) != sameClass {
				t.Fatal("offset graph incorrectly merged classes")
			}
			if generation, found := aufValueSet(t, manager, variable.set.testElements())[eight]; !found || generation != 6 {
				t.Fatal("offset did not recover 10-2 with its generation", variable.set.testElements())
			}
			wantSize := 1
			if sameClass {
				wantSize = 2
			}
			variableValues := aufValueSet(t, manager, variable.set.testElements())
			argumentValues := aufValueSet(t, manager, argument.set.testElements())
			if len(variableValues) != wantSize || len(argumentValues) != wantSize || argumentValues[ten] != 6 {
				t.Fatal("offset two-pass population or same-class cutoff", variable.set.testElements(), argument.set.testElements())
			}
		})
	}
}

func TestSMTModelFinderAUFBitVectorOffsetWraps(t *testing.T) {
	m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
	_, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	two, err := m.BitVectorNumeralUnsigned(2, 4)
	astTestMust(t, err)
	one, err := m.BitVectorNumeralUnsigned(1, 4)
	astTestMust(t, err)
	fifteen, err := m.BitVectorNumeralUnsigned(15, 4)
	astTestMust(t, err)
	sort, err := m.GetSort(two)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p := smtQuickPredicate(t, m, "auf-bv-offset-p", sort, boolean)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	shifted, err := m.MkBitVectorAdd(x, two)
	astTestMust(t, err)
	body, err := m.MkApp(p, []ast.ASTHandle{shifted})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "auf-bv-offset-x")}, Body: body,
	})
	astTestMust(t, err)
	oneNode, err := NewSMTEnode(m, one, state.context.enodes.get, SMTEnodeOptions{Generation: 9})
	astTestMust(t, err)
	astTestMust(t, state.context.RegisterEnode(oneNode, true))
	smtQuickApplicationNode(t, m, state.context, p, oneNode, true)
	graph := newSMTModelFinderAUFGraph(manager.modelFinder)
	t.Cleanup(func() { astTestMust(t, graph.close()) })
	astTestMust(t, aufTestBuild(t, graph, []ast.ASTHandle{q}))
	variable := graph.uvars[smtModelFinderAUFKey{q, 0}].root()
	argument := graph.arguments[smtModelFinderAUFKey{p, 0}].root()
	if len(variable.set.testElements()) != 1 || variable.set.testElements()[fifteen] != 9 || len(argument.set.testElements()) != 1 || argument.set.testElements()[one] != 9 {
		t.Fatal("4-bit offset did not wrap 1-2 to 15 and 15+2 to 1", variable.set.testElements(), argument.set.testElements())
	}
}

func TestSMTModelFinderAUFOffsetPassesFollowQuantifierOrder(t *testing.T) {
	for _, producerFirst := range []bool{false, true} {
		name := "offset-before-producer"
		if producerFirst {
			name = "producer-before-offset"
		}
		t.Run(name, func(t *testing.T) {
			m, _, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{})
			one, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			nine, err := m.ArithmeticIntegerNumeralText("9")
			astTestMust(t, err)
			ten, err := m.ArithmeticIntegerNumeralText("10")
			astTestMust(t, err)
			integer, err := m.GetSort(one)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			p := smtQuickPredicate(t, m, "offset-pass-p", integer, boolean)
			x, err := m.MkVar(0, integer)
			astTestMust(t, err)
			shifted, err := m.MkArithmeticAdd(x, one)
			astTestMust(t, err)
			offsetBody, err := m.MkApp(p, []ast.ASTHandle{shifted})
			astTestMust(t, err)
			direct, err := m.MkApp(p, []ast.ASTHandle{x})
			astTestMust(t, err)
			equal, err := m.MkEq(x, ten)
			astTestMust(t, err)
			notEqual, err := m.MkNot(equal)
			astTestMust(t, err)
			producerBody, err := m.MkAnd(direct, notEqual)
			astTestMust(t, err)
			makeQuantifier := func(name string, body ast.ASTHandle) ast.ASTHandle {
				q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{astTestSymbol(t, m, name)}, Body: body})
				astTestMust(t, err)
				return q
			}
			offset := makeQuantifier("offset-pass-x", offsetBody)
			producer := makeQuantifier("offset-pass-y", producerBody)
			quantifiers := []ast.ASTHandle{offset, producer}
			if producerFirst {
				quantifiers = []ast.ASTHandle{producer, offset}
			}
			graph := newSMTModelFinderAUFGraph(manager.modelFinder)
			t.Cleanup(func() { astTestMust(t, graph.close()) })
			astTestMust(t, aufTestBuild(t, graph, quantifiers))
			offsetSet := aufValueSet(t, manager, graph.uvars[smtModelFinderAUFKey{offset, 0}].root().set.testElements())
			producerSet := graph.uvars[smtModelFinderAUFKey{producer, 0}].root().set.testElements()
			if _, found := producerSet[ten]; !found {
				t.Fatal("producer did not populate shared argument class")
			}
			if producerFirst {
				if _, found := offsetSet[nine]; !found || len(offsetSet) != 1 {
					t.Fatal("offset did not consume earlier quantifier's term", offsetSet)
				}
			} else if len(offsetSet) != 0 {
				t.Fatal("offset consumed a later quantifier's first-pass term", offsetSet)
			}
		})
	}
}

// aufValueSet maps each instantiation-set element to its rewritten value.
// copy_instances and is_x_gle_t store arith_rewriter::mk_add/mk_sub results,
// which are not normalized (n - k is n + -1*k).
func aufValueSet(t *testing.T, manager *SMTQuantifierManager, elements map[ast.ASTHandle]uint32) map[ast.ASTHandle]uint32 {
	t.Helper()
	m := manager.modelFinder.manager
	values := make(map[ast.ASTHandle]uint32, len(elements))
	for h, generation := range elements {
		ref, err := m.NewRef(ast.ASTHandle{})
		astTestMust(t, err)
		astTestMust(t, manager.rewriter.Rewrite(h, ref))
		value, err := m.RefGet(ref)
		astTestMust(t, err)
		astTestPin(t, m, value)
		astTestMust(t, m.DelRef(&ref))
		values[value] = generation
	}
	return values
}
