package tactic

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	modelfpa "github.com/glycerine/g3/pkg/model/fpa"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestFPA2BVModelConverterRebuildsFloatsAndKeepsOtherDeclarations(t *testing.T) {
	m := astTestManager(t)
	b := m.BudgetField
	ctx, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: b})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: b})
	astTestMust(t, err)
	global, err := uparams.NewGlobalParamsService(m.Symbols, ctx, uparams.GlobalParamsConfig{Budget: b, Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, global.InitializeSource())
	t.Cleanup(func() {
		astTestMust(t, global.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, ctx.Close())
	})
	forward, err := rewriter.NewFPA2BVConverter(m, ctx, global)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, forward.Close()) })
	floatSort, err := m.FloatingPointSort(8, 24)
	astTestMust(t, err)
	floatDeclaration, err := m.MkFuncDecl(astTestSymbol(t, m, "model-float"), nil, floatSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	encoded, err := forward.MkConst(floatDeclaration)
	astTestMust(t, err)
	sign, err := m.GetArg(encoded, 0)
	astTestMust(t, err)
	packedValue, err := m.BitVectorNumeralUnsigned(0x3fc00000, 32)
	astTestMust(t, err)
	astTestPin(t, m, packedValue)
	var auxiliary []struct{ declaration, value ast.ASTHandle }
	if ast.ASTSourceDebug {
		// Source Z3DEBUG creates separate sign/exponent/significand constants;
		// release builds instead extract all three from one packed declaration.
		for i, part := range []struct{ value, width uint32 }{{0, 1}, {127, 8}, {0x400000, 23}} {
			h, e := m.GetArg(encoded, uint32(i))
			astTestMust(t, e)
			d, e := m.GetDecl(h)
			astTestMust(t, e)
			value, e := m.BitVectorNumeralUnsigned(part.value, part.width)
			astTestMust(t, e)
			astTestPin(t, m, value)
			auxiliary = append(auxiliary, struct{ declaration, value ast.ASTHandle }{d, value})
		}
	} else {
		packed, e := m.GetArg(sign, 0)
		astTestMust(t, e)
		d, e := m.GetDecl(packed)
		astTestMust(t, e)
		auxiliary = append(auxiliary, struct{ declaration, value ast.ASTHandle }{d, packedValue})
	}
	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	plainDeclaration, err := m.MkFuncDecl(astTestSymbol(t, m, "plain-model-value"), nil, boolSort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	plainValue, err := m.MkTrue()
	astTestMust(t, err)
	source, err := model.NewModel(m)
	astTestMust(t, err)
	for _, entry := range auxiliary {
		astTestMust(t, source.Core().RegisterConstant(entry.declaration, entry.value))
	}
	astTestMust(t, source.Core().RegisterConstant(plainDeclaration, plainValue))
	var reference ModelRef
	astTestMust(t, reference.Set(source.Core()))
	t.Cleanup(func() { astTestMust(t, reference.Close()) })
	converter, err := NewFPA2BVModelConverter(m, forward)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, converter.Close()) })
	var display bytes.Buffer
	astTestMust(t, converter.Display(&display))
	if !strings.Contains(display.String(), "fpa2bv-model-converter") || !strings.Contains(display.String(), "model-float") {
		t.Fatal("model converter display", display.String())
	}
	reverse, err := modelfpa.NewConverter(m, forward)
	astTestMust(t, err)
	wantFloat, err := reverse.ConvertPackedBV(nil, floatSort, packedValue)
	astTestMust(t, err)
	_ = astTestPin(t, m, wantFloat)
	astTestMust(t, reverse.Close())
	astTestMust(t, converter.ApplyModel(&reference, 0))
	target, err := reference.Get()
	astTestMust(t, err)
	gotFloat, err := target.ConstantInterpretation(floatDeclaration)
	astTestMust(t, err)
	if gotFloat != wantFloat {
		t.Fatal("rebuilt float", gotFloat, wantFloat)
	}
	gotPlain, err := target.ConstantInterpretation(plainDeclaration)
	astTestMust(t, err)
	if gotPlain != plainValue {
		t.Fatal("plain constant was not preserved", gotPlain, plainValue)
	}
	for _, entry := range auxiliary {
		if consumed, err := target.HasConstantInterpretation(entry.declaration); err != nil || consumed {
			t.Fatal("auxiliary float encoding declaration survived", consumed, err)
		}
	}
}
