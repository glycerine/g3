package ast

// Source: src/ast/ast.cpp:2114-2134 (actual post-registration exception catch).

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestASTTraceAppClosedStreamCatchFreesLiteralNodeWithoutTableRepair(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 28)
	name := astTracePath(t)
	filename := uparams.LiteralParamsStringView(name)
	m, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, &filename)
	paramsMust(t, e)
	bs, e := m.MkBoolSort()
	paramsMust(t, e)
	symbol, e := sy.Intern("int")
	paramsMust(t, e)
	decl, e := m.MkFuncDecl(symbol, nil, bs, ASTFuncDeclInfo{})
	paramsMust(t, e)
	paramsMust(t, m.IncRef(decl))
	slot := m.freeNode
	if slot == 0 {
		slot = m.nextNode
	}
	beforeTable := m.table.size
	declRef, _ := m.requireKind(decl, ASTFuncDecl)
	beforeRC := m.nodeAt(declRef.slot).rc
	paramsMust(t, m.TraceStream.Close())
	_, e = m.MkApp(decl, nil)
	if e == nil {
		t.Fatal("closed output accepted")
	}
	if !errors.Is(e, arena.ErrClosed) {
		t.Fatal("actual reached closed output guard", e)
	}
	// Source catch deallocate_node frees raw placement without delete_node:
	// table membership and the acquired declaration INC are not repaired.
	if m.nodeAt(slot).live {
		t.Fatal("source catch failed to deallocate already registered placement", e)
	}
	if m.table.size != beforeTable+1 || m.nodeAt(declRef.slot).rc != beforeRC+1 {
		t.Fatal("source catch changed literal table/INC prefix", m.table.size, beforeTable, m.nodeAt(declRef.slot).rc, beforeRC)
	}
	frame := m.traceAcquisitions
	if frame == nil || frame.node.slot != slot || !frame.placementConsumed || frame.tableConsumed || frame.acquired != 1 || frame.references[0] != declRef || frame.bodyCause != e && !errors.Is(e, frame.bodyCause) {
		t.Fatal("missing exact leaf-stamped catch owner", frame, e)
	}
	if b.Used() != m.Stats().AllocatedBytes {
		t.Fatal("immediate caught cold graph", b.Used(), m.Stats())
	}
	if x := m.SourceFinalize(); !errors.Is(x, ErrASTHandle) {
		t.Fatal("source reached dangling table guard", x)
	}
	if !errors.Is(m.sourceDTCause, ErrASTHandle) || frame.tableConsumed || m.traceAcquisitions != frame {
		t.Fatal("source stop adopted own catch receipt", m.sourceDTCause, frame)
	}
	if e = m.Close(); !errors.Is(e, arena.ErrClosed) || !m.Closed {
		t.Fatal("owning close reaches closed-stream EOF and completes", e)
	}
	if b.Used() != 0 {
		t.Fatal("actual owning graph after call", b.Used())
	}
}

// Source: src/ast/ast.cpp:1714-1733, 2129-2134. Independent registered
// reference prefix, with separate Go slab-generation replacement proof.
func TestASTTraceClosedStreamCatchCapturesINCAndPreservesLaterSlotGeneration(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 28)
	filename := uparams.LiteralParamsStringView(astTracePath(t))
	m, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, &filename)
	paramsMust(t, e)
	sort, e := m.MkBoolSort()
	paramsMust(t, e)
	child, e := m.MkVar(7, sort)
	paramsMust(t, e)
	paramsMust(t, m.IncRef(child))
	info, e := m.NewFuncDeclInfo(1, 1, nil, ASTFuncFlags{}, false)
	paramsMust(t, e)
	symbol, e := sy.Intern("label")
	paramsMust(t, e)
	decl, e := m.MkFuncDecl(symbol, []ASTHandle{sort}, sort, info)
	paramsMust(t, e)
	paramsMust(t, m.IncRef(decl))
	dr, _ := m.internal(decl)
	cr, _ := m.internal(child)
	declRC, childRC, tableSize := m.nodeAt(dr.slot).rc, m.nodeAt(cr.slot).rc, m.table.size
	paramsMust(t, m.TraceStream.Close())
	_, e = m.MkApp(decl, []ASTHandle{child})
	frame := m.traceAcquisitions
	if frame == nil {
		t.Fatal("missing caught reference prefix", e)
	}
	if !errors.Is(e, arena.ErrClosed) || frame.acquired != 2 || frame.references[0] != dr || frame.references[1] != cr || m.nodeAt(dr.slot).rc != declRC+1 || m.nodeAt(cr.slot).rc != childRC+1 || m.table.size != tableSize+1 {
		t.Fatal("actual per-leaf prefix", e, frame)
	}
	if b.Used() != m.Stats().AllocatedBytes {
		t.Fatal("before-lift caught graph", b.Used(), m.Stats())
	}
	// A real new slab placement reuses the physically freed slot with a new
	// generation. It remains separately owned and unregistered here; this is
	// a Go captured-pointer guard fixture, not native stale-pointer behavior.
	replacement, x := m.allocateNode(ASTVar)
	paramsMust(t, x)
	if replacement.slot != frame.node.slot || replacement.generation == frame.node.generation {
		t.Fatal("real replacement fixture", replacement, frame.node)
	}
	sr, _ := m.internal(sort)
	m.varAt(replacement.slot).sort = sr // unused borrowed payload, no INC
	capturedReferences := append([]astNodeRef(nil), frame.references...)
	bodyHistory, sourceHistory := frame.bodyCause, frame.sourceCause
	paramsMust(t, m.cleanupTraceAcquisitions())
	if !frame.retired || frame.bodyCause != bodyHistory || frame.sourceCause != sourceHistory || frame.acquired != 2 || len(frame.references) != len(capturedReferences) || frame.references[0] != capturedReferences[0] || frame.references[1] != capturedReferences[1] {
		t.Fatal("retired exact reference tuple/history erased", frame)
	}
	if x = m.validNode(replacement); x != nil || m.nodeAt(dr.slot).rc != declRC || m.nodeAt(cr.slot).rc != childRC || m.table.size != tableSize {
		t.Fatal("own ACK adopted replacement or changed foreign refs", x, m.table.size, tableSize)
	}
	paramsMust(t, m.freeNodeStorage(replacement))
	if e = m.Close(); !errors.Is(e, arena.ErrClosed) || !m.Closed {
		t.Fatal("owning close reaches closed-stream EOF and completes", e)
	}
	if b.Used() != 0 {
		t.Fatal("owning graph", b.Used())
	}
}

// Source: src/ast/ast.cpp:1321-1328,2114-2134. Actual format child
// borrows the owner's trace ostream; it does not own the trace file pointer.
func TestASTBorrowedFormatNativeTraceAndStreamLifetime(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	reached := false
	for ordinal := int64(0); ordinal < 32; ordinal++ {
		b := arena.NewBudget(1 << 28)
		name := astTracePath(t)
		filename := uparams.LiteralParamsStringView(name)
		owner, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, &filename)
		paramsMust(t, e)
		m := owner.Format
		if m == nil || m.TraceFile != nil || m.TraceStream != owner.TraceStream {
			t.Fatal("borrowed format stream")
		}
		sort, e := m.MkBoolSort()
		paramsMust(t, e)
		symbol, e := sy.Intern("int")
		paramsMust(t, e)
		decl, e := m.MkFuncDecl(symbol, nil, sort, ASTFuncDeclInfo{})
		paramsMust(t, e)
		paramsMust(t, m.IncRef(decl))
		paramsMust(t, owner.TraceStream.Flush())
		before, e := os.ReadFile(name)
		paramsMust(t, e)
		b.FailAfter(ordinal)
		_, e = m.MkApp(decl, nil)
		paramsMust(t, owner.TraceStream.Flush())
		after, x := os.ReadFile(name)
		paramsMust(t, x)
		prefix := strings.TrimPrefix(string(after), string(before))
		if strings.HasPrefix(prefix, "[mk-app] #") && e != nil {
			t.Fatal("native lexical collections introduced a backing refusal", ordinal, e, prefix)
		}
		if e == nil {
			reached = true
			if !strings.HasPrefix(prefix, "[mk-app] #") || !strings.HasSuffix(prefix, " int\n") || m.traceAcquisitions != nil || m.LowLevel.Active {
				t.Fatal("completed native trace", prefix, m.traceAcquisitions)
			}
		} else if !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal(ordinal, e)
		}
		b.FailAfter(-1)
		paramsMust(t, m.Close())
		if owner.TraceFile.Closed || owner.TraceStream.Closed {
			t.Fatal("format cleanup adopted borrowed file")
		}
		paramsMust(t, owner.TraceStream.AppendCString("[borrower-ended]\n"))
		paramsMust(t, owner.TraceStream.Flush())
		data, e := os.ReadFile(name)
		paramsMust(t, e)
		if !strings.HasSuffix(string(data), "[borrower-ended]\n") {
			t.Fatal("borrowed stream stopped writing")
		}
		if b.Used() != owner.Stats().AllocatedBytes {
			t.Fatal("remaining owner graph", b.Used(), owner.Stats())
		}
		paramsMust(t, owner.Close())
		if b.Used() != 0 {
			t.Fatal("borrowed owner refunds", ordinal, b.Used())
		}
		if reached {
			break
		}
	}
	if !reached {
		t.Fatal("factory reservation sweep never completed native trace")
	}
}

// Source: src/ast/ast.cpp:1297-1334,2129-2134. Closed pointed ostream is a
// Go lifetime-guard domain; this exercises the real nested constructor body,
// catch and completed-member unwind, not an invented allocation failpoint.
func TestASTIncompleteBorrowedFormatConstructorRetainsCaughtHistoryUntilOwnAbort(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 28)
	name := astTracePath(t)
	filename := uparams.LiteralParamsStringView(name)
	owner, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, &filename)
	paramsMust(t, e)
	borrowed := owner.TraceStream
	fd := borrowed.File
	paramsMust(t, borrowed.Close())
	child, body := constructSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, true, nil, borrowed)
	if child == nil || !errors.Is(body, arena.ErrClosed) || child.constructorComplete || child.constructorBodyCause == nil {
		t.Fatal("reached failed borrowed constructor", child, body)
	}
	f := child.traceAcquisitions
	if f == nil || !f.placementConsumed || f.acquired == 0 || !f.tableSourceEnded || !f.referencesSourceEnded || f.tableConsumed || f.ownIndex != 0 || child.constructorMemberStage != 6 || child.table.buckets != nil || child.table.entries.Len() != 0 || child.allocator != nil {
		t.Fatal("real catch and completed-member source unwind", f, child.ConstructorState())
	}
	capturedNode, hash, invocation, bodyHistory := f.node, f.hash, f.invocation, f.bodyCause
	capturedRefs := append([]astNodeRef(nil), f.references...)
	if owner.TraceFile.Closed || borrowed.File != fd {
		t.Fatal("source constructor adopted borrowed file")
	}
	if b.Used() != owner.Stats().AllocatedBytes+child.Stats().AllocatedBytes {
		t.Fatal("immediate ended-source/retained-own graph", b.Used(), owner.Stats(), child.Stats())
	}
	paramsMust(t, child.AbortConstruction())
	if b.Used() != owner.Stats().AllocatedBytes {
		t.Fatal("own abort leaked actual caught metadata", b.Used(), owner.Stats(), f.bytes)
	}
	if !child.Closed || !f.retired || f.tableConsumed || f.ownIndex != 0 || f.node != capturedNode || f.hash != hash || f.invocation != invocation || f.bodyCause != bodyHistory || len(capturedRefs) == 0 {
		t.Fatal("own abort lost exact acquired constructor history", f)
	}
	if owner.TraceFile.Closed || borrowed.File != fd {
		t.Fatal("own abort adopted borrowed file")
	}
	if _, e = fd.WriteString("[failed-borrower-ended]\n"); e != nil {
		t.Fatal("actual borrowed fd prematurely closed", e)
	}
	if b.Used() != owner.Stats().AllocatedBytes {
		t.Fatal("after abort exact remaining owner", b.Used(), owner.Stats())
	}
	if e = owner.Close(); !errors.Is(e, arena.ErrClosed) || !owner.Closed {
		t.Fatal("owner reached closed-stream EOF guard then own file closure", e)
	}
	if b.Used() != 0 {
		t.Fatal("actual owner refunds", b.Used())
	}
}
