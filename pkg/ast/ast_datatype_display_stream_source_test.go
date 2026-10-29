package ast

// Source: src/ast/datatype_decl_plugin.cpp:1045-1070.

import (
	"errors"
	"testing"
)

type datatypeDisplayRefusingWriter struct {
	calls   int
	err     error
	onFirst func()
}

func (w *datatypeDisplayRefusingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 1 && w.onFirst != nil {
		w.onFirst()
	}
	return 0, w.err
}

func TestDatatypeDisplayStickyStreamReachesLaterLookup(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterDatatypePlugin()
	astTestMust(t, err)
	sorts, err := m.MkDatatypes([]DatatypeDefinition{{
		Name: astTestSymbol(t, m, "DisplayBox"),
		Constructors: []DatatypeConstructorDefinition{{
			Name:       astTestSymbol(t, m, "display-box"),
			Recognizer: astTestSymbol(t, m, "is-display-box"),
		}},
	}})
	astTestMust(t, err)
	root, err := m.internal(sorts[0])
	astTestMust(t, err)
	definition := m.datatypePlugin.definitions[root]

	streamFailure := errors.New("datatype display stream refusal")
	w := &datatypeDisplayRefusingWriter{err: streamFailure, onFirst: func() {
		// Test-only observation point: the source has already acquired the root
		// definition, while the next loop iteration must look it up again.
		delete(m.datatypePlugin.definitions, root)
		delete(m.datatypePlugin.definitionNames, definition.name)
	}}
	err = m.DisplayDatatype(sorts[0], w)
	m.datatypePlugin.definitions[root] = definition
	m.datatypePlugin.definitionNames[definition.name] = definition
	if !errors.Is(err, streamFailure) || !errors.Is(err, ErrASTDomain) {
		t.Fatal("sticky stream did not reach later datatype lookup", err)
	}
	if w.calls != 1 {
		t.Fatal("failed ostream touched its streambuf again", w.calls)
	}
}
