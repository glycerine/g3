package spacer

// Source: spacer_util.cpp implicant_picker::process_app selects both ITE
// branches when their model values agree, and otherwise the selected branch
// and condition. The worklist visits the last appended expression first.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	smtcore "github.com/glycerine/g3/pkg/smtcore"
)

func TestSpacerImplicantITEModelBranchSelection(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	must(err)
	defer func() { must(owner.Close()) }()
	md, err := model.NewModel(m)
	must(err)
	defer func() { must(md.Close()) }()
	var pins []ast.ASTRef
	defer func() { must(releaseNormalizeRefs(m, pins)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, e := m.NewRef(h)
		must(e)
		pins = append(pins, ref)
		return h
	}
	boolean, err := m.MkBoolSort()
	must(err)
	yes, err := m.MkTrue()
	must(err)
	no, err := m.MkFalse()
	must(err)
	constants := make([]ast.ASTHandle, 4)
	for i, name := range []string{"ite-condition-true", "ite-then-true", "ite-else-false", "ite-condition-false"} {
		sym, e := m.Symbols.Intern(name)
		must(e)
		constants[i] = own(m.MkConst(sym, boolean))
		decl, e := m.GetDecl(constants[i])
		must(e)
		value := no
		if i < 2 {
			value = yes
		}
		must(md.Core().RegisterConstant(decl, value))
	}
	p, q, r, s := constants[0], constants[1], constants[2], constants[3]
	notR := own(m.MkNot(r))
	notS := own(m.MkNot(s))
	bothTrue := own(m.MkIte(r, p, q))
	bothFalse := own(m.MkIte(p, r, s))
	notBothFalse := own(m.MkNot(bothFalse))
	selectedThen := own(m.MkIte(p, q, r))
	selectedElse := own(m.MkIte(s, r, q))
	for _, tc := range []struct {
		name     string
		input    ast.ASTHandle
		expected []ast.ASTHandle
	}{
		{"both true", bothTrue, []ast.ASTHandle{q, p}},
		{"both false", notBothFalse, []ast.ASTHandle{notS, notR}},
		{"then", selectedThen, []ast.ASTHandle{q, p}},
		{"else", selectedElse, []ast.ASTHandle{q, notS}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs, err := computeImplicantLiterals(m, md.Core(), owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{tc.input})
			must(err)
			defer func() { must(releaseNormalizeRefs(m, refs)) }()
			if len(refs) != len(tc.expected) {
				t.Fatalf("got %d literals, want %d", len(refs), len(tc.expected))
			}
			for i, ref := range refs {
				h, err := m.RefGet(ref)
				must(err)
				if h != tc.expected[i] {
					t.Fatalf("literal %d differs from source branch order", i)
				}
			}
		})
	}
}
