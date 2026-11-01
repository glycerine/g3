package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestDatalogPluginPrepareCacheAndContextOwnership(t *testing.T) {
	for _, size := range []uint64{1, 2, 4} {
		t.Run(string(rune('0'+size)), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			name, e := m.Symbols.Intern("finite-domain")
			fmMust(t, e)
			sort := f.own(m.MkDatalogFiniteSort(name, size))
			x := f.constant("x", sort)
			zero := f.own(m.MkDatalogNumeral(0, sort))
			eq, e := m.MkEq(x, zero)
			fmMust(t, e)
			eqRef, e := m.NewRef(eq)
			fmMust(t, e)
			formula := f.own(m.MkBoolVal(true))
			contains, e := ast.NewContainsApp(m, x)
			fmMust(t, e)
			defer func() { fmMust(t, contains.Close()) }()
			roots := []ast.ASTHandle{x, formula, eq, zero}
			before := make([]uint32, len(roots))
			for i, root := range roots {
				before[i], e = m.GetRefCount(root)
				fmMust(t, e)
			}
			ctx, e := NewSolverContext(m, []ast.ASTHandle{x})
			fmMust(t, e)
			plugin, e := NewDatalogPlugin(ctx)
			fmMust(t, e)
			fmMust(t, ctx.AddPlugin(plugin))
			selected, e := ctx.PluginFor(x)
			fmMust(t, e)
			if selected != plugin {
				t.Fatal("plugin lookup")
			}
			atoms, e := NewArithmeticAtomSet(m)
			fmMust(t, e)
			fmMust(t, atoms.AddPositive(eq))
			fmMust(t, atoms.AddNegative(eq))
			ok, e := plugin.PrepareAtoms(contains, formula, atoms)
			fmMust(t, e)
			if !ok {
				t.Fatal("prepare failed")
			}
			fmMust(t, atoms.Close())
			fmMust(t, m.DelRef(&eqRef))
			_, e = m.GetKind(eq)
			fmMust(t, e) // The plugin now owns the atom independently.
			// A cached pair ignores later atom collections, as in update_eqs.
			ok, e = plugin.Prepare(contains, formula, nil, nil)
			fmMust(t, e)
			if !ok {
				t.Fatal("cached prepare")
			}
			service := m.NumeralService()
			count, e := service.NewValue()
			fmMust(t, e)
			ok, e = plugin.GetNumBranches(contains, formula, &count)
			fmMust(t, e)
			text, e := service.String(&count)
			fmMust(t, e)
			want := "2"
			if size == 1 {
				want = "1"
			}
			if !ok || text != want {
				t.Fatalf("branches %v %s want %s", ok, text, want)
			}
			fmMust(t, service.Del(&count))
			fmMust(t, ctx.Close())
			for i, root := range roots {
				if i == 2 { // The plugin was the equality's last owner after eqRef was released.
					if _, kindErr := m.GetKind(root); !errors.Is(kindErr, ast.ErrASTHandle) {
						t.Fatalf("plugin close kept equality alive: %v", kindErr)
					}
					continue
				}
				got, countErr := m.GetRefCount(root)
				fmMust(t, countErr)
				want := before[i]
				if i == 0 || i == 3 { // Releasing the equality also releases its argument pins.
					want--
				}
				if got != want {
					t.Fatalf("plugin close refcount for root %d = %d, want %d", i, got, want)
				}
			}
			if _, _, e = plugin.MkAtom(eq, true); !errors.Is(e, arena.ErrClosed) {
				t.Fatal("plugin remained open", e)
			}
		})
	}
}
