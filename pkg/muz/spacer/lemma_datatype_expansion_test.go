package spacer

// Source: spacer_util.cpp expand_literals and apply_accessor. Three fields
// expose the displacement order hidden by the two-field source examples.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerDatatypeExpansionSourceFieldOrderAndOrientation(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("constructor=%v/reverse=%v", direct, reverse), func(t *testing.T) {
				m, _ := newSpacerTestServices(t)
				p := &PredicateTransformer{manager: m}
				var refs []ast.ASTRef
				defer func() { depthMust(t, releaseNormalizeRefs(m, refs)) }()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					t.Helper()
					depthMust(t, err)
					ref, err := m.NewRef(h)
					depthMust(t, err)
					refs = append(refs, ref)
					return h
				}
				zero := own(m.ArithmeticIntegerNumeralText("0"))
				integer, err := m.GetSort(zero)
				depthMust(t, err)
				name, err := m.Symbols.Intern("expansion-triple")
				depthMust(t, err)
				ctorName, err := m.Symbols.Intern("expansion-mk-triple")
				depthMust(t, err)
				recName, err := m.Symbols.Intern("expansion-is-triple")
				depthMust(t, err)
				fields := make([]ast.DatatypeAccessorDefinition, 3)
				variables := make([]ast.ASTHandle, 3)
				values := make([]ast.ASTHandle, 3)
				for i := range fields {
					fieldName, err := m.Symbols.Intern(fmt.Sprintf("expansion-field-%d", i))
					depthMust(t, err)
					fields[i] = ast.DatatypeAccessorDefinition{Name: fieldName, Range: integer}
					variables[i] = own(m.MkFreshConst(fmt.Sprintf("expansion-x-%d", i), integer))
					values[i] = own(m.ArithmeticIntegerNumeralText(fmt.Sprint(i)))
				}
				sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: name, Constructors: []ast.DatatypeConstructorDefinition{{Name: ctorName, Recognizer: recName, Accessors: fields}}}})
				depthMust(t, err)
				own(sorts[0], nil)
				constructors, err := m.DatatypeConstructors(sorts[0])
				depthMust(t, err)
				ctor := constructors[0]
				value := own(m.MkApp(ctor, values))
				term := own(m.MkFreshConst("expansion-term", sorts[0]))
				if direct {
					term = own(m.MkApp(ctor, variables))
				}
				left, right := term, value
				if reverse {
					left, right = right, left
				}
				input := own(m.MkEq(left, right))
				expanded, err := p.expandInductiveLiteral(input)
				depthMust(t, err)
				for _, h := range expanded {
					own(h, nil)
				}
				if len(expanded) != 7 {
					t.Fatal("expanded size", len(expanded))
				}
				// The source first matches the right constructor. If only the left
				// side is a constructor, it swaps roles before selecting accessors.
				selectedTerm := term
				lhs, rhs := variables, values
				if direct && reverse {
					selectedTerm, lhs, rhs = value, values, variables
				}
				recognizer, err := m.DatatypeConstructorRecognizer(ctor)
				depthMust(t, err)
				expectedRecognizer := own(m.MkApp(recognizer, []ast.ASTHandle{selectedTerm}))
				if expanded[0] != expectedRecognizer {
					t.Fatal("wrong recognizer orientation")
				}
				accessors, err := m.DatatypeConstructorAccessors(ctor)
				depthMust(t, err)
				for position, field := range []int{0, 2, 1} {
					selected := lhs[field]
					if !direct {
						selected = own(m.MkApp(accessors[field], []ast.ASTHandle{term}))
					}
					le := own(m.MkArithmeticLE(selected, rhs[field]))
					ge := own(m.MkArithmeticGE(selected, rhs[field]))
					if expanded[1+2*position] != le || expanded[2+2*position] != ge {
						t.Fatalf("position %d must contain bounds for field %d", position, field)
					}
				}
			})
		}
	}
}
