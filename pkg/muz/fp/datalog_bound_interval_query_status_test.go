package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogQueryCombinesOutputNonemptiness(t *testing.T) {
	f := newDirectFactFixture(t)
	m, e := f.manager, f.engine
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(label string) ast.ASTHandle {
		t.Helper()
		name, cause := m.Symbols.Intern(label)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	populated, empty, alsoEmpty := declare("product-output-populated"), declare("product-output-empty"), declare("product-output-also-empty")
	kinds := []string{"bound_relation", "interval_relation"}
	activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, []boundIntervalDatalogPredicate{
		{declaration: populated, kinds: kinds},
		{declaration: empty, kinds: kinds},
		{declaration: alsoEmpty, kinds: kinds},
	})
	must(err)
	if !selected {
		t.Fatal("aligned product was not selected")
	}
	defer func() { must(activation.close()) }()
	value, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	must(activation.addFact(populated, []ast.ASTHandle{value}))
	for _, test := range []struct {
		name    string
		outputs []ast.ASTHandle
		want    lbool.LBool
	}{
		{"no outputs", nil, lbool.True},
		{"empty output", []ast.ASTHandle{empty}, lbool.False},
		{"all empty", []ast.ASTHandle{empty, alsoEmpty}, lbool.False},
		{"nonempty output", []ast.ASTHandle{populated}, lbool.Undef},
		{"nonempty first", []ast.ASTHandle{populated, empty}, lbool.Undef},
		{"nonempty last", []ast.ASTHandle{empty, populated}, lbool.Undef},
		{"repeated nonempty", []ast.ASTHandle{populated, empty, populated}, lbool.Undef},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, cause := activation.query(test.outputs)
			if cause != nil || status != test.want {
				t.Fatalf("query status %v, want %v (%v)", status, test.want, cause)
			}
		})
	}
}
