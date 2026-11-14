package ast

// Source: src/ast/ast.cpp:1354,2213-2238; src/ast/ast.h:1811-1825; src/test/sorting_network.cpp:150-181,184-199.

import (
	"reflect"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTFreshConstActualSourceCounterAndSkolemMetadata(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	a, e := m.MkFreshConst("a", b)
	astTestMust(t, e)
	astTestPin(t, m, a)
	x, e := m.MkFreshConst("x", b)
	astTestMust(t, e)
	astTestPin(t, m, x)
	for i, h := range []ASTHandle{a, x} {
		d, e := m.GetDecl(h)
		astTestMust(t, e)
		name, e := m.GetASTName(d)
		astTestMust(t, e)
		text, e := m.Symbols.String(name)
		astTestMust(t, e)
		expected := []string{"a!0", "x!1"}[i]
		if text != expected {
			t.Fatal("source fresh ID publication", text, expected)
		}
		info, e := m.funcInfo(m.declAt(d.slot).info)
		astTestMust(t, e)
		if !info.flags.Skolem || info.family != NullFamilyID || info.kind != -1 {
			t.Fatal("actual owned copy of local func_decl_info", info)
		}
	}
}

func TestASTFreshNameBorrowedBytesOwnedInterning(t *testing.T) {
	m := astTestManager(t)
	name := []byte{'a', 0, 'b'}
	s, e := m.Symbols.InternBytes(name)
	astTestMust(t, e)
	name[0] = 'x'
	text, e := m.Symbols.String(s)
	astTestMust(t, e)
	if text != "a" {
		t.Fatal("interner retained caller bytes", text)
	}
	same, e := m.Symbols.Intern("a")
	astTestMust(t, e)
	if same != s {
		t.Fatal("byte/string engine mismatch")
	}
}

func TestASTFreshFuncDeclNullAndSuffixSourceNames(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	suffix := astTestSymbol(t, m, "tail")
	plain, e := m.MkFreshFuncDecl(usymbol.Symbol{}, usymbol.Symbol{}, nil, b)
	astTestMust(t, e)
	astTestPin(t, m, plain)
	name, e := m.GetASTName(plain)
	astTestMust(t, e)
	if name != usymbol.Numerical(0) {
		t.Fatal("null/null numerical source name", name)
	}
	withSuffix, e := m.MkFreshFuncDecl(usymbol.Symbol{}, suffix, nil, b)
	astTestMust(t, e)
	astTestPin(t, m, withSuffix)
	name, e = m.GetASTName(withSuffix)
	astTestMust(t, e)
	text, e := m.Symbols.String(name)
	astTestMust(t, e)
	if text != "nullsk!tail!1" {
		t.Fatal("literal null insertion precedes sk", text)
	}
	if m.freshInfoActive || !reflect.DeepEqual(m.freshInfo, astInfoRecord{}) {
		t.Fatal("lexical info outlived source call")
	}
	if _, e = m.info(^uint32(0), m.freshInfoEpoch); e == nil {
		t.Fatal("ended lexical info token readable")
	}
}

func TestASTFreshFactoryEveryActualReservationAndLexicalOwner(t *testing.T) {
	success := false
	failures := 0
	for after := int64(0); after < 64; after++ {
		s, q, budget := astTestOwners(t)
		m, e := NewASTManager(ASTManagerConfig{Symbols: s, Numerals: q, Budget: budget})
		astTestMust(t, e)
		sort, e := m.MkBoolSort()
		astTestMust(t, e)
		name := astTestSymbol(t, m, "fresh-prefix-that-exceeds-the-sixty-four-byte-local-buffer-after-appending-an-ID")
		other := budget.Used() - astTestSymbolGraph(s) - m.Stats().AllocatedBytes
		budget.FailAfter(after)
		result, e := m.MkFreshFuncDecl(name, usymbol.Symbol{}, nil, sort)
		// Immediate source-completed graph before lifting the fault. The local
		// bytes.Buffer uses Go memory and is outside the injected arena budget.
		if budget.Used() != other+astTestSymbolGraph(s)+m.Stats().AllocatedBytes {
			t.Fatal("lost lexical overflow or source node", after, e, budget.Used())
		}
		if m.freshInfoActive || !reflect.DeepEqual(m.freshInfo, astInfoRecord{}) {
			t.Fatal("unfinished local info", after)
		}
		if e != nil {
			failures++
			if !result.IsNull() || m.freshID != 0 {
				t.Fatal("counter advanced before declaration success", after, result, m.freshID, e)
			}
		} else {
			if m.freshID != 1 {
				t.Fatal("missing counter publication")
			}
			success = true
		}
		budget.FailAfter(-1)
		astTestMust(t, m.Close())
		if budget.Used() != other+astTestSymbolGraph(s) {
			t.Fatal("source owner teardown", after, budget.Used())
		}
		if success {
			t.Log("actual first success", after)
			break
		}
	}
	if !success {
		t.Fatal("fresh factory did not complete", failures)
	}
}

func TestASTFreshConstDeclarationCopyPageAndApplicationPrefixes(t *testing.T) {
	success := false
	failures := 0
	for after := int64(0); after < 64; after++ {
		s, q, budget := astTestOwners(t)
		m, e := NewASTManager(ASTManagerConfig{Symbols: s, Numerals: q, Budget: budget})
		astTestMust(t, e)
		sort, e := m.MkBoolSort()
		astTestMust(t, e)
		// Reach the actual next info-page constructor by making real fresh declarations.
		// No counters, headers, allocations or failure receipts are edited.
		for (m.nextInfo-1)%astPageSize != 0 {
			_, e = m.MkFreshConst("warm", sort)
			astTestMust(t, e)
		}
		initialID := m.freshID
		other := budget.Used() - astTestSymbolGraph(s) - m.Stats().AllocatedBytes
		budget.FailAfter(after)
		result, e := m.MkFreshConst("next", sort)
		if budget.Used() != other+astTestSymbolGraph(s)+m.Stats().AllocatedBytes {
			t.Fatal("actual declaration/copy/app graph", after, e, budget.Used())
		}
		if m.freshInfoActive || !reflect.DeepEqual(m.freshInfo, astInfoRecord{}) {
			t.Fatal("lexical borrow retained", after)
		}
		if e != nil {
			failures++
			if !result.IsNull() || (m.freshID != initialID && m.freshID != initialID+1) {
				t.Fatal("counter frontier", after, result, m.freshID, e)
			}
		} else {
			success = true
			if m.freshID != initialID+1 {
				t.Fatal("counter did not follow declaration", after)
			}
		}
		budget.FailAfter(-1)
		astTestMust(t, m.Close())
		if budget.Used() != other+astTestSymbolGraph(s) {
			t.Fatal("untracked actual manager owner", after, budget.Used())
		}
		if success {
			t.Log("actual info-page/app first success", after)
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("no actual source ownership prefix", failures)
	}
}

func TestASTFreshConstApplicationFailureRetainsDeclarationCounter(t *testing.T) {
	success, sawPublishedFailure := false, false
	for after := int64(0); after < 64; after++ {
		s, q, budget := astTestOwners(t)
		m, e := NewASTManager(ASTManagerConfig{Symbols: s, Numerals: q, Budget: budget})
		astTestMust(t, e)
		sort, e := m.MkBoolSort()
		astTestMust(t, e)
		astTestAlignFreshAppToNodePage(t, m, sort, false)
		initialID := m.freshID
		other := budget.Used() - astTestSymbolGraph(s) - m.Stats().AllocatedBytes
		budget.FailAfter(after)
		result, e := m.MkFreshConst("next-app", sort)
		if budget.Used() != other+astTestSymbolGraph(s)+m.Stats().AllocatedBytes {
			t.Fatal("application failure named graph", after, e)
		}
		if m.freshInfoActive {
			t.Fatal("active lexical borrow", after)
		}
		if e != nil {
			if !result.IsNull() {
				t.Fatal("failed app published", after)
			}
			if m.freshID == initialID+1 {
				sawPublishedFailure = true
			}
		} else {
			success = true
		}
		budget.FailAfter(-1)
		astTestMust(t, m.Close())
		if budget.Used() != other+astTestSymbolGraph(s) {
			t.Fatal("application failed owner refund", after)
		}
		if success {
			t.Log("actual app-page first success", after)
			break
		}
	}
	if !success || !sawPublishedFailure {
		t.Fatal("source declaration publication before actual failed application", success, sawPublishedFailure)
	}
}

func TestASTFreshConstNullPointerPrefixSourceOverload(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	astTestMust(t, e)
	result, e := m.MkFreshConstNullable(nil, sort)
	astTestMust(t, e)
	astTestPin(t, m, result)
	decl, e := m.GetDecl(result)
	astTestMust(t, e)
	name, e := m.GetASTName(decl)
	astTestMust(t, e)
	if name != usymbol.Numerical(0) {
		t.Fatal("null source char prefix", name)
	}
}
