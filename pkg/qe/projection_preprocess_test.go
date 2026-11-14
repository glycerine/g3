package qe

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func projectionPreprocessOwner(t *testing.T, f *fmFixture) *projectionPreprocessor {
	t.Helper()
	p, err := newProjectionPreprocessor(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, p.Close()) })
	return p
}

type projectionLiteralCase struct {
	f              *fmFixture
	md             *model.Model
	forms          *[]ast.ASTRef
	original, want []ast.ASTHandle
}

var projectionLiteralModes = []string{"or-first", "and-rotation", "iff-true", "iff-false", "not-xor", "not-xor-false", "implies-true", "implies-false", "ite-true", "ite-false", "double-not", "not-and", "not-or", "xor-true", "xor-false", "not-iff", "not-implies", "not-ite-true", "not-ite-false", "not-distinct", "not-distinct-bool", "nested-positive", "nested-negative", "nested-shared", "term-ite", "true-skip"}

func projectionLiteralFixture(t *testing.T, mode string) *projectionLiteralCase {
	t.Helper()
	f := newFMFixture(t)
	m := f.r.manager
	md := predicateModel(t, f)
	a, b, c := f.constant("a", f.boolean), f.constant("b", f.boolean), f.constant("c", f.boolean)
	x, y := f.constant("x", f.integer), f.constant("y", f.integer)
	three := f.number("3")
	truth, falsehood := f.own(m.MkTrue()), f.own(m.MkFalse())
	av, bv, cv := true, true, true
	switch mode {
	case "iff-false", "not-xor-false", "not-or":
		av, bv = false, false
	case "implies-false", "ite-false", "not-and", "xor-false":
		av = false
		bv = false
		if mode == "xor-false" {
			bv = true
		}
	case "xor-true", "not-iff", "not-implies", "not-ite-true":
		bv = false
	case "not-ite-false":
		av, cv = false, false
	case "not-distinct-bool":
		av, bv = false, false
	case "nested-negative":
		av = false
	}
	for i, h := range []ast.ASTHandle{a, b, c} {
		value := falsehood
		if []bool{av, bv, cv}[i] {
			value = truth
		}
		predicateRegister(t, f, md, h, value)
	}
	predicateRegister(t, f, md, x, three)
	predicateRegister(t, f, md, y, three)
	not := func(h ast.ASTHandle) ast.ASTHandle { return f.own(m.MkNotSimplified(h)) }
	na, nb, nc := not(a), not(b), not(c)
	and := func(hs ...ast.ASTHandle) ast.ASTHandle { return f.own(m.MkFamilyApp(0, ast.ASTOpAnd, nil, hs)) }
	or := func(hs ...ast.ASTHandle) ast.ASTHandle { return f.own(m.MkFamilyApp(0, ast.ASTOpOr, nil, hs)) }
	forms, want := []ast.ASTHandle{a}, []ast.ASTHandle{a}
	switch mode {
	case "or-first":
		forms = []ast.ASTHandle{or(a, b)}
		want = []ast.ASTHandle{a}
	case "and-rotation":
		forms = []ast.ASTHandle{and(a, b), c}
		want = []ast.ASTHandle{b, c, a}
	case "iff-true":
		forms = []ast.ASTHandle{f.own(m.MkIff(a, b))}
		want = []ast.ASTHandle{a, b}
	case "iff-false":
		forms = []ast.ASTHandle{f.own(m.MkIff(a, b))}
		want = []ast.ASTHandle{na, nb}
	case "not-xor":
		forms = []ast.ASTHandle{not(f.own(m.MkXor(a, b)))}
		want = []ast.ASTHandle{a, b}
	case "not-xor-false":
		forms = []ast.ASTHandle{not(f.own(m.MkXor(a, b)))}
		want = []ast.ASTHandle{na, nb}
	case "implies-true":
		forms = []ast.ASTHandle{f.own(m.MkImplies(a, b))}
		want = []ast.ASTHandle{b}
	case "implies-false":
		forms = []ast.ASTHandle{f.own(m.MkImplies(a, b))}
		want = []ast.ASTHandle{na}
	case "ite-true":
		forms = []ast.ASTHandle{f.own(m.MkIte(a, b, c))}
		want = []ast.ASTHandle{b, a}
	case "ite-false":
		forms = []ast.ASTHandle{f.own(m.MkIte(a, b, c))}
		want = []ast.ASTHandle{c, na}
	case "double-not":
		forms = []ast.ASTHandle{f.own(m.MkNot(f.own(m.MkNot(a))))}
	case "not-and":
		forms = []ast.ASTHandle{not(and(a, b))}
		want = []ast.ASTHandle{na}
	case "not-or":
		forms = []ast.ASTHandle{not(or(a, b))}
		want = []ast.ASTHandle{nb, na}
	case "xor-true":
		forms = []ast.ASTHandle{f.own(m.MkXor(a, b))}
		want = []ast.ASTHandle{nb, a}
	case "xor-false":
		forms = []ast.ASTHandle{f.own(m.MkXor(a, b))}
		want = []ast.ASTHandle{b, na}
	case "not-iff":
		forms = []ast.ASTHandle{not(f.own(m.MkIff(a, b)))}
		want = []ast.ASTHandle{nb, a}
	case "not-implies":
		forms = []ast.ASTHandle{not(f.own(m.MkImplies(a, b)))}
		want = []ast.ASTHandle{nb, a}
	case "not-ite-true":
		forms = []ast.ASTHandle{not(f.own(m.MkIte(a, b, c)))}
		want = []ast.ASTHandle{nb, a}
	case "not-ite-false":
		forms = []ast.ASTHandle{not(f.own(m.MkIte(a, b, c)))}
		want = []ast.ASTHandle{nc, na}
	case "not-distinct":
		forms = []ast.ASTHandle{not(f.own(m.MkDistinct([]ast.ASTHandle{x, y})))}
		want = []ast.ASTHandle{f.own(m.MkEq(y, x))}
	case "not-distinct-bool":
		forms = []ast.ASTHandle{not(f.own(m.MkDistinct([]ast.ASTHandle{a, b, c})))}
		want = []ast.ASTHandle{nb, na}
	case "nested-positive", "nested-negative", "nested-shared":
		function := func(name string) ast.ASTHandle {
			n, e := m.Symbols.Intern(name)
			fmMust(t, e)
			d, e := m.MkFuncDecl(n, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
			fmMust(t, e)
			f.own(d, nil)
			fi, e := model.NewFunctionInterpretation(m, 1)
			fmMust(t, e)
			fmMust(t, fi.SetElse(f.variable(0, f.boolean)))
			fmMust(t, md.Core().RegisterFunction(d, fi))
			return d
		}
		d := function("P")
		pa := f.own(m.MkApp(d, []ast.ASTHandle{a}))
		forms = []ast.ASTHandle{pa}
		want = []ast.ASTHandle{f.own(m.MkApp(d, []ast.ASTHandle{truth})), a}
		if mode == "nested-negative" {
			forms = []ast.ASTHandle{not(pa)}
			want = []ast.ASTHandle{not(f.own(m.MkApp(d, []ast.ASTHandle{falsehood}))), na}
		}
		if mode == "nested-shared" {
			q := function("Q")
			forms = append(forms, f.own(m.MkApp(q, []ast.ASTHandle{a})))
			want = []ast.ASTHandle{want[0], f.own(m.MkApp(q, []ast.ASTHandle{truth})), a}
		}
	case "term-ite":
		forms = []ast.ASTHandle{f.own(m.MkEq(f.own(m.MkIte(a, x, y)), three))}
		want = []ast.ASTHandle{f.own(m.MkEq(f.own(m.MkIte(truth, x, y)), three)), a}
	case "true-skip":
		forms = []ast.ASTHandle{f.own(m.MkIte(truth, a, falsehood))}
		want = []ast.ASTHandle{a}
	}
	return &projectionLiteralCase{f: f, md: md, forms: predicateRefs(t, f, forms...), original: forms, want: want}
}
func projectionExpect(t *testing.T, f *fmFixture, forms []ast.ASTRef, want []ast.ASTHandle) {
	t.Helper()
	got := make([]ast.ASTHandle, len(forms))
	for i, r := range forms {
		got[i] = predicateValue(t, f, r)
	}
	if len(got) != len(want) {
		t.Fatal("literal count", len(got), len(want), got, want)
	}
	for i, h := range got {
		if h != want[i] {
			var g, w bytes.Buffer
			fmMust(t, writeExpression(f.r.manager, f.r.config, &g, h))
			fmMust(t, writeExpression(f.r.manager, f.r.config, &w, want[i]))
			t.Fatalf("literal %d: got %s want %s", i, g.String(), w.String())
		}
	}
}

func TestProjectionPreprocessLiteralSourceOrderAndModels(t *testing.T) {
	for _, mode := range projectionLiteralModes {
		t.Run(mode, func(t *testing.T) {
			c := projectionLiteralFixture(t, mode)
			p := projectionPreprocessOwner(t, c.f)
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
			fmMust(t, p.extractLiterals(c.md.Core(), c.forms))
			projectionExpect(t, c.f, *c.forms, c.want)
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
			if len(p.boolVisited) != 0 {
				t.Fatal("visited Boolean set was not reset")
			}
			// A successful call must not suppress branch literals on reuse.
			again := predicateRefs(t, c.f, c.original...)
			fmMust(t, p.extractLiterals(c.md.Core(), again))
			projectionExpect(t, c.f, *again, c.want)
		})
	}
}

func TestProjectionPreprocessEqualityOrientationChainsAndCycles(t *testing.T) {
	for _, mode := range []string{"right-selected", "chain", "cycle"} {
		t.Run(mode, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			fmMust(t, m.RegisterDeclarationPlugins())
			p := projectionPreprocessOwner(t, f)
			x, y := f.constant("sx", f.integer), f.constant("sy", f.integer)
			eq := f.own(m.MkEq(x, y))
			vars := predicateRefs(t, f, x, y)
			forms := predicateRefs(t, f, eq, f.own(m.MkArithmeticGE(x, f.number("0"))))
			if mode == "chain" {
				forms = predicateRefs(t, f, eq, f.own(m.MkEq(y, f.number("3"))), f.own(m.MkArithmeticGE(x, f.number("0"))))
			}
			if mode == "cycle" {
				forms = predicateRefs(t, f, f.own(m.MkEq(x, f.own(m.MkArithmeticAdd(x, f.number("1"))))))
				vars = predicateRefs(t, f, x)
			}
			changed, err := p.solveEqualities(vars, forms)
			fmMust(t, err)
			if mode == "cycle" {
				if changed || len(*vars) != 1 || len(*forms) != 1 {
					t.Fatal("cyclic definition accepted")
				}
				return
			}
			if !changed {
				t.Fatal("missed equality")
			}
			if mode == "right-selected" {
				projectionExpect(t, f, *vars, []ast.ASTHandle{x})
				projectionExpect(t, f, *forms, []ast.ASTHandle{f.own(m.MkArithmeticGE(x, f.number("0")))})
			} else {
				if len(*vars) != 0 || len(*forms) != 1 || !m.IsTrue(predicateValue(t, f, (*forms)[0])) {
					t.Fatal("chain did not reduce")
				}
			}
		})
	}
}

func TestProjectionPreprocessBooleanProjectionAndPatternFiltering(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	md := predicateModel(t, f)
	p := projectionPreprocessOwner(t, f)
	a := f.constant("pa", f.boolean)
	x := f.constant("px", f.integer)
	predicateRegister(t, f, md, a, f.own(m.MkTrue()))
	predicateRegister(t, f, md, x, f.number("3"))
	bound := f.own(m.MkArithmeticGE(x, f.number("1")))
	forms := predicateRefs(t, f, a, f.own(m.MkOrArray([]ast.ASTHandle{f.own(m.MkNot(a)), bound})))
	vars := predicateRefs(t, f, a, x)
	fmMust(t, p.projectBooleans(md.Core(), vars, forms))
	projectionExpect(t, f, *vars, []ast.ASTHandle{x})
	projectionExpect(t, f, *forms, []ast.ASTHandle{bound})
	arithmeticEvaluate(t, f, md, *forms)
	patternOnly, unused := f.constant("pattern-only", f.integer), f.constant("unused", f.integer)
	v := f.variable(0, f.integer)
	pat := f.own(m.MkPattern([]ast.ASTHandle{f.predicate("trigger", patternOnly, v)}))
	name, err := m.Symbols.Intern("bound")
	fmMust(t, err)
	q := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: f.own(m.MkTrue()), Patterns: []ast.ASTHandle{pat}}))
	vars = predicateRefs(t, f, unused, patternOnly)
	forms = predicateRefs(t, f, q)
	fmMust(t, p.filterVariables(vars, forms))
	projectionExpect(t, f, *vars, []ast.ASTHandle{patternOnly})
}

func TestProjectionPreprocessAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "and-rotation", "not-distinct", "not-distinct-bool", "nested-positive", "nested-negative", "nested-shared", "term-ite", "equality", "cycle", "booleans", "filter", "trace", "closed-trace"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 4096 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					mode := phase
					if phase == "constructor" || phase == "equality" || phase == "cycle" || phase == "booleans" || phase == "filter" || strings.Contains(phase, "trace") {
						mode = "term-ite"
					}
					c := projectionLiteralFixture(t, mode)
					m := c.f.r.manager
					var trace bytes.Buffer
					if strings.Contains(phase, "trace") {
						var w io.Writer = &trace
						if phase == "closed-trace" {
							w = closedPredicateTrace{}
						}
						tags := diagnostic.NewTraceTagsWithWriter(w)
						tags.Enable("qe")
						t.Cleanup(tags.Finalize)
						c.f.r.config.Trace = tags
					}
					var p *projectionPreprocessor
					var err error
					if phase != "constructor" {
						p = projectionPreprocessOwner(t, c.f)
					}
					x := c.f.constant("ax", c.f.integer)
					y := c.f.constant("ay", c.f.integer)
					vars := predicateRefs(t, c.f, x, y)
					if phase == "equality" {
						c.forms = predicateRefs(t, c.f, c.f.own(m.MkEq(x, y)), c.f.own(m.MkEq(y, c.f.number("3"))), c.f.own(m.MkArithmeticGE(x, c.f.number("1"))))
					}
					if phase == "cycle" {
						c.forms = predicateRefs(t, c.f, c.f.own(m.MkEq(x, c.f.own(m.MkArithmeticAdd(x, c.f.number("1"))))))
					}
					if phase == "booleans" {
						a := c.f.constant("model-bool", c.f.boolean)
						predicateRegister(t, c.f, c.md, a, c.f.own(m.MkTrue()))
						vars = predicateRefs(t, c.f, a, x)
						c.forms = predicateRefs(t, c.f, a, c.f.own(m.MkArithmeticGE(x, c.f.number("0"))))
					}
					m.Budget().FailAfter(stop)
					switch phase {
					case "constructor":
						p, err = newProjectionPreprocessor(m, c.f.r.config)
					case "equality", "cycle":
						_, err = p.solveEqualities(vars, c.forms)
					case "booleans":
						err = p.projectBooleans(c.md.Core(), vars, c.forms)
					case "filter":
						err = p.filterVariables(vars, c.forms)
					default:
						err = p.extractLiterals(c.md.Core(), c.forms)
					}
					m.Budget().FailAfter(-1)
					var failure memory.ConstructionFailure
					if errors.As(err, &failure) {
						fmMust(t, failure.Cleanup())
					}
					if phase == "constructor" && p != nil {
						fmMust(t, p.Close())
					}
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						var memout memory.Failure
						if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
							t.Fatalf("%v\n%s", err, trace.String())
						}
					}
				})
				if t.Failed() {
					t.Fatal("projection preprocessing frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}

func TestProjectionPreprocessTraceVectorsAndOwnerGuards(t *testing.T) {
	c := projectionLiteralFixture(t, "and-rotation")
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("qe")
	t.Cleanup(tags.Finalize)
	c.f.r.config.Trace = tags
	p := projectionPreprocessOwner(t, c.f)
	fmMust(t, p.extractLiterals(c.md.Core(), c.forms))
	projectionExpect(t, c.f, *c.forms, c.want)
	if tags.ShouldTrace("qe") {
		if !strings.Contains(out.String(), "extract_literals src/qe/qe_mbp.cpp:310") || !strings.Contains(out.String(), "\n(and a b)\nc\n") || !strings.Contains(out.String(), "\nb\nc\na\n") {
			t.Fatal(out.String())
		}
	}
	copy := *p
	if err := copy.filterVariables(predicateRefs(t, c.f), c.forms); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	other := newFMFixture(t)
	foreign := predicateModel(t, other)
	if err := p.extractLiterals(foreign.Core(), c.forms); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	bad := predicateRefs(t, c.f, c.f.number("1"))
	if err := p.extractLiterals(c.md.Core(), bad); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	fmMust(t, p.Close())
	if err := p.extractLiterals(c.md.Core(), c.forms); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}

// extract_bools tests SASSERT(m.is_true(val) || m.is_false(val)), which the
// release build compiles out: a Boolean atom whose value is not a literal
// (division by zero under a partial model) takes mk_not instead of failing.
// AUDIT.md FUZZ_C-F4.
func TestProjectionPreprocessNonLiteralBooleanValueIsNegated(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	md := predicateModel(t, f)
	a := f.constant("a", f.boolean)
	rx := f.constant("rx", f.real)
	real := func(text string) ast.ASTHandle { return f.own(m.ArithmeticRealNumeralText(text)) }
	predicateRegister(t, f, md, a, f.own(m.MkTrue()))
	predicateRegister(t, f, md, rx, real("3"))
	atom := func(c ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkArithmeticLE(f.own(m.MkArithmeticDiv(rx, real("0"))), f.own(m.MkIte(c, real("2"), real("3")))))
	}
	forms := predicateRefs(t, f, atom(a))
	p := projectionPreprocessOwner(t, f)
	fmMust(t, p.extractLiterals(md.Core(), forms))
	projectionExpect(t, f, *forms, []ast.ASTHandle{f.own(m.MkNot(atom(f.own(m.MkTrue())))), a})
}
