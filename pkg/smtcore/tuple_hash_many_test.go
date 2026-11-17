package smtcore

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Many instances of one quantifier land in one (quantifier, arity) group.
// fingerprint_set and cached_var_subst hash the whole tuple (Ivy runs put
// ~96k bindings in one group); a per-group linear scan made both quadratic.
const smtManyTuples = 30000

func smtManyTupleFixture(t *testing.T) (*ast.ASTManager, ast.ASTHandle, uint32, []*SMTEnode) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	bound, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(2400)}, Body: bound})
	astTestMust(t, err)
	astTestMust(t, m.IncRef(q))
	t.Cleanup(func() { astTestMust(t, m.DecRef(q)) })
	qID, err := m.GetASTID(q)
	astTestMust(t, err)
	nodes := make([]*SMTEnode, smtManyTuples)
	for i := range nodes {
		owner, node := smtEnodeConstant(t, m, uint32(10000+i), boolean, 0)
		astTestMust(t, m.IncRef(owner))
		h := owner
		t.Cleanup(func() { astTestMust(t, m.DecRef(h)) })
		node.root = node
		nodes[i] = node
	}
	return m, q, qID, nodes
}

func TestSMTFingerprintSetManyTuplesOfOneQuantifier(t *testing.T) {
	_, q, qID, nodes := smtManyTupleFixture(t)
	set := NewSMTFingerprintSet()
	start := time.Now()
	for _, n := range nodes {
		f, err := set.Insert(q, qID, []*SMTEnode{n})
		astTestMust(t, err)
		if f == nil {
			t.Fatal("distinct fingerprint rejected")
		}
	}
	astTestMust(t, set.PushScope())
	for _, n := range nodes {
		if f, err := set.Insert(q, qID, []*SMTEnode{n}); err != nil || f != nil {
			t.Fatal("duplicate fingerprint inserted", err)
		}
		if found, err := set.Contains(q, qID, []*SMTEnode{n}); err != nil || !found {
			t.Fatal("inserted fingerprint missing", err)
		}
	}
	astTestMust(t, set.PopScopes(1))
	if set.Size() != smtManyTuples {
		t.Fatal("size", set.Size())
	}
	t.Logf("%d fingerprints of one quantifier: %v", smtManyTuples, time.Since(start))
}

func TestSMTCachedVarSubstitutionManyBindingsOfOneQuantifier(t *testing.T) {
	m, q, _, nodes := smtManyTupleFixture(t)
	cache, err := NewSMTCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cache.Close()) })
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&out)) })
	start := time.Now()
	for _, n := range nodes {
		cached, err := cache.Instantiate(q, []*SMTEnode{n}, out)
		astTestMust(t, err)
		if cached {
			t.Fatal("first instantiation reported cached")
		}
	}
	for _, n := range nodes {
		cached, err := cache.Instantiate(q, []*SMTEnode{n}, out)
		astTestMust(t, err)
		if !cached {
			t.Fatal("repeated instantiation missed the cache")
		}
		v, err := m.RefGet(out)
		astTestMust(t, err)
		if v != n.owner {
			t.Fatal("cached instance is not the substituted body")
		}
	}
	if cache.Size() != smtManyTuples {
		t.Fatal("size", cache.Size())
	}
	t.Logf("%d instances of one quantifier: %v", smtManyTuples, time.Since(start))
}
