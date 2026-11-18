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

func modelProjectionOwner(t *testing.T, f *fmFixture) *ModelBasedProjection {
	t.Helper()
	p, err := NewModelBasedProjection(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, p.Close()) })
	return p
}
func projectionTargetAbsent(t *testing.T, f *fmFixture, forms []ast.ASTRef, target ast.ASTHandle) {
	t.Helper()
	pred := predicateOwner(t, f)
	for _, r := range forms {
		vars := predicateRefs(t, f)
		fmMust(t, pred.GetFreeVariables(predicateValue(t, f, r), vars))
		for _, v := range *vars {
			if predicateValue(t, f, v) == target {
				t.Fatal("projected target remains")
			}
		}
	}
}
func TestModelBasedProjectionConcreteDispatch(t *testing.T) {
	for _, mode := range []string{"real-bounds", "integer-equality", "integer-equality-modulo", "integer-nonunit", "modulo", "ite-negated-condition", "opaque", "negative-distinct"} {
		t.Run(mode, func(t *testing.T) {
			c := arithmeticCase(t, mode)
			p := modelProjectionOwner(t, c.f)
			fmMust(t, p.Project(true, c.vars, c.md.Core(), c.forms))
			if len(*c.vars) != 0 {
				t.Fatal("arithmetic target retained")
			}
			projectionTargetAbsent(t, c.f, *c.forms, c.x)
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
			fmMust(t, p.Close())
			arithmeticEvaluate(t, c.f, c.md, *c.forms)
		})
	}
	for _, mode := range arrayProjectionModes {
		for _, force := range []bool{false, true} {
			t.Run(fmt.Sprintf("array/%s/%t", mode, force), func(t *testing.T) {
				c := arrayCase(t, mode)
				p := modelProjectionOwner(t, c.f)
				vars := predicateRefs(t, c.f, c.target)
				fmMust(t, p.Project(force, vars, c.md.Core(), c.lits))
				arithmeticEvaluate(t, c.f, c.md, *c.lits)
				if c.success || force {
					wantRemaining := 0
					if !force && (mode == "select-bool" || strings.HasPrefix(mode, "store-")) {
						wantRemaining = c.fresh
					}
					if len(*vars) != wantRemaining {
						t.Fatal("source fresh-variable residue", len(*vars), wantRemaining)
					}
					projectionTargetAbsent(t, c.f, *c.lits, c.target)
				} else {
					projectionExpect(t, c.f, *vars, []ast.ASTHandle{c.target})
					projectionExpect(t, c.f, *c.lits, c.original)
				}
			})
		}
	}
	for _, mode := range datatypeProjectionModes {
		t.Run("datatype/"+mode, func(t *testing.T) {
			c := datatypeCase(t, mode)
			p := modelProjectionOwner(t, c.d.f)
			vars := predicateRefs(t, c.d.f, c.target)
			fmMust(t, p.Project(true, vars, c.d.md.Core(), c.forms))
			if len(*vars) != 0 {
				t.Fatal("datatype variables retained")
			}
			projectionTargetAbsent(t, c.d.f, *c.forms, c.target)
			arithmeticEvaluate(t, c.d.f, c.d.md, *c.forms)
		})
	}
}

func TestModelBasedProjectionSolveFixedPointAndLiteralForwarding(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f := d.f
	m := f.r.manager
	p := modelProjectionOwner(t, f)
	x := f.constant("fx", f.integer)
	b := f.constant("fb", f.boolean)
	r := f.constant("fr", d.pair)
	three := f.number("3")
	truth := f.own(m.MkTrue())
	predicateRegister(t, f, d.md, x, three)
	predicateRegister(t, f, d.md, b, truth)
	predicateRegister(t, f, d.md, r, d.app(d.pairCtor, three, truth))
	vars := predicateRefs(t, f, x)
	forms := predicateRefs(t, f, f.own(m.MkEq(d.app(d.pairCtor, x, b), r)))
	fmMust(t, p.Solve(d.md.Core(), vars, forms))
	if len(*vars) != 0 {
		t.Fatal("datatype lifting did not feed a second equality-solving round")
	}
	projectionTargetAbsent(t, f, *forms, x)
	arithmeticEvaluate(t, f, d.md, *forms)
	for _, mode := range projectionLiteralModes {
		t.Run(mode, func(t *testing.T) {
			c := projectionLiteralFixture(t, mode)
			owner := modelProjectionOwner(t, c.f)
			fmMust(t, owner.ExtractLiterals(c.md.Core(), c.forms))
			projectionExpect(t, c.f, *c.forms, c.want)
		})
	}
}

type forcedProjectionCase struct {
	f           *fmFixture
	md          *model.Model
	vars, forms *[]ast.ASTRef
	x, y        ast.ASTHandle
}

func forcedCase(t *testing.T) *forcedProjectionCase {
	f := newFMFixture(t)
	m := f.r.manager
	md := predicateModel(t, f)
	n, err := m.Symbols.Intern("U")
	fmMust(t, err)
	s := f.own(m.MkUninterpretedSort(n, nil))
	x, y := f.constant("ux", s), f.constant("uy", s)
	predicateRegister(t, f, md, x, f.own(m.MkModelValue(0, s)))
	predicateRegister(t, f, md, y, f.own(m.MkModelValue(1, s)))
	return &forcedProjectionCase{f: f, md: md, vars: predicateRefs(t, f, x, y), forms: predicateRefs(t, f, f.own(m.MkNot(f.own(m.MkEq(x, y))))), x: x, y: y}
}
func TestModelBasedProjectionForceOrderReuseAndGuards(t *testing.T) {
	c := forcedCase(t)
	p := modelProjectionOwner(t, c.f)
	fmMust(t, p.Project(false, c.vars, c.md.Core(), c.forms))
	projectionExpect(t, c.f, *c.vars, []ast.ASTHandle{c.y, c.x}) // pop order, then append failed targets
	fmMust(t, p.Project(true, c.vars, c.md.Core(), c.forms))
	if len(*c.vars) != 0 || len(*c.forms) != 0 {
		t.Fatal("forced substitutions did not clear true residue")
	}
	copy := *p
	if err := copy.Solve(c.md.Core(), c.vars, c.forms); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	other := newFMFixture(t)
	foreign := predicateModel(t, other)
	if err := p.Project(false, c.vars, foreign.Core(), c.forms); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err := p.Solve(c.md.Core(), nil, c.forms); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, p.Close())
	if err := p.ExtractLiterals(c.md.Core(), c.forms); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}

func TestModelBasedProjectionMaximizeForwarding(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(fmt.Sprint(strict), func(t *testing.T) {
			c := arithmeticCase(t, "real-bounds")
			f := c.f
			m := f.r.manager
			p := modelProjectionOwner(t, f)
			three := f.own(m.ArithmeticRealNumeralText("3"))
			op := int32(2)
			if strict {
				op = 4
			}
			forms := predicateRefs(t, f, f.own(m.MkFamilyApp(f.r.arithmetic, op, nil, []ast.ASTHandle{c.x, three})))
			bounds := predicateRefs(t, f, ast.ASTHandle{}, ast.ASTHandle{})
			v, err := p.Maximize(c.md.Core(), *forms, c.x, (*bounds)[0], (*bounds)[1])
			fmMust(t, err)
			if v.Infinite || optimumText(m, &v.Value) != "3" || (optimumSign(m, &v.Epsilon) < 0) != strict {
				t.Fatal(v)
			}
			arithmeticEvaluate(t, f, c.md, *forms)
			arithmeticEvaluate(t, f, c.md, (*bounds)[:1])
		})
	}
}

func TestModelBasedProjectionMixedFamiliesAndEmptyInputs(t *testing.T) {
	d := newDatatypeProjectionFixture(t)
	f := d.f
	m := f.r.manager
	md := d.md
	p := modelProjectionOwner(t, f)
	pair := f.constant("mixedPair", d.pair)
	y := f.constant("mixedY", f.integer)
	b := f.constant("mixedBool", f.boolean)
	i := f.constant("mixedIndex", f.integer)
	unused := f.constant("unused", f.integer)
	s := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("mixedArray", s)
	truth := f.own(m.MkTrue())
	one, two, three := f.number("1"), f.number("2"), f.number("3")
	predicateRegister(t, f, md, pair, d.app(d.pairCtor, three, truth))
	predicateRegister(t, f, md, y, two)
	predicateRegister(t, f, md, b, truth)
	predicateRegister(t, f, md, i, two)
	predicateRegister(t, f, md, a, f.own(m.MkConstArray(s, one)))
	fid, err := m.ArrayFamilyID()
	fmMust(t, err)
	read := f.own(m.MkArrayAppArguments(fid, ast.ASTArrayOpSelect, ast.SliceArguments([]ast.ASTHandle{a, i})))
	forms := predicateRefs(t, f, b, f.own(m.MkArithmeticLE(read, y)), f.own(m.MkArithmeticLE(y, d.app(d.pairFields[0], pair))), d.app(d.pairFields[1], pair))
	targets := []ast.ASTHandle{unused, y, b, pair, a}
	vars := predicateRefs(t, f, targets...)
	arithmeticEvaluate(t, f, md, *forms)
	fmMust(t, p.Project(true, vars, md.Core(), forms))
	if len(*vars) != 0 {
		t.Fatal("mixed variables retained")
	}
	for _, h := range targets {
		projectionTargetAbsent(t, f, *forms, h)
	}
	arithmeticEvaluate(t, f, md, *forms)
	empty := predicateRefs(t, f)
	again := predicateRefs(t, f, unused)
	fmMust(t, p.Project(false, again, md.Core(), empty))
	if len(*again) != 0 {
		t.Fatal("empty formula retained variables")
	}
	if ast.ASTSourceDebug {
		bad := predicateRefs(t, f, f.own(m.MkFalse()))
		if err = p.Project(true, again, md.Core(), bad); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("unsatisfied source-debug model", err)
		}
	}
}

func TestModelBasedProjectionAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "arithmetic", "array", "array-store", "datatype", "solve", "extract", "force", "maximize", "trace", "closed-trace"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 8192 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					var f *fmFixture
					var md *model.Model
					var vars, forms *[]ast.ASTRef
					var objective ast.ASTHandle
					switch phase {
					case "array", "array-store":
						mode := "select-distinct"
						if phase == "array-store" {
							mode = "store-distinct"
						}
						c := arrayCase(t, mode)
						f, md, forms = c.f, c.md, c.lits
						vars = predicateRefs(t, f, c.target)
					case "datatype":
						c := datatypeCase(t, "recursive-fallback")
						f, md, forms = c.d.f, c.d.md, c.forms
						vars = predicateRefs(t, f, c.target)
					case "force":
						c := forcedCase(t)
						f, md, vars, forms = c.f, c.md, c.vars, c.forms
					default:
						c := arithmeticCase(t, "real-bounds")
						f, md, vars, forms, objective = c.f, c.md, c.vars, c.forms, c.x
					}
					var out bytes.Buffer
					if strings.Contains(phase, "trace") {
						var w io.Writer = &out
						if phase == "closed-trace" {
							w = closedPredicateTrace{}
						}
						tags := diagnostic.NewTraceTagsWithWriter(w)
						tags.Enable("qe")
						t.Cleanup(tags.Finalize)
						f.r.config.Trace = tags
					}
					var p *ModelBasedProjection
					var err error
					if phase != "constructor" {
						p = modelProjectionOwner(t, f)
					}
					bounds := predicateRefs(t, f, ast.ASTHandle{}, ast.ASTHandle{})
					f.r.manager.Budget().FailAfter(stop)
					switch phase {
					case "constructor":
						p, err = NewModelBasedProjection(f.r.manager, f.r.config)
					case "solve":
						err = p.Solve(md.Core(), vars, forms)
					case "extract":
						err = p.ExtractLiterals(md.Core(), forms)
					case "maximize":
						_, err = p.Maximize(md.Core(), *forms, objective, (*bounds)[0], (*bounds)[1])
					default:
						err = p.Project(true, vars, md.Core(), forms)
					}
					f.r.manager.Budget().FailAfter(-1)
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
							t.Fatalf("%v\n%s", err, out.String())
						}
					}
				})
				if t.Failed() {
					t.Fatal("MBP allocation frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}

func TestProjectionEqualityStoppedRewritePreservesCallerSlot(t *testing.T) {
	success := false
	for stop := int64(0); stop < 2048 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p := projectionPreprocessOwner(t, f)
			x, y, z := f.constant("stopX", f.integer), f.constant("stopY", f.integer), f.constant("stopZ", f.integer)
			zero := f.number("0")
			original := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(x, zero)), z))
			intermediate := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(y, zero)), z))
			final := f.own(m.MkArithmeticLE(y, z))
			if original == intermediate || intermediate == final {
				t.Fatal("fixture must distinguish substitution from rewriting")
			}
			forms := predicateRefs(t, f, f.own(m.MkEq(x, y)), original)
			vars := predicateRefs(t, f, x)
			m.Budget().FailAfter(stop)
			_, err := p.solveEqualities(vars, forms)
			m.Budget().FailAfter(-1)
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				fmMust(t, failure.Cleanup())
			}
			for _, r := range *forms {
				if predicateValue(t, f, r) == intermediate {
					t.Fatal("substitution published before theory rewriting completed")
				}
			}
			if err == nil {
				success = true
				projectionExpect(t, f, *forms, []ast.ASTHandle{final})
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				var memout memory.Failure
				if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
					t.Fatal(err)
				}
			}
		})
		if t.Failed() {
			t.Fatal("stopped rewrite frontier", stop)
		}
	}
	if !success {
		t.Fatal("no successful frontier")
	}
}
