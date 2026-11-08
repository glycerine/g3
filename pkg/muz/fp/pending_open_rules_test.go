package fp

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPendingOpenDisplayPromotesRulesAndSwapsRawArchive(t *testing.T) {
	for _, backend := range []string{"pdr", "duality"} {
		t.Run(backend, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			must(err)
			defer func() { must(e.Close()) }()
			must(e.SelectEngine(backend))
			var roots []ast.ASTRef
			pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				must(err)
				r, err := m.NewRef(h)
				must(err)
				roots = append(roots, r)
				return h
			}
			defer func() {
				for i := len(roots) - 1; i >= 0; i-- {
					must(m.DelRef(&roots[i]))
				}
			}()
			boolean := pin(m.MkBoolSort())
			v := pin(m.MkVar(0, boolean))
			truth := pin(m.MkTrue())
			var declarations []ast.ASTHandle
			for i, name := range []string{"closed-a", "open-b", "closed-c", "open-d", "closed-e"} {
				symbol, err := m.Symbols.Intern(name)
				must(err)
				d := pin(m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
				must(e.RegisterPredicate(d))
				declarations = append(declarations, d)
				arg := truth
				if i == 1 || i == 3 {
					arg = v
				}
				app := pin(m.MkApp(d, []ast.ASTHandle{arg}))
				must(e.addRuleWithBoundEager(app, name, uint32(i+1)))
			}
			checkRaw := func(wantNames []string, wantBounds []uint32) {
				t.Helper()
				forms, names, bounds, err := e.RawRules()
				must(err)
				if len(forms) != len(wantNames) {
					t.Fatal("raw count", len(forms), wantNames)
				}
				for i := range forms {
					if names[i] != wantNames[i] || bounds[i] != wantBounds[i] {
						t.Fatal("source raw swap/bound order", names, bounds)
					}
					must(m.DelRef(&forms[i]))
				}
			}
			display := func() string { var out bytes.Buffer; must(e.WriteContextSMT2(&out, nil, nil)); return out.String() }
			must(e.Push())
			text := display()
			position := -1
			for _, name := range []string{"open-b", "open-d", "closed-a", "closed-e", "closed-c"} {
				next := strings.Index(text, ":named "+name)
				if next <= position {
					t.Fatal("wrong source display order", name, text)
				}
				position = next
			}
			checkRaw([]string{"closed-a", "closed-e", "closed-c"}, []uint32{1, 2, 3})
			if repeated := display(); repeated != text {
				t.Fatal("repeat display changed normalized/archive order", text, repeated)
			}
			must(e.Pop())
			checkRaw([]string{"closed-a", "open-b", "closed-c", "open-d", "closed-e"}, []uint32{1, 2, 3, 4, 5})
			if err := e.WriteContextSMT2(pendingDisplayRefusingWriter{}, nil, nil); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal("display writer refusal", err)
			}
			checkRaw([]string{"closed-a", "closed-e", "closed-c"}, []uint32{1, 2, 3})
			if again := display(); again != text {
				t.Fatal("scope did not restore promotion state", again, text)
			}
			status, err := e.QueryPredicate(declarations[1])
			must(err)
			want := lbool.True
			if backend == "duality" {
				want = lbool.False
			}
			if status != want {
				t.Fatal("normalized versus raw engine boundary", status, want)
			}
			status, err = e.QueryPredicate(declarations[4])
			must(err)
			if status != lbool.True {
				t.Fatal("moved closed raw formula lost", status)
			}
		})
	}
}

type pendingDisplayRefusingWriter struct{}

func (pendingDisplayRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
