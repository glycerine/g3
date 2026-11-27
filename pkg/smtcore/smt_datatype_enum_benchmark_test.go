package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Internalizing a fresh enum-valued term registers a datatype variable and
// its initial recognizer. Pop removes both while keeping the input AST live.
func BenchmarkSMTScopedEnumTerm(b *testing.B) {
	for _, width := range []int{4, 16, 64} {
		b.Run(fmt.Sprintf("constructors=%d", width), func(b *testing.B) {
			s, x, _ := smtScopedEnumFixture(b, width)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				astTestMust(b, s.PushScope())
				astTestMust(b, s.Internalize(x, false))
				astTestMust(b, s.PopScopes(1))
			}
		})
	}
}

func smtScopedEnumFixture(b *testing.B, width int) (*SMTInternalizerState, ast.ASTHandle, ast.ASTHandle) {
	b.Helper()
	m := astTestManager(b)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(b, err)
	solver, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: params})
	astTestMust(b, err)
	b.Cleanup(func() {
		astTestMust(b, solver.Close())
		astTestMust(b, params.Close())
	})
	cs := make([]ast.DatatypeConstructorDefinition, width)
	for i := range cs {
		cs[i] = ast.DatatypeConstructorDefinition{
			Name: astTestSymbol(b, m, fmt.Sprintf("value-%d", i)), Recognizer: astTestSymbol(b, m, fmt.Sprintf("is-value-%d", i)),
		}
	}
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: astTestSymbol(b, m, "color"), Constructors: cs}})
	astTestMust(b, err)
	x, err := m.MkConst(astTestSymbol(b, m, "x"), sorts[0])
	astTestMust(b, err)
	astTestPin(b, m, x)
	return solver.kernel.internalizer, x, sorts[0]
}

// Revisit a split after its preferred constructor has been ruled out.
// Once the next recognizer is relevant and unassigned, it remains the choice.
func BenchmarkSMTEnumSplitAfterFalsePreference(b *testing.B) {
	for _, width := range []int{4, 16, 64} {
		b.Run(fmt.Sprintf("constructors=%d", width), func(b *testing.B) {
			s, x, sort := smtScopedEnumFixture(b, width)
			m := s.manager
			family, err := m.DatatypeFamilyID()
			astTestMust(b, err)
			dt := s.theories[family].(*SMTDatatypeTheory)
			astTestMust(b, s.Internalize(x, false))
			astTestMust(b, dt.split(x, sort))
			preferred, found, err := dt.utility.NonRecursiveConstructor(sort)
			astTestMust(b, err)
			if !found {
				b.Fatal("missing preferred constructor")
			}
			slots, err := dt.recognizerSlots(x)
			astTestMust(b, err)
			negative, err := m.MkNot(slots[preferred].expression)
			astTestMust(b, err)
			astTestMust(b, s.Assert(negative, ast.ASTHandle{}, 0))
			consistent, err := s.PropagateCore()
			astTestMust(b, err)
			if !consistent {
				b.Fatal("unexpected conflict")
			}
			astTestMust(b, dt.split(x, sort))
			slots, err = dt.recognizerSlots(x)
			astTestMust(b, err)
			if len(slots) != 2 {
				b.Fatal("expected two recognizers", len(slots))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				astTestMust(b, dt.split(x, sort))
			}
		})
	}
}
