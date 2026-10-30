package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp lemma_eq_generalizer.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerEqualityGeneralizerOwnsExpandedTransitiveEqualities(t *testing.T) {
	for _, count := range []int{3, 4} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			p := c.transformers[f.query]
			zero, err := m.ArithmeticIntegerNumeralText("0")
			depthMust(t, err)
			sort, err := m.GetSort(zero)
			depthMust(t, err)
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			terms := make([]ast.ASTHandle, count)
			for i := range terms {
				terms[i] = own(m.MkFreshConst(fmt.Sprintf("eq-class-%d", i), sort))
			}
			var cube []ast.ASTHandle
			for i := 1; i < count; i++ {
				cube = append(cube, own(m.MkEq(terms[i-1], terms[i])))
			}
			post := own(m.MkAndArray(cube))
			obligation, err := p.NewProofObligation(nil, 1, 0, post, nil)
			depthMust(t, err)
			original, err := NewLemmaFromProofObligation(obligation, cube, 2)
			depthMust(t, err)
			defer func() { depthMust(t, original.Close()) }()
			expanded, err := p.generalizeEqualityClasses(obligation, original)
			depthMust(t, err)
			if expanded == original {
				t.Fatal("transitive equality class did not expand")
			}
			defer func() { depthMust(t, expanded.Close()) }()
			// All newly generated pairs must remain usable after the temporary
			// equivalence-class vector has been destroyed.
			actual, err := expanded.Cube()
			depthMust(t, err)
			if len(actual) != count*(count-1)/2 {
				t.Fatal("expanded pair count", len(actual))
			}
			pairs := make(map[[2]ast.ASTHandle]bool)
			for _, equality := range actual {
				kind, err := m.GetDeclKind(equality)
				depthMust(t, err)
				if kind != ast.ASTOpEq {
					t.Fatal("expected equality", kind)
				}
				left, err := m.GetArg(equality, 0)
				depthMust(t, err)
				right, err := m.GetArg(equality, 1)
				depthMust(t, err)
				pairs[[2]ast.ASTHandle{left, right}] = true
			}
			for i := range terms {
				for j := i + 1; j < len(terms); j++ {
					if !pairs[[2]ast.ASTHandle{terms[i], terms[j]}] && !pairs[[2]ast.ASTHandle{terms[j], terms[i]}] {
						t.Fatal("missing equality pair", i, j)
					}
				}
			}
			level, err := expanded.Level()
			depthMust(t, err)
			if level != 2 {
				t.Fatal("equality expansion changed level", level)
			}
		})
	}
}

func TestSpacerArrayGeneralizerOwnsCandidateAcrossSolverPop(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	depthMust(t, err)
	a, err := m.MkFreshConst("array-lifetime-a", arraySort)
	depthMust(t, err)
	b, err := m.MkFreshConst("array-lifetime-b", arraySort)
	depthMust(t, err)
	index, err := m.MkTrue()
	depthMust(t, err)
	arrays, err := m.ArrayFamilyID()
	depthMust(t, err)
	x, err := m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index})
	depthMust(t, err)
	y, err := m.MkFamilyApp(arrays, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, index})
	depthMust(t, err)
	equality, err := m.MkEq(x, y)
	depthMust(t, err)
	literal, err := m.MkNot(equality)
	depthMust(t, err)
	pin, err := m.NewRef(literal)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&pin)) }()
	strengthened, applicable, err := c.transformers[f.query].strengthenArrayEqualities([]ast.ASTHandle{literal})
	depthMust(t, err)
	if !applicable || len(strengthened) != 1 {
		t.Fatal("expected a strengthened array disequality")
	}
	defer func() { depthMust(t, releaseNormalizeRefs(m, strengthened)) }()
	h, err := m.RefGet(strengthened[0])
	depthMust(t, err)
	negated, pair, err := m.IsNot(h)
	depthMust(t, err)
	if !negated {
		t.Fatal("expected negated array equality")
	}
	left, err := m.GetArg(pair, 0)
	depthMust(t, err)
	right, err := m.GetArg(pair, 1)
	depthMust(t, err)
	if !((left == a && right == b) || (left == b && right == a)) {
		t.Fatal("wrong array pair", left, right)
	}
}
