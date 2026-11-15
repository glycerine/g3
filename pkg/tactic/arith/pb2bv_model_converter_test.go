package arith

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func pbModelFixture(t *testing.T) (*LIA2PBTactic, []PB2BVBinding, *BoundManager) {
	t.Helper()
	r := lia2pbFixture(t)
	m := r.manager
	boolean, err := m.MkBoolSort()
	equationMust(t, err)
	var bindings []PB2BVBinding
	for i := 0; i < 4; i++ {
		x := fixDLConst(t, m, true, fmt.Sprintf("x%d", i))
		decl, e := m.GetDecl(x)
		equationMust(t, e)
		sym, e := m.Symbols.Intern(fmt.Sprintf("b%d", i))
		equationMust(t, e)
		b, e := m.MkConst(sym, boolean)
		equationMust(t, e)
		fixDLPin(t, m, b)
		bd, e := m.GetDecl(b)
		equationMust(t, e)
		bindings = append(bindings, PB2BVBinding{decl, bd})
	}
	dontCare := fixDLConst(t, m, true, "dontcare")
	goal := fixDLGoal(t, m, false, false, false, diffNeqTestBounds(t, m, dontCare, "1")...)
	equationMust(t, r.bounds.CollectGoal(goal))
	return r, bindings, r.bounds
}

func TestPB2BVModelConverterValuesDisplayAndTranslation(t *testing.T) {
	r, bindings, bounds := pbModelFixture(t)
	m := r.manager
	converter, err := NewPB2BVModelConverter(m, r.context, r.globals, bindings, bounds)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, converter.Close()) })
	var display bytes.Buffer
	equationMust(t, converter.Display(&display))
	want := "(pb2bv-model-converter\n  (x0 b0)\n  (x1 b1)\n  (x2 b2)\n  (x3 b3)\n  (dontcare 0))\n"
	if display.String() != want {
		t.Fatal("source converter display", display.String(), want)
	}
	md, err := model.NewModel(m)
	equationMust(t, err)
	var out tactic.ModelRef
	equationMust(t, out.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, out.Close()) })
	truth, err := m.MkTrue()
	equationMust(t, err)
	falsity, err := m.MkFalse()
	equationMust(t, err)
	equationMust(t, md.CoreField.RegisterConstant(bindings[0].Boolean, truth))
	equationMust(t, md.CoreField.RegisterConstant(bindings[1].Boolean, falsity))
	// The source treats every present nonfalse value as 1, without checking
	// that it is literally true. A nonconstant Boolean term exercises that branch.
	name, err := m.Symbols.Intern("uninterpreted-boolean-value")
	equationMust(t, err)
	bs, err := m.MkBoolSort()
	equationMust(t, err)
	other, err := m.MkConst(name, bs)
	equationMust(t, err)
	fixDLPin(t, m, other)
	equationMust(t, md.CoreField.RegisterConstant(bindings[3].Boolean, other))
	var trace, before bytes.Buffer
	equationMust(t, md.CoreField.WriteV2(&before, r.context, r.globals, false))
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("pb2bv")
	converter.trace = tags
	equationMust(t, converter.ApplyUnindexed(&out))
	if tags.ShouldTrace("pb2bv") {
		expected := "-------- [pb2bv] operator() src/tactic/arith/pb2bv_model_converter.cpp:66 ---------\nconverting model:\n" + before.String() + want + "------------------------------------------------\n"
		if trace.String() != expected {
			t.Fatal("source model conversion trace", trace.String(), expected)
		}
	} else if trace.Len() != 0 {
		t.Fatal("erased model trace emitted output")
	}
	refused := diagnostic.NewTraceTagsWithWriter(fixDLRefusedWriter{})
	refused.Enable("pb2bv")
	converter.trace = refused
	equationMust(t, converter.ApplyModel(&out, 0))
	for i, pair := range converter.bindings {
		value, e := md.CoreField.ConstantInterpretation(pair.Integer)
		equationMust(t, e)
		text, isInt, found, e := m.ArithmeticNumeralText(value)
		equationMust(t, e)
		want := "0"
		if i == 0 || i == 3 {
			want = "1"
		}
		if !isInt || !found || text != want {
			t.Fatal("Boolean to integer model", i, text, want)
		}
	}
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, target.Close()) })
	translator, err := ast.NewASTTranslation(m, target, true)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, translator.Close()) })
	translated, err := converter.Translate(translator)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, translated.Close()) })
	var copied bytes.Buffer
	equationMust(t, translated.Display(&copied))
	if copied.String() != want {
		t.Fatal("translated pairs and null don't-care", copied.String())
	}
	copy := *converter
	if err = copy.IncRef(); !errors.Is(err, tactic.ErrModelConverterOwner) {
		t.Fatal("copied owner", err)
	}
	if ast.ASTSourceDebug {
		if err = converter.ApplyModel(&out, 1); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("source goal index assertion", err)
		}
	}
}

func TestPB2BVModelConverterAllocationFrontiers(t *testing.T) {
	for _, phase := range []string{"constructor", "apply", "translate"} {
		success := false
		for stop := int64(0); stop < 256; stop++ {
			t.Run(fmt.Sprint(phase, "/", stop), func(t *testing.T) {
				r, bindings, bounds := pbModelFixture(t)
				m := r.manager
				if phase == "constructor" {
					m.Budget().FailAfter(stop)
					fresh, err := NewPB2BVModelConverter(m, r.context, r.globals, bindings, bounds)
					m.Budget().FailAfter(-1)
					if fresh != nil {
						equationMust(t, fresh.Close())
					}
					if err == nil {
						success = true
					} else if !errors.Is(err, arena.ErrInjectedFailure) {
						t.Fatal(err)
					}
					return
				}
				converter, err := NewPB2BVModelConverter(m, r.context, r.globals, bindings, bounds)
				equationMust(t, err)
				t.Cleanup(func() { equationMust(t, converter.Close()) })
				if phase == "translate" {
					target, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
					equationMust(t, e)
					t.Cleanup(func() { equationMust(t, target.Close()) })
					tr, e := ast.NewASTTranslation(m, target, true)
					equationMust(t, e)
					t.Cleanup(func() { equationMust(t, tr.Close()) })
					m.Budget().FailAfter(stop)
					fresh, e := converter.Translate(tr)
					m.Budget().FailAfter(-1)
					if fresh != nil {
						equationMust(t, fresh.Close())
					}
					err = e
				} else {
					md, e := model.NewModel(m)
					equationMust(t, e)
					var out tactic.ModelRef
					equationMust(t, out.Set(&md.CoreField))
					t.Cleanup(func() { equationMust(t, out.Close()) })
					m.Budget().FailAfter(stop)
					err = converter.ApplyModel(&out, 0)
					m.Budget().FailAfter(-1)
				}
				if err == nil {
					success = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
			})
			if success {
				break
			}
		}
		if !success {
			t.Fatal("no successful reservation frontier", phase)
		}
	}
}

// AUDIT.md tactic_arith_03-F4: the filter and pb2bv_model_converter both
// iterate m_const2bit (obj_map<func_decl, expr*>) in table order.
func TestPB2BVModelConverterObjMapOrder(t *testing.T) {
	r := pbTacticFixture(t)
	m := r.manager
	var forms, terms []ast.ASTHandle
	names := map[ast.ASTHandle]string{}
	for i := 0; i < 12; i++ {
		x := fixDLConst(t, m, true, fmt.Sprintf("x%d", i))
		d, err := m.GetDecl(x)
		equationMust(t, err)
		names[d] = fmt.Sprintf("x%d", i)
		forms = append(forms, diffNeqTestBounds(t, m, x, "1")...)
		terms = append(terms, fixDLApp(t, m, 9, fixDLNumber(t, m, true, fmt.Sprint(i+1)), x))
	}
	forms = append(forms, fixDLApp(t, m, 3, fixDLApp(t, m, 6, terms...), fixDLNumber(t, m, true, "20")))
	g := fixDLGoal(t, m, false, true, false, forms...)
	var out tactic.TacticResult
	t.Cleanup(func() { equationMust(t, out.Close()) })
	equationMust(t, r.Apply(g, &out))
	if len(r.entries) < 9 {
		t.Fatal("entries", len(r.entries))
	}
	decls := make([]ast.ASTHandle, len(r.entries))
	for i, entry := range r.entries {
		decls[i] = entry.declaration
	}
	order, err := pbObjMapOrder(m, decls)
	equationMust(t, err)
	var display bytes.Buffer
	equationMust(t, out.ModelConverter.Display(&display))
	text := display.String()
	last, inOrder := -1, true
	for k, i := range order {
		s := names[decls[i]]
		at := bytes.Index([]byte(text), []byte("("+s+" "))
		if at < 0 || at < last {
			t.Fatalf("binding %d (%s) out of obj_map order:\n%s", k, s, text)
		}
		last = at
		inOrder = inOrder && i == k
	}
	if inOrder {
		t.Fatal("fixture does not distinguish obj_map order from creation order")
	}
}
