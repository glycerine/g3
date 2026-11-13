//go:build g3trace

package fp

// Source: pdr_dl_interface.cpp:92-101 prints the original query argument,
// aggregate background and original rules after mk_query/check_reset.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRInputTracePreservesOriginalExpressionAndBackground(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pdr")
	e, _ := newPDRTransformedTraceFixture(t, tags)
	m := e.manager
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	var axioms []ast.ASTHandle
	for _, label := range []string{"bg-a", "bg-b"} {
		name, err := m.Symbols.Intern(label)
		fragmentMust(t, err)
		h, err := m.MkConst(name, boolean)
		fragmentMust(t, err)
		fragmentMust(t, e.AssertBackground(h))
		axioms = append(axioms, h)
	}
	background, err := m.MkAndArray(axioms)
	fragmentMust(t, err)
	fragmentMust(t, e.SetBackground(background))
	fact, err := e.set.Rule(0)
	fragmentMust(t, err)
	head, err := fact.Head()
	fragmentMust(t, err)
	binder, err := m.Symbols.Intern("user-x")
	fragmentMust(t, err)
	original, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{binder}, Body: head})
	fragmentMust(t, err)
	originalRef, err := m.NewRef(original)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&originalRef)); fragmentMust(t, fact.Close()) }()
	before, err := e.set.Len()
	fragmentMust(t, err)
	status, err := e.QueryExpression(original)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("expression query status", status)
	}
	text := output.String()
	input := strings.Index(text, "pdr_dl_interface.cpp:92")
	transformed := strings.Index(text, "pdr_dl_interface.cpp:144")
	if input < 0 || transformed <= input {
		t.Fatal("input/transformed trace order", text)
	}
	for _, part := range []string{"axioms:\n(and bg-a bg-b)\nquery: (exists", "user-x", "dump-P", "dump-step:"} {
		if !strings.Contains(text, part) {
			t.Fatal("source input trace missing", part, text)
		}
	}
	if strings.Contains(text, "query: query!") {
		t.Fatal("original expression replaced by synthetic query in input trace", text)
	}
	if e.queryClone == nil || !e.queryClone.pdrTraceQuery.IsNull() {
		t.Fatal("borrowed input query escaped its call scope")
	}
	after, err := e.set.Len()
	fragmentMust(t, err)
	if after != before {
		t.Fatal("trace/query changed persistent rules", after, before)
	}
	got, err := m.RefGet(originalRef)
	fragmentMust(t, err)
	if got != original {
		t.Fatal("trace changed caller query owner")
	}
}

func TestPDRInputTraceOriginalOwnerSurvivesDumpFailure(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("pdr")
	e, _ := newPDRTransformedTraceFixture(t, tags)
	m := e.manager
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.print_fixedpoint_extensions", "false"))
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	fragmentMust(t, e.AssertBackground(truth))
	fact, err := e.set.Rule(0)
	fragmentMust(t, err)
	head, err := fact.Head()
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("failed-input-x")
	fragmentMust(t, err)
	original, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: head})
	fragmentMust(t, err)
	ref, err := m.NewRef(original)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&ref)); fragmentMust(t, fact.Close()) }()
	// context::query constructs its PDR engine before entering mk_query. A
	// dump failure leaves that pre-query context owned by the parent. Measure
	// its references independently so the exact count still detects leaked
	// temporary query owners without treating the live context as a leak.
	controlTags := diagnostic.NewTraceTagsWithWriter(&bytes.Buffer{})
	control, _ := newPDRTransformedTraceFixture(t, controlTags)
	controlBefore := control.manager.Stats().LiveReferences
	_, err = control.pdrContext()
	fragmentMust(t, err)
	contextReferences := control.manager.Stats().LiveReferences - controlBefore
	before := m.Stats().LiveReferences
	_, err = e.QueryExpression(original)
	if err == nil || !strings.Contains(err.Error(), "Background axioms cannot be used with SMT-LIB2 HORN format") {
		t.Fatal("missing later dump failure", err)
	}
	// The context's rule manager keeps the query rule's m_head and m_body
	// until its next rule, and pins the fresh query!N predicate
	// (context::register_predicate); release the former and count the latter.
	fragmentMust(t, e.rules.ReleaseHornTemporaries())
	const pinnedQueryPredicate = 1
	if e.pdr == nil || m.Stats().LiveReferences != before+contextReferences+pinnedQueryPredicate {
		t.Fatal("failed expression trace lost temporary owners or its PDR context", before, contextReferences, m.Stats().LiveReferences)
	}
	prequery := e.pdr
	initialized, err := prequery.Initialized()
	fragmentMust(t, err)
	if initialized {
		t.Fatal("failed dump installed a PDR program")
	}
	if e.queryClone != nil || !e.pdrTraceQuery.IsNull() {
		t.Fatal("failed query published a clone or borrowed input")
	}
	got, err := m.RefGet(ref)
	fragmentMust(t, err)
	if got != original {
		t.Fatal("failed trace damaged caller query")
	}
	tags.Disable("pdr")
	status, err := e.QueryExpression(original)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("expression query retry", status)
	}
	if e.queryClone == nil || e.queryClone.pdr != prequery {
		t.Fatal("expression query retry lost its pre-query context")
	}
}
