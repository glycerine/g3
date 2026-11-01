package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerExtendedPrettySourceRewritesAndTrace(t *testing.T) {
	base, globals := newSpacerTestServices(t)
	if err := globals.SourceSetName("pp.max_depth", "1"); err != nil {
		t.Fatal(err)
	}
	if err := globals.SourceSetName("pp.min_alias_size", "1"); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("spacer.expand-add")
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: base.Symbols, Numerals: base.NumeralService(), Budget: base.Budget(), TraceTags: tags, Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var pins []ast.ASTRef
	defer func() {
		for i := len(pins) - 1; i >= 0; i-- {
			must(m.DelRef(&pins[i]))
		}
	}()
	pin := func(h ast.ASTHandle) { ref, err := m.NewRef(h); must(err); pins = append(pins, ref) }
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	minus, err := m.ArithmeticIntegerNumeralText("-1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	sort, err := m.GetSort(zero)
	must(err)
	constant := func(name string) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		must(err)
		h, err := m.MkConst(symbol, sort)
		must(err)
		return h
	}
	x, y := constant("pretty-x"), constant("pretty-y")
	negative, err := m.MkArithmeticMul(minus, y)
	must(err)
	lhs, err := m.MkArithmeticAdd(x, negative)
	must(err)
	family, err := m.ArithmeticFamilyID()
	must(err)
	for _, op := range []int32{2, 3, 4, 5} {
		for _, rhs := range []ast.ASTHandle{zero, three} {
			input, err := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{lhs, rhs})
			must(err)
			pin(input)
			bound := y
			if rhs == three {
				bound, err = m.MkArithmeticAdd(y, three)
				must(err)
			}
			want, err := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{x, bound})
			must(err)
			pin(want)
			ref, err := rewriteExtendedPretty(m, input)
			must(err)
			got, err := m.RefGet(ref)
			must(err)
			if got != want {
				t.Fatal("ordering rewrite", op, rhs, got, want)
			}
			must(m.DelRef(&ref))
			negated, err := m.MkNot(input)
			must(err)
			pin(negated)
			inverse := map[int32]int32{2: 5, 3: 4, 4: 3, 5: 2}[op]
			want, err = m.MkFamilyApp(family, inverse, nil, []ast.ASTHandle{x, bound})
			must(err)
			ref, err = rewriteExtendedPretty(m, negated)
			must(err)
			got, err = m.RefGet(ref)
			must(err)
			if got != want {
				t.Fatal("negated ordering rewrite", op, got, want)
			}
			must(m.DelRef(&ref))
		}
	}
	input, err := m.MkEq(lhs, zero)
	must(err)
	pin(input)
	want, err := m.MkEq(x, y)
	must(err)
	ref, err := rewriteExtendedPretty(m, input)
	must(err)
	got, err := m.RefGet(ref)
	must(err)
	if got != want {
		t.Fatal("equality rewrite", got, want)
	}
	must(m.DelRef(&ref))
	p := &PredicateTransformer{manager: m, name: "pretty-predicate"}
	functionName, err := m.Symbols.Intern("pretty-deep-f")
	must(err)
	function, err := m.MkFuncDecl(functionName, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	must(err)
	deep := x
	for i := 0; i < 12; i++ {
		deep, err = m.MkApp(function, []ast.ASTHandle{deep})
		must(err)
	}
	shared, err := m.MkArithmeticAdd(deep, deep)
	must(err)
	pin(shared)
	var complete bytes.Buffer
	must(p.writeExtendedPretty(&complete, shared))
	if strings.Contains(complete.String(), "(let") || strings.Count(complete.String(), "pretty-deep-f") != 24 {
		t.Fatal("extended printer inherited alias/depth limits", complete.String())
	}
	must(p.traceExtendedLemma(input, InfinityLevel))
	post, err := m.NewRef(input)
	must(err)
	must(p.traceExtendedNode(&ProofObligation{post: post, level: InfinityLevel, depth: 7}, 3))
	must(m.DelRef(&post))
	if tags.ShouldTrace("spacer.expand-add") {
		if !strings.Contains(output.String(), "add-lemma: oo pretty-predicate (= pretty-x pretty-y)\n\n") {
			t.Fatal("source short trace", output.String())
		}
		if !strings.Contains(output.String(), "expand-node: pretty-predicate level: 4294967295 depth: 4\n(= pretty-x pretty-y)\n\n") {
			t.Fatal("source expansion short trace", output.String())
		}
	} else if output.Len() != 0 {
		t.Fatal("disabled short trace emitted output")
	}
}
