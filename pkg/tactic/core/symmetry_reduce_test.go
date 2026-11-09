package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func symmetryConstant(t *testing.T, m *ast.ASTManager, sort ast.ASTHandle, name string) ast.ASTHandle {
	t.Helper()
	symbol, err := m.Symbols.Intern(name)
	simplifyMust(t, err)
	h, err := m.MkConst(symbol, sort)
	simplifyMust(t, err)
	simplifyPin(t, m, h)
	return h
}

func TestSymmetryOccurrencesAndTermPreference(t *testing.T) {
	r, _, constants, terms := symmetryFixture(t, 3)
	m := r.manager
	eq, err := m.MkEq(terms[0], constants[0])
	simplifyMust(t, err)
	simplifyPin(t, m, eq)
	shared, err := m.MkOr(eq, eq)
	simplifyMust(t, err)
	simplifyPin(t, m, shared)
	graph, err := r.symmetryGraph(shared)
	simplifyMust(t, err)
	if graph.occurrences[eq] != 2 || graph.occurrences[terms[0]] != 1 || graph.occurrences[constants[0]] != 1 {
		t.Fatal("occurrences must count distinct application edges", graph.occurrences)
	}
	sort, err := m.GetSort(constants[0])
	simplifyMust(t, err)
	name, err := m.Symbols.Intern("f")
	simplifyMust(t, err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, m, decl)
	fa, err := m.MkApp(decl, []ast.ASTHandle{constants[0]})
	simplifyMust(t, err)
	simplifyPin(t, m, fa)
	fx, err := m.MkApp(decl, []ast.ASTHandle{terms[0]})
	simplifyMust(t, err)
	simplifyPin(t, m, fx)
	for _, tc := range []struct {
		terms  []ast.ASTHandle
		counts map[ast.ASTHandle]uint32
		want   int
	}{
		{[]ast.ASTHandle{fa, terms[0]}, map[ast.ASTHandle]uint32{fa: 10, terms[0]: 1}, 1},
		{[]ast.ASTHandle{terms[0], terms[1]}, map[ast.ASTHandle]uint32{terms[0]: 2, terms[1]: 3}, 1},
		{[]ast.ASTHandle{fa, fx}, map[ast.ASTHandle]uint32{fa: 2, fx: 2}, 1},
		{[]ast.ASTHandle{fa, fx}, map[ast.ASTHandle]uint32{fa: 3, fx: 2}, 0},
	} {
		index, e := r.promisingSymmetryTerm(tc.terms, nil, constants, tc.counts)
		simplifyMust(t, e)
		if index != tc.want {
			t.Fatal("source arity/weight/support preference", index, tc.want)
		}
	}
}

func TestSymmetryConstructorAllocationFrontiers(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		succeeded := false
		for stop := int64(0); stop < 64; stop++ {
			t.Run(fmt.Sprintf("proofs%v/%d", proofs, stop), func(t *testing.T) {
				m, c, g := simplifyFixture(t)
				if proofs {
					simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				m.Budget().FailAfter(stop)
				r, err := NewSymmetryReduceTactic(m, c, g)
				m.Budget().FailAfter(-1)
				if err == nil {
					succeeded = true
				} else if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal(err)
				}
				if r != nil {
					simplifyMust(t, r.Close())
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded {
			t.Fatal("constructor never reached success")
		}
	}
}

func TestSymmetryApplyAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 256; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			r, goal, _, _ := symmetryFixture(t, 3)
			var result tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, result.Close()) })
			r.manager.Budget().FailAfter(stop)
			err := r.Apply(goal, &result)
			r.manager.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(err)
			}
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("apply never reached success")
	}
}
func symmetryFixture(t *testing.T, count int) (*SymmetryReduceTactic, *tactic.Goal, []ast.ASTHandle, []ast.ASTHandle) {
	t.Helper()
	m, c, g := simplifyFixture(t)
	r, err := NewSymmetryReduceTactic(m, c, g)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	name, err := m.Symbols.Intern("U")
	simplifyMust(t, err)
	sort, err := m.MkSort(name, ast.ASTSortInfo{})
	simplifyMust(t, err)
	simplifyPin(t, m, sort)
	var constants []ast.ASTHandle
	for i := 0; i < count; i++ {
		constants = append(constants, symmetryConstant(t, m, sort, string(rune('a'+i))))
	}
	terms := []ast.ASTHandle{symmetryConstant(t, m, sort, "x"), symmetryConstant(t, m, sort, "y")}
	goal, err := tactic.NewGoal(m, false, true, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, goal.Close()) })
	for _, term := range terms {
		var eqs []ast.ASTHandle
		for _, constant := range constants {
			eq, err := m.MkEq(term, constant)
			simplifyMust(t, err)
			simplifyPin(t, m, eq)
			eqs = append(eqs, eq)
		}
		form, err := m.MkOrArgs(eqs)
		simplifyMust(t, err)
		simplifyMust(t, goal.Assert(form))
	}
	return r, goal, constants, terms
}

func TestSymmetryReduceAddsOrbitRestrictionsAndPreservesModels(t *testing.T) {
	r, goal, constants, terms := symmetryFixture(t, 3)
	m := r.manager
	var out tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, out.Close()) })
	simplifyMust(t, r.Apply(goal, &out))
	size, err := goal.Size()
	simplifyMust(t, err)
	if size != 4 {
		t.Fatal("expected two symmetry-breaking predicates", size)
	}
	depth, err := goal.Depth()
	simplifyMust(t, err)
	if depth != 1 {
		t.Fatal("depth", depth)
	}
	if out.ModelConverter != nil || out.ProofConverter != nil {
		t.Fatal("source has no converters")
	}
	// Every original assignment of x,y to the three distinct constants has
	// a permuted representative satisfying the added restrictions. Check
	// all nine assignments and all six permutations, not one chosen model.
	var evaluate func(ast.ASTHandle, map[ast.ASTHandle]int) bool
	evaluate = func(h ast.ASTHandle, values map[ast.ASTHandle]int) bool {
		op, e := m.GetDeclKind(h)
		simplifyMust(t, e)
		n, e := r.symmetryNode(h)
		simplifyMust(t, e)
		if op == ast.ASTOpEq {
			return values[n.args[0]] == values[n.args[1]]
		}
		if op == ast.ASTOpOr {
			for _, child := range n.args {
				if evaluate(child, values) {
					return true
				}
			}
			return false
		}
		t.Fatal("unexpected restriction operator", op)
		return false
	}
	permutations := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			found := false
			for _, p := range permutations {
				values := map[ast.ASTHandle]int{constants[0]: 0, constants[1]: 1, constants[2]: 2, terms[0]: p[x], terms[1]: p[y]}
				valid := true
				for i := uint32(2); i < size; i++ {
					h, e := goal.Form(i)
					simplifyMust(t, e)
					valid = valid && evaluate(h, values)
				}
				found = found || valid
			}
			if !found {
				t.Fatal("symmetry breaking removed an entire orbit", x, y)
			}
		}
	}
	// Cleanup is a source no-op; cached normalization and repeated calls
	// must remain usable and the already restricted goal remains satisfiable.
	simplifyMust(t, r.Cleanup())
	simplifyMust(t, r.Apply(goal, &out))
}

func TestSymmetryReduceCPPApply(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the native executable")
	}
	r, goal, _, _ := symmetryFixture(t, 3)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, r.Apply(goal, &result))
	size, err := goal.Size()
	simplifyMust(t, err)
	depth, err := goal.Depth()
	simplifyMust(t, err)
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = strings.NewReader("(set-logic QF_UF)(declare-sort U 0)(declare-const a U)(declare-const b U)(declare-const c U)(declare-const x U)(declare-const y U)(assert (or (= x a) (= x b) (= x c)))(assert (or (= y a) (= y b) (= y c)))(apply symmetry-reduce)")
	native, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(native))
	}
	// C++ hash order chooses different constant names. Both source and Go
	// must add two range restrictions; the exhaustive orbit test separately
	// checks every model class rather than depending on a representative.
	if strings.Count(string(native), "(or ") != int(size) || !strings.Contains(string(native), fmt.Sprintf(":depth %d", depth)) {
		t.Fatal("native strengthening/depth differs", size, depth, string(native))
	}
}

func TestSymmetryReduceRejectsEqualColorNonSymmetries(t *testing.T) {
	for _, cycleOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "directed", true: "swap-but-not-cycle"}[cycleOnly], func(t *testing.T) {
			count := 2
			if cycleOnly {
				count = 3
			}
			r, goal, constants, _ := symmetryFixture(t, count)
			m := r.manager
			boolean, err := m.MkBoolSort()
			simplifyMust(t, err)
			sort, err := m.GetSort(constants[0])
			simplifyMust(t, err)
			name, err := m.Symbols.Intern("R")
			simplifyMust(t, err)
			decl, err := m.MkFuncDecl(name, []ast.ASTHandle{sort, sort}, boolean, ast.ASTFuncDeclInfo{})
			simplifyMust(t, err)
			simplifyPin(t, m, decl)
			pairs := [][2]int{{0, 1}}
			if cycleOnly {
				pairs = append(pairs, [2]int{1, 0}, [2]int{2, 2})
			}
			var disj []ast.ASTHandle
			for _, p := range pairs {
				h, e := m.MkApp(decl, []ast.ASTHandle{constants[p[0]], constants[p[1]]})
				simplifyMust(t, e)
				simplifyPin(t, m, h)
				disj = append(disj, h)
			}
			form, err := m.MkOrArgs(disj)
			simplifyMust(t, err)
			simplifyMust(t, goal.Assert(form))
			var out tactic.TacticResult
			t.Cleanup(func() { simplifyMust(t, out.Close()) })
			simplifyMust(t, r.Apply(goal, &out))
			size, err := goal.Size()
			simplifyMust(t, err)
			if size != 3 {
				t.Fatal("equal colors were mistaken for a symmetry", size)
			}
			if cycleOnly {
				saved := solveEquationFrame{manager: m}
				defer func() { simplifyMust(t, saved.Close()) }()
				normalized, e := symmetryNormalizeForTest(r, &saved, form)
				simplifyMust(t, e)
				ok, e := r.checkPermutation(normalized, constants, false)
				simplifyMust(t, e)
				if !ok {
					t.Fatal("test fixture not invariant under swap")
				}
				ok, e = r.checkPermutation(normalized, constants, true)
				simplifyMust(t, e)
				if ok {
					t.Fatal("cycle check omitted")
				}
			}
		})
	}
}

func TestSymmetryNormalizationSortsWithoutFlattening(t *testing.T) {
	m, c, g := simplifyFixture(t)
	r, err := NewSymmetryReduceTactic(m, c, g)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	a, b, d := propagateAtoms(t, m)
	inner, err := m.MkAnd(d, a)
	simplifyMust(t, err)
	simplifyPin(t, m, inner)
	outer, err := m.MkAnd(inner, b)
	simplifyMust(t, err)
	simplifyPin(t, m, outer)
	saved := solveEquationFrame{manager: m}
	defer func() { simplifyMust(t, saved.Close()) }()
	value, err := symmetryNormalizeForTest(r, &saved, outer)
	simplifyMust(t, err)
	n, err := m.GetNumArgs(value)
	simplifyMust(t, err)
	if n != 2 {
		t.Fatal("AC normalization flattened", n)
	}
	graph, err := r.symmetryGraph(value)
	simplifyMust(t, err)
	for _, h := range graph.apps {
		isAnd, e := r.symmetryOp(h, ast.ASTOpAnd)
		simplifyMust(t, e)
		if !isAnd {
			continue
		}
		args := graph.nodes[h].args
		for i := 1; i < len(args); i++ {
			before, e := m.GetASTID(args[i-1])
			simplifyMust(t, e)
			after, e := m.GetASTID(args[i])
			simplifyMust(t, e)
			if before > after {
				t.Fatal("arguments not sorted")
			}
		}
	}
}

func TestSymmetryNormalizationPreservesQuantifierAnnotations(t *testing.T) {
	m, c, g := simplifyFixture(t)
	r, err := NewSymmetryReduceTactic(m, c, g)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, r.Close()) })
	a, b, _ := propagateAtoms(t, m)
	boolean, err := m.MkBoolSort()
	simplifyMust(t, err)
	v, err := m.MkVar(0, boolean)
	simplifyMust(t, err)
	simplifyPin(t, m, v)
	body, err := m.MkAnd(b, a)
	simplifyMust(t, err)
	simplifyPin(t, m, body)
	unsorted, err := m.MkOr(b, a)
	simplifyMust(t, err)
	simplifyPin(t, m, unsorted)
	name, err := m.Symbols.Intern("trigger")
	simplifyMust(t, err)
	f, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	simplifyMust(t, err)
	simplifyPin(t, m, f)
	trigger, err := m.MkApp(f, []ast.ASTHandle{unsorted, v})
	simplifyMust(t, err)
	simplifyPin(t, m, trigger)
	pattern, err := m.MkPattern([]ast.ASTHandle{trigger})
	simplifyMust(t, err)
	simplifyPin(t, m, pattern)
	for _, positive := range []bool{true, false} {
		spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body, Weight: 1}
		if positive {
			spec.Patterns = []ast.ASTHandle{pattern}
		} else {
			spec.NoPatterns = []ast.ASTHandle{trigger}
		}
		q, err := m.MkQuantifier(spec)
		simplifyMust(t, err)
		simplifyPin(t, m, q)
		saved := solveEquationFrame{manager: m}
		defer func() { simplifyMust(t, saved.Close()) }()
		value, err := symmetryNormalizeForTest(r, &saved, q)
		simplifyMust(t, err)
		if positive {
			got, e := m.GetPattern(value, 0)
			simplifyMust(t, e)
			if got != pattern {
				t.Fatal("pattern was normalized")
			}
		} else {
			got, e := m.GetNoPattern(value, 0)
			simplifyMust(t, e)
			if got != trigger {
				t.Fatal("no-pattern was normalized")
			}
		}
		got, err := m.GetQuantifierBody(value)
		simplifyMust(t, err)
		if got == body {
			t.Fatal("quantifier body was not normalized")
		}
		graph, err := r.symmetryGraph(value)
		simplifyMust(t, err)
		want := uint32(0)
		if positive {
			want = 1
		}
		if graph.occurrences[trigger] != want || graph.occurrences[v] != 0 {
			t.Fatal("annotation traversal", graph.occurrences)
		}
		// Source max-depth intentionally visits only bodies. An annotation-only
		// application violates merge_colors' VERIFY(find), represented as a
		// checked Go domain error instead of consuming an uninitialized color.
		if _, err = r.candidatePermutations(value, graph); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("missing source depth precondition", err)
		}
	}
}

func TestSymmetryReduceGuardsDepthTranslationAndTrace(t *testing.T) {
	r, goal, _, _ := symmetryFixture(t, 3)
	m := r.manager
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("symmetry_reduce")
	simplifyMust(t, r.BindTrace(tags))
	var out tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, out.Close()) })
	simplifyMust(t, r.Apply(goal, &out))
	if tags.ShouldTrace("symmetry_reduce") {
		for _, text := range []string{"find_candidate_permutations", "Symmetric: ", "Support set: ", "Constants: ", "member predicate: "} {
			if !strings.Contains(trace.String(), text) {
				t.Fatal("missing source trace", text, trace.String())
			}
		}
	} else if trace.Len() != 0 {
		t.Fatal("erased trace wrote output")
	}
	refused := diagnostic.NewTraceTagsWithWriter(simplifyRefusedWriter{})
	refused.EnableAll(true)
	simplifyMust(t, r.BindTrace(refused))
	simplifyMust(t, r.Apply(goal, &out))
	copy := *r
	if err := copy.IncRef(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner", err)
	}
	for _, flags := range [][2]bool{{true, false}, {false, true}} {
		simplifyMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
		guarded, e := tactic.NewGoal(m, flags[0], false, flags[1])
		simplifyMust(t, e)
		if e = r.Apply(guarded, &out); e == nil {
			t.Fatal("accepted proof/core goal")
		}
		simplifyMust(t, guarded.Close())
	}
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, target.Close()) })
	translated, err := r.Translate(target)
	simplifyMust(t, err)
	simplifyMust(t, translated.IncRef())
	t.Cleanup(func() { simplifyMust(t, translated.DecRef()) })
	simplifyMust(t, r.Close())
	if err = r.IncRef(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed owner", err)
	}
	empty, err := tactic.NewGoal(target, false, false, false)
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, empty.Close()) })
	var translatedResult tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, translatedResult.Close()) })
	simplifyMust(t, translated.Apply(empty, &translatedResult))
	depth, err := empty.Depth()
	simplifyMust(t, err)
	if depth != 1 {
		t.Fatal("empty goal did not increment depth", depth)
	}
	falseTerm, err := target.MkFalse()
	simplifyMust(t, err)
	simplifyMust(t, empty.Assert(falseTerm))
	simplifyMust(t, translated.Apply(empty, &translatedResult))
	depth, err = empty.Depth()
	simplifyMust(t, err)
	if depth != 2 {
		t.Fatal("inconsistent goal did not increment depth", depth)
	}
}

// symmetryNormalizeForTest normalizes term and keeps the result in saved.
func symmetryNormalizeForTest(r *SymmetryReduceTactic, saved *solveEquationFrame, term ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := r.manager.NewRef(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { _ = r.manager.DelRef(&ref) }()
	if err = r.normalize(ref); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := r.manager.RefGet(ref)
	if err == nil {
		err = saved.pin(value)
	}
	return value, err
}
