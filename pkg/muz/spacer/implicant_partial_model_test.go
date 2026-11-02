package spacer

// Source: src/muz/spacer/spacer_util.cpp:884-920.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestSpacerImplicantVectorUsesPartialModelWithoutCompletion(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	defer func() { depthMust(t, owner.Close()) }()
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	var pins []ast.ASTRef
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		depthMust(t, err)
		ref, err := m.NewRef(h)
		depthMust(t, err)
		pins = append(pins, ref)
		return h
	}
	defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
	p := own(m.MkFreshConst("partial-vector-p", boolean))
	q := own(m.MkFreshConst("partial-vector-q", boolean))
	unknown := own(m.MkFreshConst("partial-vector-unknown", boolean))
	truth := own(m.MkTrue())
	falsity := own(m.MkFalse())
	notQ := own(m.MkNot(q))
	conjunction := own(m.MkAnd(p, notQ))
	disjunction := own(m.MkOr(p, unknown))
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	for i, variable := range []ast.ASTHandle{p, q} {
		decl, err := m.GetDecl(variable)
		depthMust(t, err)
		depthMust(t, md.Core().RegisterConstant(decl, []ast.ASTHandle{truth, falsity}[i]))
	}
	unknownDecl, err := m.GetDecl(unknown)
	depthMust(t, err)
	cases := []struct {
		name        string
		input, want []ast.ASTHandle
	}{
		// flatten_and appends p/notQ, then swaps the last appended child
		// into the conjunction slot: notQ, unknown, disjunction, p.
		{"partial", []ast.ASTHandle{conjunction, unknown, disjunction}, []ast.ASTHandle{notQ, p}},
		{"shared", []ast.ASTHandle{p, disjunction, p, notQ}, []ast.ASTHandle{p, notQ}},
		{"false", []ast.ASTHandle{conjunction, falsity, disjunction}, nil},
		{"unknown", []ast.ASTHandle{unknown}, nil},
		{"empty", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := m.Stats().LiveReferences
			refs, err := computeImplicantLiterals(m, md.Core(), owner.contexts[0].config.ModelEvaluator, tc.input)
			depthMust(t, err)
			defer func() {
				depthMust(t, releaseNormalizeRefs(m, refs))
				if got := m.Stats().LiveReferences; got != before {
					t.Errorf("temporary vector references: got %d want %d", got, before)
				}
			}()
			if len(refs) != len(tc.want) {
				t.Fatal("implicant size", len(refs), len(tc.want))
			}
			for i, ref := range refs {
				h, err := m.RefGet(ref)
				depthMust(t, err)
				if h != tc.want[i] {
					t.Fatal("source literal order", i, h, tc.want[i])
				}
			}
			assigned, err := md.Core().HasConstantInterpretation(unknownDecl)
			depthMust(t, err)
			if assigned {
				t.Fatal("implicant extraction completed an unknown constant")
			}
		})
	}
}
