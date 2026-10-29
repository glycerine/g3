package rewriter

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func enum2BVSingleton(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: astTestSymbol(t, m, "SingletonEnum"),
		Constructors: []ast.DatatypeConstructorDefinition{{
			Name:       astTestSymbol(t, m, "only-singleton"),
			Recognizer: astTestSymbol(t, m, "is-only-singleton"),
		}},
	}})
	astTestMust(t, err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	astTestMust(t, err)
	if len(constructors) != 1 {
		t.Fatal("singleton datatype constructor count", len(constructors))
	}
	return sorts[0], constructors[0]
}

// Source: src/ast/rewriter/enum2bv_rewriter.cpp:79-133,203-210.
func TestEnum2BVSingletonFreshAndCachedArgumentSourceBehavior(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	singleton, only := enum2BVSingleton(t, m)
	r, err := enum2BVNewForTest(t, m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })

	constant, err := m.MkConst(astTestSymbol(t, m, "singleton-value"), singleton)
	astTestMust(t, err)
	astTestPin(t, m, constant)
	out, proof := enum2BVRefs(t, m)
	converted, err := r.reduceArgument(constant, &out)
	astTestMust(t, err)
	if !converted {
		t.Fatal("singleton constant was not converted")
	}
	first, err := m.RefGet(out)
	astTestMust(t, err)
	convertedSort, err := m.GetSort(first)
	astTestMust(t, err)
	if width, e := m.BitVectorSize(convertedSort); e != nil || width != 1 {
		t.Fatal("singleton source bit width", width, e)
	}
	if len(r.bounds) != 1 || len(r.entries) != 1 || len(r.enum2BV) != 1 || len(r.bv2Enum) != 1 || len(r.enum2Def) != 1 || r.translated != 1 {
		t.Fatal("singleton publication prefix", len(r.bounds), len(r.entries), len(r.enum2BV), len(r.bv2Enum), len(r.enum2Def), r.translated)
	}
	zero, err := m.BitVectorNumeralUnsigned(0, 1)
	astTestMust(t, err)
	upper, err := m.GetArg(r.bounds[0], 1)
	astTestMust(t, err)
	if upper != zero {
		t.Fatal("nc == 1 did not publish the explicit one-bit <= 0 bound", upper, zero)
	}
	originalDecl, err := m.GetDecl(constant)
	astTestMust(t, err)
	definition := r.enum2Def[originalDecl]
	definitionDecl, err := m.GetDecl(definition)
	astTestMust(t, err)
	if definitionDecl != only {
		t.Fatal("singleton definition did not use its only constructor", definitionDecl, only)
	}

	// A source cache hit returns before bounds, maps, vectors, and the translated
	// counter are touched. It only constructs the mapped fresh application.
	astTestMust(t, m.SetRef(out, ast.ASTHandle{}))
	converted, err = r.reduceArgument(constant, &out)
	astTestMust(t, err)
	second, err := m.RefGet(out)
	astTestMust(t, err)
	if !converted || second != first || len(r.bounds) != 1 || len(r.entries) != 1 || len(r.enum2BV) != 1 || len(r.bv2Enum) != 1 || len(r.enum2Def) != 1 || r.translated != 1 {
		t.Fatal("cached source conversion changed owning state", converted, first, second, len(r.bounds), len(r.entries), r.translated)
	}
	_ = proof
}

// Source: src/ast/rewriter/enum2bv_rewriter.cpp:152-210.
func TestEnum2BVMixedQuantifierSingletonBoundsAndBinderOrder(t *testing.T) {
	for _, forall := range []bool{false, true} {
		t.Run(fmt.Sprintf("forall=%t", forall), func(t *testing.T) {
			m := astTestManager(t)
			astTestMust(t, m.RegisterDeclarationPlugins())
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			singleton, _ := enum2BVSingleton(t, m)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			body, err := m.MkTrue()
			astTestMust(t, err)
			astTestPin(t, m, body)
			names := []usymbol.Symbol{
				astTestSymbol(t, m, "singleton-left"),
				astTestSymbol(t, m, "ordinary-middle"),
				astTestSymbol(t, m, "singleton-right"),
			}
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall:   forall,
				Sorts:    []ast.ASTHandle{singleton, boolean, singleton},
				Names:    names,
				Body:     body,
				Weight:   19,
				QID:      astTestSymbol(t, m, "mixed-singleton-qid"),
				SkolemID: astTestSymbol(t, m, "mixed-singleton-skid"),
			})
			astTestMust(t, err)
			astTestPin(t, m, q)
			asserted, err := m.MkProofAsserted(body)
			astTestMust(t, err)
			out := astTestPin(t, m, q)
			proof := astTestPin(t, m, asserted)
			r, err := enum2BVNewForTest(t, m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, r.Close()) })

			changed, err := r.config.ReduceQuantifier(q, body, ast.SliceArguments(nil), ast.SliceArguments(nil), out, proof)
			astTestMust(t, err)
			result, err := m.RefGet(out)
			astTestMust(t, err)
			cleared, err := m.RefGet(proof)
			astTestMust(t, err)
			if !changed || result == q || !cleared.IsNull() {
				t.Fatal("mixed quantifier publication/proof order", changed, result, cleared)
			}
			bv1, err := m.BitVectorSort(1)
			astTestMust(t, err)
			for i, want := range []ast.ASTHandle{bv1, boolean, bv1} {
				got, e := m.GetDeclarationSort(result, uint32(i))
				astTestMust(t, e)
				if got != want {
					t.Fatal("mixed declaration order", i, got, want)
				}
				name, e := m.GetDeclarationName(result, uint32(i))
				astTestMust(t, e)
				if name != names[i] {
					t.Fatal("mixed declaration name", i, name, names[i])
				}
			}
			rewritten, err := m.GetQuantifierBody(result)
			astTestMust(t, err)
			var bounds ast.ASTHandle
			if forall {
				if kind, e := m.GetDeclKind(rewritten); e != nil || kind != ast.ASTOpImplies {
					t.Fatal("forall singleton guard", kind, e)
				}
				bounds, err = m.GetArg(rewritten, 0)
				astTestMust(t, err)
				retainedBody, e := m.GetArg(rewritten, 1)
				astTestMust(t, e)
				if retainedBody != body {
					t.Fatal("forall body changed", retainedBody, body)
				}
			} else {
				bounds = rewritten
				if kind, e := m.GetDeclKind(bounds); e != nil || kind != ast.ASTOpAnd {
					t.Fatal("exists singleton guard", kind, e)
				}
				retainedBody, e := m.GetArg(bounds, 2)
				astTestMust(t, e)
				if retainedBody != body {
					t.Fatal("exists body publication order", retainedBody, body)
				}
			}
			if kind, e := m.GetDeclKind(bounds); e != nil || kind != ast.ASTOpAnd {
				t.Fatal("mixed bound conjunction", kind, e)
			}
			zero, err := m.BitVectorNumeralUnsigned(0, 1)
			astTestMust(t, err)
			for i, wantIndex := range []uint32{2, 0} {
				bound, e := m.GetArg(bounds, uint32(i))
				astTestMust(t, e)
				variable, e := m.GetArg(bound, 0)
				astTestMust(t, e)
				index, e := m.GetVarIndex(variable)
				astTestMust(t, e)
				upper, e := m.GetArg(bound, 1)
				astTestMust(t, e)
				if index != wantIndex || upper != zero {
					t.Fatal("mixed singleton bound", i, index, wantIndex, upper, zero)
				}
			}
			weight, err := m.GetQuantifierWeight(result)
			astTestMust(t, err)
			qid, skid, err := m.GetQuantifierIDs(result)
			astTestMust(t, err)
			oldQID, oldSkid, err := m.GetQuantifierIDs(q)
			astTestMust(t, err)
			if weight != 19 || qid != oldQID || skid != oldSkid {
				t.Fatal("mixed quantifier metadata", weight, qid, skid)
			}
		})
	}
}
