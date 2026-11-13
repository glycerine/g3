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
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var qsatPreparationModes = []string{"ground", "exists", "forall", "alternating", "dependent", "dependent-reverse", "integer-dependent", "real-universal", "mixed", "negative", "ite", "label", "blocked", "multiple"}

type qsatPreparationFixture struct {
	f              *fmFixture
	formula        ast.ASTRef
	original, free ast.ASTHandle
	mode           string
}

func qsatPrepareFixture(t *testing.T, mode string) *qsatPreparationFixture {
	t.Helper()
	f := newFMFixture(t)
	m := f.r.manager
	free := f.constant("free", f.boolean)
	v0, v1, v2 := f.variable(0, f.boolean), f.variable(1, f.boolean), f.variable(2, f.boolean)
	sym := func(s string) usymbol.Symbol { n, e := m.Symbols.Intern(s); fmMust(t, e); return n }
	q := func(forall bool, name string, body ast.ASTHandle) ast.ASTHandle {
		return f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: []ast.ASTHandle{f.boolean}, Names: []usymbol.Symbol{sym(name)}, Body: body}))
	}
	or := func(args ...ast.ASTHandle) ast.ASTHandle { return f.own(m.MkOrArray(args)) }
	formula := free
	switch mode {
	case "exists", "label":
		formula = q(false, "x", or(v0, free))
		if mode == "label" {
			formula = f.own(m.MkLabel(true, []usymbol.Symbol{sym("source-label")}, formula))
		}
	case "forall":
		formula = q(true, "x", or(v0, free))
	case "alternating":
		formula = q(false, "x", q(true, "y", q(false, "z", or(v2, f.own(m.MkNot(v1)), v0, free))))
	case "dependent":
		formula = q(false, "x", q(true, "y", or(f.own(m.MkIff(v1, v0)), free)))
	case "dependent-reverse":
		formula = q(true, "x", q(false, "y", f.own(m.MkAndArray([]ast.ASTHandle{f.own(m.MkIff(v1, v0)), free}))))
	case "integer-dependent":
		a, b := f.variable(1, f.integer), f.variable(0, f.integer)
		body := or(f.own(m.MkArithmeticGE(a, b)), free)
		inner := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{sym("integerY")}, Body: body}))
		formula = f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{sym("integerX")}, Body: inner}))
	case "real-universal":
		a := f.variable(0, f.real)
		zero := f.own(m.ArithmeticRealNumeralText("0"))
		formula = f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.real}, Names: []usymbol.Symbol{sym("realX")}, Body: or(f.own(m.MkArithmeticLE(a, zero)), free)}))
	case "mixed":
		formula = f.own(m.MkAndArray([]ast.ASTHandle{q(true, "x", or(v0, free)), q(false, "y", or(v0, free))}))
	case "negative":
		formula = f.own(m.MkNot(q(false, "x", f.own(m.MkAndArray([]ast.ASTHandle{v0, free})))))
	case "ite":
		formula = f.own(m.MkIte(free, q(true, "x", v0), q(false, "y", v0)))
	case "blocked":
		decl := f.own(m.MkFuncDecl(sym("opaque"), []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{}))
		formula = or(f.own(m.MkApp(decl, []ast.ASTHandle{q(true, "x", v0)})), free)
	case "multiple":
		formula = f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{f.boolean, f.boolean}, Names: []usymbol.Symbol{sym("same"), sym("same")}, Body: or(v0, v1, free)}))
	}
	refs := predicateRefs(t, f, formula)
	return &qsatPreparationFixture{f: f, formula: (*refs)[0], original: formula, free: free, mode: mode}
}
func qsatPrepareOwner(t *testing.T, f *fmFixture) *qsatPreparation {
	t.Helper()
	p, err := newQSATPreparation(f.r.manager, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, p.Close()) })
	return p
}
func qsatExpectedBlocks(mode string, sat bool) []int {
	if sat {
		switch mode {
		case "ground", "blocked":
			return []int{1, 0}
		case "exists", "label":
			return []int{2, 0}
		case "forall", "negative", "real-universal":
			return []int{1, 1, 0}
		case "alternating":
			return []int{2, 1, 1, 0}
		case "dependent", "integer-dependent":
			return []int{2, 1, 0}
		case "dependent-reverse":
			return []int{1, 1, 1, 0}
		case "mixed", "ite":
			return []int{2, 1, 0}
		case "multiple":
			return []int{3, 0}
		}
	}
	switch mode {
	case "ground", "blocked":
		return []int{1, 0, 0}
	case "exists", "label":
		return []int{1, 0, 1, 0}
	case "forall", "negative", "real-universal":
		return []int{1, 1, 0}
	case "alternating":
		return []int{1, 0, 1, 1, 1, 0}
	case "dependent", "integer-dependent":
		return []int{1, 0, 1, 1, 0}
	case "dependent-reverse":
		return []int{1, 1, 1, 0}
	case "mixed", "ite":
		return []int{1, 1, 1, 0}
	case "multiple":
		return []int{1, 0, 2, 0}
	}
	return nil
}
func TestQSATPreparationPrefixLevelsAndFilters(t *testing.T) {
	for _, mode := range qsatPreparationModes {
		for _, kind := range []qsatMode{qsatSAT, qsatQE, qsatRecursiveQE, qsatMaximize} {
			t.Run(fmt.Sprintf("%s/%d", mode, kind), func(t *testing.T) {
				c := qsatPrepareFixture(t, mode)
				p := qsatPrepareOwner(t, c.f)
				m := c.f.r.manager
				fmMust(t, p.hoist(kind, c.formula))
				want := qsatExpectedBlocks(mode, kind == qsatSAT)
				if len(p.variables) != len(want) {
					t.Fatal("prefix blocks", len(p.variables), want)
				}
				seen := make(map[ast.ASTHandle]bool)
				var filtered bytes.Buffer
				filter, err := p.abstraction.FilterModelConverter()
				fmMust(t, err)
				fmMust(t, filter.Display(&filtered))
				for i, block := range p.variables {
					if len(block) != want[i] {
						t.Fatal("block", i, len(block), want)
					}
					for _, r := range block {
						h := predicateValue(t, c.f, r)
						if seen[h] {
							t.Fatal("fresh variable identity reused")
						}
						seen[h] = true
						level, err := p.abstraction.ComputeLevel(h)
						fmMust(t, err)
						expected := EmptyMaxLevel()
						if i%2 == 0 {
							expected.Existential = uint32(i)
						} else {
							expected.Universal = uint32(i)
						}
						if level != expected {
							t.Fatal("variable level", level, expected)
						}
						var name bytes.Buffer
						fmMust(t, writeExpression(m, c.f.r.config, &name, h))
						if h != c.free && !strings.Contains(filtered.String(), name.String()) {
							t.Fatal("fresh declaration missing from model filter", name.String(), filtered.String())
						}
					}
				}
				if predicateValue(t, c.f, p.variables[0][0]) != c.free {
					t.Fatal("initial free variable was reordered")
				}
				if strings.Contains(filtered.String(), "free") {
					t.Fatal("original free parameter was filtered", filtered.String())
				}
				for level := uint32(0); level < uint32(len(p.variables))+2; level++ {
					out := predicateRefs(t, c.f, c.free)
					fmMust(t, p.getVariables(level, out))
					var want []ast.ASTHandle
					for i := level; i < uint32(len(p.variables)); i++ {
						for _, r := range p.variables[i] {
							want = append(want, predicateValue(t, c.f, r))
						}
					}
					projectionExpect(t, c.f, *out, want)
				}
			})
		}
	}
}
func TestQSATPreparationResetTraceAndGuards(t *testing.T) {
	c := qsatPrepareFixture(t, "alternating")
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("qe")
	t.Cleanup(tags.Finalize)
	c.f.r.config.Trace = tags
	p := qsatPrepareOwner(t, c.f)
	fmMust(t, p.hoist(qsatSAT, c.formula))
	if tags.ShouldTrace("qe") && !strings.Contains(trace.String(), "hoist src/qe/qsat.cpp:750") {
		t.Fatal(trace.String())
	}
	fmMust(t, p.reset())
	if len(p.variables) != 0 {
		t.Fatal("reset retained prefixes")
	}
	fmMust(t, c.f.r.manager.SetRef(c.formula, c.original))
	fmMust(t, p.hoist(qsatQE, c.formula))
	if len(p.variables) != 6 {
		t.Fatal("reuse prefix", len(p.variables))
	}
	copy := *p
	if err := copy.hoist(qsatSAT, c.formula); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	if err := p.hoist(qsatMode(255), c.formula); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	other := newFMFixture(t)
	foreign := predicateRefs(t, other, other.own(other.r.manager.MkTrue()))
	if err := p.hoist(qsatSAT, (*foreign)[0]); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal(err)
	}
	fmMust(t, p.Close())
	if err := p.reset(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
func TestQSATPreparationAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "alternating", "qe", "mixed", "label", "blocked", "trace", "closed-trace", "reuse"} {
		t.Run(phase, func(t *testing.T) {
			success := false
			for stop := int64(0); stop < 8192 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					mode := phase
					if mode == "constructor" || mode == "qe" || mode == "reuse" || strings.Contains(mode, "trace") {
						mode = "alternating"
					}
					c := qsatPrepareFixture(t, mode)
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
					var p *qsatPreparation
					var err error
					if phase != "constructor" {
						p = qsatPrepareOwner(t, c.f)
					}
					if phase == "reuse" {
						fmMust(t, p.hoist(qsatSAT, c.formula))
						fmMust(t, p.reset())
						fmMust(t, m.SetRef(c.formula, c.original))
					}
					m.Budget().FailAfter(stop)
					if phase == "constructor" {
						p, err = newQSATPreparation(m, c.f.r.config)
					} else {
						kind := qsatSAT
						if phase == "qe" {
							kind = qsatQE
						}
						err = p.hoist(kind, c.formula)
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
					t.Fatal("QSAT preparation frontier", stop)
				}
			}
			if !success {
				t.Fatal("no successful frontier")
			}
		})
	}
}
