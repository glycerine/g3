package protomodel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func protoMust(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func protoSymbol(t testing.TB, m *ast.ASTManager, text string) usymbol.Symbol {
	t.Helper()
	s, err := m.Symbols.Intern(text)
	protoMust(t, err)
	return s
}
func protoPin(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) ast.ASTRef {
	t.Helper()
	r, err := m.NewRef(h)
	protoMust(t, err)
	t.Cleanup(func() { protoMust(t, m.DelRef(&r)) })
	return r
}
func protoFixture(t testing.TB) (*ast.ASTManager, *ProtoModel) {
	t.Helper()
	m, p, _ := protoFixtureWithConfig(t)
	return m, p
}
func protoFixtureWithConfig(t testing.TB) (*ast.ASTManager, *ProtoModel, model.ModelEvaluatorConfig) {
	t.Helper()
	return protoFixtureWithParams(t, nil)
}

// protoFixtureWithParams is protoFixtureWithConfig with a hook that updates
// the evaluator parameters before NewProtoModel.
func protoFixtureWithParams(t testing.TB, setup func(*uparams.ParamsContext, *uparams.ParamsRef)) (*ast.ASTManager, *ProtoModel, model.ModelEvaluatorConfig) {
	t.Helper()
	budget := arena.NewBudget(^uint64(0))
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget, PageSize: 64, DescriptorPageSize: 2})
	protoMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: budget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}}}})
	protoMust(t, err)
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}}})
	protoMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget, Descriptions: arena.BytesRegionConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}}, Text: arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: budget}}})
	protoMust(t, err)
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog, DescriptorPageSize: 2, DescriptorWordsPageSize: 16})
	protoMust(t, err)
	protoMust(t, globals.InitializeSource())
	params, err := context.NewValue()
	protoMust(t, err)
	if setup != nil {
		setup(context, &params)
	}
	memory, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget})
	protoMust(t, err)
	cooperation, err := cooperate.NewCooperation(budget)
	protoMust(t, err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	protoMust(t, err)
	protoMust(t, manager.RegisterDeclarationPlugins())
	configuration := model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: &params, Memory: memory, Cooperation: cooperation}
	proto, err := NewProtoModel(manager, configuration)
	protoMust(t, err)
	t.Cleanup(func() {
		protoMust(t, proto.Close())
		protoMust(t, manager.Close())
		protoMust(t, cooperation.Close())
		protoMust(t, memory.Close())
		protoMust(t, context.Del(&params))
		protoMust(t, globals.Close())
		protoMust(t, catalog.Close())
		protoMust(t, context.Close())
		protoMust(t, numerals.Close())
		protoMust(t, symbols.Close())
		if used := budget.Used(); used != 0 {
			t.Fatalf("prototype-model fixture retained %d bytes", used)
		}
	})
	return manager, proto, configuration
}

func TestValueFactoriesSomeFreshFreezeAndNumericValues(t *testing.T) {
	m, p := protoFixture(t)
	b, err := m.MkBoolSort()
	protoMust(t, err)
	no, err := p.SomeValue(b)
	protoMust(t, err)
	yes, err := m.MkTrue()
	protoMust(t, err)
	v1, v2, ok, err := protoTestSomeValues(t, m, p, b)
	protoMust(t, err)
	if !ok || v1 != no || v2 != yes {
		t.Fatal("Boolean values", v1, v2, ok)
	}
	u, err := m.MkUninterpretedSort(protoSymbol(t, m, "U"), nil)
	protoMust(t, err)
	protoPin(t, m, u)
	u0, err := p.SomeValue(u)
	protoMust(t, err)
	u1, err := p.FreshValue(u)
	protoMust(t, err)
	if u0 == u1 || u0.IsNull() || u1.IsNull() {
		t.Fatal("fresh user values", u0, u1)
	}
	protoMust(t, p.FreezeUniverse(u))
	if fresh, err := p.FreshValue(u); err != nil || !fresh.IsNull() {
		t.Fatal("frozen universe admitted fresh value", fresh, err)
	}
	if finite, err := p.IsFinite(u); err != nil || !finite {
		t.Fatal("finite universe", finite, err)
	}
	if universe, err := p.KnownUniverse(u); err != nil || len(universe) != 2 {
		t.Fatal("known universe", universe, err)
	}
	_, err = m.RegisterArithmeticPlugin()
	protoMust(t, err)
	af, err := NewArithmeticFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(af))
	real, err := m.MkFamilySort(af.FamilyID(), 0, nil)
	protoMust(t, err)
	r0, err := p.SomeValue(real)
	protoMust(t, err)
	r1, err := p.FreshValue(real)
	protoMust(t, err)
	if r0 == r1 {
		t.Fatal("arithmetic factory did not advance")
	}
	threeHalves, err := protoMkNumValue(t, af, 3, 2, false)
	protoMust(t, err)
	if text, integer, matched, err := m.ArithmeticNumeralText(threeHalves); err != nil || !matched || integer || text != "3/2" {
		t.Fatal("explicit arithmetic factory numeral", text, integer, matched, err)
	}
	_, err = m.RegisterBitVectorPlugin()
	protoMust(t, err)
	bf, err := NewBitVectorFactory(m)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(bf))
	bv, err := m.BitVectorSort(8)
	protoMust(t, err)
	b0, err := p.SomeValue(bv)
	protoMust(t, err)
	b1, err := p.FreshValue(bv)
	protoMust(t, err)
	if b0 == b1 {
		t.Fatal("bit-vector factory did not advance")
	}
	b255, err := protoMkNumValue(t, bf, -1, 1, 8)
	protoMust(t, err)
	if text, matched, err := m.BitVectorNumeralText(b255); err != nil || !matched || text != "255" {
		t.Fatal("explicit bit-vector factory numeral", text, matched, err)
	}
}

func TestArrayAndDatatypeFactories(t *testing.T) {
	m, p := protoFixture(t)
	_, err := m.RegisterArrayPlugin()
	protoMust(t, err)
	af, err := NewArrayFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(af))
	b, err := m.MkBoolSort()
	protoMust(t, err)
	a, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{b}), b)
	protoMust(t, err)
	protoPin(t, m, a)
	x, y, ok, err := protoTestSomeValues(t, m, p, a)
	protoMust(t, err)
	if !ok || x.IsNull() || y.IsNull() || x == y {
		t.Fatal("array values", x, y, ok)
	}
	for _, value := range []ast.ASTHandle{x, y} {
		if _, err = m.AsArrayFunction(value); err != nil {
			t.Fatal("array factory result is not as-array", err)
		}
	}
	u, err := m.MkUninterpretedSort(protoSymbol(t, m, "ArrayIndex"), nil)
	protoMust(t, err)
	protoPin(t, m, u)
	freshSort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{u}), b)
	protoMust(t, err)
	protoPin(t, m, freshSort)
	_, err = p.SomeValue(freshSort)
	protoMust(t, err)
	freshArray, err := p.FreshValue(freshSort)
	protoMust(t, err)
	if freshArray.IsNull() {
		t.Fatal("fresh-index array construction failed")
	}
	_, err = m.RegisterDatatypePlugin()
	protoMust(t, err)
	color, err := m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: protoSymbol(t, m, "Color"),
		Constructors: []ast.DatatypeConstructorDefinition{
			{Name: protoSymbol(t, m, "red"), Recognizer: protoSymbol(t, m, "is-red")},
			{Name: protoSymbol(t, m, "blue"), Recognizer: protoSymbol(t, m, "is-blue")},
		},
	}})
	protoMust(t, err)
	protoPin(t, m, color[0])
	df, err := NewDatatypeFactory(p)
	protoMust(t, err)
	protoMust(t, p.RegisterFactory(df))
	c0, c1, ok, err := protoTestSomeValues(t, m, p, color[0])
	protoMust(t, err)
	if !ok || c0.IsNull() || c1.IsNull() || c0 == c1 {
		t.Fatal("datatype values", c0, c1, ok)
	}
	nat, err := m.MkDatatypes([]ast.DatatypeDefinition{{
		Name: protoSymbol(t, m, "Nat"),
		Constructors: []ast.DatatypeConstructorDefinition{
			{Name: protoSymbol(t, m, "zero"), Recognizer: protoSymbol(t, m, "is-zero")},
			{Name: protoSymbol(t, m, "succ"), Recognizer: protoSymbol(t, m, "is-succ"), Accessors: []ast.DatatypeAccessorDefinition{{Name: protoSymbol(t, m, "pred"), Recursive: true}}},
		},
	}})
	protoMust(t, err)
	protoPin(t, m, nat[0])
	n0, err := p.SomeValue(nat[0])
	protoMust(t, err)
	n1, err := p.FreshValue(nat[0])
	protoMust(t, err)
	n2, err := p.FreshValue(nat[0])
	protoMust(t, err)
	if n0.IsNull() || n1.IsNull() || n2.IsNull() || n0 == n1 || n1 == n2 || n0 == n2 {
		t.Fatal("recursive datatype growth", n0, n1, n2)
	}
}

func TestProtoModelCompletionCleanupEvaluationAndFinalModel(t *testing.T) {
	m, p := protoFixture(t)
	b, err := m.MkBoolSort()
	protoMust(t, err)
	d, err := m.MkFuncDecl(protoSymbol(t, m, "f"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, d)
	fi, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	yes, err := m.MkTrue()
	protoMust(t, err)
	no, err := m.MkFalse()
	protoMust(t, err)
	protoMust(t, fi.InsertEntry([]ast.ASTHandle{yes}, no))
	protoMust(t, p.Core().RegisterFunction(d, fi))
	protoMust(t, p.CompletePartialFunctions())
	if fallback, err := fi.Else(); err != nil || fallback != no {
		t.Fatal("partial completion", fallback, err)
	}
	oldAux, err := m.MkFuncDecl(protoSymbol(t, m, "f-old"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, oldAux)
	replacement, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	protoMust(t, replacement.SetElse(yes))
	protoMust(t, p.ReregisterFunction(d, replacement, oldAux))
	oldCopy, err := p.Core().FunctionInterpretation(oldAux)
	protoMust(t, err)
	if fallback, err := oldCopy.Else(); err != nil || fallback != no {
		t.Fatal("reregistration did not preserve old interpretation", fallback, err)
	}
	atom, err := m.MkConst(protoSymbol(t, m, "unknown"), b)
	protoMust(t, err)
	protoPin(t, m, atom)
	out := protoPin(t, m, ast.ASTHandle{})
	ok, err := p.Eval(atom, out, true)
	protoMust(t, err)
	if got, err := m.RefGet(out); err != nil || !ok || got != no {
		t.Fatal("prototype evaluation", got, ok, err)
	}
	unused, err := m.MkFuncDecl(protoSymbol(t, m, "unused-aux"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, unused)
	auxFI, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	protoMust(t, auxFI.SetElse(no))
	protoMust(t, p.RegisterAuxFunction(unused, auxFI))
	late, err := m.MkConst(protoSymbol(t, m, "cleanup-constant"), b)
	protoMust(t, err)
	protoPin(t, m, late)
	cleanupDecl, err := m.MkFuncDecl(protoSymbol(t, m, "cleanup-function"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, cleanupDecl)
	cleanupFI, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	protoMust(t, cleanupFI.SetElse(late))
	protoMust(t, p.Core().RegisterFunction(cleanupDecl, cleanupFI))
	protoMust(t, p.Cleanup())
	if got, err := p.Core().FunctionInterpretation(unused); err != nil || got != nil {
		t.Fatal("unused auxiliary declaration survived cleanup", got, err)
	}
	lateDecl, err := m.GetDecl(late)
	protoMust(t, err)
	if value, err := p.Core().ConstantInterpretation(lateDecl); err != nil || value != no {
		t.Fatal("cleanup did not complete uninterpreted constant", value, err)
	}
	if fallback, err := cleanupFI.Else(); err != nil || fallback != no {
		t.Fatal("cleanup did not rewrite fallback", fallback, err)
	}
	final, err := p.FinalModel()
	protoMust(t, err)
	t.Cleanup(func() { protoMust(t, final.Close()) })
	copyFI, err := final.Core().FunctionInterpretation(d)
	protoMust(t, err)
	if copyFI == nil {
		t.Fatal("final model lost function interpretation")
	}
}

// AUDIT.md smtcore_14-F3: proto_model::eval catches model_evaluator_exception
// (max. steps exceeded here) and returns false; the next evaluation starts
// from a clean rewriter, as rewriter_tpl::operator() does in later z3.
func TestProtoModelEvalReturnsFalseOnEvaluatorException(t *testing.T) {
	for _, limited := range []bool{true, false} {
		m, p, _ := protoFixtureWithParams(t, func(context *uparams.ParamsContext, params *uparams.ParamsRef) {
			if limited {
				name := "max_steps"
				protoMust(t, context.SetUIntName(params, &name, 0))
			}
		})
		b, err := m.MkBoolSort()
		protoMust(t, err)
		d, err := m.MkFuncDecl(protoSymbol(t, m, "d"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		protoMust(t, err)
		protoPin(t, m, d)
		fi, err := model.NewFunctionInterpretation(m, 1)
		protoMust(t, err)
		yes, err := m.MkTrue()
		protoMust(t, err)
		protoMust(t, fi.SetElse(yes))
		protoMust(t, p.Core().RegisterFunction(d, fi))
		app, err := m.MkApp(d, []ast.ASTHandle{yes})
		protoMust(t, err)
		protoPin(t, m, app)
		out := protoPin(t, m, ast.ASTHandle{})
		ok, err := p.Eval(app, out, true)
		if err != nil || ok == limited {
			t.Fatal("eval under max_steps", limited, ok, err)
		}
		// The interrupted traversal must not leak into the next evaluation.
		ok, err = p.Eval(yes, out, true)
		protoMust(t, err)
		value, err := m.RefGet(out)
		protoMust(t, err)
		if !ok || value != yes {
			t.Fatal("evaluation after a caught exception", ok, value)
		}
	}
}

// protoMkNumValue is mk_num_value(rational(n, d), arg) for an arithmetic
// (bool is_int) or bit-vector (uint32 width) factory.
func protoMkNumValue(t *testing.T, f any, n, d int32, arg any) (ast.ASTHandle, error) {
	t.Helper()
	var m *ast.ASTManager
	switch f := f.(type) {
	case *ArithmeticFactory:
		m = f.manager
	case *BitVectorFactory:
		m = f.manager
	}
	q := m.NumeralService().Manager
	r := unumeral.NewRational()
	protoMust(t, q.SetFractionSigned(&r, n, d))
	defer q.Del(&r)
	switch f := f.(type) {
	case *ArithmeticFactory:
		return f.MkNumValueRational(&r, arg.(bool))
	case *BitVectorFactory:
		return f.MkNumValueRational(&r, uint32(arg.(int)))
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}
