package spacer

// Source: src/muz/spacer/spacer_util.cpp expand_literals. Constructor field
// equalities appended to conjs remain in the growing work vector and are
// themselves expanded when the field value is a constructor.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerDatatypeExpansionRevisitsAppendedConstructorField(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	p := &PredicateTransformer{manager: m}
	var pins []ast.ASTRef
	defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		depthMust(t, err)
		ref, err := m.NewRef(h)
		depthMust(t, err)
		pins = append(pins, ref)
		return h
	}
	name := func(text string) usymbol.Symbol {
		t.Helper()
		s, err := m.Symbols.Intern(text)
		depthMust(t, err)
		return s
	}
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: name("expand-recursive-nat"),
		Constructors: []ast.DatatypeConstructorDefinition{
			{Name: name("expand-zero"), Recognizer: name("expand-is-zero")},
			{Name: name("expand-succ"), Recognizer: name("expand-is-succ"), Accessors: []ast.DatatypeAccessorDefinition{{Name: name("expand-pred"), Recursive: true}}},
		},
	}})
	depthMust(t, err)
	sort := own(sorts[0], nil)
	constructors, err := m.DatatypeConstructors(sort)
	depthMust(t, err)
	zero := own(m.MkApp(constructors[0], nil))
	value := own(m.MkApp(constructors[1], []ast.ASTHandle{zero}))
	term := own(m.MkFreshConst("expand-recursive-term", sort))
	input := own(m.MkEq(term, value))
	expanded, err := p.expandInductiveLiteral(input)
	depthMust(t, err)
	defer func() { depthMust(t, expanded.Close()) }()
	if len(expanded.literals) != 2 {
		t.Fatal("appended constructor field equality was not revisited", len(expanded.literals))
	}
	succRecognizer, err := m.DatatypeConstructorIs(constructors[1])
	depthMust(t, err)
	wantOuter := own(m.MkApp(succRecognizer, []ast.ASTHandle{term}))
	accessors, err := m.DatatypeConstructorAccessors(constructors[1])
	depthMust(t, err)
	selected := own(m.MkApp(accessors[0], []ast.ASTHandle{term}))
	zeroRecognizer, err := m.DatatypeConstructorIs(constructors[0])
	depthMust(t, err)
	wantField := own(m.MkApp(zeroRecognizer, []ast.ASTHandle{selected}))
	if expanded.literals[0] != wantOuter || expanded.literals[1] != wantField {
		t.Fatal("recursive datatype expansion lost source append order")
	}
}
