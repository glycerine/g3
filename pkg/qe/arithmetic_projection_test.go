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
)

type arithmeticProjectionCase struct {
	f           *fmFixture
	md          *model.Model
	vars, forms *[]ast.ASTRef
	original    []ast.ASTHandle
	x, sort     ast.ASTHandle
}

func arithmeticProjectionOwner(t *testing.T, f *fmFixture) *ArithmeticProjection {
	t.Helper()
	p, err := NewArithmeticProjection(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, p.Close()) })
	return p
}
func arithmeticCase(t *testing.T, mode string) *arithmeticProjectionCase {
	t.Helper()
	f := newFMFixture(t)
	m := f.r.manager
	md := predicateModel(t, f)
	integer := strings.HasPrefix(mode, "integer") || mode == "modulo"
	sort := f.real
	if integer {
		sort = f.integer
	}
	x, y, z := f.constant("x", sort), f.constant("y", sort), f.constant("z", sort)
	b := f.constant("b", f.boolean)
	number := func(s string) ast.ASTHandle {
		if integer {
			return f.own(m.ArithmeticIntegerNumeralText(s))
		}
		return f.own(m.ArithmeticRealNumeralText(s))
	}
	xv, yv, zv := "2", "3", "1"
	if strings.HasPrefix(mode, "integer-equality") {
		yv = "4"
	}
	if mode == "integer-nonunit" {
		yv, zv = "7", "3"
	}
	if mode == "modulo" {
		xv, yv = "5", "8"
	}
	if mode == "negative-distinct" {
		yv = "2"
	}
	if mode == "opaque" {
		yv = "5"
	}
	for _, pair := range [][2]ast.ASTHandle{{x, number(xv)}, {y, number(yv)}, {z, number(zv)}} {
		predicateRegister(t, f, md, pair[0], pair[1])
	}
	boolValue := f.own(m.MkTrue())
	if mode == "ite-false" {
		boolValue = f.own(m.MkFalse())
	}
	predicateRegister(t, f, md, b, boolValue)
	cmp := func(op int32, l, r ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkFamilyApp(f.r.arithmetic, op, nil, []ast.ASTHandle{l, r}))
	}
	forms := []ast.ASTHandle{cmp(2, z, x), cmp(2, x, y)}
	switch mode {
	case "strict":
		forms = []ast.ASTHandle{cmp(4, z, x), cmp(4, x, y)}
	case "negated-ge":
		forms = []ast.ASTHandle{f.own(m.MkNot(cmp(3, z, x))), f.own(m.MkNot(cmp(3, x, y)))}
	case "disequality":
		forms = []ast.ASTHandle{f.own(m.MkNot(f.own(m.MkEq(x, y)))), cmp(2, z, x)}
	case "distinct":
		forms = []ast.ASTHandle{f.own(m.MkDistinct([]ast.ASTHandle{x, y, z}))}
	case "negative-distinct":
		forms = []ast.ASTHandle{f.own(m.MkNot(f.own(m.MkDistinct([]ast.ASTHandle{x, y, z})))), cmp(2, z, x)}
	case "integer-equality", "integer-equality-negative", "integer-equality-modulo":
		forms = []ast.ASTHandle{f.own(m.MkEq(f.own(m.MkArithmeticMul(number("2"), x)), y)), cmp(2, z, x)}
		if mode == "integer-equality-negative" {
			forms[0] = f.own(m.MkEq(f.own(m.MkArithmeticNeg(f.own(m.MkArithmeticMul(number("2"), x)))), f.own(m.MkArithmeticNeg(y))))
		}
		if mode == "integer-equality-modulo" {
			mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{x, number("3")}))
			forms = append(forms, f.own(m.MkEq(mod, number("2"))))
		}
	case "integer-nonunit":
		forms = []ast.ASTHandle{cmp(2, f.own(m.MkArithmeticMul(number("3"), x)), y), cmp(3, f.own(m.MkArithmeticMul(number("2"), x)), z)}
	case "modulo":
		mod := f.own(m.MkFamilyApp(f.r.arithmetic, 13, nil, []ast.ASTHandle{x, number("3")}))
		forms = append(forms, f.own(m.MkEq(mod, number("2"))))
	case "ite-true", "ite-false", "ite-negated-condition":
		left, right := y, z
		if mode == "ite-false" {
			left, right = z, y
		}
		condition := b
		if mode == "ite-negated-condition" {
			condition = f.own(m.MkNot(b))
			left, right = z, y
		}
		branch := f.own(m.MkIte(condition, left, right))
		forms = []ast.ASTHandle{cmp(2, z, x), cmp(2, x, branch)}
	case "opaque":
		forms = []ast.ASTHandle{cmp(2, f.own(m.MkArithmeticMul(x, x)), y)}
	case "boolean-residue":
		forms = append(forms, b)
	case "coefficient":
		coefficient := f.own(m.MkArithmeticAdd(number("1/3"), number("2/3")))
		forms = []ast.ASTHandle{cmp(2, z, x), cmp(2, f.own(m.MkArithmeticMul(coefficient, x)), y)}
	case "large-coefficient":
		coefficient := number("12345678901234567890123456789/7")
		forms = []ast.ASTHandle{cmp(2, z, x), cmp(2, f.own(m.MkArithmeticMul(coefficient, x)), f.own(m.MkArithmeticMul(coefficient, y)))}
	case "zero-coefficient":
		forms = []ast.ASTHandle{f.own(m.MkEq(f.own(m.MkArithmeticSub(x, x)), number("0")))}
	}
	return &arithmeticProjectionCase{f: f, md: md, vars: predicateRefs(t, f, x), forms: predicateRefs(t, f, forms...), original: forms, x: x, sort: sort}
}
func arithmeticEvaluate(t *testing.T, f *fmFixture, md *model.Model, forms []ast.ASTRef) {
	t.Helper()
	m := f.r.manager
	ev, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
	fmMust(t, err)
	defer func() { fmMust(t, ev.Close()) }()
	out, err := m.NewRef(ast.ASTHandle{})
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&out)) }()
	for _, r := range forms {
		h, err := m.RefGet(r)
		fmMust(t, err)
		fmMust(t, ev.Evaluate(h, out))
		v, err := m.RefGet(out)
		fmMust(t, err)
		if !m.IsTrue(v) {
			var text bytes.Buffer
			fmMust(t, writeExpression(m, f.r.config, &text, h))
			t.Fatal("model does not satisfy output", text.String())
		}
	}
}

var arithmeticProjectionModes = []string{"real-bounds", "strict", "negated-ge", "disequality", "distinct", "negative-distinct", "integer-equality", "integer-equality-negative", "integer-equality-modulo", "integer-nonunit", "modulo", "ite-true", "ite-false", "ite-negated-condition", "opaque", "boolean-residue", "coefficient", "large-coefficient", "zero-coefficient"}

func TestArithmeticProjectionModelPreservationAndVariables(t *testing.T) {
	for _, mode := range arithmeticProjectionModes {
		t.Run(mode, func(t *testing.T) {
			c := arithmeticCase(t, mode)
			var trace bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&trace)
			tags.Enable("qe")
			t.Cleanup(tags.Finalize)
			c.f.r.config.Trace = tags
			p := arithmeticProjectionOwner(t, c.f)
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
			if err := p.Project(c.md.Core(), c.vars, c.forms); err != nil {
				t.Fatalf("%v\n%s", err, trace.String())
			}
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
			want := 0
			if mode == "opaque" {
				want = 1
			}
			if len(*c.vars) != want {
				t.Fatal("remaining variables", len(*c.vars), want)
			}
			fmMust(t, p.Close())
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
		})
	}
}
func TestArithmeticProjectionSingletonAndNoopHooks(t *testing.T) {
	c := arithmeticCase(t, "real-bounds")
	p := arithmeticProjectionOwner(t, c.f)
	additional := predicateRefs(t, c.f, c.x)
	ok, err := p.ProjectVariable(c.md.Core(), c.x, additional, c.forms)
	fmMust(t, err)
	if !ok || len(*additional) != 1 {
		t.Fatal("singleton projection changed additional variables")
	}
	b := c.f.constant("extra-bool", c.f.boolean)
	vars := predicateRefs(t, c.f, b)
	before := len(*c.forms)
	ok, err = p.Solve(c.md.Core(), vars, c.forms)
	fmMust(t, err)
	if ok {
		t.Fatal("cheap solve must be source no-op")
	}
	fmMust(t, p.Project(c.md.Core(), vars, c.forms))
	if len(*vars) != 1 || len(*c.forms) != before {
		t.Fatal("nonarithmetic early return")
	}
	id, err := p.FamilyID()
	fmMust(t, err)
	if id != c.f.r.arithmetic {
		t.Fatal(id)
	}
	copy := *p
	if _, err = copy.FamilyID(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, p.Close())
	if err = p.Project(c.md.Core(), vars, c.forms); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}

func TestArithmeticProjectionSourceAndOwnerGuards(t *testing.T) {
	c := arithmeticCase(t, "real-bounds")
	p := arithmeticProjectionOwner(t, c.f)
	m := c.f.r.manager
	other := newFMFixture(t)
	foreign := predicateModel(t, other)
	if err := p.Project(foreign.Core(), c.vars, c.forms); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	bound := c.f.variable(0, c.f.real)
	vars := predicateRefs(t, c.f, bound)
	if err := p.Project(c.md.Core(), vars, c.forms); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	integer := c.f.constant("integer-objective", c.f.integer)
	out := predicateRefs(t, c.f, ast.ASTHandle{}, ast.ASTHandle{})
	if _, err := p.Maximize(c.md.Core(), *c.forms, integer, (*out)[0], (*out)[1]); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	// Source only checks its model-satisfied-literal precondition in debug
	// builds; release leaves unsupported Boolean literals in the residue.
	falsehood := c.f.own(m.MkFalse())
	forms := predicateRefs(t, c.f, falsehood)
	err := p.Project(c.md.Core(), c.vars, forms)
	if ast.ASTSourceDebug {
		if !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal(err)
		}
	} else {
		fmMust(t, err)
	}
}

func TestArithmeticProjectionMaximizeModelsAndBounds(t *testing.T) {
	for _, mode := range []string{"finite", "strict", "unbounded", "sum", "ite"} {
		t.Run(mode, func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			md := predicateModel(t, f)
			p := arithmeticProjectionOwner(t, f)
			x, y := f.constant("mx", f.real), f.constant("my", f.real)
			one := f.own(m.ArithmeticRealNumeralText("1"))
			two := f.own(m.ArithmeticRealNumeralText("2"))
			three := f.own(m.ArithmeticRealNumeralText("3"))
			predicateRegister(t, f, md, x, one)
			predicateRegister(t, f, md, y, one)
			objective := x
			forms := predicateRefs(t, f, f.own(m.MkArithmeticLE(x, three)))
			expected := "3"
			switch mode {
			case "strict":
				fmMust(t, m.SetRef((*forms)[0], f.own(m.MkFamilyApp(f.r.arithmetic, 4, nil, []ast.ASTHandle{x, three}))))
			case "unbounded":
				fmMust(t, m.SetRef((*forms)[0], f.own(m.MkArithmeticGE(x, one))))
			case "sum":
				objective = f.own(m.MkArithmeticAdd(x, y))
				forms = predicateRefs(t, f, f.own(m.MkArithmeticLE(x, two)), f.own(m.MkArithmeticLE(y, three)))
				expected = "5"
			case "ite":
				b := f.constant("choose", f.boolean)
				predicateRegister(t, f, md, b, f.own(m.MkTrue()))
				objective = f.own(m.MkIte(b, x, y))
				forms = predicateRefs(t, f, f.own(m.MkArithmeticLE(x, two)), f.own(m.MkArithmeticLE(y, three)))
				expected = "2"
			}
			bounds := predicateRefs(t, f, ast.ASTHandle{}, ast.ASTHandle{})
			value, err := p.Maximize(md.Core(), *forms, objective, (*bounds)[0], (*bounds)[1])
			fmMust(t, err)
			if mode == "unbounded" {
				if !value.Infinite || !m.IsFalse(predicateValue(t, f, (*bounds)[1])) {
					t.Fatal(value)
				}
			} else {
				if value.Infinite || value.Value.RatString() != expected {
					t.Fatal(value)
				}
				if (value.Epsilon.Sign() < 0) != (mode == "strict") {
					t.Fatal(value)
				}
			}
			arithmeticEvaluate(t, f, md, *forms)
			arithmeticEvaluate(t, f, md, (*bounds)[:1])
			fmMust(t, p.Close())
			arithmeticEvaluate(t, f, md, *forms)
		})
	}
}

func TestArithmeticProjectionAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "real-bounds", "integer-equality", "integer-equality-modulo", "integer-nonunit", "modulo", "ite-true", "ite-negated-condition", "disequality", "distinct", "negative-distinct", "large-coefficient", "opaque", "singleton", "maximize", "maximize-strict", "maximize-unbounded", "trace", "closed-trace"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 4096 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					mode := phase
					if phase == "constructor" || phase == "singleton" || strings.HasPrefix(phase, "maximize") || strings.Contains(phase, "trace") {
						mode = "real-bounds"
					}
					c := arithmeticCase(t, mode)
					m := c.f.r.manager
					if phase == "maximize" || phase == "maximize-strict" {
						three := c.f.own(m.ArithmeticRealNumeralText("3"))
						op := int32(2)
						if phase == "maximize-strict" {
							op = 4
						}
						c.forms = predicateRefs(t, c.f, c.f.own(m.MkFamilyApp(c.f.r.arithmetic, op, nil, []ast.ASTHandle{c.x, three})))
					}
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
					var p *ArithmeticProjection
					var err error
					if phase != "constructor" {
						p = arithmeticProjectionOwner(t, c.f)
					}
					bounds := predicateRefs(t, c.f, ast.ASTHandle{}, ast.ASTHandle{})
					extra := predicateRefs(t, c.f)
					m.Budget().FailAfter(stop)
					switch phase {
					case "constructor":
						p, err = NewArithmeticProjection(m, c.f.r.config)
					case "singleton":
						_, err = p.ProjectVariable(c.md.Core(), c.x, extra, c.forms)
					case "maximize", "maximize-strict", "maximize-unbounded":
						_, err = p.Maximize(c.md.Core(), *c.forms, c.x, (*bounds)[0], (*bounds)[1])
					default:
						err = p.Project(c.md.Core(), c.vars, c.forms)
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
					t.Fatal("arithmetic projection allocation frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}
