package qe_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestEqualityDERSelectorCancellationAndOwnerGuards(t *testing.T) {
	f := setup(t)
	x, three := f.variable(t, 0, f.integer), f.numeral(t, "3")
	eq := f.equality(t, x, three)
	conjs := conjunctions(t, f, eq, predicate(t, f, "guard-p", x))
	d := equalityReducer(t, f, nil, selector(t, f, nil, true))
	must(t, d.EliminateConjunctions(conjs))
	if len(*conjs) != 2 || f.get(t, (*conjs)[0]) != eq {
		t.Fatal("empty selector changed input")
	}
	must(t, d.SetVariableTest(selector(t, f, []uint32{0}, true)))
	f.m.ResourceLimit().IncCancel()
	err := d.EliminateConjunctions(conjs)
	reason := f.m.ResourceLimit().CancelMessage()
	f.m.ResourceLimit().DecCancel()
	var canceled tactic.TacticException
	if !errors.As(err, &canceled) || canceled.Message() != reason {
		t.Fatal("source cancellation", err)
	}
	if len(*conjs) != 2 || f.get(t, (*conjs)[0]) != eq {
		t.Fatal("canceled call changed input")
	}
	must(t, d.EliminateConjunctions(conjs))
	if len(*conjs) != 1 || f.get(t, (*conjs)[0]) != predicate(t, f, "guard-p", three) {
		t.Fatal("cancellation recovery or selector update failed")
	}
	copy := *d
	if err := copy.EliminateConjunctions(conjs); err == nil {
		t.Fatal("copied owner accepted")
	}
	if err := copy.Close(); err == nil {
		t.Fatal("copied owner closed original")
	}
	must(t, d.Close())
	if err := d.EliminateConjunctions(conjs); err == nil {
		t.Fatal("closed owner accepted")
	}
}

func equalityReducer(t *testing.T, f *fixture, p *uparams.ParamsRef, selection *qe.VariableTest) *qe.EqualityDER {
	t.Helper()
	d, err := qe.NewEqualityDER(f.m, p, f.config)
	must(t, err)
	t.Cleanup(func() { must(t, d.Close()) })
	if selection != nil {
		must(t, d.SetVariableTest(selection))
	}
	return d
}
func conjunctions(t *testing.T, f *fixture, hs ...ast.ASTHandle) *[]ast.ASTRef {
	t.Helper()
	result := new([]ast.ASTRef)
	t.Cleanup(func() {
		for i := len(*result) - 1; i >= 0; i-- {
			if (*result)[i] != (ast.ASTRef{}) {
				must(t, f.m.DelRef(&(*result)[i]))
			}
		}
	})
	for _, h := range hs {
		r, err := f.m.NewRef(h)
		must(t, err)
		*result = append(*result, r)
	}
	return result
}
func predicate(t *testing.T, f *fixture, name string, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	var sorts []ast.ASTHandle
	for _, h := range args {
		s, err := f.m.GetSort(h)
		must(t, err)
		sorts = append(sorts, s)
	}
	n, err := f.m.Symbols.Intern(name)
	must(t, err)
	decl, err := f.m.MkFuncDecl(n, sorts, f.boolean, ast.ASTFuncDeclInfo{})
	must(t, err)
	h, err := f.m.MkApp(decl, args)
	must(t, err)
	return f.pin(t, h)
}
func termFunction(t *testing.T, f *fixture, name string, result ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	var sorts []ast.ASTHandle
	for _, h := range args {
		s, err := f.m.GetSort(h)
		must(t, err)
		sorts = append(sorts, s)
	}
	n, err := f.m.Symbols.Intern(name)
	must(t, err)
	decl, err := f.m.MkFuncDecl(n, sorts, result, ast.ASTFuncDeclInfo{})
	must(t, err)
	h, err := f.m.MkApp(decl, args)
	must(t, err)
	return f.pin(t, h)
}
func TestEqualityDERChainsCyclesAndOccursRefusals(t *testing.T) {
	for _, mode := range []string{"chain", "cycle", "self", "quantified-definition", "value-preference", "constant-preference"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			x, y := f.variable(t, 0, f.integer), f.variable(t, 1, f.integer)
			three := f.numeral(t, "3")
			p := predicate(t, f, "chain-p", x, y)
			forms := []ast.ASTHandle{f.equality(t, x, y), f.equality(t, y, three), p}
			indices := []uint32{0, 1}
			if mode == "cycle" {
				forms = []ast.ASTHandle{f.equality(t, x, y), f.equality(t, y, x), p}
			}
			if mode == "self" {
				forms = []ast.ASTHandle{f.equality(t, x, termFunction(t, f, "recursive", f.integer, x)), p}
				indices = []uint32{0}
			}
			if mode == "quantified-definition" {
				b := f.variable(t, 0, f.boolean)
				inner := f.variable(t, 0, f.integer)
				name, err := f.m.Symbols.Intern("inner")
				must(t, err)
				q, err := f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: predicate(t, f, "inner-p", inner)})
				must(t, err)
				f.pin(t, q)
				forms = []ast.ASTHandle{f.equality(t, b, q), predicate(t, f, "outer-b", b)}
				indices = []uint32{0}
			}
			if mode == "value-preference" || mode == "constant-preference" {
				k := f.constant(t, "preferred-constant", f.integer)
				forms = []ast.ASTHandle{f.equality(t, x, termFunction(t, f, "nonground", f.integer, y)), f.equality(t, x, k), predicate(t, f, "preferred-p", x)}
				if mode == "value-preference" {
					forms = append(forms, f.equality(t, x, three))
				}
				indices = []uint32{0}
			}
			conjs := conjunctions(t, f, forms...)
			d := equalityReducer(t, f, nil, selector(t, f, indices, true))
			must(t, d.EliminateConjunctions(conjs))
			switch mode {
			case "chain":
				if len(*conjs) != 1 || f.get(t, (*conjs)[0]) != predicate(t, f, "chain-p", three, three) {
					t.Fatal("transitive substitution failed")
				}
			case "cycle":
				if len(*conjs) != 1 || f.get(t, (*conjs)[0]) != predicate(t, f, "chain-p", x, x) {
					t.Fatal("cycle was not broken at source index", render(t, f, f.get(t, (*conjs)[0])))
				}
			case "self", "quantified-definition":
				if len(*conjs) != len(forms) {
					t.Fatal("unsafe definition accepted")
				}
				for i, h := range forms {
					if f.get(t, (*conjs)[i]) != h {
						t.Fatal("refused definition changed", i)
					}
				}
			default:
				want := three
				if mode == "constant-preference" {
					want = f.constant(t, "preferred-constant", f.integer)
				}
				found := false
				expected := predicate(t, f, "preferred-p", want)
				for _, r := range *conjs {
					h := f.get(t, r)
					if h == expected {
						found = true
					}
					occurs, err := qe.OccursVariable(f.m, 0, h)
					must(t, err)
					if occurs {
						t.Fatal("eliminated variable remained", render(t, f, h))
					}
				}
				if !found {
					t.Fatal("definition preference not applied")
				}
			}
		})
	}
}

// AUDIT.md qe_02-F1: reduce_var_set and the flatten_definitions traces use
// m.mk_and, so a single conjunct prints as the unary (and c).
func TestEqualityDERSingletonConjunctionUsesPlainAnd(t *testing.T) {
	f := setup(t)
	x := f.variable(t, 0, f.integer)
	p := predicate(t, f, "singleton-p", x)
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("qe_lite")
	t.Cleanup(tags.Finalize)
	f.config.Trace = tags
	conjs := conjunctions(t, f, p)
	d := equalityReducer(t, f, nil, selector(t, f, []uint32{0}, true))
	must(t, d.EliminateConjunctions(conjs))
	if !tags.ShouldTrace("qe_lite") {
		t.Skip("TRACE records need -tags g3trace")
	}
	want := "(and " + render(t, f, p) + ")"
	if !strings.Contains(trace.String(), want) {
		t.Fatalf("flatten_definitions trace lacks %q:\n%s", want, trace.String())
	}
}

func TestEqualityDERArithmeticCoefficientAndITEBranches(t *testing.T) {
	for _, mode := range []string{"int-one", "int-minus-one", "int-two-refused", "real-two", "real-unary-negative", "ite", "ite-variable-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			sort := f.integer
			if mode == "real-two" || mode == "real-unary-negative" {
				var err error
				family, e := f.m.ArithmeticFamilyID()
				must(t, e)
				sort, err = f.m.MkFamilySort(family, 0, nil)
				must(t, err)
				f.pin(t, sort)
			}
			x, y := f.variable(t, 0, sort), f.variable(t, 1, sort)
			num := func(s string) ast.ASTHandle {
				t.Helper()
				integer := sort == f.integer
				owned, err := f.m.NumeralService().NewValue()
				must(t, err)
				must(t, f.m.NumeralService().SetString(&owned, s))
				h, err := f.m.ArithmeticNumeral(&owned, integer)
				must(t, err)
				must(t, f.m.NumeralService().Del(&owned))
				return f.pin(t, h)
			}
			coefficient := num("1")
			if mode == "int-minus-one" {
				coefficient = num("-1")
			}
			if mode == "int-two-refused" || mode == "real-two" || mode == "real-unary-negative" {
				coefficient = num("2")
			}
			if mode == "real-unary-negative" {
				h, err := f.m.MkArithmeticNeg(coefficient)
				must(t, err)
				coefficient = f.pin(t, h)
			}
			product, err := f.m.MkArithmeticMul(coefficient, x)
			must(t, err)
			f.pin(t, product)
			left, err := f.m.MkArithmeticAdd(product, num("3"))
			must(t, err)
			f.pin(t, left)
			definition := f.equality(t, left, num("7"))
			expected := num("4")
			if mode == "int-minus-one" {
				expected = num("-4")
			}
			if mode == "real-two" {
				expected = num("2")
			}
			if mode == "real-unary-negative" {
				expected = num("-2")
			}
			if mode == "ite" || mode == "ite-variable-mismatch" {
				condition := f.constant(t, "branch", f.boolean)
				other := x
				if mode == "ite-variable-mismatch" {
					other = y
				}
				h, err := f.m.MkIte(condition, f.equality(t, x, num("1")), f.equality(t, other, num("2")))
				must(t, err)
				definition = f.pin(t, h)
				expected, err = f.m.MkIte(condition, num("1"), num("2"))
				must(t, err)
				f.pin(t, expected)
			}
			p := predicate(t, f, "arithmetic-p", x)
			conjs := conjunctions(t, f, definition, p)
			d := equalityReducer(t, f, nil, selector(t, f, []uint32{0, 1}, true))
			must(t, d.EliminateConjunctions(conjs))
			if mode == "int-two-refused" || mode == "ite-variable-mismatch" {
				if len(*conjs) != 2 || f.get(t, (*conjs)[0]) != definition {
					t.Fatal("noninvertible or mismatched definition accepted")
				}
				return
			}
			want := predicate(t, f, "arithmetic-p", expected)
			if len(*conjs) != 1 || f.get(t, (*conjs)[0]) != want {
				for _, r := range *conjs {
					t.Log(render(t, f, f.get(t, r)))
				}
				t.Fatal("arithmetic/ITE substitution mismatch", render(t, f, want))
			}
		})
	}
}

func TestEqualityDERUnconstrainedDisequalityAndBooleanLiterals(t *testing.T) {
	for _, mode := range []string{"unconstrained", "other-occurrence", "boolean-positive", "boolean-negative", "negative-iff"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			x := f.variable(t, 0, f.integer)
			three := f.numeral(t, "3")
			forms := []ast.ASTHandle{f.not(t, f.equality(t, x, three))}
			if mode == "other-occurrence" {
				forms = append(forms, predicate(t, f, "used", x))
			}
			if mode == "boolean-positive" || mode == "boolean-negative" {
				x = f.variable(t, 0, f.boolean)
				literal := x
				if mode == "boolean-negative" {
					literal = f.not(t, x)
				}
				forms = []ast.ASTHandle{literal, predicate(t, f, "bool-p", x)}
			}
			if mode == "negative-iff" {
				x = f.variable(t, 0, f.boolean)
				forms = []ast.ASTHandle{f.not(t, f.equality(t, x, f.constant(t, "free-b", f.boolean)))}
			}
			conjs := conjunctions(t, f, forms...)
			d := equalityReducer(t, f, nil, selector(t, f, []uint32{0}, true))
			must(t, d.EliminateConjunctions(conjs))
			if mode == "unconstrained" {
				if len(*conjs) != 1 || !f.m.IsTrue(f.get(t, (*conjs)[0])) {
					t.Fatal("source true literal was not retained")
				}
				return
			}
			if mode == "other-occurrence" || mode == "negative-iff" {
				if len(*conjs) != len(forms) {
					t.Fatal("source disequality guard lost literals")
				}
				return
			}
			value, err := f.m.MkTrue()
			must(t, err)
			if mode == "boolean-negative" {
				value, err = f.m.MkFalse()
				must(t, err)
			}
			if len(*conjs) != 1 || f.get(t, (*conjs)[0]) != predicate(t, f, "bool-p", value) {
				t.Fatal("Boolean literal definition failed")
			}
		})
	}
}
