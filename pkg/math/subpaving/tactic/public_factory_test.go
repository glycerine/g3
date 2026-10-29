package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func publicFactoryFixture(t *testing.T) (*ast.ASTManager, *params.ParamsContext, *params.GlobalParamsService) {
	t.Helper()
	b := arena.NewBudget(1 << 29)
	process := memory.NewProcessAccounting()
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: b, Process: process, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Process: process, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	c, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := params.NewGlobalParamsCatalog(sy, params.GlobalParamsCatalogConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	g, err := params.NewGlobalParamsService(sy, c, params.GlobalParamsConfig{Budget: b, Catalog: catalog})
	if err != nil {
		t.Fatal(err)
	}
	if err = g.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	previous, err := params.BindProcessGlobalParamsService(g)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := params.BindProcessGlobalParamsService(previous)
		if err != nil {
			t.Error(err)
		}
		for _, err := range []error{g.Close(), catalog.Close(), c.Close(), m.Close(), q.Close(), sy.Close(), runtime.Close()} {
			if err != nil {
				t.Error(err)
			}
		}
		if b.Used() != 0 {
			t.Error("factory roots", b.Used())
		}
	})
	return m, c, g
}

func TestSubpavingCoreFactoryAllConcreteEngines(t *testing.T) {
	m, c, _ := publicFactoryFixture(t)
	for _, name := range []string{"mpq", "mpf", "hwf", "mpff", "mpfx", "unknown-selects-hwf"} {
		t.Run(name, func(t *testing.T) {
			p, err := c.NewValue()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := c.Del(&p); err != nil {
					t.Error(err)
				}
			}()
			s, err := c.SymbolsField.Intern(name)
			if err != nil {
				t.Fatal(err)
			}
			key := "numeral"
			if err = c.SetSymbolName(&p, &key, s); err != nil {
				t.Fatal(err)
			}
			created, err := NewSubpavingCoreTactic(m, c, &p)
			if err != nil {
				t.Fatal(err)
			}
			if err = created.IncRef(); err != nil {
				t.Fatal(err)
			}
			if err = created.DecRef(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSubpavingPublicFactoryOwnsSimplifyComposition(t *testing.T) {
	m, c, g := publicFactoryFixture(t)
	created, err := NewSubpavingTactic(m, c, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = created.IncRef(); err != nil {
		t.Fatal(err)
	}
	if err = created.DecRef(); err != nil {
		t.Fatal(err)
	}
}
