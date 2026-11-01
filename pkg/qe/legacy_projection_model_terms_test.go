package qe

// Source: src/muz/spacer/spacer_util.cpp mbqi_project and index_term_finder.
import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"strings"
	"testing"
)

func TestLegacyModelTermProjectionUsesFirstEquivalentIndex(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("mbqi_project")
	tags.Enable("mbqi_project_verbose")
	config := f.r.config
	config.Trace = tags
	p, err := NewLegacyProjection(m, config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mbqi-x", f.integer)
	y := f.constant("mbqi-y", f.integer)
	k := f.constant("mbqi-k", f.integer)
	j := f.constant("mbqi-j", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	seven := f.own(m.ArithmeticIntegerNumeralText("7"))
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("mbqi-a", sort)
	arrays, err := m.ArrayFamilyID()
	fmMust(t, err)
	selectK := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, k}))
	selectJ := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, j}))
	square := f.own(m.MkArithmeticMul(x, x))
	positive := f.own(m.MkArithmeticGT(square, zero))
	eqK := f.own(m.MkEq(selectK, zero))
	eqJ := f.own(m.MkEq(selectJ, zero))
	formula := f.own(m.MkAndArray([]ast.ASTHandle{positive, eqK, eqJ}))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	base := f.own(m.MkConstArray(sort, zero))
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{x: two, y: seven, k: two, j: two, a: base} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	current, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&current)) }()
	remaining, err := p.projectModelTerms(md.Core(), current, []ast.ASTHandle{x, y})
	fmMust(t, err)
	if len(remaining) != 1 || remaining[0] != y {
		t.Fatal("swap-removal of projected variable", remaining)
	}
	if tags.ShouldTrace("mbqi_project") {
		text := output.String()
		if !strings.Contains(text, "MBQI: replacing mbqi-x with mbqi-k") || !strings.Contains(text, "MBQI: failed to eliminate mbqi-y from ") {
			t.Fatal("source MBQI projection decisions", text)
		}
	}
	if tags.ShouldTrace("mbqi_project_verbose") {
		text := output.String()
		input := strings.Index(text, "MBQI: var: mbqi-x\nfml: ")
		candidates := strings.Index(text, "terms:\n0: mbqi-k\n1: mbqi-j\n")
		values := strings.Index(text, "term: mbqi-k tval: 2 val: 2\n")
		decision := strings.Index(text, "MBQI: replacing mbqi-x with mbqi-k")
		if input < 0 || candidates < input || values < candidates || decision < values {
			t.Fatal("source verbose MBQI record order", text)
		}
	}
	root, err := m.RefGet(current)
	fmMust(t, err)
	wantSquare := f.own(m.MkArithmeticMul(k, k))
	wantPositive := f.own(m.MkArithmeticGT(wantSquare, zero))
	want := f.own(m.MkAndArray([]ast.ASTHandle{wantPositive, eqK, eqJ}))
	if root != want {
		t.Fatal("projection must choose the first model-equivalent index", root, want)
	}
	projected, err := p.Project(md.Core(), []ast.ASTHandle{x}, formula, false, true)
	fmMust(t, err)
	defer func() { fmMust(t, projected.Close()) }()
	variables, err := projected.Variables()
	fmMust(t, err)
	if len(variables) != 0 {
		t.Fatal("legacy driver skipped symbolic model-term fallback", variables)
	}
	projectedRoot, err := projected.Formula()
	fmMust(t, err)
	occurs, err := m.OccursExpr(x, projectedRoot)
	fmMust(t, err)
	if occurs {
		t.Fatal("legacy driver left the projected variable in the result")
	}

}

func TestLegacyModelTermProjectionRejectsSelfOccurrence(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("mbqi-occurs-x", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
	a := f.constant("mbqi-occurs-a", sort)
	index := f.own(m.MkArithmeticAdd(x, zero))
	arrays, err := m.ArrayFamilyID()
	fmMust(t, err)
	selected := f.own(m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index}))
	formula := f.own(m.MkEq(selected, zero))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	base := f.own(m.MkConstArray(sort, zero))
	for variable, value := range map[ast.ASTHandle]ast.ASTHandle{x: two, a: base} {
		decl, err := m.GetDecl(variable)
		fmMust(t, err)
		fmMust(t, md.Core().RegisterConstant(decl, value))
	}
	current, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&current)) }()
	remaining, err := p.projectModelTerms(md.Core(), current, []ast.ASTHandle{x})
	fmMust(t, err)
	root, err := m.RefGet(current)
	fmMust(t, err)
	if len(remaining) != 1 || remaining[0] != x || root != formula {
		t.Fatal("self-containing candidate changed projection", remaining, root)
	}
}

func TestLegacyProjectionRemainingArithmeticSubstitutionPolicy(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("spacer_mbp")
	config := f.r.config
	config.Trace = tags
	p, err := NewLegacyProjection(m, config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("remaining-policy-x", f.integer)
	zero := f.own(m.ArithmeticIntegerNumeralText("0"))
	two := f.own(m.ArithmeticIntegerNumeralText("2"))
	square := f.own(m.MkArithmeticMul(x, x))
	formula := f.own(m.MkArithmeticGT(square, zero))
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	decl, err := m.GetDecl(x)
	fmMust(t, err)
	fmMust(t, md.Core().RegisterConstant(decl, two))
	for _, retain := range []bool{true, false} {
		output.Reset()
		result, err := p.Project(md.Core(), []ast.ASTHandle{x}, formula, false, retain)
		fmMust(t, err)
		root, err := result.Formula()
		fmMust(t, err)
		remaining, err := result.Variables()
		fmMust(t, err)
		occurs, err := m.OccursExpr(x, root)
		fmMust(t, err)
		if retain {
			if len(remaining) != 1 || remaining[0] != x || !occurs {
				t.Fatal("dont_sub discarded the nonlinear variable", remaining, occurs)
			}
		} else if len(remaining) != 0 || occurs || !m.IsTrue(root) {
			t.Fatal("model substitution did not simplify the nonlinear witness", remaining, root)
		}
		if tags.ShouldTrace("spacer_mbp") {
			text := output.String()
			previous := -1
			for _, marker := range []string{"After qe_lite:\n", "Vars:\nremaining-policy-x\n", "Array vars:\n", "extended model:\n", "Auxiliary variables of index and value sorts:\n", "Arith vars:\nremaining-policy-x\n", "Projected arith vars:\n", "Remaining arith vars:\nremaining-policy-x\n"} {
				offset := strings.Index(text[previous+1:], marker)
				if offset < 0 {
					t.Fatalf("missing ordered marker %q in %s", marker, text)
				}
				previous += offset + 1
			}
			marker := "After substituting remaining arith vars:\n"
			at := strings.Index(text, marker)
			if retain && at >= 0 {
				t.Fatal("dont_sub emitted substitution record", text)
			}
			if !retain && (at <= previous || !strings.Contains(text[at:], "(* 2 2)")) {
				t.Fatal("substitution trace must precede final arithmetic rewrite", text)
			}
		}
		fmMust(t, result.Close())
	}
}

func TestLegacyModelTermProjectionEqualityDirections(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		f := newFMFixture(t)
		m := f.r.manager
		p, err := NewLegacyProjection(m, f.r.config)
		fmMust(t, err)
		x := f.constant("mbqi-eq-x", f.integer)
		y := f.constant("mbqi-eq-y", f.integer)
		two := f.own(m.ArithmeticIntegerNumeralText("2"))
		left, right := x, y
		if reverse {
			left, right = right, left
		}
		formula := f.own(m.MkEq(left, right))
		md, err := model.NewModel(m)
		fmMust(t, err)
		for _, variable := range []ast.ASTHandle{x, y} {
			decl, err := m.GetDecl(variable)
			fmMust(t, err)
			fmMust(t, md.Core().RegisterConstant(decl, two))
		}
		current, err := m.NewRef(formula)
		fmMust(t, err)
		remaining, err := p.projectModelTerms(md.Core(), current, []ast.ASTHandle{x})
		fmMust(t, err)
		root, err := m.RefGet(current)
		fmMust(t, err)
		occurs, err := m.OccursExpr(x, root)
		fmMust(t, err)
		if len(remaining) != 0 || occurs {
			t.Fatal("equality candidate direction", reverse, remaining, root)
		}
		fmMust(t, m.DelRef(&current))
		fmMust(t, md.Close())
		fmMust(t, p.Close())
	}
}
