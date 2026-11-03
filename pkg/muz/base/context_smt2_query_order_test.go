package base

// Source: dl_context.cpp:1215-1219 emits the fresh query declaration before
// constructing the argument variables, including unused Boolean positions.

import (
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type contextQueryDeclarationRefusal struct {
	manager *ast.ASTManager
	stats   ast.ASTManagerStats
	prefix  string
}

func (w *contextQueryDeclarationRefusal) Write(body []byte) (int, error) {
	w.stats = w.manager.Stats()
	w.prefix = string(body)
	return 0, io.ErrClosedPipe
}

func TestContextSMT2FreshQueryDeclarationPrecedesArguments(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	rules, err := NewRuleManager(m)
	must(err)
	set, err := NewRuleSet(rules)
	must(err)
	var query ast.ASTRef
	defer func() {
		must(set.Close())
		must(rules.Close())
		if query != (ast.ASTRef{}) {
			must(m.DelRef(&query))
		}
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		if budget.Used() != 0 {
			t.Fatal("query declaration refusal leaked bytes", budget.Used())
		}
	}()
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := symbols.Intern("query-order-P")
	must(err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(declaration))
	// Only variable 128 exists. Constructing the bridge arguments would create
	// 128 Boolean variables that the source has not created at its first write.
	v, err := m.MkVar(128, boolean)
	must(err)
	h, err := m.MkApp(declaration, []ast.ASTHandle{v})
	must(err)
	query, err = m.NewRef(h)
	must(err)
	printer, err := ast.NewLegacySMTPrinter(m)
	must(err)
	before := m.Stats()
	out := &contextQueryDeclarationRefusal{manager: m}
	err = set.writeContextQuery(out, printer, h, false)
	if !errors.Is(err, io.ErrClosedPipe) || out.prefix != "(declare-rel " {
		t.Fatal("fresh query declaration refusal", out.prefix, err)
	}
	if out.stats.LiveNodes != before.LiveNodes+1 || out.stats.LiveReferences != before.LiveReferences+1 {
		t.Fatalf("bridge arguments preceded declaration emission: nodes %d/%d references %d/%d", out.stats.LiveNodes, before.LiveNodes+1, out.stats.LiveReferences, before.LiveReferences+1)
	}
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters || after.LiveReferences != before.LiveReferences {
		t.Fatal("declaration refusal did not release its fresh owner", before, after)
	}
}
