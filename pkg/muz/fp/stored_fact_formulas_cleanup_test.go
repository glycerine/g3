package fp

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestStoredFactFormulaEnumerationReleasesFailedPrefix(t *testing.T) {
	failures, successes := 0, 0
	for allowance := int64(0); allowance < 256; allowance++ {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			domain, err := m.BitVectorSort(4)
			fragmentMust(t, err)
			symbol, err := m.Symbols.Intern("stored-cleanup-p")
			fragmentMust(t, err)
			d, err := m.MkFuncDecl(symbol, []ast.ASTHandle{domain}, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, e.RegisterPredicate(d))
			for i := uint32(0); i < 12; i++ {
				fragmentMust(t, e.AddTableFact(d, []uint32{i}))
			}
			baseline := m.Stats().LiveReferences
			m.Budget().FailAfter(allowance)
			refs, err := e.StoredFactFormulas()
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				if refs != nil {
					t.Fatal("failed stored fact enumeration published prefix")
				}
			} else {
				successes++
				if len(refs) != 12 {
					t.Fatal("stored fact count", len(refs))
				}
				for i := range refs {
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}
			if got := m.Stats().LiveReferences; got != baseline {
				t.Fatal("stored fact enumeration leaked owning prefix", baseline, got, err)
			}
			if err != nil {
				refs, cause := e.StoredFactFormulas()
				fragmentMust(t, cause)
				if len(refs) != 12 {
					t.Fatal("retry changed stored tuples", len(refs))
				}
				for i := range refs {
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}
		})
		if successes > 0 {
			break
		}
	}
	if failures == 0 || successes == 0 {
		t.Fatal("allocation scan missed refusal or success", failures, successes)
	}
}

func TestStoredFactFormulasPreservePredicateAndTableOrder(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	domain, err := m.BitVectorSort(4)
	fragmentMust(t, err)
	type expectedFact struct {
		declaration ast.ASTHandle
		value       uint64
	}
	var expected []expectedFact
	for index, values := range [][]uint32{{9, 2, 7}, {3, 1}} {
		name, cause := m.Symbols.Intern(fmt.Sprintf("stored-order-%d", index))
		fragmentMust(t, cause)
		d, cause := m.MkFuncDecl(name, []ast.ASTHandle{domain}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(d))
		for _, value := range values {
			fragmentMust(t, e.AddTableFact(d, []uint32{value}))
		}
		facts, cause := e.predicates[d].table.Facts()
		fragmentMust(t, cause)
		for _, fact := range facts {
			expected = append(expected, expectedFact{d, uint64(fact[0])})
		}
	}
	refs, err := e.StoredFactFormulas()
	fragmentMust(t, err)
	if len(refs) != len(expected) {
		t.Fatal("stored output count", len(refs))
	}
	for i, ref := range refs {
		application, cause := m.RefGet(ref)
		fragmentMust(t, cause)
		d, cause := m.GetDecl(application)
		fragmentMust(t, cause)
		argument, cause := m.GetArg(application, 0)
		fragmentMust(t, cause)
		value, width, matched, cause := m.BitVectorNumeralBigInt(argument)
		fragmentMust(t, cause)
		if d != expected[i].declaration || !matched || width != 4 || value.Uint64() != expected[i].value {
			t.Fatal("stored output order/value", i, value, expected[i].value)
		}
		fragmentMust(t, m.DelRef(&refs[i]))
	}
}
