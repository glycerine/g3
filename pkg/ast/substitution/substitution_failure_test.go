package substitution

// Source: substitution::apply uses var_shifter/expr_ref locals and m_new_exprs
// to own fresh expressions before a result escapes its traversal.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSubstitutionFreshExpressionsAllocationFailureAndRetry(t *testing.T) {
	for _, shape := range []string{"variable", "application", "quantifier", "shift-variable", "shift-application", "shift-quantifier", "modulo-application", "modulo-quantifier"} {
		t.Run(shape, func(t *testing.T) {
			failures := 0
			for available := 0; available <= 16; available++ {
				succeeded := false
				for stop := int64(0); stop < 128; stop++ {
					t.Run(fmt.Sprintf("references=%d/allowance=%d", available, stop), func(t *testing.T) {
						if testSubstitutionFreshExpressionFailure(t, shape, available, stop) {
							failures++
						} else {
							succeeded = true
						}
					})
					if t.Failed() {
						return
					}
					if succeeded {
						break
					}
				}
				if !succeeded {
					t.Fatal("substitution did not reach successful allocation", available)
				}
			}
			if failures == 0 {
				t.Fatal("fresh substitution did not exercise allocation refusal")
			}
		})
	}
}

func testSubstitutionFreshExpressionFailure(t *testing.T, shape string, available int, stop int64) bool {
	t.Helper()
	m := offsetManager(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var roots, pressure []ast.ASTRef
	t.Cleanup(func() {
		m.Budget().FailAfter(-1)
		for _, list := range [][]ast.ASTRef{pressure, roots} {
			for i := len(list) - 1; i >= 0; i-- {
				must(m.DelRef(&list[i]))
			}
		}
	})
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		reference, err := m.NewRef(h)
		must(err)
		roots = append(roots, reference)
		return h
	}
	boolean, err := m.MkBoolSort()
	must(err)
	f := own(m.MkFuncDecl(symbol.Numerical(7301), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	g := own(m.MkFuncDecl(symbol.Numerical(7302), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	variable := func(index uint32) ast.ASTHandle { return own(m.MkVar(index, boolean)) }
	app := func(declaration, argument ast.ASTHandle) ast.ASTHandle {
		return own(m.MkApp(declaration, []ast.ASTHandle{argument}))
	}
	quantifier := func(body ast.ASTHandle) ast.ASTHandle {
		return own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(7303)}, Body: body}))
	}
	s, err := New(m)
	must(err)
	t.Cleanup(func() { must(s.Close()) })
	must(s.Reserve(2, 8))
	var input ast.ASTHandle
	var replace, replacement ExprOffset
	offset := uint32(1)
	deltas := []uint32{0, 3}
	var wrappers []string
	wantIndex := uint32(3)
	switch shape {
	case "variable":
		input = variable(0)
	case "application":
		input = app(f, variable(0))
		wrappers = []string{"f"}
	case "quantifier":
		input = quantifier(app(f, variable(1)))
		wrappers, wantIndex = []string{"q", "f"}, 4
	case "shift-variable", "shift-application", "shift-quantifier":
		offset, deltas = 0, []uint32{0, 0}
		input = quantifier(variable(1))
		var target ast.ASTHandle
		switch shape {
		case "shift-variable":
			target = variable(3)
			wrappers, wantIndex = []string{"q"}, 4
		case "shift-application":
			target = app(f, variable(3))
			wrappers, wantIndex = []string{"q", "f"}, 4
		case "shift-quantifier":
			target = quantifier(app(f, variable(2)))
			wrappers, wantIndex = []string{"q", "q", "f"}, 3
		}
		must(s.Insert(0, 0, NewExprOffset(target, 0)))
	case "modulo-application":
		offset, deltas = 0, []uint32{0, 0}
		input = quantifier(app(f, variable(1)))
		replace = NewExprOffset(app(f, variable(0)), 0)
		replacement = NewExprOffset(app(g, variable(2)), 0)
		wrappers = []string{"q", "g"}
	case "modulo-quantifier":
		offset, deltas = 0, []uint32{0, 0}
		input = quantifier(variable(1))
		replace = NewExprOffset(variable(0), 0)
		replacement = NewExprOffset(quantifier(app(f, variable(2))), 0)
		wrappers = []string{"q", "q", "f"}
	default:
		t.Fatal("unknown substitution fixture", shape)
	}
	out, err := m.NewRef(ast.ASTHandle{})
	must(err)
	t.Cleanup(func() { must(m.DelRef(&out)) })
	apply := func() error {
		if !replace.Expr().IsNull() {
			return s.ApplyModulo(2, deltas, NewExprOffset(input, offset), replace, replacement, out)
		}
		return s.Apply(2, deltas, NewExprOffset(input, offset), out)
	}
	checkOutput := func() {
		t.Helper()
		node, err := m.RefGet(out)
		must(err)
		for _, wrapper := range wrappers {
			kind, err := m.GetKind(node)
			must(err)
			if wrapper == "q" {
				if kind != ast.ASTQuantifier {
					t.Fatal("substitution lost a quantifier", shape)
				}
				node, err = m.GetQuantifierBody(node)
				must(err)
				continue
			}
			if kind != ast.ASTApp {
				t.Fatal("substitution lost an application", shape)
			}
			declaration, err := m.GetDecl(node)
			must(err)
			want := f
			if wrapper == "g" {
				want = g
			}
			if declaration != want {
				t.Fatal("substitution changed the application declaration", shape)
			}
			node, err = m.GetArg(node, 0)
			must(err)
		}
		index, err := m.GetVarIndex(node)
		must(err)
		if index != wantIndex {
			t.Fatal("substitution changed the expected variable offset", shape, index, wantIndex)
		}
	}

	// Vary the number of reference slots available before the next physical
	// page. Every allocation allowance starts with a fresh, cold traversal.
	for {
		m.Budget().FailAfter(0)
		for {
			reference, cause := m.NewRef(ast.ASTHandle{})
			if cause != nil {
				m.Budget().FailAfter(-1)
				if reference != (ast.ASTRef{}) || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("reference allocation frontier", reference, cause)
				}
				break
			}
			pressure = append(pressure, reference)
		}
		if len(pressure) >= available {
			break
		}
		reference, err := m.NewRef(ast.ASTHandle{})
		must(err)
		pressure = append(pressure, reference)
	}
	for i := 0; i < available; i++ {
		last := len(pressure) - 1
		must(m.DelRef(&pressure[last]))
		pressure = pressure[:last]
	}
	before := m.Stats()
	inputCount, err := m.GetRefCount(input)
	must(err)
	checkOwners := func(label string) {
		t.Helper()
		after := m.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
			t.Fatal(label, "changed live AST ownership", before, after)
		}
		refs, err := m.GetRefCount(input)
		must(err)
		if refs != inputCount {
			t.Fatal(label, "changed input ownership", inputCount, refs)
		}
	}
	m.Budget().FailAfter(stop)
	cause := apply()
	m.Budget().FailAfter(-1)
	if cause != nil {
		value, err := m.RefGet(out)
		must(err)
		if !errors.Is(cause, arena.ErrInjectedFailure) || !value.IsNull() {
			t.Fatal("failed substitution changed its output or cause", value, cause)
		}
	} else {
		checkOutput()
	}
	must(m.ResetRef(out))
	must(s.ResetCache())
	checkOwners("allocation campaign")
	must(apply())
	checkOutput()
	must(m.ResetRef(out))
	must(s.ResetCache())
	must(s.ResetCache())
	checkOwners("completed retry")
	return cause != nil
}
