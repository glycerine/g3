package tab_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/tab"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSemanticMatcherRepeatedVariablesAndConditions(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	sort, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("matcher-p")
	must(err)
	pred, err := m.MkFuncDecl(name, []ast.ASTHandle{sort, sort}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	constant := func(name string) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		must(e)
		h, e := m.MkConst(symbol, sort)
		must(e)
		return h
	}
	a, b := constant("matcher-a"), constant("matcher-b")
	x, err := m.MkVar(0, sort)
	must(err)
	pattern, err := m.MkApp(pred, []ast.ASTHandle{x, x})
	must(err)
	term, err := m.MkApp(pred, []ast.ASTHandle{a, b})
	must(err)
	patternRef, err := m.NewRef(pattern)
	must(err)
	defer func() { must(m.DelRef(&patternRef)) }()
	termRef, err := m.NewRef(term)
	must(err)
	defer func() { must(m.DelRef(&termRef)) }()
	bindings, err := substitution.New(m)
	must(err)
	defer func() { must(bindings.Close()) }()
	must(bindings.Reserve(2, 1))
	matcher, err := tab.NewSemanticMatcher(m)
	must(err)
	var conditions []ast.ASTRef
	defer func() {
		for i := len(conditions) - 1; i >= 0; i-- {
			must(m.DelRef(&conditions[i]))
		}
	}()
	matched, err := matcher.Match(pattern, term, bindings, &conditions)
	must(err)
	if !matched || len(conditions) != 1 {
		t.Fatal("symbolic repeated match", matched, len(conditions))
	}
	bound, found, err := bindings.Find(0, 0)
	must(err)
	if !found || bound.Expr() != b || bound.Offset() != 1 {
		t.Fatal("source reverse argument order", bound, found)
	}
	condition, err := m.RefGet(conditions[0])
	must(err)
	expected, err := m.MkEq(b, a)
	must(err)
	if condition != expected {
		t.Fatal("equality condition", condition, expected)
	}
	must(bindings.Reset())
	must(bindings.Reserve(2, 1))
	conflict, err := m.MkApp(pred, []ast.ASTHandle{zero, one})
	must(err)
	conflictRef, err := m.NewRef(conflict)
	must(err)
	defer func() { must(m.DelRef(&conflictRef)) }()
	matched, err = matcher.Match(pattern, conflict, bindings, &conditions)
	must(err)
	if matched || len(conditions) != 1 {
		t.Fatal("distinct numeral values matched", matched, len(conditions))
	}
	bound, found, err = bindings.Find(0, 0)
	must(err)
	if !found || bound.Expr() != one {
		t.Fatal("non-match lost the source partial substitution", bound, found)
	}
}

func TestSemanticMatcherDatatypeConstructors(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	_, err = m.RegisterDatatypePlugin()
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	sym := func(name string) usymbol.Symbol { value, e := m.Symbols.Intern(name); must(e); return value }
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: sym("matcher-dt"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: sym("matcher-left"), Recognizer: sym("is-matcher-left"), Accessors: []ast.DatatypeAccessorDefinition{{Name: sym("left-field"), Range: integer}}},
		{Name: sym("matcher-right"), Recognizer: sym("is-matcher-right"), Accessors: []ast.DatatypeAccessorDefinition{{Name: sym("right-field"), Range: integer}}},
	}}})
	must(err)
	constructors, err := m.DatatypeConstructors(sorts[0])
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	predicate, err := m.MkFuncDecl(sym("matcher-dt-p"), []ast.ASTHandle{sorts[0]}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	constant := func(name string) ast.ASTHandle { h, e := m.MkConst(sym(name), integer); must(e); return h }
	a, b := constant("dt-a"), constant("dt-b")
	wrap := func(constructor ast.ASTHandle, arg ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(constructor, []ast.ASTHandle{arg})
		must(e)
		out, e := m.MkApp(predicate, []ast.ASTHandle{h})
		must(e)
		return out
	}
	pattern := wrap(constructors[0], a)
	uncertain := wrap(constructors[0], b)
	different := wrap(constructors[1], a)
	value0, value1 := wrap(constructors[0], zero), wrap(constructors[0], one)
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			must(m.DelRef(&refs[i]))
		}
	}()
	for _, h := range []ast.ASTHandle{pattern, uncertain, different, value0, value1} {
		ref, e := m.NewRef(h)
		must(e)
		refs = append(refs, ref)
	}
	bindings, err := substitution.New(m)
	must(err)
	defer func() { must(bindings.Close()) }()
	must(bindings.Reserve(2, 0))
	matcher, err := tab.NewSemanticMatcher(m)
	must(err)
	var conditions []ast.ASTRef
	defer func() {
		for i := len(conditions) - 1; i >= 0; i-- {
			must(m.DelRef(&conditions[i]))
		}
	}()
	matched, err := matcher.Match(pattern, uncertain, bindings, &conditions)
	must(err)
	if !matched || len(conditions) != 1 {
		t.Fatal("uncertain constructor equality", matched, len(conditions))
	}
	matched, err = matcher.Match(pattern, different, bindings, &conditions)
	must(err)
	if matched {
		t.Fatal("different constructors matched")
	}
	matched, err = matcher.Match(value0, value1, bindings, &conditions)
	must(err)
	if matched {
		t.Fatal("different constructor values matched")
	}
}
