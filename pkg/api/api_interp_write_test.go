package api

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

func TestInterpolationWriteProblemSerializesSequenceAndTree(t *testing.T) {
	f := newInterpolationFixture(t)
	boolean := f.pin(f.w.BoolType())
	a := f.variable("write-a", boolean)
	b := f.variable("write-b", boolean)
	c := f.variable("write-c", boolean)
	d := f.variable("write-d", boolean)
	background := f.variable("write-background", boolean)
	notB := f.make(interp.OperatorNot, b)
	constraints := []ast.ASTHandle{a, notB, c, d}
	theory := []ast.ASTHandle{background}

	outputs := make([]string, 0, 2)
	for _, test := range []struct {
		name    string
		parents []int
	}{
		{name: "sequence"},
		{name: "tree", parents: []int{2, 2, 3, interp.ScopeTop}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := f.c.WriteInterpolationProblem(&encoded, constraints, test.parents, nil); err != nil {
				t.Fatal(err)
			}
			text := encoded.String()
			if !strings.HasPrefix(text, "(set-option :produce-interpolants true)\n") || strings.Count(text, ":named f!") != len(constraints) || !strings.Contains(text, "(get-interpolant ") {
				t.Fatalf("unexpected interpolation problem output:\n%s", text)
			}
			outputs = append(outputs, text)
		})
	}
	if outputs[0] == outputs[1] {
		t.Fatal("different parent vectors produced identical interpolation trees")
	}
	var withTheory bytes.Buffer
	if err := f.c.WriteInterpolationProblem(&withTheory, constraints[:2], nil, theory); err != nil {
		t.Fatal(err)
	}
	constraintPosition := strings.Index(withTheory.String(), "write-a")
	theoryPosition := strings.Index(withTheory.String(), "write-background")
	if constraintPosition < 0 || theoryPosition <= constraintPosition {
		t.Fatal("writer omitted a source theory assertion")
	}
	var empty bytes.Buffer
	if err := f.c.WriteInterpolationProblem(&empty, nil, nil, nil); err != nil || empty.Len() != 0 {
		t.Fatalf("empty problem output length=%d err=%v", empty.Len(), err)
	}
}

func TestInterpolationWriteProblemRejectsInvalidParentsAndSorts(t *testing.T) {
	f := newInterpolationFixture(t)
	boolean := f.pin(f.w.BoolType())
	integer := f.pin(f.w.IntType())
	p := f.variable("write-invalid-p", boolean)
	x := f.variable("write-invalid-x", integer)
	for _, parents := range [][]int{{0, interp.ScopeTop}, {1, 0}, {1}} {
		if err := f.c.WriteInterpolationProblem(&bytes.Buffer{}, []ast.ASTHandle{p, p}, parents, nil); err == nil {
			t.Errorf("accepted invalid parents %v", parents)
		}
	}
	if err := f.c.WriteInterpolationProblem(&bytes.Buffer{}, []ast.ASTHandle{x}, nil, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("accepted non-Boolean constraint: %v", err)
	}
}
