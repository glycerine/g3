package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// m_last_obj holds the last object an API call returned: closing that
// object's handle makes the handle unusable at once, but the object (here
// a built solver, with its smt context and ASTs) dies only when the next
// object-returning call, reset_last_result or the context releases it.
func TestAPILastObjectDefersDestruction(t *testing.T) {
	f := newAPIProgramFixture(t)
	ref, err := f.c.ParseSMT2String(solverLazyDecls+solverLazyBase, SMT2Bindings{})
	astTestMust(t, err)
	*f.trail = append(*f.trail, ref)
	base, err := f.m.RefGet(ref)
	astTestMust(t, err)
	boolean, err := f.m.MkBoolSort()
	astTestMust(t, err)
	first, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	last, err := f.c.NewSimpleSolver() // releases first's slot reference
	astTestMust(t, err)
	astTestMust(t, first.Assert(base))
	astTestMust(t, last.Assert(base))
	if first.last.held || !last.last.held {
		t.Fatal("m_last_obj is not the last solver returned", first.last.held, last.last.held)
	}

	// first is not the last object: Close destroys it at once.
	astTestMust(t, first.Close())
	if !first.closed {
		t.Fatal("closing an object m_last_obj does not hold left it alive")
	}

	// last is: its handle dies, its solver lives on.
	astTestMust(t, last.Close())
	if last.closed || last.solver == nil {
		t.Fatal("closing m_last_obj's object destroyed it")
	}
	if err := last.Assert(base); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("a closed handle stayed usable", err)
	}
	astTestMust(t, last.Close()) // a second Close is a no-op

	// The next object-returning call releases it.
	p, err := f.c.NewParameterSet()
	astTestMust(t, err)
	if !last.closed || last.solver != nil {
		t.Fatal("replacing m_last_obj did not destroy its closed object")
	}
	astTestMust(t, p.Close())

	// reset_last_result (a datatype constructor) and the context's close
	// release it too.
	kept, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	astTestMust(t, kept.Assert(base))
	astTestMust(t, kept.Close())
	if kept.closed {
		t.Fatal("closing m_last_obj's object destroyed it")
	}
	_, err = f.c.TupleSort("pair", []string{"first"}, []ast.ASTHandle{boolean})
	astTestMust(t, err)
	if !kept.closed {
		t.Fatal("reset_last_result did not destroy the closed last object")
	}
	held, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	astTestMust(t, held.Close())
	for i := range *f.trail {
		astTestMust(t, f.m.DelRef(&(*f.trail)[i]))
	}
	*f.trail = nil
	astTestMust(t, f.c.Close())
	if !held.closed {
		t.Fatal("closing the context did not destroy the closed last object")
	}
}
