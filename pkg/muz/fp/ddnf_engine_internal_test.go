package fp

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/ddnf"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDDNFRegistryPersistsAcrossQueryClones(t *testing.T) {
	engine, manager := newFragmentTestEngine(t, "ddnf")
	var output bytes.Buffer
	if err := engine.SetDDNFOutput(&output); err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	bitVector, err := manager.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("ddnf-persistent-registry")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{bitVector}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.RegisterPredicate(predicate); err != nil {
		t.Fatal(err)
	}
	variable, err := manager.MkVar(0, bitVector)
	if err != nil {
		t.Fatal(err)
	}
	body, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	numerals := manager.NumeralService()
	value, err := numerals.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	if err = numerals.SetUint64(&value, 3); err != nil {
		t.Fatal(err)
	}
	constant, err := manager.BitVectorNumeral(&value, 4)
	if cleanupErr := numerals.Del(&value); cleanupErr != nil {
		t.Fatal(cleanupErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	equality, err := manager.MkEq(variable, constant)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := manager.MkImplies(equality, body)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.AddRule(rule, "ddnf-persistent-rule"); err != nil {
		t.Fatal(err)
	}
	status, err := engine.QueryPredicate(predicate)
	if err != nil || status != lbool.Undef {
		t.Fatalf("first query result = %v, %v", status, err)
	}
	first := engine.ddnf.registry
	if first == nil {
		t.Fatal("query did not retain its DDNF width registry")
	}
	if widths := first.Widths(); len(widths) != 1 || widths[0] != 4 {
		t.Fatalf("query DDNF widths = %v, want [4]", widths)
	}
	status, err = engine.QueryPredicate(predicate)
	if !errors.Is(err, ddnf.ErrCore) || status != lbool.Undef {
		t.Fatalf("query after descendant-cache freeze = %v, %v, want source frozen-core refusal", status, err)
	}
	if engine.ddnf.registry != first {
		t.Fatal("query clone replaced the source engine's DDNF registry")
	}
	if got := bytes.Count(output.Bytes(), []byte("(set-logic HORN)")); got != 1 {
		t.Fatalf("query dump count = %d, want only the completed first query", got)
	}
	if err = engine.SelectEngine("datalog"); err != nil {
		t.Fatal(err)
	}
	if engine.ddnf.registry != nil {
		t.Fatal("replacing the DDNF engine retained its width registry")
	}
}

func TestDDNFQueryExpressionTransfersCompiledASTTrail(t *testing.T) {
	engine, manager := newFragmentTestEngine(t, "ddnf")
	var output bytes.Buffer
	if err := engine.SetDDNFOutput(&output); err != nil {
		t.Fatal(err)
	}
	bitVector, err := manager.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	variable, err := manager.MkVar(0, bitVector)
	if err != nil {
		t.Fatal(err)
	}
	numerals := manager.NumeralService()
	value, err := numerals.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	if err = numerals.SetUint64(&value, 3); err != nil {
		t.Fatal(err)
	}
	constant, err := manager.BitVectorNumeral(&value, 4)
	if closeErr := numerals.Del(&value); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	query, err := manager.MkEq(variable, constant)
	if err != nil {
		t.Fatal(err)
	}
	queryRef, err := manager.NewRef(query)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.DelRef(&queryRef); err != nil {
			t.Error(err)
		}
	})
	before := manager.Stats().LiveReferences
	status, err := engine.QueryExpression(query)
	if err != nil || status != lbool.Undef {
		t.Fatalf("DDNF expression query = %v, %v", status, err)
	}
	trail := len(engine.ddnf.trail)
	if trail == 0 {
		t.Fatal("query clone discarded the source-lifetime compiled AST trail")
	}
	for index, reference := range engine.ddnf.trail {
		if _, err = manager.RefGet(reference); err != nil {
			t.Fatalf("transferred DDNF trail reference %d is invalid: %v", index, err)
		}
	}
	after := manager.Stats().LiveReferences
	if uint64(after) < uint64(before)+uint64(trail) {
		t.Fatalf("DDNF trail lifetime not reflected in AST refs: before=%d after=%d trail=%d", before, after, trail)
	}
	if err = engine.SelectEngine("datalog"); err != nil {
		t.Fatal(err)
	}
	replaced := manager.Stats().LiveReferences
	if len(engine.ddnf.trail) != 0 || uint64(replaced) > uint64(after)-uint64(trail) {
		t.Fatalf("DDNF backend replacement retained source trail: after=%d replaced=%d trail=%d", after, replaced, trail)
	}
	if err = engine.SelectEngine("ddnf"); err != nil {
		t.Fatal(err)
	}
	status, err = engine.QueryExpression(query)
	if err != nil || status != lbool.Undef {
		t.Fatalf("DDNF expression query after backend replacement = %v, %v", status, err)
	}
	trail = len(engine.ddnf.trail)
	if trail == 0 {
		t.Fatal("reselected DDNF did not build a new source-lifetime trail")
	}
	after = manager.Stats().LiveReferences
	if err = engine.Close(); err != nil {
		t.Fatal(err)
	}
	closed := manager.Stats().LiveReferences
	if uint64(closed) > uint64(after)-uint64(trail) {
		t.Fatalf("DDNF engine close retained its trail refs: before close=%d after close=%d trail=%d", after, closed, trail)
	}
}
