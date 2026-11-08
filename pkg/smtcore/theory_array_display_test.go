package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtArrayDisplayErrorWriter struct{}

func (smtArrayDisplayErrorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSMTArrayDisplayPrivateRootsParentsAndScopes(t *testing.T) {
	for _, simple := range []bool{true, false} {
		t.Run(fmt.Sprint(simple), func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArrayPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			var theory *SMTArrayTheory
			if simple {
				plugin, err := NewSMTSimpleArrayTheory(family)
				astTestMust(t, err)
				theory = plugin.SMTArrayTheory
				astTestMust(t, state.RegisterTheory(plugin))
			} else {
				theory, err = NewSMTArrayTheory(family)
				astTestMust(t, err)
				astTestMust(t, state.RegisterTheory(theory))
			}
			var output bytes.Buffer
			astTestMust(t, state.DisplayTheories(&output))
			if output.Len() != 0 {
				t.Fatal("empty array theory printed a header")
			}
			indexSort, indices := smtArrayQueueIndices(t, m, "array-display-index-", 1)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolean)
			astTestMust(t, err)
			base, err := m.MkConst(astTestSymbol(t, m, "array-display-base"), sort)
			astTestMust(t, err)
			alias, err := m.MkConst(astTestSymbol(t, m, "array-display-alias"), sort)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(base, false))
			astTestMust(t, state.Internalize(alias, false))
			baseNode, aliasNode := state.context.enodes.get(base), state.context.enodes.get(alias)
			baseID, err := m.GetASTID(base)
			astTestMust(t, err)
			aliasID, err := m.GetASTID(alias)
			astTestMust(t, err)
			suffix := ""
			if !simple {
				suffix = " maps: {} p_parent_maps: {} p_const: {}\n"
			}
			baseRow := fmt.Sprintf("v0    #%-4d -> #%-4d is_array: 1 is_select: 0 upward: 0 stores: {} p_stores: {} p_selects: {}\n", baseID, baseID) + suffix
			aliasRow := fmt.Sprintf("v1    #%-4d -> #%-4d is_array: 1 is_select: 0 upward: 0 stores: {} p_stores: {} p_selects: {}\n", aliasID, aliasID) + suffix
			astTestMust(t, state.DisplayTheories(&output))
			if output.String() != "Theory array:\n"+baseRow+aliasRow {
				t.Fatal("initial array diagnostic", output.String())
			}
			astTestMust(t, state.PushScope())
			astTestMust(t, theory.NewEquality(0, 1))
			if baseNode.root == aliasNode.root || theory.arrayUnion.Find(0) != 1 {
				t.Fatal("fixture did not distinguish private and egraph roots")
			}
			read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{alias, indices[0]})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(read, false))
			readNode := state.context.enodes.get(read)
			astTestMust(t, theory.addParentSelect(1, readNode))
			value := smtRelevancyBool(t, m, "array-display-value")
			store, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{alias, indices[0], value})
			astTestMust(t, err)
			astTestMust(t, state.Internalize(store, false))
			astTestMust(t, theory.addParentStore(1, state.context.enodes.get(store)))
			astTestMust(t, theory.setArrayUpward(1))
			readID, err := m.GetASTID(read)
			astTestMust(t, err)
			storeID, err := m.GetASTID(store)
			astTestMust(t, err)
			output.Reset()
			astTestMust(t, theory.DisplayVariable(&output, 0))
			want := fmt.Sprintf("v0    #%-4d -> #%-4d is_array: 1 is_select: 0 upward: 0 stores: {} p_stores: {} p_selects: {}\n", baseID, aliasID) + suffix
			if output.String() != want {
				t.Fatal("non-root diagnostic used egraph root or root data", output.String(), want)
			}
			output.Reset()
			astTestMust(t, theory.DisplayVariable(&output, 1))
			want = fmt.Sprintf("v1    #%-4d -> #%-4d is_array: 1 is_select: 0 upward: 1 stores: {} p_stores: {#%d} p_selects: {#%d}\n", aliasID, aliasID, storeID, readID) + suffix
			if output.String() != want {
				t.Fatal("root parent-list diagnostic", output.String(), want)
			}
			output.Reset()
			astTestMust(t, theory.DisplayVariable(&output, readNode.TheoryVar(family)))
			if !strings.Contains(output.String(), "is_array: 0 is_select: 1 upward: 0") {
				t.Fatal("Boolean select diagnostic flags", output.String())
			}
			if err := state.DisplayTheories(smtArrayDisplayErrorWriter{}); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal("diagnostic writer error", err)
			}
			astTestMust(t, state.PopScopes(1))
			output.Reset()
			astTestMust(t, state.DisplayTheories(&output))
			if output.String() != "Theory array:\n"+baseRow+aliasRow {
				t.Fatal("scope pop retained diagnostic parents or private roots", output.String())
			}
			if err := theory.DisplayVariable(&output, SMTNullTheoryVar); !errors.Is(err, ast.ErrASTDomain) {
				t.Fatal("invalid variable diagnostic", err)
			}
			if err := state.DisplayTheories(nil); !errors.Is(err, ast.ErrASTHandle) {
				t.Fatal("nil diagnostic writer", err)
			}
		})
	}
}

func TestSMTFullArrayDisplayConstructorLists(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTArrayTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	value := smtRelevancyBool(t, m, "display-const-value")
	constant, err := m.MkConstArray(sort, value)
	astTestMust(t, err)
	function, err := m.MkFuncDecl(astTestSymbol(t, m, "display-map-function"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	mapped, err := m.MkArrayMap(function, ast.SliceArguments([]ast.ASTHandle{constant}))
	astTestMust(t, err)
	astTestMust(t, state.Internalize(mapped, false))
	constantID, err := m.GetASTID(constant)
	astTestMust(t, err)
	mapID, err := m.GetASTID(mapped)
	astTestMust(t, err)
	var output bytes.Buffer
	astTestMust(t, theory.DisplayVariable(&output, state.context.enodes.get(constant).TheoryVar(family)))
	// internalize_term adds the map to its argument's parent maps; then
	// try_assign_eq's mark_as_relevant propagates relevancy to the map, and
	// relevant_eh adds it again, as in the source.
	want := fmt.Sprintf(" maps: {} p_parent_maps: {#%d #%d} p_const: {#%d}\n", mapID, mapID, constantID)
	if !strings.HasSuffix(output.String(), want) {
		t.Fatal("constant and parent-map lists", output.String(), want)
	}
	output.Reset()
	astTestMust(t, theory.DisplayVariable(&output, state.context.enodes.get(mapped).TheoryVar(family)))
	want = fmt.Sprintf(" maps: {#%d} p_parent_maps: {} p_const: {}\n", mapID)
	if !strings.HasSuffix(output.String(), want) {
		t.Fatal("map constructor list", output.String(), want)
	}
}
