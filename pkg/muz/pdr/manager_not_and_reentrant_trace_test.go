//go:build g3trace

package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type notAndReleaseWriter struct {
	owner   *Manager
	next    *ast.ASTRef
	invoked bool
	err     error
}

func (w *notAndReleaseWriter) Write(p []byte) (int, error) {
	if !w.invoked {
		w.invoked = true
		w.err = w.owner.releaseNotAndRef(w.next)
	}
	return len(p), nil
}

func TestManagerNotAndRefQueuesReentrantReleaseDuringDeletion(t *testing.T) {
	base, baseOwner, transformer, _, closeBase := closureFixture(t)
	globals := baseOwner.contexts.config.Globals
	writer := &notAndReleaseWriter{}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags, Params: globals.Context, Globals: globals})
	closureMust(t, err)
	closureMust(t, m.RegisterDeclarationPlugins())
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	closureMust(t, err)
	sort, err := m.MkBoolSort()
	closureMust(t, err)
	first, err := m.MkVar(5000, sort)
	closureMust(t, err)
	second, err := m.MkVar(5001, sort)
	closureMust(t, err)
	firstRef, err := m.NewRef(first)
	closureMust(t, err)
	secondRef, err := m.NewRef(second)
	closureMust(t, err)
	writer.owner, writer.next = owner, &secondRef
	owner.notAndPending = append(owner.notAndPending, firstRef)
	owner.notAndActive = true
	tags.Enable("delete_node_bug")
	err = owner.releaseNotAndPending()
	owner.finishNotAndActive()
	tags.Disable("delete_node_bug")
	closureMust(t, err)
	if !writer.invoked || !errors.Is(writer.err, ast.ErrASTPending) || secondRef != (ast.ASTRef{}) || len(owner.notAndPending) != 1 {
		t.Fatalf("reentrant release lost its owner: invoked=%v err=%v pending=%d", writer.invoked, writer.err, len(owner.notAndPending))
	}
	closureMust(t, owner.Close())
	closureMust(t, m.Close())
	closureMust(t, transformer.Close())
	closureMust(t, baseOwner.Close())
	closeBase()
}
