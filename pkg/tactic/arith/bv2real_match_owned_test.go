package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:25-71,159-176,279-295.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Count only source-persistent owners whose population can change during
// this operation: the actual cached rationals and reusable integer cells.
func bv2RealRecognitionPersistentObjects(t *testing.T, s *unumeral.RationalService) uint32 {
	t.Helper()
	refs := map[unumeral.NumericCellRef]bool{}
	add := func(ref unumeral.NumericCellRef) {
		if ref == 0 {
			return
		}
		cell, err := s.Manager.Z.Cells.Read(arena.Slot(ref))
		equationMust(t, err)
		if !cell.Released {
			refs[ref] = true
		}
	}
	for i := range s.Cache {
		add(s.Cache[i].Num.Cell)
		add(s.Cache[i].Den.Cell)
	}
	for _, ref := range s.Manager.Z.Tmp {
		add(ref)
	}
	for _, ref := range s.Manager.Z.Arg {
		add(ref)
	}
	return uint32(len(refs))
}

func TestBV2RealMatchedProjectOwnersOutliveInputAndUtility(t *testing.T) {
	u, f := bv2RealFixture(t, 256)
	m := f.m
	s := m.NumeralService()
	sort, err := m.BitVectorSort(4)
	equationMust(t, err)
	a, err := m.MkFreshConst("owned-a", sort)
	equationMust(t, err)
	b, err := m.MkFreshConst("owned-b", sort)
	equationMust(t, err)
	d, r := bv2RealWideCachePair()
	out := bv2RealOut(t, m)
	equationMust(t, u.MkBV2RealC(a, b, d, r, out))
	h, err := m.RefGet(out)
	equationMust(t, err)
	before, err := s.Stats()
	equationMust(t, err)
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	term, ok := c.matchTerm(h)
	equationMust(t, c.err)
	if !ok || term.first != a || term.second != b {
		t.Fatal("cached term recognition")
	}
	after, err := s.Stats()
	equationMust(t, err)
	if after.LiveNumericObjects != before.LiveNumericObjects+2 {
		t.Fatal("caller divisor/root copies did not survive recognition", before, after)
	}
	equationMust(t, m.SetRef(out, ast.ASTHandle{}))
	equationMust(t, u.Close())
	for i, q := range []*big.Rat{d, r} {
		value := term.divisor
		if i == 1 {
			value = term.root
		}
		text, err := s.String(value)
		equationMust(t, err)
		if text != q.RatString() {
			t.Fatal("utility close damaged caller numeric owner", i, text)
		}
	}
	for _, h := range []ast.ASTHandle{a, b} {
		if _, err = m.GetKind(h); err != nil {
			t.Fatal("input close damaged caller expression owner", err)
		}
	}
	equationMust(t, c.close())
	for _, h := range []ast.ASTHandle{a, b} {
		if _, err = m.GetKind(h); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal("caller expression owner was not released", err)
		}
	}
}

func TestBV2RealOwnedMatchingAlignmentDoesNotMutateCache(t *testing.T) {
	u, f := bv2RealFixture(t, 256)
	makeEncoding := func(d int64) ast.ASTHandle {
		out := bv2RealOut(t, f.m)
		equationMust(t, u.MkBV2RealC(f.bv(t, 1, 4), f.bv(t, 2, 4), big.NewRat(d, 1), big.NewRat(3, 1), out))
		h, err := f.m.RefGet(out)
		equationMust(t, err)
		return h
	}
	left, right := makeEncoding(6), makeEncoding(10)
	c := &bv2RealCall{u: u}
	t.Cleanup(func() { equationMust(t, c.close()) })
	a, b, x, y, d1, d2, root, ok := c.pair(left, right)
	equationMust(t, c.err)
	if !ok {
		t.Fatal("pair recognition refused matching roots")
	}
	c.alignDivisorsOwned(a, b, x, y, d1, d2)
	equationMust(t, c.err)
	for _, q := range []*unumeral.OwnedRational{d1, d2} {
		text, err := f.m.NumeralService().String(q)
		equationMust(t, err)
		if text != "30" {
			t.Fatal("common divisor was not assigned to actual caller owner", text)
		}
	}
	text, err := f.m.NumeralService().String(root)
	equationMust(t, err)
	if text != "3" {
		t.Fatal(text)
	}
	for i, h := range []ast.ASTHandle{left, right} {
		decl, err := f.m.GetDecl(h)
		equationMust(t, err)
		sig := u.reverse[decl]
		text, err := f.m.NumeralService().String(sig.divisor)
		equationMust(t, err)
		want := "6"
		if i == 1 {
			want = "10"
		}
		if text != want {
			t.Fatal("caller alignment changed stored signature", text, want)
		}
	}
}

func TestBV2RealRealConstantOwnedReadAndFailureFrontiers(t *testing.T) {
	for _, integer := range []bool{false, true} {
		succeeded := false
		for stop := int64(0); stop < 2048; stop++ {
			t.Run(fmt.Sprintf("integer=%v/%d", integer, stop), func(t *testing.T) {
				u, f := bv2RealFixture(t, 256)
				text := "-18446744073709551619/36893488147419103233"
				if integer {
					text = "-18446744073709551619"
				}
				literal := fixDLNumber(t, f.m, integer, text)
				managedBefore := bv2RealRecognitionPersistentObjects(t, f.m.NumeralService())
				before, err := f.m.NumeralService().Stats()
				equationMust(t, err)
				c := &bv2RealCall{u: u}
				t.Cleanup(func() { equationMust(t, c.close()) })
				f.m.Budget().FailAfter(stop)
				term, found := c.matchTerm(literal)
				f.m.Budget().FailAfter(-1)
				if c.err == nil {
					succeeded = true
					if found == integer {
						t.Fatal("source integer/real classification", found, integer)
					}
					if found {
						d, err := f.m.NumeralService().String(term.divisor)
						equationMust(t, err)
						root, err := f.m.NumeralService().String(term.root)
						equationMust(t, err)
						if d != "36893488147419103233" || root != "2" {
							t.Fatal("constant denominator/root", d, root)
						}
						if got := bv2RealSignedValue(t, f, term.first); got.String() != "-18446744073709551619" {
							t.Fatal("constant numerator", got)
						}
					}
				} else {
					var failure umemory.Failure
					if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(c.err)
					}
				}
				equationMust(t, c.close())
				if len(c.readPending) != 0 {
					t.Fatal("arithmetic-read invocation escaped cleanup")
				}
				actual, flag, yes, readErr := f.m.ArithmeticNumeralText(literal)
				equationMust(t, readErr)
				if !yes || flag != integer || actual != text {
					t.Fatal("matching damaged the source numeral", actual, text, flag, integer)
				}
				// Source operations may consume/replenish reusable result cells and
				// grow the power cache. Compare call-owned payloads after accounting
				// for those actual persistent owners, rather than a fixed total.
				if c.err != nil || integer {
					after, err := f.m.NumeralService().Stats()
					equationMust(t, err)
					managedAfter := bv2RealRecognitionPersistentObjects(t, f.m.NumeralService())
					if after.LiveNumericObjects-managedAfter != before.LiveNumericObjects-managedBefore {
						t.Fatal("failed/refused numeral copy leaked call-owned project payloads", before, after, managedBefore, managedAfter)
					}
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("real numeral copy success frontier not reached", integer)
		}
	}
}

func TestBV2RealOwnedDefaultsAndConstructorFailureFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 256; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			_, f := bv2RealFixture(t, 64)
			f.m.Budget().FailAfter(stop)
			u, err := NewBV2RealUtil(f.m, ^uint32(0), ^uint32(0), 65)
			f.m.Budget().FailAfter(-1)
			if u != nil {
				t.Cleanup(func() { equationMust(t, u.Close()) })
			}
			if err == nil {
				succeeded = true
				root, divisor, maximum, err := u.Defaults()
				equationMust(t, err)
				if root.RatString() != "4294967295" || divisor.RatString() != "4294967295" || maximum.RatString() != "8589934590" {
					t.Fatal("uint default construction/product", root.RatString(), divisor.RatString(), maximum.RatString())
				}
				// The Go snapshot cannot alter the project field or subsequent encodings.
				root.SetInt64(7)
				actual, _, _, err := u.Defaults()
				equationMust(t, err)
				if actual.RatString() != "4294967295" {
					t.Fatal("default snapshot aliases owner")
				}
			} else {
				var failure umemory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(err)
				}
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("default constructor allocation success frontier not reached")
	}
}
