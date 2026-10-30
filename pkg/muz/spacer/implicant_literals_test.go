package spacer

// Source: src/muz/spacer/spacer_util.cpp implicant_picker traversal policy.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerImplicantPickerUsesOneTrueBranchAndStableOrder(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	owner, err := NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	}()
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals, ModelEvaluator: owner.contexts[0].config.ModelEvaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = backend.SetProduceModels(true); err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	makeConst := func(name string) ast.ASTHandle {
		symbol, e := manager.Symbols.Intern("spacer-implicant-" + name)
		if e != nil {
			t.Fatal(e)
		}
		value, e := manager.MkConst(symbol, boolean)
		if e != nil {
			t.Fatal(e)
		}
		return value
	}
	p, q, r, s := makeConst("p"), makeConst("q"), makeConst("r"), makeConst("s")
	notP, err := manager.MkNot(p)
	if err != nil {
		t.Fatal(err)
	}
	notS, err := manager.MkNot(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, assertion := range []ast.ASTHandle{notP, q, r, notS} {
		if err = backend.Assert(assertion); err != nil {
			t.Fatal(err)
		}
	}
	if status, checkErr := backend.CheckSat(nil); checkErr != nil || status != lbool.True {
		t.Fatal("model setup", status, checkErr)
	}
	md, err := backend.AcquireModel()
	if err != nil || md == nil {
		t.Fatal("model acquisition", md, err)
	}
	defer func() {
		if err := md.DecRef(); err != nil {
			t.Error(err)
		}
	}()
	branch, err := manager.MkOrArgs([]ast.ASTHandle{p, q})
	if err != nil {
		t.Fatal(err)
	}
	formula, err := manager.MkAndArgs([]ast.ASTHandle{branch, r})
	if err != nil {
		t.Fatal(err)
	}
	literals, err := computeImplicantLiterals(manager, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{formula})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := releaseNormalizeRefs(manager, literals); err != nil {
			t.Error(err)
		}
	}()
	if len(literals) != 2 {
		t.Fatalf("implicant literals %d, want two selected literals", len(literals))
	}
	first, err := manager.RefGet(literals[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.RefGet(literals[1])
	if err != nil {
		t.Fatal(err)
	}
	if !((first == r && second == q) || (first == q && second == r)) {
		t.Fatal("picker did not select the satisfiable conjunction and first-true disjunct", first, second)
	}
	evaluator, err := model.NewModelEvaluator(md.DerivedModel().Core(), owner.contexts[0].config.ModelEvaluator)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := evaluator.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, literal := range []ast.ASTHandle{first, second} {
		value, evalErr := evaluator.EvaluateValue(literal)
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		h, evalErr := manager.RefGet(value)
		if evalErr != nil {
			_ = manager.DelRef(&value)
			t.Fatal(evalErr)
		}
		if !manager.IsTrue(h) {
			t.Fatal("selected implicant literal is false in source model", literal)
		}
		if evalErr = manager.DelRef(&value); evalErr != nil {
			t.Fatal(evalErr)
		}
	}

	implication, err := manager.MkImplies(p, s)
	if err != nil {
		t.Fatal(err)
	}
	implicant, err := computeImplicantLiterals(manager, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{implication})
	if err != nil {
		t.Fatal(err)
	}
	if len(implicant) != 1 {
		_ = releaseNormalizeRefs(manager, implicant)
		t.Fatalf("true implication implicant size %d, want false antecedent", len(implicant))
	}
	selected, err := manager.RefGet(implicant[0])
	if err != nil {
		_ = releaseNormalizeRefs(manager, implicant)
		t.Fatal(err)
	}
	if selected != notP {
		_ = releaseNormalizeRefs(manager, implicant)
		t.Fatal("true implication did not select its false antecedent", selected)
	}
	if err = releaseNormalizeRefs(manager, implicant); err != nil {
		t.Fatal(err)
	}

	equality, err := manager.MkEq(p, q)
	if err != nil {
		t.Fatal(err)
	}
	notEquality, err := manager.MkNot(equality)
	if err != nil {
		t.Fatal(err)
	}
	implicant, err = computeImplicantLiterals(manager, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{notEquality})
	if err != nil {
		t.Fatal(err)
	}
	if len(implicant) != 1 {
		t.Fatalf("Boolean constant disequality: got %d literals, want one", len(implicant))
	}
	selected, err = manager.RefGet(implicant[0])
	if err != nil {
		t.Fatal(err)
	}
	if selected != notEquality {
		t.Fatal("Boolean constant disequality was expanded", selected)
	}
	if err = releaseNormalizeRefs(manager, implicant); err != nil {
		t.Fatal(err)
	}
	one, err := manager.ArithmeticRealNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	two, err := manager.ArithmeticRealNumeralText("2")
	if err != nil {
		t.Fatal(err)
	}
	distinct, err := manager.MkDistinct([]ast.ASTHandle{one, two})
	if err != nil {
		t.Fatal(err)
	}
	numeric, err := computeImplicantLiterals(manager, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{distinct})
	if err != nil {
		t.Fatal(err)
	}
	if len(numeric) != 1 {
		t.Fatalf("binary arithmetic distinct yielded %d literals", len(numeric))
	}
	actual, err := manager.RefGet(numeric[0])
	if err != nil {
		t.Fatal(err)
	}
	expected, err := manager.MkArithmeticLT(one, two)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatal("binary arithmetic distinct did not split after normalization")
	}
	if err = releaseNormalizeRefs(manager, numeric); err != nil {
		t.Fatal(err)
	}

	// A composite Boolean operand does take the recursive branch in C++.
	composite, err := manager.MkAnd(p, r)
	if err != nil {
		t.Fatal(err)
	}
	compositeEq, err := manager.MkEq(composite, q)
	if err != nil {
		t.Fatal(err)
	}
	compositeNot, err := manager.MkNot(compositeEq)
	if err != nil {
		t.Fatal(err)
	}
	implicant, err = computeImplicantLiterals(manager, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{compositeNot})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := releaseNormalizeRefs(manager, implicant); err != nil {
			t.Error(err)
		}
	}()
	if len(implicant) != 2 {
		t.Fatalf("composite disequality: got %d literals, want two", len(implicant))
	}
	seen := make(map[ast.ASTHandle]bool)
	for _, ref := range implicant {
		h, e := manager.RefGet(ref)
		if e != nil {
			t.Fatal(e)
		}
		seen[h] = true
	}
	if !seen[notP] || !seen[q] {
		t.Fatal("composite disequality operands", seen)
	}
}

// Non-arithmetic disequalities must survive picking: dropping them weakens
// Spacer's predecessor obligation to true and repeats the same reachable child.
func TestSpacerImplicantPreservesNonArithmeticDisequality(t *testing.T) {
	for _, arrays := range []bool{false, true} {
		t.Run(map[bool]string{false: "uninterpreted", true: "array"}[arrays], func(t *testing.T) {
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
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("disequality-sort")
			must(err)
			sort, err := m.MkSort(name, ast.ASTSortInfo{})
			must(err)
			if arrays {
				sort, err = m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
				must(err)
			}
			var terms []ast.ASTHandle
			names := []string{"left", "right", "third"}
			if arrays {
				names = names[:2]
			}
			for _, text := range names {
				n, e := m.Symbols.Intern(text)
				must(e)
				term, e := m.MkConst(n, sort)
				must(e)
				terms = append(terms, term)
			}
			eq, err := m.MkEq(terms[0], terms[1])
			must(err)
			neq, err := m.MkNot(eq)
			must(err)
			backend, err := smtcore.NewSMTSolver(m, nil, "ALL", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals, ModelEvaluator: owner.contexts[0].config.ModelEvaluator})
			must(err)
			defer func() { must(backend.Close()) }()
			must(backend.SetProduceModels(true))
			distinct, err := m.MkDistinct(terms)
			must(err)
			must(backend.Assert(distinct))
			status, err := backend.CheckSat(nil)
			must(err)
			if status != lbool.True {
				t.Fatal(status)
			}
			md, err := backend.AcquireModel()
			must(err)
			defer func() { must(md.DecRef()) }()
			literals, err := computeImplicantLiterals(m, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{neq})
			must(err)
			defer func() { must(releaseNormalizeRefs(m, literals)) }()
			if len(literals) != 1 {
				t.Fatalf("disequality dropped: got %d literals", len(literals))
			}
			actual, err := m.RefGet(literals[0])
			must(err)
			if actual != neq {
				t.Fatal("disequality changed", actual, neq)
			}
			for count := 2; count <= len(terms); count++ {
				input, e := m.MkDistinct(terms[:count])
				must(e)
				picked, e := computeImplicantLiterals(m, md, owner.contexts[0].config.ModelEvaluator, []ast.ASTHandle{input})
				must(e)
				if len(picked) != count*(count-1)/2 {
					t.Fatalf("distinct(%d) yielded %d literals", count, len(picked))
				}
				expected := make(map[ast.ASTHandle]bool)
				for i := 0; i < count; i++ {
					for j := i + 1; j < count; j++ {
						eq, e := m.MkEq(terms[i], terms[j])
						must(e)
						ne, e := m.MkNot(eq)
						must(e)
						expected[ne] = true
					}
				}
				for _, ref := range picked {
					h, e := m.RefGet(ref)
					must(e)
					if !expected[h] {
						t.Fatal("distinct pair missing or not normalized", h)
					}
					delete(expected, h)
				}
				if len(expected) != 0 {
					t.Fatal("distinct lost pairs", expected)
				}
				must(releaseNormalizeRefs(m, picked))
			}

		})
	}
}
