package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source: ref_vector_core::shrink releases its suffix forward. The actual
// FPA external-parameter finalizer recycles value IDs, which mk_id reuses LIFO.
// Fresh literals therefore expose deletion order without a failure injection
// or an artificial finalizer callback.
func TestFixedpointPopPreservesSourceExternalFinalizerOrder(t *testing.T) {
	for _, rawRules := range []bool{false, true} {
		name := "background"
		if rawRules {
			name = "raw-rules"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			m, _ := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngine(m)
			must(err)
			var owned []ast.ASTRef
			defer func() {
				must(e.Close())
				for i := len(owned) - 1; i >= 0; i-- {
					must(m.DelRef(&owned[i]))
				}
			}()
			pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				must(err)
				ref, err := m.NewRef(h)
				must(err)
				owned = append(owned, ref)
				return h
			}
			_, err = m.RegisterBitVectorPlugin()
			must(err)
			_, err = m.RegisterFloatingPointPlugin()
			must(err)
			floatSort := pin(m.FloatingPointSort(8, 24))
			boolean := pin(m.MkBoolSort())
			declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				t.Helper()
				s, err := m.Symbols.Intern(name)
				must(err)
				return pin(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
			}
			wrapper := declare("scope-finalizer-background", []ast.ASTHandle{floatSort})
			var integer, variable, patternDeclaration, body ast.ASTHandle
			if rawRules {
				arithmetic, err := m.RegisterArithmeticPlugin()
				must(err)
				integer = pin(m.MkFamilySort(arithmetic, 1, nil))
				variable = pin(m.MkVar(0, integer))
				patternDeclaration = declare("scope-finalizer-pattern", []ast.ASTHandle{integer, floatSort})
				p := declare("scope-finalizer-tail", []ast.ASTHandle{integer})
				h := declare("scope-finalizer-head", nil)
				must(e.RegisterPredicate(p))
				must(e.RegisterPredicate(h))
				tail := pin(m.MkApp(p, []ast.ASTHandle{variable}))
				head := pin(m.MkApp(h, nil))
				body = pin(m.MkImplies(tail, head))
			}
			externalID := func(numeral ast.ASTHandle) uint32 {
				t.Helper()
				declaration, err := m.GetDecl(numeral)
				must(err)
				parameter, err := m.GetDeclParameterCopy(declaration, 0)
				must(err)
				id, err := m.ParameterExternal(parameter)
				must(err)
				must(m.DelParameter(&parameter))
				return id
			}
			add := func(value float64) (ast.ASTHandle, uint32) {
				t.Helper()
				numeral, err := m.MkFPANumeralFloat64(value, floatSort)
				must(err)
				ref, err := m.NewRef(numeral)
				must(err)
				id := externalID(numeral)
				var formula ast.ASTHandle
				if rawRules {
					// Already-Horn normalization strips this universal prefix.
					// Its pattern belongs only to the original raw formula, so
					// the compiled rule cannot delay the numeral's finalizer.
					application, err := m.MkApp(patternDeclaration, []ast.ASTHandle{variable, numeral})
					must(err)
					pattern, err := m.MkPattern([]ast.ASTHandle{application})
					must(err)
					formula, err = m.MkQuantifier(ast.ASTQuantifierSpec{
						Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)},
						Body: body, Patterns: []ast.ASTHandle{pattern},
					})
					must(err)
					must(e.AddRule(formula, "pattern-only-literal"))
				} else {
					formula, err = m.MkApp(wrapper, []ast.ASTHandle{numeral})
					must(err)
					must(e.AssertBackground(formula))
				}
				must(m.DelRef(&ref))
				if count, err := m.GetRefCount(numeral); err != nil || count != 1 {
					t.Fatal("numeral has an extra owner", count, err)
				}
				return formula, id
			}
			baseline, baselineID := add(9)
			must(e.Push())
			_, first := add(1)
			_, second := add(2)
			must(e.Pop())
			third := pin(m.MkFPANumeralFloat64(3, floatSort))
			fourth := pin(m.MkFPANumeralFloat64(4, floatSort))
			got := []uint32{externalID(third), externalID(fourth)}
			var originals []ast.ASTRef
			if rawRules {
				originals, _, _, err = e.RawRules()
			} else {
				originals, err = e.BackgroundAssertions()
			}
			must(err)
			if len(originals) != 1 {
				t.Fatal("outer scope owner count", len(originals))
			}
			baselineAfter, err := m.RefGet(originals[0])
			must(err)
			must(m.DelRef(&originals[0]))
			if baselineAfter != baseline {
				t.Fatal("outer scope formula changed")
			}
			if first == second || first == baselineID || second == baselineID {
				t.Fatal("fixture does not distinguish numeral values")
			}
			if got[0] != second || got[1] != first {
				t.Fatal("Pop reversed actual external finalizer order", got, []uint32{second, first})
			}
		})
	}
}
