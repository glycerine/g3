package arith

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func bv2RealWideCachePair() (*big.Rat, *big.Rat) {
	d := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 100), big.NewInt(3))
	r := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 90), big.NewInt(7))
	return new(big.Rat).SetInt(d), new(big.Rat).SetInt(r)
}
func TestBV2RealCacheOwnsProjectRationalCopiesAcrossReset(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	a, b := f.bv(t, 1, 3), f.bv(t, 2, 5)
	out := bv2RealOut(t, f.m)
	d, r := bv2RealWideCachePair()
	wantD, wantR := d.RatString(), r.RatString()
	service := f.m.NumeralService()
	before, err := service.Stats()
	equationMust(t, err)
	equationMust(t, u.MkBV2RealC(a, b, d, r, out))
	h, err := f.m.RefGet(out)
	equationMust(t, err)
	decl, err := f.m.GetDecl(h)
	equationMust(t, err)
	if len(u.cacheOwners) != 1 || !u.cacheOwners[0].forward {
		t.Fatal("missing source cache signature owner")
	}
	entry := u.cacheOwners[0]
	for i := range entry.values {
		value, err := service.String(&entry.values[i])
		equationMust(t, err)
		want := wantD
		if i%2 == 1 {
			want = wantR
		}
		if value != want {
			t.Fatal("cache rational copy", i, value, want)
		}
	}
	after, err := service.Stats()
	equationMust(t, err)
	if after.LiveNumericObjects <= before.LiveNumericObjects {
		t.Fatal("wide signatures have no project numeral payloads", before, after)
	}
	// Caller values are independent; equivalent keys reuse the same declaration.
	d.SetInt64(1)
	r.SetInt64(1)
	originalD, _ := new(big.Rat).SetString(wantD)
	originalR, _ := new(big.Rat).SetString(wantR)
	equationMust(t, u.MkBV2RealC(a, b, originalD, originalR, out))
	same, err := f.m.RefGet(out)
	equationMust(t, err)
	sameDecl, err := f.m.GetDecl(same)
	equationMust(t, err)
	if sameDecl != decl || len(u.cacheOwners) != 1 {
		t.Fatal("cache hit copied persistent signatures again")
	}
	hit, err := service.Stats()
	equationMust(t, err)
	if hit.LiveNumericObjects != after.LiveNumericObjects {
		t.Fatal("cache lookup left local signature payloads", after, hit)
	}
	equationMust(t, u.Reset())
	if len(u.forward) != 1 || len(u.reverse) != 1 || len(u.cacheOwners) != 1 {
		t.Fatal("reset destroyed source cache")
	}
	for i := range entry.values {
		if _, err := service.String(&entry.values[i]); err != nil {
			t.Fatal("reset invalidated cache numerator", i, err)
		}
	}
	equationMust(t, u.Close())
	for i, value := range entry.values {
		if value != (unumeral.OwnedRational{}) {
			t.Fatal("cache numeral not destroyed", i)
		}
	}
	closed, err := service.Stats()
	equationMust(t, err)
	if closed.LiveNumericObjects != before.LiveNumericObjects {
		t.Fatal("cache rational payload leaked", before, closed)
	}
}

func TestBV2RealCacheInsertionFailurePublicationPrefixes(t *testing.T) {
	succeeded, forwardOnly := false, false
	for stop := int64(0); stop < 512; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			u, f := bv2RealFixture(t, 64)
			a, b := f.bv(t, 1, 3), f.bv(t, 2, 5)
			out := bv2RealOut(t, f.m)
			d, r := bv2RealWideCachePair()
			f.m.Budget().FailAfter(stop)
			err := u.MkBV2RealC(a, b, d, r, out)
			f.m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				var failure umemory.Failure
				if !errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory {
					t.Fatal(err)
				}
			}
			if len(u.reverse) > len(u.forward) {
				t.Fatal("reverse signature published before forward key")
			}
			if len(u.forward) == 1 && len(u.reverse) == 0 {
				forwardOnly = true
				key := bv2RealKey{first: 3, second: 5, divisor: d.RatString(), root: r.RatString()}
				decl := u.forward[key]
				known, e := u.IsEncodingDecl(decl)
				equationMust(t, e)
				if known {
					t.Fatal("source reverse entry appeared after a failed second insertion")
				}
				equationMust(t, u.MkBV2RealC(a, b, d, r, out))
				app, e := f.m.RefGet(out)
				equationMust(t, e)
				actual, e := f.m.GetDecl(app)
				equationMust(t, e)
				if actual != decl || len(u.reverse) != 0 || len(u.cacheOwners) != 1 {
					t.Fatal("cache hit repaired or duplicated a source partial insertion")
				}
			}
			for _, entry := range u.cacheOwners {
				if entry.forward {
					for i := 0; i < 2; i++ {
						if entry.values[i] == (unumeral.OwnedRational{}) {
							t.Fatal("published key lost project numeral ownership", i)
						}
					}
				}
			}
			equationMust(t, u.Close())
		})
		if succeeded {
			break
		}
	}
	if !succeeded || !forwardOnly {
		t.Fatal("source insertion stages not reached", succeeded, forwardOnly)
	}
}

func TestBV2RealCacheHitEndsSignatureNumericScope(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	a, b := f.bv(t, 1, 3), f.bv(t, 2, 5)
	out := bv2RealOut(t, f.m)
	d, r := bv2RealWideCachePair()
	equationMust(t, u.MkBV2RealC(a, b, d, r, out))
	c := &bv2RealCall{u: u}
	h := c.encoding(a, b, d, r)
	equationMust(t, c.err)
	if h.IsNull() || len(c.numbers) != 0 || len(c.refs) == 0 {
		t.Fatal("signature rationals must end before app handoff", h, len(c.numbers), len(c.refs))
	}
	equationMust(t, c.close())
	// The public result remains independent of the short-lived lookup signature.
	value, err := f.m.RefGet(out)
	equationMust(t, err)
	if value.IsNull() {
		t.Fatal("lost encoding result")
	}
}

func TestBV2RealCacheLookupReadsOwnedReverseValues(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	a, b := f.bv(t, 1, 3), f.bv(t, 2, 5)
	out := bv2RealOut(t, f.m)
	d, r := bv2RealWideCachePair()
	equationMust(t, u.MkBV2RealC(a, b, d, r, out))
	h, err := f.m.RefGet(out)
	equationMust(t, err)
	decl, err := f.m.GetDecl(h)
	equationMust(t, err)
	entry := u.cacheOwners[0]
	signature := u.reverse[decl]
	if signature.divisor != &entry.values[2] || signature.root != &entry.values[3] {
		t.Fatal("reverse map does not reference its owned source signature")
	}
	service := f.m.NumeralService()
	equationMust(t, service.SetInt64(signature.divisor, 5))
	equationMust(t, service.SetInt64(signature.root, 3))
	aout, bout := bv2RealOut(t, f.m), bv2RealOut(t, f.m)
	var divisor, root big.Rat
	before, err := service.Stats()
	equationMust(t, err)
	matched, err := u.Match(h, aout, bout, &divisor, &root)
	equationMust(t, err)
	if !matched || divisor.Cmp(big.NewRat(5, 1)) != 0 || root.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("match read detached metadata", matched, &divisor, &root)
	}
	after, err := service.Stats()
	equationMust(t, err)
	if after.LiveNumericObjects != before.LiveNumericObjects {
		t.Fatal("local reverse signature copy leaked", before, after)
	}
	// Forward and reverse signatures are independent copies in the source maps.
	text, err := service.String(&entry.values[0])
	equationMust(t, err)
	if text != d.RatString() {
		t.Fatal("reverse value mutation changed forward key", text, d)
	}
}
