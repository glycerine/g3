package ast

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// withoutNestedNodeTraces drops the format manager's register_node_core and
// delete_node blocks. mk_pp builds its format while the mk_quantifier trace
// body is printing, and format_ref frees it afterwards, so every print nests
// those TRACE("ast") blocks inside the pattern line, as in the source.
func withoutNestedNodeTraces(got string) string {
	const end = "------------------------------------------------\n"
	for _, header := range []string{"-------- [ast] register_node_core ", "-------- [ast] delete_node "} {
		for {
			i := strings.Index(got, header)
			if i < 0 {
				break
			}
			j := strings.Index(got[i:], end)
			if j < 0 {
				break
			}
			got = got[:i] + got[i+j+len(end):]
		}
	}
	return got
}

func TestASTQuantifierPatternSourceTraceBeforeRegistration(t *testing.T) {
	c, g, _, budget := dynAckSourceFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget, TraceTags: tags})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.Close()) }()
	previous, err := uparams.BindProcessGlobalParamsService(g)
	paramsMust(t, err)
	defer func() { _, e := uparams.BindProcessGlobalParamsService(previous); paramsMust(t, e) }()
	sort, err := m.MkBoolSort()
	paramsMust(t, err)
	truth, err := m.MkFamilyApp(0, ASTOpTrue, nil, nil)
	paramsMust(t, err)
	pattern, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{truth})
	paramsMust(t, err)
	tags.Enable("ast")
	spec := ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: truth, Patterns: []ASTHandle{pattern}}
	q, err := m.MkQuantifier(spec)
	paramsMust(t, err)
	if q == (ASTHandle{}) {
		t.Fatal("missing quantifier")
	}
	if ASTSourceDebug && tags.ShouldTrace("ast") {
		got := output.String()
		if !strings.HasPrefix(got, "-------- [ast] mk_quantifier src/ast/ast.cpp:2358 ---------\n0 ") ||
			!strings.Contains(got, "(true)\n------------------------------------------------\n") ||
			strings.Index(got, "(true)\n------------------------------------------------\n") > strings.LastIndex(got, "register_node_core") {
			t.Fatalf("pattern trace missing or late: %q", got)
		}
	} else if output.Len() != 0 {
		t.Fatalf("erased pattern trace emitted %q", output.String())
	}
	output.Reset()
	spec.Patterns = []ASTHandle{truth}
	_, err = m.MkQuantifier(spec)
	if ASTSourceDebug {
		if err != ErrASTDomain {
			t.Fatalf("invalid pattern was not refused: %v", err)
		}
		if tags.ShouldTrace("ast") && (!strings.HasPrefix(output.String(), "-------- [ast] mk_quantifier src/ast/ast.cpp:2358 ---------\n0 ") ||
			!strings.HasSuffix(output.String(), "true\n------------------------------------------------\n")) {
			t.Fatalf("invalid pattern was not traced before refusal: %q", output.String())
		}
	} else {
		paramsMust(t, err)
	}
	output.Reset()
	variable, err := m.MkVar(0, sort)
	paramsMust(t, err)
	badPattern, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{variable})
	paramsMust(t, err)
	output.Reset()
	spec.Patterns = []ASTHandle{badPattern}
	_, err = m.MkQuantifier(spec)
	if ASTSourceDebug {
		if err != ErrASTDomain {
			t.Fatalf("pattern with variable child was not refused after TRACE: %v, output %q", err, output.String())
		}
		if tags.ShouldTrace("ast") && !strings.HasSuffix(output.String(), "(pattern (:var 0))\n------------------------------------------------\n") {
			t.Fatalf("pattern fallback source TRACE missing: %q", output.String())
		}
	} else {
		paramsMust(t, err)
	}
	output.Reset()
	spec.Patterns = []ASTHandle{truth, pattern}
	_, err = m.MkQuantifier(spec)
	if ASTSourceDebug {
		if err != ErrASTDomain {
			t.Fatalf("first invalid pattern was not refused: %v", err)
		}
		if tags.ShouldTrace("ast") {
			got := withoutNestedNodeTraces(output.String())
			if strings.Count(got, "[ast] mk_quantifier src/ast/ast.cpp:2358") != 1 ||
				!strings.Contains(got, "\n0 true\n") || strings.Contains(got, "\n1 ") {
				t.Fatalf("later pattern traced after first assertion: %q", got)
			}
		} else if output.Len() != 0 {
			t.Fatalf("erased first-pattern trace emitted %q", output.String())
		}
	} else {
		paramsMust(t, err)
	}
	output.Reset()
	spec.Patterns = []ASTHandle{pattern, truth}
	_, err = m.MkQuantifier(spec)
	if ASTSourceDebug {
		if err != ErrASTDomain {
			t.Fatalf("second invalid pattern was not refused: %v", err)
		}
		if tags.ShouldTrace("ast") {
			got := withoutNestedNodeTraces(output.String())
			if strings.Count(got, "[ast] mk_quantifier src/ast/ast.cpp:2358") != 2 ||
				!strings.Contains(got, "\n0 (true)\n") || !strings.Contains(got, "\n1 true\n") {
				t.Fatalf("second pattern trace/source assertion order: %q", got)
			}
		} else if output.Len() != 0 {
			t.Fatalf("erased second-pattern trace emitted %q", output.String())
		}
	} else {
		paramsMust(t, err)
	}
}

func TestASTQuantifierPatternTraceUninterpretedTrigger(t *testing.T) {
	c, g, _, budget := dynAckSourceFixture(t)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	m, err := NewASTManager(ASTManagerConfig{Symbols: c.SymbolsField, Numerals: c.NumeralsField, Budget: budget, TraceTags: tags})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.Close()) }()
	previous, err := uparams.BindProcessGlobalParamsService(g)
	paramsMust(t, err)
	defer func() { _, e := uparams.BindProcessGlobalParamsService(previous); paramsMust(t, e) }()
	sort, err := m.MkBoolSort()
	paramsMust(t, err)
	variable, err := m.MkVar(0, sort)
	paramsMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "trigger-f"), []ASTHandle{sort}, sort, ASTFuncDeclInfo{})
	paramsMust(t, err)
	gDecl, err := m.MkFuncDecl(astTestSymbol(t, m, "trigger-g"), []ASTHandle{sort}, sort, ASTFuncDeclInfo{})
	paramsMust(t, err)
	first, err := m.MkApp(f, []ASTHandle{variable})
	paramsMust(t, err)
	second, err := m.MkApp(gDecl, []ASTHandle{variable})
	paramsMust(t, err)
	pattern, err := m.MkFamilyApp(2, 0, nil, []ASTHandle{first, second})
	paramsMust(t, err)
	tags.Enable("ast")
	output.Reset()
	_, err = m.MkQuantifier(ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{sort}, Names: []usymbol.Symbol{astTestSymbol(t, m, "x")}, Body: first, Patterns: []ASTHandle{pattern}})
	paramsMust(t, err)
	if ASTSourceDebug && tags.ShouldTrace("ast") {
		got := output.String()
		body := "((trigger-f (:var 0)) (trigger-g (:var 0)))\n------------------------------------------------\n"
		if !strings.HasPrefix(got, "-------- [ast] mk_quantifier src/ast/ast.cpp:2358 ---------\n0 ") ||
			!strings.Contains(got, body) ||
			strings.LastIndex(got, "register_node_core") < strings.Index(got, body) {
			t.Fatalf("uninterpreted multi-trigger trace: %q", got)
		}
	} else if output.Len() != 0 {
		t.Fatalf("erased multi-trigger trace emitted %q", output.String())
	}
}
