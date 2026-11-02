package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornModelConversionKeepsSourceOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var m *ast.ASTManager
	var source *model.Model
	var original, derived ast.ASTHandle
	var conversions []hornModelConversion
	var pending memory.ConstructionFailure
	active, called := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			engine := &PropositionalEngine{manager: m, pdrConversions: conversions}
			result, cause := (pdrCertificateModelConverter{engine: engine}).ConvertCertificateModel(source)
			if result != nil || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &pending) {
				t.Fatalf("conversion dropped source model: %v", cause)
			}
			for i := 0; i < 2; i++ {
				if cause := pending.Cleanup(); !errors.Is(cause, ast.ErrASTPending) {
					t.Fatalf("guarded model cleanup changed refusal: %v", cause)
				}
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("conversion cleanup leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			must(m.DelRef(&pins[i]))
		}
	}()
	declare := func(name string, arity int) ast.ASTHandle {
		s, err := symbols.Intern(name)
		must(err)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = boolean
		}
		d, err := m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		pin, err := m.NewRef(d)
		must(err)
		pins = append(pins, pin)
		return d
	}
	original, derived = declare("model-original", 2), declare("model-derived", 1)
	x, err := m.MkVar(0, boolean)
	must(err)
	y, err := m.MkVar(1, boolean)
	must(err)
	for _, h := range []ast.ASTHandle{x, y} {
		pin, err := m.NewRef(h)
		must(err)
		pins = append(pins, pin)
	}
	baseline := m.Stats().LiveReferences
	makeModel := func() *model.Model {
		md, err := model.NewModel(m)
		must(err)
		fi, err := model.NewFunctionInterpretation(m, 1)
		must(err)
		must(fi.SetElse(x))
		must(md.Core().RegisterFunction(derived, fi))
		return md
	}
	source = makeModel()
	conversions = []hornModelConversion{{kind: hornConvertSlice, predicates: map[ast.ASTHandle]ast.ASTHandle{original: derived}, columns: map[ast.ASTHandle][]bool{original: {true, false}}}}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !called || pending == nil {
		t.Fatal("native refusal did not reach presentation cleanup", body, called)
	}
	fi, err := source.Core().FunctionInterpretation(derived)
	must(err)
	got, err := fi.Else()
	must(err)
	if got != x {
		t.Fatal("refused conversion changed derived interpretation")
	}
	if pending.RuntimeOwner() != runtime || pending.CleanupCause() == nil {
		t.Fatal("cleanup continuation lost runtime or refusal")
	}
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.CleanupCause() != nil {
		t.Fatal("successful cleanup did not retire model")
	}
	if got := m.Stats().LiveReferences; got != baseline {
		t.Fatal("conversion cleanup leaked references", baseline, got)
	}
	engine := &PropositionalEngine{manager: m, pdrConversions: conversions}
	reconstructed, err := (pdrCertificateModelConverter{engine: engine}).ConvertCertificateModel(makeModel())
	must(err)
	interpretation, err := reconstructed.Core().FunctionInterpretation(original)
	must(err)
	got, err = interpretation.Else()
	must(err)
	if got != y {
		t.Fatal("retry did not restore original second argument", got, y)
	}
	present, err := reconstructed.Core().HasInterpretation(derived)
	must(err)
	if present {
		t.Fatal("retry exposed transformed declaration")
	}
	must(reconstructed.Close())
	if got := m.Stats().LiveReferences; got != baseline {
		t.Fatal("retry leaked references", baseline, got)
	}
}
