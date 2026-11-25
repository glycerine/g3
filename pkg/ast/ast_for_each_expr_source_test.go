package ast

// Source: src/ast/for_each_expr.h:27-105; src/ast/for_each_expr.cpp:23-66.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTExprVisitorSourceCountSharedAndAllMarks(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "visitor-x"), b)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "visitor-f"), []ASTHandle{b, b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	root, e := m.MkApp(d, []ASTHandle{x, x})
	astTestMust(t, e)
	astTestPin(t, m, root)
	n, e := m.GetNumExprs(root)
	astTestMust(t, e)
	if n != 2 {
		t.Fatal("shared constant visited once, parent once", n)
	}
	em, e := NewASTExprMark(m)
	astTestMust(t, e)
	defer em.Close()
	n, e = m.GetNumExprsWithExprMark(root, em)
	astTestMust(t, e)
	if n != 2 {
		t.Fatal(n)
	}
	n, e = m.GetNumExprsWithExprMark(root, em)
	astTestMust(t, e)
	if n != 0 {
		t.Fatal("all-mark root shortcut", n)
	}
	fm, e := NewASTFastMark1(m)
	astTestMust(t, e)
	defer fm.Close()
	n, e = m.GetNumExprsWithFastMark1(root, fm)
	astTestMust(t, e)
	if n != 2 {
		t.Fatal(n)
	}
	n, e = m.GetNumExprsWithFastMark1(root, fm)
	astTestMust(t, e)
	if n != 1 {
		t.Fatal("unshared root repeated, shared constant skipped", n)
	}
}
func TestASTExprVisitorSourceSkolemAndQuantifierNoPatterns(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(0, b)
	astTestMust(t, e)
	info, e := m.NewFuncDeclInfo(NullFamilyID, -1, nil, ASTFuncFlags{Skolem: true}, false)
	astTestMust(t, e)
	dc, e := m.MkFuncDecl(astTestSymbol(t, m, "visitor-sk0"), nil, b, info)
	astTestMust(t, e)
	c, e := m.MkApp(dc, nil)
	astTestMust(t, e)
	astTestPin(t, m, c)
	yes, e := m.HasSkolemFunctions(c)
	astTestMust(t, e)
	if yes {
		t.Fatal("nullary skolem is not a skolem function")
	}
	ds, e := m.MkFuncDecl(astTestSymbol(t, m, "visitor-sk1"), []ASTHandle{b}, b, info)
	astTestMust(t, e)
	s, e := m.MkApp(ds, []ASTHandle{v})
	astTestMust(t, e)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")}, Body: c, NoPatterns: []ASTHandle{s}})
	astTestMust(t, e)
	astTestPin(t, m, q)
	yes, e = m.HasSkolemFunctions(q)
	astTestMust(t, e)
	if !yes {
		t.Fatal("no-pattern child must be visited")
	}
	n, e := m.GetNumExprs(q)
	astTestMust(t, e)
	if n != 4 {
		t.Fatal("expr-only body/no-pattern traversal", n)
	}
}

// A bounded independent recursive oracle uses public expression accessors and
// a handle set. It intentionally has no declaration/sort traversal.
func exprVisitorCountOracle(t *testing.T, m *ASTManager, h ASTHandle, seen map[ASTHandle]bool, all bool) uint32 {
	t.Helper()
	rc, e := m.GetRefCount(h)
	astTestMust(t, e)
	if all || rc > 1 {
		if seen[h] {
			return 0
		}
		seen[h] = true
	}
	k, e := m.GetKind(h)
	astTestMust(t, e)
	n := uint32(1)
	switch k {
	case ASTApp:
		count, e := m.GetNumArgs(h)
		astTestMust(t, e)
		for i := uint32(0); i < count; i++ {
			a, e := m.GetArg(h, i)
			astTestMust(t, e)
			n += exprVisitorCountOracle(t, m, a, seen, all)
		}
		d, e := m.GetDecl(h)
		astTestMust(t, e)
		dr, e := m.internal(d)
		astTestMust(t, e)
		flags, e := m.funcFlags(dr)
		astTestMust(t, e)
		if flags.LeftAssociative && flags.RightAssociative {
			n += count - 2
		}
	case ASTQuantifier:
		body, e := m.GetQuantifierBody(h)
		astTestMust(t, e)
		n += exprVisitorCountOracle(t, m, body, seen, all)
		count, e := m.GetNumPatterns(h)
		astTestMust(t, e)
		for i := uint32(0); i < count; i++ {
			a, e := m.GetPattern(h, i)
			astTestMust(t, e)
			n += exprVisitorCountOracle(t, m, a, seen, all)
		}
		count, e = m.GetNumNoPatterns(h)
		astTestMust(t, e)
		for i := uint32(0); i < count; i++ {
			a, e := m.GetNoPattern(h, i)
			astTestMust(t, e)
			n += exprVisitorCountOracle(t, m, a, seen, all)
		}
	case ASTVar:
	default:
		t.Fatal("expression oracle received non-expression", k)
	}
	return n
}
func TestASTExprVisitorSourceUnsignedAssociativityAndOracle(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "assoc-x"), b)
	astTestMust(t, e)
	astTestPin(t, m, x)
	// Direct core constructs source app layout; source wrapper has distinct
	// normalization for nullary/unary assoc arguments. These are legal core
	// app representations and preserve source unsigned arithmetic verbatim.
	for fi, flags := range []ASTFuncFlags{{LeftAssociative: true}, {RightAssociative: true}, {LeftAssociative: true, RightAssociative: true}} {
		info, e := m.NewFuncDeclInfo(NullFamilyID, -1, nil, flags, false)
		astTestMust(t, e)
		d, e := m.MkFuncDecl(astTestSymbol(t, m, []string{"left-f", "right-f", "both-f"}[fi]), []ASTHandle{b, b}, b, info)
		astTestMust(t, e)
		maxArgs := 2
		if flags.LeftAssociative && flags.RightAssociative {
			maxArgs = 5
		}
		for argc := 0; argc <= maxArgs; argc++ {
			args := make([]ASTHandle, argc)
			for i := range args {
				args[i] = x
			}
			dr, e := m.internal(d)
			astTestMust(t, e)
			r, e := m.mkAppCore(dr, args)
			astTestMust(t, e)
			h := m.external(r)
			pin := astTestPin(t, m, h)
			_ = pin
			want := exprVisitorCountOracle(t, m, h, make(map[ASTHandle]bool), false)
			got, e := m.GetNumExprs(h)
			astTestMust(t, e)
			if got != want {
				t.Fatal(flags, argc, got, want)
			}
			if flags.LeftAssociative && flags.RightAssociative && argc == 0 && got != ^uint32(0) {
				t.Fatal("nullary unsigned subtract", got)
			}
		}
	}
}
func TestASTExprVisitorSourceEarlyFoundErasesLaterMarkAllocation(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(61000, b)
	astTestMust(t, e)
	info, e := m.NewFuncDeclInfo(NullFamilyID, -1, nil, ASTFuncFlags{Skolem: true}, false)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "early-sk"), []ASTHandle{b}, b, info)
	astTestMust(t, e)
	s, e := m.MkApp(d, []ASTHandle{v})
	astTestMust(t, e)
	c, e := m.MkConst(astTestSymbol(t, m, "later-shared"), b)
	astTestMust(t, e)
	astTestPin(t, m, c)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, m, "qv")}, Body: s, NoPatterns: []ASTHandle{c}})
	astTestMust(t, e)
	astTestPin(t, m, q)
	before := m.BudgetField.Used()
	graph := m.Stats()
	m.BudgetField.FailAfter(0)
	yes, e := m.HasSkolemFunctions(q)
	if e != nil || !yes {
		t.Fatal("found before no-pattern shared mark", yes, e)
	}
	if m.BudgetField.Used() != before || m.Stats() != graph || m.exprVisit.active {
		t.Fatal("unread later child reserved storage or retained frame")
	}
	m.BudgetField.FailAfter(-1)
}
func TestASTExprVisitorSourceStackEveryGrowthPrefixAndWarmBound(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	h, e := m.MkVar(62000, b)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "deep-f"), []ASTHandle{b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	for i := 0; i < 70; i++ {
		h, e = m.MkApp(d, []ASTHandle{h})
		astTestMust(t, e)
	}
	astTestPin(t, m, h)
	before := m.BudgetField.Used()
	graph := m.Stats()
	success := false
	for after := int64(0); after < 8; after++ {
		m.BudgetField.FailAfter(after)
		n, e := m.GetNumExprs(h)
		// Inspect before lifting the actual reservation fault.
		if m.BudgetField.Used() != before || m.Stats() != graph || m.exprVisit.active || m.exprVisit.bytes != 0 {
			t.Fatal("lost/retained actual lexical backing", after, e)
		}
		if e == nil {
			if n != 71 || after != 3 {
				t.Fatal("inline16 then32/64/128 source growth", after, n)
			}
			success = true
			m.BudgetField.FailAfter(-1)
			break
		}
		if e != arena.ErrInjectedFailure && !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal(after, e)
		}
		m.BudgetField.FailAfter(-1)
	}
	if !success {
		t.Fatal("did not reach first actual success")
	}
	for i := 0; i < 12; i++ {
		n, e := m.GetNumExprs(h)
		astTestMust(t, e)
		if n != 71 || m.BudgetField.Used() != before {
			t.Fatal("warm storage leak", n)
		}
	}
}

func TestASTExprVisitorSourceMarkedBeforeTrailAppendOwnsExactPendingLeaf(t *testing.T) {
	success := false
	for after := int64(0); after < 6; after++ {
		m := astTestManager(t)
		b, e := m.MkBoolSort()
		astTestMust(t, e)
		const count = 80
		args := make([]ASTHandle, count)
		dom := make([]ASTHandle, count)
		for i := range args {
			args[i], e = m.MkVar(uint32(63000+i), b)
			astTestMust(t, e)
			astTestMust(t, m.IncRef(args[i]))
			dom[i] = b
		}
		d, e := m.MkFuncDecl(astTestSymbol(t, m, "wide-f"), dom, b, ASTFuncDeclInfo{})
		astTestMust(t, e)
		root, e := m.MkApp(d, args)
		astTestMust(t, e)
		astTestMust(t, m.IncRef(root))
		before := m.BudgetField.Used()
		graph := m.Stats()
		m.BudgetField.FailAfter(after)
		n, e := m.GetNumExprs(root)
		f := &m.exprVisit
		if m.BudgetField.Used() != before || m.Stats() != graph {
			t.Fatal("lexical backing untracked before lift", after, e)
		}
		if e == nil {
			if after != 3 || n != count+1 || f.active || f.pendingMark.slot != 0 {
				t.Fatal("first complete trail growth", after, n)
			}
			success = true
			m.BudgetField.FailAfter(-1)
			astTestMust(t, m.Close())
			break
		}
		if !errors.Is(e, arena.ErrInjectedFailure) || !errors.Is(f.bodyCause, arena.ErrInjectedFailure) || f.sourceCause != nil || f.localFast.size != 0 || !f.active {
			t.Fatal("source unwind frontier", after, e, *f)
		}
		pending := f.pendingMark
		owner := f.pendingOwner
		serial := f.invocation
		body := f.bodyCause
		idx := 16 << uint(after)
		want, e2 := m.internal(args[idx])
		astTestMust(t, e2)
		if pending != want || owner == 0 {
			t.Fatal("actual failing append leaf", after, pending, want)
		}
		yes, e2 := m.nodeMark(pending, 1, owner, 0, false)
		astTestMust(t, e2)
		if !yes {
			t.Fatal("source DT cleared unlisted marked leaf")
		}
		for i := 0; i < idx; i++ {
			r, e2 := m.internal(args[i])
			astTestMust(t, e2)
			if m.nodeAt(r.slot).hasFlag(astFirstMark) {
				t.Fatal("source completed trail mark remained", i)
			}
		}
		// Exact manager-owned leaf ACK, still under the same injected refusal.
		m.mu.Lock()
		e2 = m.cleanupExprVisitFrame()
		m.mu.Unlock()
		astTestMust(t, e2)
		if f.active || f.pendingMark.slot != 0 || f.invocation != serial || f.bodyCause != body || f.sourceCause != nil || f.ownCause != nil || f.unlistedNode != pending || f.unlistedOwner != owner || m.nodeAt(pending.slot).hasFlag(astFirstMark) {
			t.Fatal("own ACK replayed body or lost history")
		}
		if m.BudgetField.Used() != before || m.Stats() != graph {
			t.Fatal("own ACK cold graph", after)
		}
		m.BudgetField.FailAfter(-1)
		astTestMust(t, m.Close()) // real enclosing context end, not a lexical public ACK
		if f.unlistedNode != pending || f.unlistedOwner != owner || f.bodyCause != body {
			t.Fatal("context-end diagnostics lost")
		}
	}
	if !success {
		t.Fatal("did not cross every actual trail acquisition")
	}
}

func TestASTExprVisitorSourceBorrowedMarksForeignClosedAndCopyGuards(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "guard-x"), b)
	astTestMust(t, e)
	astTestPin(t, m, x)
	em, e := NewASTExprMark(m)
	astTestMust(t, e)
	defer em.Close()
	fm, e := NewASTFastMark1(m)
	astTestMust(t, e)
	other := astTestManager(t)
	foreign, e := NewASTExprMark(other)
	astTestMust(t, e)
	defer foreign.Close()
	before := m.BudgetField.Used()
	graph := m.Stats()
	if _, e = m.GetNumExprsWithExprMark(x, foreign); !errors.Is(e, ErrASTHandle) {
		t.Fatal("foreign mark accepted", e)
	}
	copied := *em
	if _, e = m.GetNumExprsWithExprMark(x, &copied); !errors.Is(e, ErrASTHandle) {
		t.Fatal("copied mark accepted", e)
	}
	astTestMust(t, fm.Close())
	afterClose := m.BudgetField.Used()
	if _, e = m.GetNumExprsWithFastMark1(x, fm); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("closed mark accepted", e)
	}
	if m.BudgetField.Used() != afterClose || m.exprVisit.active || m.Stats().LiveNodes != graph.LiveNodes || before < afterClose {
		t.Fatal("guard mutated owner")
	}
}

func TestASTExprVisitorSourceReleaseRetryKeepsOriginalUnlistedMarkHistory(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	args := make([]ASTHandle, 18)
	domain := make([]ASTHandle, 18)
	for i := range args {
		args[i], e = m.MkVar(uint32(64000+i), b)
		astTestMust(t, e)
		astTestPin(t, m, args[i])
		domain[i] = b
	}
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "retry-f"), domain, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	root, e := m.MkApp(d, args)
	astTestMust(t, e)
	astTestPin(t, m, root)
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	_, e = m.GetNumExprs(root)
	if !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal(e)
	}
	pending := m.exprVisit.pendingMark
	owner := m.exprVisit.pendingOwner
	serial := m.exprVisit.invocation
	body := m.exprVisit.bodyCause
	if pending.slot == 0 || owner == 0 || m.BudgetField.Used() != before {
		t.Fatal("missing actual unlisted mark")
	}
	// Go archive-header refusal precedes overwriting the actual lexical frame.
	m.BudgetField.FailAfter(0)
	_, e = m.GetNumExprs(root)
	if !errors.Is(e, arena.ErrInjectedFailure) || m.exprVisit.pendingMark != pending || m.exprVisit.invocation != serial || m.exprVisit.bodyCause != body || m.BudgetField.Used() != before {
		t.Fatal("archive refusal changed exact prior action", e)
	}
	m.BudgetField.FailAfter(-1)
	n, e := m.GetNumExprs(root)
	if ASTSourceDebug {
		if !errors.Is(e, ErrASTDomain) {
			t.Fatal("source DEBUG mark owner predicate", e)
		}
	} else if e != nil || n != 18 {
		t.Fatal("source release sees original mark and skips it, no repaired count", n, e)
	}
	if m.exprVisitHistory == nil || m.exprVisitHistory.node != pending || m.exprVisitHistory.owner != owner || m.exprVisitHistory.invocation != serial || m.exprVisitHistory.bodyCause != body {
		t.Fatal("lost exact source-ended action")
	}
	history := m.exprVisitHistory
	m.mu.Lock()
	e = m.cleanupExprVisitFrame()
	m.mu.Unlock()
	astTestMust(t, e)
	if history == nil || !history.acknowledged || history.node != pending || history.owner != owner || history.invocation != serial || history.bodyCause != body {
		t.Fatal("archived exact tuple lost after own ACK")
	}
	if m.exprVisitHistory != history || m.nodeAt(pending.slot).hasFlag(astFirstMark) || m.BudgetField.Used() != before+history.bytes || m.Stats().AllocatedBytes < history.bytes {
		t.Fatal("own leaf ACK lost enclosing-context charged history")
	}
}

func TestASTExprVisitorSourceFinalizeEndsNodesWithoutAdoptingUnlistedMark(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	budget := arena.NewBudget(1 << 30)
	m, e := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: budget})
	astTestMust(t, e)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	args := make([]ASTHandle, 36)
	domain := make([]ASTHandle, 36)
	for i := 0; i < 18; i++ {
		v, e := m.MkVar(uint32(65000+i), b)
		astTestMust(t, e)
		args[i*2] = v
		args[i*2+1] = v
		domain[i*2] = b
		domain[i*2+1] = b
	}
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "dt-f"), domain, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	h, e := m.MkApp(d, args)
	astTestMust(t, e)
	budget.FailAfter(0)
	_, e = m.GetNumExprs(h)
	if !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal(e)
	}
	f := &m.exprVisit
	pending := f.pendingMark
	owner := f.pendingOwner
	serial := f.invocation
	body := f.bodyCause
	if budget.Used() != m.Stats().AllocatedBytes || !f.active {
		t.Fatal("actual source-ended mark graph")
	}
	budget.FailAfter(-1) // only after the exact stopped graph above
	// Source ast_manager destruction owns the node context, not this lexical
	// helper's owning acknowledgment. Its real node release ends the raw token.
	astTestMust(t, m.SourceFinalize())
	if f.pendingMark != pending || f.pendingOwner != owner || f.invocation != serial || f.bodyCause != body || !f.active || !m.sourceDTCompleted || m.nodeAt(pending.slot).live {
		t.Fatal("source DT repaired/replayed the helper")
	}
	if budget.Used() != m.Stats().AllocatedBytes {
		t.Fatal("remaining owning context graph")
	}
	budget.FailAfter(-1)
	astTestMust(t, m.Close())
	if budget.Used() != 0 || f.bodyCause != body || f.active {
		t.Fatal("ended generation own ACK lost history")
	}
}
func TestASTExprVisitorSourcePendingMarkCannotClearForeignActualMark(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	args := make([]ASTHandle, 17)
	domain := make([]ASTHandle, 17)
	for i := range args {
		args[i], e = m.MkVar(uint32(66000+i), b)
		astTestMust(t, e)
		astTestPin(t, m, args[i])
		domain[i] = b
	}
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "foreign-mark-f"), domain, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	h, e := m.MkApp(d, args)
	astTestMust(t, e)
	astTestPin(t, m, h)
	before := m.BudgetField.Used()
	m.BudgetField.FailAfter(0)
	_, e = m.GetNumExprs(h)
	if !errors.Is(e, arena.ErrInjectedFailure) {
		t.Fatal(e)
	}
	f := &m.exprVisit
	token := f.pendingMark
	owner := f.pendingOwner
	serial := f.invocation
	body := f.bodyCause
	actual := m.external(token)
	astTestMust(t, m.ResetNodeMark1(actual, owner))
	const foreign = uint64(9223372036854775000)
	astTestMust(t, m.MarkNode1(actual, true, foreign))
	m.mu.Lock()
	e = m.cleanupExprVisitFrame()
	m.mu.Unlock()
	if !errors.Is(e, ErrASTDomain) || !errors.Is(f.ownCause, ErrASTDomain) || f.pendingMark != token || f.pendingOwner != owner || f.invocation != serial || f.bodyCause != body || !m.nodeAt(token.slot).hasFlag(astFirstMark) || m.nodeMarkOwner(token.slot, 1) != foreign || m.BudgetField.Used() != before {
		t.Fatal("foreign mark adopted", e)
	}
	// Real owner actions remove the foreign mark; then exact old invocation can
	// observe its action already consumed without touching any new owner.
	astTestMust(t, m.ResetNodeMark1(actual, foreign))
	m.mu.Lock()
	e = m.cleanupExprVisitFrame()
	m.mu.Unlock()
	astTestMust(t, e)
	if !errors.Is(f.ownCause, ErrASTDomain) || f.bodyCause != body || f.active || m.BudgetField.Used() != before {
		t.Fatal("own retry lost immutable history")
	}
	m.BudgetField.FailAfter(-1)
}

func TestASTExprVisitorSourcePatternBodyOrderAndAllMarkedUnreadStack(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(67000, b)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "pattern-f"), []ASTHandle{b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	a, e := m.MkApp(d, []ASTHandle{v})
	astTestMust(t, e)
	p, e := m.MkFamilyApp(2, 0, nil, []ASTHandle{a})
	astTestMust(t, e)
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, m, "pv")}, Body: a, Patterns: []ASTHandle{p}})
	astTestMust(t, e)
	astTestPin(t, m, q)
	want := exprVisitorCountOracle(t, m, q, make(map[ASTHandle]bool), false)
	got, e := m.GetNumExprs(q)
	astTestMust(t, e)
	if got != want || got != 4 {
		t.Fatal("body before pattern with shared app skip", got, want)
	}
	em, e := NewASTExprMark(m)
	astTestMust(t, e)
	defer em.Close()
	got, e = m.GetNumExprsWithExprMark(q, em)
	astTestMust(t, e)
	if got != 4 {
		t.Fatal(got)
	}
	before := m.BudgetField.Used()
	serial := m.exprVisit.invocation
	m.BudgetField.FailAfter(0)
	got, e = m.GetNumExprsWithExprMark(q, em)
	if e != nil || got != 0 || m.BudgetField.Used() != before || m.exprVisit.invocation != serial {
		t.Fatal("root all-mark shortcut constructed unread stack", got, e)
	}
	m.BudgetField.FailAfter(-1)
}

func TestASTExprVisitorSourceSkolemParentProcFollowsDeepChild(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	h, e := m.MkVar(68000, b)
	astTestMust(t, e)
	d, e := m.MkFuncDecl(astTestSymbol(t, m, "before-parent-f"), []ASTHandle{b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	for i := 0; i < 24; i++ {
		h, e = m.MkApp(d, []ASTHandle{h})
		astTestMust(t, e)
	}
	info, e := m.NewFuncDeclInfo(NullFamilyID, -1, nil, ASTFuncFlags{Skolem: true}, false)
	astTestMust(t, e)
	sd, e := m.MkFuncDecl(astTestSymbol(t, m, "after-child-sk"), []ASTHandle{b}, b, info)
	astTestMust(t, e)
	root, e := m.MkApp(sd, []ASTHandle{h})
	astTestMust(t, e)
	astTestPin(t, m, root)
	before := m.BudgetField.Used()
	graph := m.Stats()
	m.BudgetField.FailAfter(0)
	found, e := m.HasSkolemFunctions(root)
	if found || !errors.Is(e, arena.ErrInjectedFailure) || m.exprVisit.active || m.BudgetField.Used() != before || m.Stats() != graph {
		t.Fatal("parent skolem proc bypassed actual child stack growth", found, e)
	}
	m.BudgetField.FailAfter(-1)
	found, e = m.HasSkolemFunctions(root)
	astTestMust(t, e)
	if !found {
		t.Fatal("parent proc after completed child")
	}
}
