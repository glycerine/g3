package ast

// Source: src/ast/datatype_decl_plugin.cpp:477-520.

import "testing"

func TestDatatypeFullyInterpretedPreservesRetainedEarlyReturn(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)

	field, err := m.MkUninterpretedSort(astTestSymbol(t, m, "datatype-uninterpreted-field"), nil)
	astTestMust(t, err)
	fully, err := m.IsFullyInterpreted(field)
	astTestMust(t, err)
	if fully {
		t.Fatal("uninterpreted field sort classified as fully interpreted")
	}

	sorts, err := m.MkDatatypes([]DatatypeDefinition{{
		Name: astTestSymbol(t, m, "datatype-early-fully-interpreted"),
		Constructors: []DatatypeConstructorDefinition{{
			Name:       astTestSymbol(t, m, "mk-datatype-early-fully-interpreted"),
			Recognizer: astTestSymbol(t, m, "is-datatype-early-fully-interpreted"),
			Accessors: []DatatypeAccessorDefinition{{
				Name:  astTestSymbol(t, m, "datatype-early-uninterpreted-field"),
				Range: field,
			}},
		}},
	}})
	astTestMust(t, err)
	if len(sorts) != 1 {
		t.Fatal("datatype sort count", len(sorts))
	}
	fully, err = m.IsFullyInterpreted(sorts[0])
	astTestMust(t, err)
	if !fully {
		t.Fatal("retained source returns true before inspecting datatype fields")
	}
}
