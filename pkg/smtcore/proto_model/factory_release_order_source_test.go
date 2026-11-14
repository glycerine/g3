package protomodel

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Each value/sort comes from the actual retained concrete specialization. The
// test observes the manager's source deletion records, not a destructor hook.
func TestFactorySourceForwardReferenceDestruction(t *testing.T) {
	for _, kind := range []string{"datalog", "user", "arith", "bv", "array", "datatype"} {
		t.Run(kind, func(t *testing.T) {
			owner, _, config := protoFixtureWithConfig(t)
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: owner.Symbols, Numerals: owner.NumeralService(), Budget: owner.Budget(), TraceTags: tags})
			protoMust(t, err)
			protoMust(t, m.RegisterDeclarationPlugins())
			t.Cleanup(func() { protoMust(t, m.Close()) })
			var factory ValueFactory
			var sort ast.ASTHandle
			var prefix []ast.ASTHandle
			var constructors []ast.ASTHandle
			switch kind {
			case "datalog":
				f, err := NewDatalogFactory(m)
				protoMust(t, err)
				factory = f
				// Empty per-sort sets have no value holding their sort alive. They
				// expose both sort-vector order and sort-before-value member order.
				for i := 0; i < 2; i++ {
					empty, err := m.MkDatalogFiniteSort(protoSymbol(t, m, fmt.Sprintf("empty-domain-%d", i)), 8)
					protoMust(t, err)
					_, err = f.valueSet(empty)
					protoMust(t, err)
					prefix = append(prefix, empty)
				}
				sort, err = m.MkDatalogFiniteSort(protoSymbol(t, m, "owned-domain"), 8)
				protoMust(t, err)
			case "user":
				f := newUserSortFactory(m)
				factory = f
				for i := 0; i < 2; i++ {
					empty, err := m.MkUninterpretedSort(protoSymbol(t, m, fmt.Sprintf("empty-universe-%d", i)), nil)
					protoMust(t, err)
					_, err = f.valueSet(empty)
					protoMust(t, err)
					prefix = append(prefix, empty)
				}
				sort, err = m.MkUninterpretedSort(protoSymbol(t, m, "owned-universe"), nil)
				protoMust(t, err)
			case "arith":
				f, err := NewArithmeticFactory(m)
				protoMust(t, err)
				factory = f
				sort, err = m.MkFamilySort(f.FamilyID(), 0, nil)
				protoMust(t, err)
			case "array":
				md, err := NewProtoModel(m, config)
				protoMust(t, err)
				t.Cleanup(func() { protoMust(t, md.Close()) })
				f, err := NewArrayFactory(md)
				protoMust(t, err)
				factory = f
				index, err := m.MkBoolSort()
				protoMust(t, err)
				for i := 0; i < 2; i++ {
					emptyRange, err := m.MkUninterpretedSort(protoSymbol(t, m, fmt.Sprintf("empty-array-range-%d", i)), nil)
					protoMust(t, err)
					empty, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{index}), emptyRange)
					protoMust(t, err)
					_, err = f.valueSet(empty)
					protoMust(t, err)
					prefix = append(prefix, empty)
				}
				rangeSort, err := m.BitVectorSort(8)
				protoMust(t, err)
				sort, err = m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{index}), rangeSort)
				protoMust(t, err)
			case "datatype":
				md, err := NewProtoModel(m, config)
				protoMust(t, err)
				t.Cleanup(func() { protoMust(t, md.Close()) })
				f, err := NewDatatypeFactory(md)
				protoMust(t, err)
				factory = f
				var definitions []ast.DatatypeConstructorDefinition
				for i := 0; i < 4; i++ {
					definitions = append(definitions, ast.DatatypeConstructorDefinition{Name: protoSymbol(t, m, fmt.Sprintf("release-ctor-%d", i)), Recognizer: protoSymbol(t, m, fmt.Sprintf("is-release-ctor-%d", i))})
				}
				sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: protoSymbol(t, m, "ReleaseEnum"), Constructors: definitions}})
				protoMust(t, err)
				sort = sorts[0]
				constructors, err = m.DatatypeConstructors(sort)
				protoMust(t, err)
			case "bv":
				f, err := NewBitVectorFactory(m)
				protoMust(t, err)
				factory = f
				sort, err = m.BitVectorSort(8)
				protoMust(t, err)
			}
			t.Cleanup(func() { protoMust(t, factory.Close()) })
			var values []ast.ASTHandle
			for position, n := range []uint64{3, 1, 5, 7} {
				var value ast.ASTHandle
				switch kind {
				case "datalog":
					value, err = m.MkDatalogNumeral(n, sort)
				case "user":
					value, err = m.MkModelValue(uint32(n), sort)
				case "arith":
					value, err = m.ArithmeticNumeralRat(big.NewRat(int64(n), 2), false)
				case "array":
					number, e := m.BitVectorNumeralUint64(n, 8)
					protoMust(t, e)
					value, err = m.MkConstArray(sort, number)
				case "datatype":
					value, err = m.MkApp(constructors[position], nil)
				case "bv":
					value, err = m.BitVectorNumeralUint64(n, 8)
				}
				protoMust(t, err)
				protoMust(t, factory.RegisterValue(value))
				values = append(values, value)
			}
			// An independent older owner keeps the last value and its sort alive.
			// It must survive the factory's own forward destruction.
			held, err := m.NewRef(values[3])
			protoMust(t, err)
			defer func() { protoMust(t, m.DelRef(&held)) }()
			heldCount, err := m.GetRefCount(values[3])
			protoMust(t, err)
			expected := append(prefix, values[:3]...)
			var ids []uint32
			for _, node := range expected {
				count, err := m.GetRefCount(node)
				protoMust(t, err)
				if count != 1 {
					t.Fatal("deletion-order fixture requires one real factory owner", node, count)
				}
				id, err := m.GetASTID(node)
				protoMust(t, err)
				ids = append(ids, id)
			}
			tags.Enable("ast")
			output.Reset()
			protoMust(t, factory.Close())
			after := output.String()
			for _, node := range expected {
				if m.IsLiveHandle(node) {
					t.Fatal("factory left its sole-owned node alive", node)
				}
			}
			count, err := m.GetRefCount(values[3])
			protoMust(t, err)
			if count+1 != heldCount {
				t.Fatal("factory consumed caller ownership or leaked its value", count, heldCount)
			}
			if tags.ShouldTrace("ast") {
				previous := -1
				for _, id := range ids {
					needle := fmt.Sprintf("Deleting object %d ", id)
					at := strings.Index(after, needle)
					if at <= previous || strings.Count(after, needle) != 1 {
						t.Fatalf("source forward reference release order for %v: id%d at%d after%d\n%s", kind, id, at, previous, after)
					}
					previous = at
				}
			} else if after != "" {
				t.Fatal("erased trace emitted deletion records", after)
			}
			protoMust(t, factory.Close())
			if output.String() != after {
				t.Fatal("repeated Close replayed source deletion")
			}
		})
	}
}
