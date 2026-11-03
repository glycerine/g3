package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocNegativeShapePreflightDoesNotPublishGroundPrefix(t *testing.T) {
	for _, mode := range []string{"constant", "unbound", "negative-only"} {
		t.Run(mode, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
			must(e.SelectEngine("datalog"))
			boolean, err := m.MkBoolSort()
			must(err)
			sort, err := m.BitVectorSort(2)
			must(err)
			declare := func(label string, arity int) ast.ASTHandle {
				t.Helper()
				name, cause := m.Symbols.Intern(label)
				must(cause)
				sorts := make([]ast.ASTHandle, arity)
				for index := range sorts {
					sorts[index] = sort
				}
				declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(cause)
				must(e.RegisterPredicate(declaration))
				return declaration
			}
			p := declare("doc-negative-shape-P", 1)
			prefix := declare("doc-negative-shape-prefix", 1)
			negative := declare("doc-negative-shape-negative", 1)
			stop := declare("doc-negative-shape-stop", 0)
			outputArity := 1
			if mode == "negative-only" {
				outputArity = 0
			}
			output := declare("doc-negative-shape-output", outputArity)
			must(e.AddTableFact(p, []uint32{1}))
			pin := func(term ast.ASTHandle, cause error) ast.ASTHandle {
				t.Helper()
				must(cause)
				root, cause := m.NewRef(term)
				must(cause)
				deferClose := root
				t.Cleanup(func() { must(m.DelRef(&deferClose)) })
				return term
			}
			x := pin(m.MkVar(0, sort))
			y := pin(m.MkVar(1, sort))
			zero := pin(m.BitVectorNumeralUnsigned(0, 2))
			one := pin(m.BitVectorNumeralUnsigned(1, 2))
			two := pin(m.BitVectorNumeralUnsigned(2, 2))
			app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
				t.Helper()
				term, cause := m.MkApp(declaration, arguments)
				must(cause)
				return term
			}
			add := func(head ast.ASTHandle, tails []ast.ASTHandle, flags []bool, name string) {
				t.Helper()
				rule, cause := e.rules.NewRule(head, tails, flags, name)
				must(cause)
				must(e.set.Add(rule))
				must(rule.Close())
			}
			// This valid ground rule precedes the unsupported rule. Whole-program
			// negative preflight must reject before that prefix can be evaluated.
			add(app(prefix, two), nil, nil, "negative-shape-ground-prefix")
			switch mode {
			case "constant":
				add(app(output, x), []ast.ASTHandle{app(p, x), app(negative, zero)}, []bool{false, true}, "negative-shape-constant")
			case "unbound":
				add(app(output, x), []ast.ASTHandle{app(p, x), app(negative, y)}, []bool{false, true}, "negative-shape-unbound")
			case "negative-only":
				add(app(output), []ast.ASTHandle{app(stop)}, []bool{true}, "negative-shape-only")
			}
			for iteration := 0; iteration < 2; iteration++ {
				status, err := e.QueryPredicate(output)
				if status != lbool.Undef || !errors.Is(err, ErrUDocDatalogFragment) {
					t.Fatal("unsupported negative shape query", status, err)
				}
				if e.udocDatalog == nil || e.udocDatalog.result[prefix] == nil {
					t.Fatal("query did not reach concrete UDoc preflight")
				}
				if empty, cause := e.udocDatalog.result[prefix].Empty(); cause != nil || !empty {
					t.Fatal("rejected negative rule published earlier ground prefix", empty, cause)
				}
				if empty, cause := e.udocDatalog.seeds[prefix].Empty(); cause != nil || !empty {
					t.Fatal("rejected negative rule changed fact seeds", empty, cause)
				}
				if found, cause := e.udocDatalog.seeds[p].ContainsFact([]ast.ASTHandle{one}); cause != nil || !found {
					t.Fatal("negative preflight lost original fact seed", found, cause)
				}
				if finalized, cause := e.set.IsFinalized(); cause != nil || finalized {
					t.Fatal("failed query did not restore initially open rule set", finalized, cause)
				}
				if _, cause := e.Answer(prefix); cause == nil {
					t.Fatal("failed negative query exposed a prefix answer")
				}
			}
		})
	}
}
