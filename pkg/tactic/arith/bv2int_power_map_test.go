package arith

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func bv2IntPowerBucketKey(t *testing.T, m *ast.ASTManager, bucket uint32) ast.ASTHandle {
	t.Helper()
	for i := 0; i < 4096; i++ {
		h := fixDLConst(t, m, true, fmt.Sprintf("bucket-%d-%d", bucket, i))
		hash, err := m.GetHash(h)
		equationMust(t, err)
		if hash&15 == bucket {
			return h
		}
	}
	t.Fatal("no source hash bucket key", bucket)
	return ast.ASTHandle{}
}

func TestBV2IntPowerMapSourceCollisionGrowthAndReplacementOrder(t *testing.T) {
	r, m := bv2IntContextFixture(t)
	keys := map[uint32]ast.ASTHandle{}
	for _, bucket := range []uint32{7, 15, 0, 3, 4, 5, 6} {
		keys[bucket] = bv2IntPowerBucketKey(t, m, bucket)
		equationMust(t, r.insertPower2(keys[bucket], keys[bucket]))
	}
	check := func(want []uint32) {
		t.Helper()
		entries, err := r.power2Entries()
		equationMust(t, err)
		if len(entries) != len(want) {
			t.Fatal("source map size", len(entries), want)
		}
		for i, bucket := range want {
			if entries[i].key != keys[bucket] {
				t.Fatal("source bucket traversal order", i, bucket)
			}
		}
	}
	// Two keys wrap from bucket seven; a direct bucket-zero key probes again.
	check([]uint32{15, 0, 3, 4, 5, 6, 7})
	if len(r.power2) != 8 {
		t.Fatal("source grows only before the next insertion")
	}
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	err := r.insertPower2(keys[7], keys[0])
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || len(r.power2) != 8 || m.Budget().Used() != before {
		t.Fatal("failed pre-replacement growth changed the table", err)
	}
	check([]uint32{15, 0, 3, 4, 5, 6, 7})
	value, yes, err := r.IsPower2(keys[7])
	equationMust(t, err)
	if !yes || value != keys[7] {
		t.Fatal("failed growth published the replacement")
	}
	equationMust(t, r.insertPower2(keys[7], keys[0]))
	if len(r.power2) != 16 || r.power2Size != 7 {
		t.Fatal("replacement skipped source load check or changed key count")
	}
	check([]uint32{0, 3, 4, 5, 6, 7, 15})
	value, yes, err = r.IsPower2(keys[7])
	equationMust(t, err)
	if !yes || value != keys[0] {
		t.Fatal("replacement not visible to source lookup")
	}
	snapshot, err := r.Power2()
	equationMust(t, err)
	delete(snapshot, keys[7])
	check([]uint32{0, 3, 4, 5, 6, 7, 15})
}

func TestBV2IntPowerMapSparseResetFailureRetryAndDestruction(t *testing.T) {
	r, m := bv2IntContextFixture(t)
	var key ast.ASTHandle
	for i := 0; i < 26; i++ {
		key = fixDLConst(t, m, true, fmt.Sprintf("sparse-%d", i))
		equationMust(t, r.insertPower2(key, key))
	}
	if len(r.power2) != 64 {
		t.Fatal("source load thresholds", len(r.power2))
	}
	equationMust(t, r.Reset())
	if len(r.power2) != 64 || r.power2Size != 0 {
		t.Fatal("dense reset should preserve table storage")
	}
	equationMust(t, r.insertPower2(key, key))
	m.Budget().FailAfter(0)
	err := r.Reset()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || len(r.power2) != 64 {
		t.Fatal("sparse reset replacement allocation", err)
	}
	if _, _, err = r.IsPower2(key); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("stopped reset allowed reuse", err)
	}
	equationMust(t, r.Reset())
	if len(r.power2) != 32 || r.power2Size != 0 {
		t.Fatal("source sparse reset halves storage")
	}
	equationMust(t, r.insertPower2(key, key))
	equationMust(t, r.Reset())
	if len(r.power2) != 16 {
		t.Fatal("second sparse reset did not halve again")
	}
	equationMust(t, r.Close())
	if r.power2 != nil || r.power2Bytes != 0 {
		t.Fatal("table storage survived destruction")
	}
	if _, err = m.GetKind(key); err != nil {
		t.Fatal("borrowed map key was released", err)
	}
}

func TestNLA2BVPowerSubstitutionUsesSourceBucketOrder(t *testing.T) {
	r := nla2BVFixture(t, false)
	m := r.manager
	var forms []ast.ASTHandle
	for i := 0; i < 16; i++ {
		x := fixDLConst(t, m, true, fmt.Sprintf("ordered-power-%d", i))
		forms = append(forms, bv2IntDisjunction(t, m, x, []string{"1", "2", "4"}))
	}
	g := fixDLGoal(t, m, false, false, false, forms...)
	c, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, c.close()) })
	equationMust(t, c.collectPower2(g))
	entries, err := c.integer.power2Entries()
	equationMust(t, err)
	if len(entries) != 16 || len(c.trail) != 16 {
		t.Fatal("missing power substitutions")
	}
	for i, entry := range entries {
		shift, err := m.GetArg(c.trail[i], 0)
		equationMust(t, err)
		exponent, err := m.GetArg(shift, 1)
		equationMust(t, err)
		if exponent != entry.value {
			t.Fatal("NLA2BV substituted in a different order from source buckets", i)
		}
	}
}
