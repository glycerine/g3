package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Exercise real AST page and vector boundaries around the equality and NOT
// constructors while the parser holds both operands.
func TestDatalogParserNotEqualFailureReleasesIntermediate(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	must(err)
	nums, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: nums, Budget: budget})
	must(err)
	var refs []ast.ASTRef
	defer func() {
		budget.FailAfter(-1)
		for i := len(refs) - 1; i >= 0; i-- {
			must(m.DelRef(&refs[i]))
		}
		must(m.Close())
		must(nums.Close())
		must(symbols.Close())
		must(rt.Close())
		if budget.Used() != 0 {
			t.Fatal("budget leak", budget.Used())
		}
	}()
	pin := func(h ast.ASTHandle) { ref, e := m.NewRef(h); must(e); refs = append(refs, ref) }
	b, err := m.MkBoolSort()
	must(err)
	vars := make([]ast.ASTHandle, 132)
	for i := range vars {
		vars[i], err = m.MkVar(uint32(i), b)
		must(err)
		pin(vars[i])
	}
	sample, err := m.MkEq(vars[2], vars[3])
	must(err)
	pin(sample)
	notSample, err := m.MkNot(sample)
	must(err)
	pin(notSample)
	name, err := symbols.Intern("infix-page-padding")
	must(err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	pin(d)
	// Guarantee available reference records, so refusal is in AST creation.
	var spare []ast.ASTRef
	for i := 0; i < 16; i++ {
		r, e := m.NewRef(ast.ASTHandle{})
		must(e)
		spare = append(spare, r)
	}
	for i := range spare {
		must(m.DelRef(&spare[i]))
	}
	failed := false
	for i := 4; i < len(vars); i++ {
		before := m.Stats()
		lexer, e := newDatalogLexer(newStringReader("Y"))
		must(e)
		p := &datalogParser{manager: m, lexer: lexer, token: datalogToken{kind: datalogNotEqual}, vars: map[string]ast.ASTHandle{"X": vars[0], "Y": vars[1]}}
		budget.FailAfter(0)
		got, e := p.parseInfix(datalogToken{kind: datalogIdentifier, text: "X"})
		budget.FailAfter(-1)
		must(p.closeArguments())
		if e == nil {
			ref, e := m.NewRef(got)
			must(e)
			must(m.DelRef(&ref))
		} else if !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal("unexpected error", i, e)
		} else {
			failed = true
		}
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatalf("boundary %d leaked AST after %v: nodes %d -> %d refs %d -> %d", i, e, before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences)
		}
		if e != nil {
			lexer, retryErr := newDatalogLexer(newStringReader("Y"))
			must(retryErr)
			retry := &datalogParser{manager: m, lexer: lexer, token: datalogToken{kind: datalogNotEqual}, vars: map[string]ast.ASTHandle{"X": vars[0], "Y": vars[1]}}
			result, retryErr := retry.parseInfix(datalogToken{kind: datalogIdentifier, text: "X"})
			must(retryErr)
			must(retry.closeArguments())
			owner, retryErr := m.NewRef(result)
			must(retryErr)
			must(m.DelRef(&owner))
			if retried := m.Stats(); retried.LiveNodes != before.LiveNodes || retried.LiveReferences != before.LiveReferences {
				t.Fatalf("boundary %d retry leaked AST", i)
			}
		}
		app, e := m.MkApp(d, []ast.ASTHandle{vars[i]})
		must(e)
		pin(app)
	}
	if !failed {
		t.Fatal("no actual AST allocation refusal reached")
	}
}
