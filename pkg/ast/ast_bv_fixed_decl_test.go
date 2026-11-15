package ast

// Source: src/ast/bv_decl_plugin.cpp:181-347,470-519.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"
)

func TestASTBitVectorOriginalFixedDeclarations(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	for _, width := range []uint32{1, 8, 65} {
		s, e := m.BitVectorSort(width)
		astTestMust(t, e)
		for _, kind := range append(append([]int32{}, bvFixedTestKinds()...), 1, 2, 59, 60) {
			arity := 2
			if kind == 3 || kind >= 12 && kind <= 16 || kind == 32 || kind == 42 || kind == 43 {
				arity = 1
			}
			if kind == 1 || kind == 2 {
				arity = 0
			}
			if kind == 59 || kind == 60 {
				arity = 3
			}
			domain := make([]ASTHandle, arity)
			for i := range domain {
				domain[i] = s
			}
			if kind == 59 || kind == 60 {
				b, e := m.MkBoolSort()
				astTestMust(t, e)
				for i := range domain {
					domain[i] = b
				}
			}
			d, e := m.MkFamilyFuncDecl(fid, kind, nil, domain, ASTHandle{})
			astTestMust(t, e)
			astTestPin(t, m, d)
			got, e := m.GetArity(d)
			astTestMust(t, e)
			if got != uint32(arity) {
				t.Fatal("source operator arity", width, kind, got, arity)
			}
			ref, e := m.internal(d)
			astTestMust(t, e)
			flags, e := m.funcFlags(ref)
			astTestMust(t, e)
			ac := kind == 4 || kind == 6 || kind == 30 || kind == 31 || kind == 33
			if flags.LeftAssociative != ac || flags.RightAssociative != ac || flags.FlatAssociative != ac || flags.Commutative != (ac || kind == 44) || flags.Idempotent != (kind == 30 || kind == 31) {
				t.Fatal("source fixed flags", width, kind, flags)
			}
			r, e := m.GetRange(d)
			astTestMust(t, e)
			want := s
			if kind >= 22 && kind <= 29 || kind >= 52 && kind <= 54 || kind == 59 || kind == 60 {
				want, e = m.MkBoolSort()
				astTestMust(t, e)
			}
			if kind == 42 || kind == 43 || kind == 44 || kind == 1 || kind == 2 {
				want, e = m.BitVectorSort(1)
				astTestMust(t, e)
			}
			if r != want {
				t.Fatal("source result sort", width, kind, r, want)
			}
			m.Budget().FailAfter(0)
			again, e := m.MkFamilyFuncDecl(fid, kind, []ASTParameter{{}}, domain, ASTHandle{})
			m.Budget().FailAfter(-1)
			astTestMust(t, e)
			if again != d {
				t.Fatal("cached identity/ignored parameter", width, kind)
			}
		}
	}
}

func bvFixedTestKinds() []int32 {
	var kinds []int32
	for k := int32(3); k <= 36; k++ {
		kinds = append(kinds, k)
	}
	for k := int32(42); k <= 47; k++ {
		kinds = append(kinds, k)
	}
	return append(kinds, 50, 51, 52, 53, 54)
}

func TestASTBitVectorFixedValidationPublicationAndApplications(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	other, e := m.BitVectorSort(16)
	astTestMust(t, e)
	boolSort, e := m.MkBoolSort()
	astTestMust(t, e)
	for _, row := range []struct {
		kind    int32
		domain  []ASTHandle
		message string
	}{
		{4, nil, "no arguments supplied to bit-vector operator"},
		{4, []ASTHandle{boolSort, boolSort}, "could not extract bit-vector size"},
		{5, []ASTHandle{s}, "declared arity mismatches supplied arity"},
		{6, []ASTHandle{s, other}, "declared sorts do not match supplied sorts"},
	} {
		_, e := m.MkFamilyFuncDecl(fid, row.kind, nil, row.domain, ASTHandle{})
		if e == nil || e.Error() != row.message {
			t.Fatal(row, e)
		}
	}
	for _, kind := range []int32{5, 6} {
		r := m.bitVectorPlugin.fixed[kind][8]
		if r.slot == 0 || m.nodeAt(r.slot).rc != 1 {
			t.Fatal("validation must follow cache INC", kind, r)
		}
	}
	// Associative factory checks its two formal domains and ignores the tail.
	d, e := m.MkFamilyFuncDecl(fid, 4, nil, []ASTHandle{s, s, other}, ASTHandle{})
	astTestMust(t, e)
	astTestPin(t, m, d)
	x, e := m.MkConst(astTestSymbol(t, m, "bv-fixed-x"), s)
	astTestMust(t, e)
	astTestPin(t, m, x)
	y, e := m.MkConst(astTestSymbol(t, m, "bv-fixed-other"), other)
	astTestMust(t, e)
	astTestPin(t, m, y)
	if _, e = m.MkFamilyApp(fid, 4, nil, []ASTHandle{x, x, y}); e == nil {
		t.Fatal("application must check ignored factory tail")
	}
	args := make([]ASTHandle, 18)
	for i := range args {
		args[i] = x
	}
	h, e := m.MkFamilyApp(fid, 4, nil, args)
	astTestMust(t, e)
	astTestPin(t, m, h)
	n, e := m.GetNumArgs(h)
	astTestMust(t, e)
	if n != 18 {
		t.Fatal("associative application", n)
	}
	// Source eager constants ignore the supplied signature and parameters.
	for _, k := range []int32{1, 2, 59, 60} {
		_, e := m.MkFamilyFuncDecl(fid, k, []ASTParameter{{}}, []ASTHandle{{}}, other)
		astTestMust(t, e)
	}
}

func TestASTBitVectorFixedEveryColdCacheReservation(t *testing.T) {
	success := false
	for ordinal := int64(0); ordinal < 80; ordinal++ {
		sy, q, _ := astTestOwners(t)
		b := arena.NewBudget(1 << 30)
		m, e := NewASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
		astTestMust(t, e)
		_, e = m.RegisterArithmeticPlugin()
		astTestMust(t, e)
		fid, e := m.RegisterBitVectorPlugin()
		astTestMust(t, e)
		s, e := m.BitVectorSort(65)
		astTestMust(t, e)
		b.FailAfter(ordinal)
		_, call := m.MkFamilyFuncDecl(fid, 4, nil, []ASTHandle{s, s}, ASTHandle{})
		if call != nil && !errors.Is(call, arena.ErrInjectedFailure) {
			t.Fatal(ordinal, call)
		}
		v := &m.bitVectorPlugin.fixed[4]
		if call != nil && len(*v) != 0 && len(*v) != 66 {
			t.Fatal("partial logical size publication", ordinal, len(*v))
		}
		if b.Used() != m.Stats().AllocatedBytes {
			t.Fatal("actual cache backing accounting", ordinal, b.Used(), m.Stats())
		}
		b.FailAfter(-1)
		_, e = m.MkFamilyFuncDecl(fid, 4, nil, []ASTHandle{s, s}, ASTHandle{})
		astTestMust(t, e)
		astTestMust(t, m.Close())
		if b.Used() != 0 {
			t.Fatal("owning closure", ordinal, b.Used())
		}
		if call == nil {
			success = true
			break
		}
	}
	if !success {
		t.Fatal("unreached successful cache construction")
	}
}

func TestASTBitVectorFixedPublishedBeforeRejectedINC(t *testing.T) {
	if !ASTSourceDebug {
		t.Skip("checked source counter boundary")
	}
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	_, e = m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	p := m.bitVectorPlugin
	s, e := p.mkSort(8)
	astTestMust(t, e)
	r, e := m.makePluginDeclaration(p.family, "bvsub", 5, []astNodeRef{s, s}, s, ASTFuncFlags{})
	astTestMust(t, e)
	m.nodeAt(r.slot).rc = ^uint32(0)
	_, e = p.fixedDecl(5, 8)
	if e != ErrASTDomain || p.fixedPending != r {
		t.Fatal("actual rejected cache INC", e, p.fixedPending, r)
	}
	captured := p.fixed[5][8]
	if captured != r {
		t.Fatal("source assignment before INC", captured, r)
	}
	if e = p.finalizeFixedPending(false); e != ErrASTPending {
		t.Fatal("source destruction must retain interrupted acquisition", e)
	}
	_, e = p.fixedDecl(4, 8)
	if e != ErrASTPending {
		t.Fatal("pending acquisition guard", e)
	}
	astTestMust(t, p.finalizeFixedPending(true))
	if m.nodeAt(r.slot).rc != ^uint32(0) {
		t.Fatal("own abort fabricated DEC")
	}
	captured = p.fixed[5][8]
	if captured.slot != 0 {
		t.Fatal("unacquired raw slot retained", captured)
	}
	m.nodeAt(r.slot).rc = 0
	_, e = p.fixedDecl(5, 8)
	astTestMust(t, e)
	if m.nodeAt(r.slot).rc != 1 {
		t.Fatal("actual retry acquisition")
	}
}

func TestASTBitVectorFixedCachesSourceFinalize(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	for _, width := range []uint32{1, 8, 65} {
		s, e := m.BitVectorSort(width)
		astTestMust(t, e)
		for _, k := range bvFixedTestKinds() {
			domain := []ASTHandle{s, s}
			if k == 3 || k >= 12 && k <= 16 || k == 32 || k == 42 || k == 43 {
				domain = domain[:1]
			}
			_, e = m.MkFamilyFuncDecl(fid, k, nil, domain, ASTHandle{})
			astTestMust(t, e)
		}
	}
	astTestMust(t, m.SourceFinalize())
	if m.bitVectorPlugin != nil {
		t.Fatal("completed fixed cache owner retained")
	}
	astTestMust(t, m.Close())
}

// AUDIT.md ast_02-F1: bv_decl_plugin::mk_func_decl's expression overload
// prints the mismatching argument and the declaration.
func TestASTBitVectorExprDeclArgumentMismatchMessage(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	s8, e := m.BitVectorSort(8)
	astTestMust(t, e)
	s16, e := m.BitVectorSort(16)
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "x"), s8)
	astTestMust(t, e)
	astTestPin(t, m, x)
	y, e := m.MkConst(astTestSymbol(t, m, "y"), s16)
	astTestMust(t, e)
	astTestPin(t, m, y)
	for _, row := range []struct {
		kind    int32
		message string
	}{
		{4, "Argument y at position 1 does not match declaration (declare-fun bvadd ((_ BitVec 8) (_ BitVec 8)) (_ BitVec 8))"},
		{22, "Argument y at position 1 does not match declaration (declare-fun bvule ((_ BitVec 8) (_ BitVec 8)) Bool)"},
	} {
		_, e := m.MkFamilyApp(fid, row.kind, nil, []ASTHandle{x, y})
		if e == nil || e.Error() != row.message {
			t.Fatalf("kind %d: %v", row.kind, e)
		}
	}
	_, e = m.MkBitVectorAddArguments(SliceArguments([]ASTHandle{x, y}))
	if e == nil || e.Error() != "Argument y at position 1 does not match declaration (declare-fun bvadd ((_ BitVec 8) (_ BitVec 8)) (_ BitVec 8))" {
		t.Fatalf("borrowed bvadd: %v", e)
	}
}

// AUDIT.md ast_02-F2: bv_decl_plugin::mk_sort rejects width 0 with its own
// message.
func TestASTBitVectorSortZeroWidthMessage(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	_, e = m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	if _, e = m.BitVectorSort(0); e == nil || e.Error() != "bit-vector size must be greater than zero" {
		t.Fatalf("zero width: %v", e)
	}
}
