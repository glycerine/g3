package ast

// Source: src/test/sorting_network.cpp:150-181.

import (
	"testing"
)

func TestPsortASTContextActualFactoriesClausesAndTrail(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	x, e := c.Fresh()
	astTestMust(t, e)
	y, e := c.Fresh()
	astTestMust(t, e)
	n, e := c.Not(x)
	astTestMust(t, e)
	z, e := c.Not(n)
	astTestMust(t, e)
	if z != x {
		t.Fatal("source double-not returns borrowed child", z, x)
	}
	a, e := c.Min(x, y)
	astTestMust(t, e)
	b, e := c.Max(x, y)
	astTestMust(t, e)
	if a == b {
		t.Fatal("actual AND and OR")
	}
	astTestMust(t, c.Clause(0, ASTLiteralVectorSpan{}))
	trail := len(c.trail)
	clauses := len(c.clauses)
	if trail != 5 || clauses != 1 {
		t.Fatal("literal factory trail and independent clauses", trail, clauses)
	}
	astTestMust(t, c.SourceFinalize())
	astTestMust(t, c.Close())
	if m.IsLiveHandle(x) {
		t.Fatal("source owned trail reference remained")
	}
}

func TestPsortASTContextRetainedConsumerTransfers(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	astTestMust(t, c.SetFreshPrefix("sn"))
	fresh, e := c.Fresh()
	astTestMust(t, e)
	if n, e := c.FreshCount(); e != nil || n != 1 || c.freshPrefix != "sn" {
		t.Fatal("fresh specialization state", n, c.freshPrefix, e)
	}
	if got, e := c.FreshApplication(0); e != nil || got != fresh {
		t.Fatal("borrowed fresh application", got, fresh, e)
	}
	if e = c.SetFreshPrefix("late"); e != ErrASTPending {
		t.Fatal("late prefix update", e)
	}
	astTestMust(t, c.Clause(0, ASTLiteralVectorSpan{}))
	if n, e := c.ClauseCount(); e != nil || n != 1 {
		t.Fatal("clause count", n, e)
	}
	clauses, e := c.TakeClauses()
	astTestMust(t, e)
	if len(clauses) != 1 {
		t.Fatal("transferred clauses", len(clauses))
	}
	before, e := m.GetRefCount(clauses[0])
	astTestMust(t, e)
	astTestMust(t, c.Close())
	after, e := m.GetRefCount(clauses[0])
	astTestMust(t, e)
	if after != before {
		t.Fatal("context consumed transferred clause", before, after)
	}
	astTestMust(t, m.DecRef(clauses[0]))
}

func TestPsortASTContextActualConstructorRefusalPrefixes(t *testing.T) {
	success := int64(-1)
	for ordinal := int64(0); ordinal < 16; ordinal++ {
		m := astTestManager(t)
		external := m.BudgetField.Used() - m.Stats().AllocatedBytes
		m.BudgetField.FailAfter(ordinal)
		c, cause := NewPsortASTContext(m)
		cold := external
		if c != nil {
			cold += c.header
			if cause != nil {
				t.Fatal("actual complete constructor publication", ordinal, cause)
			}
		}
		if m.BudgetField.Used() != m.Stats().AllocatedBytes+cold {
			t.Fatal("immediate failed child/header graph", ordinal, m.BudgetField.Used(), cold)
		}
		m.BudgetField.FailAfter(-1)
		if c != nil {
			astTestMust(t, c.Close())
		}
		if cause == nil {
			success = ordinal
			break
		}
	}
	if success < 0 {
		t.Fatal("constructor actual first success not reached")
	}
	t.Log("actual constructor first success", success)
}

func TestPsortASTContextNativeWideTrailAndClauseFinalization(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	var args [32]ASTHandle
	for i := range args {
		args[i], e = m.MkVar(uint32(10000+i), sort)
		astTestMust(t, e)
	}
	wide, e := m.MkFamilyApp(0, ASTOpAnd, nil, args[:])
	astTestMust(t, e)
	f, e := m.MkFalse()
	astTestMust(t, e)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	_, e = c.Max(wide, f)
	astTestMust(t, e)
	astTestMust(t, c.Clause(0, ASTLiteralVectorSpan{}))
	external := m.BudgetField.Used() - m.Stats().AllocatedBytes - c.header
	m.BudgetField.FailAfter(0)
	cause := c.SourceFinalize()
	astTestMust(t, cause)
	if c.stage != 2 || c.destructorCause != nil {
		t.Fatal("native trail and clause finalization", c.stage, c.destructorCause)
	}
	if m.BudgetField.Used() != m.Stats().AllocatedBytes+c.header+external {
		t.Fatal("actual finalized member graph")
	}
	before := m.Stats()
	astTestMust(t, c.SourceFinalize())
	if m.Stats() != before {
		t.Fatal("repeated finalization changed AST ownership")
	}
	astTestMust(t, c.Close())
	if c.destructorCause != nil || c.stage != 2 || !c.closed {
		t.Fatal("native owning closure")
	}
	m.BudgetField.FailAfter(-1)
}

func TestPsortASTContextNativeRepeatedFactoriesExactReferences(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	x, e := c.Fresh()
	astTestMust(t, e)
	n, e := c.Not(x)
	astTestMust(t, e)
	astTestMust(t, m.IncRef(x))
	astTestMust(t, m.IncRef(n))
	literals, e := NewASTLiteralVector(m.BudgetField)
	astTestMust(t, e)
	defer literals.Close()
	astTestMust(t, literals.Push(x))
	span, e := literals.Span()
	astTestMust(t, e)
	xr, e := m.GetRefCount(x)
	astTestMust(t, e)
	nr, e := m.GetRefCount(n)
	astTestMust(t, e)
	used := m.BudgetField.Used()
	// Debug factories retain their actual well-sorted allocation checks.
	if !ASTSourceDebug {
		m.BudgetField.FailAfter(0)
	}
	for i := 0; i < 40; i++ {
		got, e := c.Not(x)
		astTestMust(t, e)
		if got != n {
			t.Fatal("cached NOT identity", got, n)
		}
		astTestMust(t, c.Clause(1, span))
		got, e = c.Not(n)
		astTestMust(t, e)
		if got != x {
			t.Fatal("borrowed double NOT", got, x)
		}
	}
	if !ASTSourceDebug && m.BudgetField.Used() != used {
		t.Fatal("native storage charged as source vector backing")
	}
	m.BudgetField.FailAfter(-1)
	if len(c.trail) != 42 || len(c.clauses) != 40 {
		t.Fatal("append prefix", len(c.trail), len(c.clauses))
	}
	for _, h := range c.trail[2:] {
		if h != n {
			t.Fatal("trail order")
		}
	}
	for _, h := range c.clauses {
		if h != x {
			t.Fatal("clause order")
		}
	}
	gotX, e := m.GetRefCount(x)
	astTestMust(t, e)
	gotN, e := m.GetRefCount(n)
	astTestMust(t, e)
	if gotX != xr+40 || gotN != nr+40 {
		t.Fatal("exact owning appends and borrowed double NOT", xr, nr, gotX, gotN)
	}
	copied := *c
	if e = copied.Close(); e != ErrASTHandle {
		t.Fatal("copied owner", e)
	}
	astTestMust(t, c.SourceFinalize())
	gotX, e = m.GetRefCount(x)
	astTestMust(t, e)
	gotN, e = m.GetRefCount(n)
	astTestMust(t, e)
	if gotX != xr-1 || gotN != nr-1 {
		t.Fatal("retired context references", xr, nr, gotX, gotN)
	}
	if _, e = c.Fresh(); e != ErrASTPending {
		t.Fatal("finalized context accepted new ownership", e)
	}
	astTestMust(t, c.Close())
	astTestMust(t, c.Close())
	astTestMust(t, m.DecRef(n))
	astTestMust(t, m.DecRef(x))
}

func TestPsortASTContextEscapedNativeStorageAfterManagerClose(t *testing.T) {
	m := astTestManager(t)
	c, e := NewPsortASTContext(m)
	astTestMust(t, e)
	_, e = c.Fresh()
	astTestMust(t, e)
	astTestMust(t, c.Clause(0, ASTLiteralVectorSpan{}))
	astTestMust(t, m.Close())
	astTestMust(t, c.Close())
	if c.trail != nil || c.clauses != nil || c.header != 0 {
		t.Fatal("escaped native owner retained storage")
	}
	astTestMust(t, c.Close())
}
