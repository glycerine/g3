package ast

// Source regression: datatype_decl_plugin.cpp datatype::util::get_defs,
// lines 1023-1043. The returned accessor range is instantiated, while the
// sibling walk deliberately examines accessor_decl::get_range().

import (
	"bytes"
	"strings"
	"testing"
)

func TestDatatypeDefinitionsDoesNotTraverseInstantiatedSiblingArgument(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	formal, err := m.MkUninterpretedSort(astTestSymbol(t, m, "DefsT"), nil)
	astTestMust(t, err)
	astTestPin(t, m, formal)
	aName := astTestSymbol(t, m, "DefsA")
	bName := astTestSymbol(t, m, "DefsB")
	sorts, err := m.MkDatatypes([]DatatypeDefinition{
		{
			Name:       aName,
			Parameters: []ASTHandle{formal},
			Constructors: []DatatypeConstructorDefinition{{
				Name:       astTestSymbol(t, m, "defs-a"),
				Recognizer: astTestSymbol(t, m, "is-defs-a"),
				Accessors: []DatatypeAccessorDefinition{{
					Name:  astTestSymbol(t, m, "defs-payload"),
					Range: formal,
				}},
			}},
		},
		{
			Name:       bName,
			Parameters: []ASTHandle{formal},
			Constructors: []DatatypeConstructorDefinition{{
				Name:       astTestSymbol(t, m, "defs-b"),
				Recognizer: astTestSymbol(t, m, "is-defs-b"),
			}},
		},
	})
	astTestMust(t, err)
	if len(sorts) != 2 {
		t.Fatal("datatype block sort count", len(sorts))
	}

	nameParameter, err := m.NewParameterSymbol(aName)
	astTestMust(t, err)
	argumentParameter, err := m.NewParameterAST(sorts[1])
	astTestMust(t, err)
	aOfB, err := m.MkFamilySort(family, ASTDatatypeSort, []ASTParameter{nameParameter, argumentParameter})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&argumentParameter))
	astTestMust(t, m.DelParameter(&nameParameter))
	astTestPin(t, m, aOfB)

	definitions, err := m.DatatypeDefinitions(aOfB)
	astTestMust(t, err)
	if len(definitions) != 1 || definitions[0].Name != aName {
		t.Fatal("instantiated type argument was traversed as a declared sibling", definitions)
	}
	if got := definitions[0].Constructors[0].Accessors[0].Range; got != sorts[1] {
		t.Fatal("description did not retain instantiated accessor range", got, sorts[1])
	}

	// The legacy declaration consumer first selects the generic definition.
	// It must inherit get_defs' declared-range traversal and declare only A.
	generic, err := m.legacyDatatypeDefinitions(aOfB)
	astTestMust(t, err)
	if len(generic) != 1 || generic[0].Name != aName || generic[0].Constructors[0].Accessors[0].Range != formal {
		t.Fatal("legacy declaration did not retain the generic A-only block", generic)
	}

	// datatype::util::display has a separate instantiated-range walk. Preserve
	// that behavior: displaying A[B[T]] reaches B even though get_defs does not.
	var display bytes.Buffer
	astTestMust(t, m.DisplayDatatype(aOfB, &display))
	text := display.String()
	if !strings.Contains(text, "(defs-payload: DefsB)") || !strings.Contains(text, "\nDefsB =\n") {
		t.Fatal("display lost its instantiated sibling traversal", text)
	}
}
