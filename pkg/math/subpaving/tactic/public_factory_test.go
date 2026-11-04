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

func TestSubpavingCoreUpdateReplacesSelectedEngine(t *testing.T) {
	m, c, _ := publicFactoryFixture(t)
	p, err := c.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Del(&p); err != nil {
			t.Error(err)
		}
	})
	core, err := NewSubpavingCoreTactic(m, c, &p)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.IncRef(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := core.DecRef(); err != nil {
			t.Error(err)
		}
	})
	selected, ok := core.(*selectedSubpavingTactic)
	if !ok || selected.engine != EngineMPQ {
		t.Fatal("source default selection", selected, ok)
	}
	for _, entry := range []struct {
		name string
		kind EngineKind
		want any
	}{
		{"mpff", EngineMPFF, (*fastTacticAdapter)(nil)},
		{"hwf", EngineHWF, (*hardwareTacticAdapter)(nil)},
		{"mpq", EngineMPQ, (*rationalTacticAdapter)(nil)},
	} {
		value, err := c.SymbolsField.Intern(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		key := "numeral"
		if err := c.SetSymbolName(&p, &key, value); err != nil {
			t.Fatal(err)
		}
		if err := core.UpdateParams(&p); err != nil {
			t.Fatal(err)
		}
		if selected.engine != entry.kind {
			t.Fatalf("selected engine %s: got %d want %d", entry.name, selected.engine, entry.kind)
		}
		switch entry.want.(type) {
		case *fastTacticAdapter:
			if _, ok := selected.inner.(*fastTacticAdapter); !ok {
				t.Fatalf("%s replacement type %T", entry.name, selected.inner)
			}
		case *hardwareTacticAdapter:
			if _, ok := selected.inner.(*hardwareTacticAdapter); !ok {
				t.Fatalf("%s replacement type %T", entry.name, selected.inner)
			}
		case *rationalTacticAdapter:
			if _, ok := selected.inner.(*rationalTacticAdapter); !ok {
				t.Fatalf("%s replacement type %T", entry.name, selected.inner)
			}
		}
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
