package smtcore

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSMTDatatypeStatisticsSourceKeysAndOrder(t *testing.T) {
	_, dt, _, _, _ := smtDatatypeSplitFixture(t)
	dt.statistics = smtDatatypeStatistics{occursChecks: 1, splits: 2, constructorAxioms: 3, accessorAxioms: 4, updateAxioms: 5}
	out := statistics.New()
	astTestMust(t, dt.CollectStatistics(out))
	keys := []string{"datatype occurs check", "datatype splits", "datatype constructor ax", "datatype accessor ax", "datatype update ax"}
	if out.Size() != len(keys) {
		t.Fatal("source datatype statistic count", out.Size())
	}
	for i, key := range keys {
		if string(out.Key(i)) != key || !out.IsUint(i) || out.UintValue(i) != uint32(i+1) {
			t.Fatal("source datatype statistic key/order/value", i, out.Key(i))
		}
	}
}

func TestSMTDatatypeStatisticsWrapThroughRetainedOperations(t *testing.T) {
	for _, operation := range []string{"occurs", "split", "constructor", "accessor", "update"} {
		t.Run(operation, func(t *testing.T) {
			m, dt, x, sort, constructors := smtDatatypeSplitFixture(t)
			s, err := dt.State()
			astTestMust(t, err)
			switch operation {
			case "occurs":
				dt.statistics.occursChecks = math.MaxUint32
				check := smtDatatypeOccursCheck{theory: dt, onStack: make(map[*SMTEnode]bool), cycleFree: make(map[*SMTEnode]bool), parent: make(map[*SMTEnode]*SMTEnode)}
				cycle, err := check.run(s.context.enodes.get(x))
				astTestMust(t, err)
				if cycle || dt.statistics.occursChecks != 0 || !check.cycleFree[s.context.enodes.get(x).root] {
					t.Fatal("source occurs counter wrap changed traversal", dt.statistics.occursChecks)
				}
			case "split":
				dt.statistics.splits = math.MaxUint32
				astTestMust(t, dt.split(x, sort))
				if dt.statistics.splits != 0 {
					t.Fatal("source split counter did not wrap", dt.statistics.splits)
				}
				observed, err := dt.recognizerSlots(x)
				astTestMust(t, err)
				if len(observed) != 1 {
					t.Fatal("wrapped split lost its recognizer")
				}
			case "constructor":
				dt.statistics.constructorAxioms = math.MaxUint32
				astTestMust(t, dt.assertConstructor(x, constructors[1], SMTNullLiteral))
				_, err = s.PropagateCore()
				astTestMust(t, err)
				_, data, err := dt.rootData(s.context.enodes.get(x).TheoryVar(dt.FamilyID()))
				astTestMust(t, err)
				if s.Inconsistent() || dt.statistics.constructorAxioms != 0 || data.constructor == nil || data.constructor.declaration != constructors[1] {
					t.Fatal("source constructor counter wrap changed reconstruction", dt.statistics.constructorAxioms)
				}
			case "accessor":
				dt.statistics.accessorAxioms = math.MaxUint32
				value, err := m.MkApp(constructors[0], nil)
				astTestMust(t, err)
				astTestMust(t, s.Internalize(value, false))
				if dt.statistics.accessorAxioms != 0 || s.context.enodes.get(value) == nil {
					t.Fatal("source accessor counter did not wrap during constructor internalization", dt.statistics.accessorAxioms)
				}
			case "update":
				boolean, err := m.MkBoolSort()
				astTestMust(t, err)
				sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: astTestSymbol(t, m, "counter-update-sort"), Constructors: []ast.DatatypeConstructorDefinition{
					{Name: astTestSymbol(t, m, "counter-update-box"), Recognizer: astTestSymbol(t, m, "counter-update-is-box"), Accessors: []ast.DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "counter-update-field"), Range: boolean}}},
					{Name: astTestSymbol(t, m, "counter-update-empty"), Recognizer: astTestSymbol(t, m, "counter-update-is-empty")},
				}}})
				astTestMust(t, err)
				ctors, err := m.DatatypeConstructors(sorts[0])
				astTestMust(t, err)
				accessors, err := m.DatatypeConstructorAccessors(ctors[0])
				astTestMust(t, err)
				parameter, err := m.NewParameterAST(accessors[0])
				astTestMust(t, err)
				decl, err := m.MkFamilyFuncDecl(dt.FamilyID(), ast.ASTDatatypeUpdateField, []ast.ASTParameter{parameter}, []ast.ASTHandle{sorts[0], boolean}, ast.ASTHandle{})
				astTestMust(t, err)
				astTestMust(t, m.DelParameter(&parameter))
				base, err := m.MkConst(astTestSymbol(t, m, "counter-update-value"), sorts[0])
				astTestMust(t, err)
				truth, err := m.MkTrue()
				astTestMust(t, err)
				updated, err := m.MkApp(decl, []ast.ASTHandle{base, truth})
				astTestMust(t, err)
				dt.statistics.updateAxioms = math.MaxUint32
				astTestMust(t, s.Internalize(updated, false))
				if s.Inconsistent() || dt.statistics.updateAxioms != 0 || s.context.enodes.get(updated) == nil {
					t.Fatal("source update counter did not wrap through actual axiom publication", dt.statistics.updateAxioms)
				}
			}
		})
	}
}
