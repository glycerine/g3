package smtcore

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtEqualityUndoFixture struct {
	m       *ast.ASTManager
	s       *SMTInternalizerState
	sort    ast.ASTHandle
	base    []ast.ASTHandle
	unary   ast.ASTHandle
	binary  ast.ASTHandle
	ternary ast.ASTHandle
}

func newSMTEqualityUndoFixture(t testing.TB, prefix string, constants int) *smtEqualityUndoFixture {
	t.Helper()
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	sort, err := m.MkSort(astTestSymbol(t, m, prefix+"-U"), ast.ASTSortInfo{})
	astTestMust(t, err)
	f := &smtEqualityUndoFixture{m: m, s: s, sort: sort}
	for i := 0; i < constants; i++ {
		value, err := m.MkConst(astTestSymbol(t, m, fmt.Sprintf("%s-c%d", prefix, i)), sort)
		astTestMust(t, err)
		astTestMust(t, s.Internalize(value, false))
		f.base = append(f.base, value)
	}
	declare := func(name string, arity int) ast.ASTHandle {
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = sort
		}
		decl, err := m.MkFuncDecl(astTestSymbol(t, m, prefix+name), domain, sort, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		return decl
	}
	f.unary, f.binary, f.ternary = declare("-f", 1), declare("-g", 2), declare("-h", 3)
	return f
}

func (f *smtEqualityUndoFixture) application(t testing.TB, r *rand.Rand) ast.ASTHandle {
	t.Helper()
	pick := func() ast.ASTHandle { return f.base[r.Intn(len(f.base))] }
	var result ast.ASTHandle
	var err error
	switch r.Intn(3) {
	case 0:
		result, err = f.m.MkApp(f.unary, []ast.ASTHandle{pick()})
	case 1:
		result, err = f.m.MkApp(f.binary, []ast.ASTHandle{pick(), pick()})
	default:
		result, err = f.m.MkApp(f.ternary, []ast.ASTHandle{pick(), pick(), pick()})
	}
	astTestMust(t, err)
	astTestMust(t, f.s.Internalize(result, false))
	return result
}

type smtEnodeImage struct {
	node        *SMTEnode
	root, next  *SMTEnode
	classSize   uint32
	parents     []*SMTEnode
	transTarget *SMTEnode
	justKind    SMTEqualityJustificationKind
}

func smtEqualityUndoImage(s *SMTInternalizerState) []smtEnodeImage {
	image := make([]smtEnodeImage, 0, len(s.enodeStack))
	for _, node := range s.enodeStack {
		image = append(image, smtEnodeImage{
			node: node, root: node.root, next: node.next, classSize: node.classSize,
			parents: slices.Clone(node.sourceParents), transTarget: node.trans.Target,
			justKind: node.trans.Justification.Kind(),
		})
	}
	return image
}

func requireSMTEqualityImage(t *testing.T, s *SMTInternalizerState, want []smtEnodeImage) {
	t.Helper()
	if len(s.enodeStack) != len(want) {
		t.Fatalf("%d enodes, want %d", len(s.enodeStack), len(want))
	}
	for i, saved := range want {
		node := s.enodeStack[i]
		switch {
		case node != saved.node:
			t.Fatalf("enode %d replaced", i)
		case node.root != saved.root || node.next != saved.next || node.classSize != saved.classSize:
			t.Fatalf("enode %d class differs after undo", i)
		case node.trans.Target != saved.transTarget || node.trans.Justification.Kind() != saved.justKind:
			t.Fatalf("enode %d proof tree differs after undo", i)
		case !slices.Equal(node.sourceParents, saved.parents):
			t.Fatalf("enode %d parents %d differ from saved %d", i, len(node.sourceParents), len(saved.parents))
		}
	}
}

// requireSMTCongruenceInvariant checks the table against a from-scratch reading
// of the current classes: exactly the representatives are stored, each under its
// current signature, and every other node points at a congruent representative.
func requireSMTCongruenceInvariant(t *testing.T, s *SMTInternalizerState) {
	t.Helper()
	astTestMust(t, s.congruence.CheckInvariant())
	stored := 0
	s.congruence.forEachEntry(func(_ *SMTEnode, current bool) {
		if !current {
			t.Fatal("table entry filed under a stale signature")
		}
		stored++
	})
	representatives := 0
	for _, node := range s.enodeStack {
		if len(node.arguments) == 0 || !node.congruenceEnabled || node.IsTrueEquality() {
			continue
		}
		if node.congruence == nil {
			t.Fatal("live enode has no congruence representative")
		}
		congruent, _, err := SMTEnodesCongruent(node, node.congruence)
		astTestMust(t, err)
		if !congruent {
			t.Fatal("enode's representative is no longer congruent")
		}
		found, _, err := s.congruence.Find(node)
		astTestMust(t, err)
		// As in the source, a nonrepresentative may point at a former
		// representative while it stays congruent; lookups find the current one.
		if found == nil || found.congruence != found || node.congruence == node && found != node {
			t.Fatalf("Find returned %p for %p, representative %p", found, node, node.congruence)
		}
		if node.congruence == node {
			representatives++
		}
	}
	if stored != representatives {
		t.Fatalf("table holds %d entries for %d representatives", stored, representatives)
	}
}

// Random nested scopes must undo exactly what full rebuilds used to recompute.
func TestEqualityUndoRestoresClassesAndCongruence(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			f := newSMTEqualityUndoFixture(t, fmt.Sprintf("undo-%d", seed), 8)
			s := f.s
			for i := 0; i < 8; i++ {
				f.application(t, r)
			}
			var saved [][]smtEnodeImage
			for step := 0; step < 120; step++ {
				switch action := r.Intn(10); {
				case action < 3 || len(saved) == 0:
					saved = append(saved, smtEqualityUndoImage(s))
					astTestMust(t, s.PushScope())
				case action < 7:
					left, right := f.base[r.Intn(len(f.base))], f.base[r.Intn(len(f.base))]
					astTestMust(t, s.addEquality(s.context.enodes.get(left), s.context.enodes.get(right), NewSMTEqualityAxiom()))
					astTestMust(t, s.propagateCoreEqualities())
				case action < 8:
					f.application(t, r)
					astTestMust(t, s.propagateCoreEqualities())
				default:
					count := 1 + r.Intn(len(saved))
					astTestMust(t, s.PopScopes(uint32(count)))
					target := saved[len(saved)-count]
					saved = saved[:len(saved)-count]
					requireSMTEqualityImage(t, s, target)
				}
				requireSMTCongruenceInvariant(t, s)
			}
			if len(saved) != 0 {
				astTestMust(t, s.PopScopes(uint32(len(saved))))
				requireSMTEqualityImage(t, s, saved[0])
				requireSMTCongruenceInvariant(t, s)
			}
		})
	}
}

// A merge inside a scope creates a congruence that the undo must take apart.
func TestEqualityUndoSeparatesCongruentApplications(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "undo-cg", 2)
	s := f.s
	first, err := f.m.MkApp(f.unary, []ast.ASTHandle{f.base[0]})
	astTestMust(t, err)
	second, err := f.m.MkApp(f.unary, []ast.ASTHandle{f.base[1]})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(first, false))
	astTestMust(t, s.Internalize(second, false))
	saved := smtEqualityUndoImage(s)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1]), NewSMTEqualityAxiom()))
	astTestMust(t, s.propagateCoreEqualities())
	if s.context.enodes.get(first).root != s.context.enodes.get(second).root {
		t.Fatal("merge did not propagate congruence")
	}
	astTestMust(t, s.PopScopes(1))
	if s.context.enodes.get(first).root == s.context.enodes.get(second).root {
		t.Fatal("undo left applications merged")
	}
	requireSMTEqualityImage(t, s, saved)
	requireSMTCongruenceInvariant(t, s)
}

// A node created after a merge in the same scope is cut from the target's parent
// vector by the undo, and deletion must still succeed and leave no table entry.
func TestEqualityUndoRetiresNodesCreatedAfterMerge(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "undo-late", 3)
	s := f.s
	saved := smtEqualityUndoImage(s)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1]), NewSMTEqualityAxiom()))
	late, err := f.m.MkApp(f.binary, []ast.ASTHandle{f.base[0], f.base[2]})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(late, false))
	astTestMust(t, s.PopScopes(1))
	if s.context.enodes.get(late) != nil {
		t.Fatal("late enode survived its scope")
	}
	requireSMTEqualityImage(t, s, saved)
	requireSMTCongruenceInvariant(t, s)
}

// A pop refused after a stage must finish from that stage. Finishing stage 1
// twice would swap the class links back together.
func TestEqualityUndoResumesFromRecordedStage(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "undo-stage", 2)
	s := f.s
	application, err := f.m.MkApp(f.unary, []ast.ASTHandle{f.base[0]})
	astTestMust(t, err)
	astTestMust(t, s.Internalize(application, false))
	saved := smtEqualityUndoImage(s)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.addEquality(s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1]), NewSMTEqualityAxiom()))
	entry := &s.equalityTrail[len(s.equalityTrail)-1]
	astTestMust(t, s.undoEquality(entry))
	if entry.stage != 4 {
		t.Fatalf("stage %d after a complete undo", entry.stage)
	}
	// The retry finds every stage done.
	astTestMust(t, s.undoEquality(entry))
	astTestMust(t, s.PopScopes(1))
	requireSMTEqualityImage(t, s, saved)
	requireSMTCongruenceInvariant(t, s)
}

// An entry recorded before the merge finished restores only theory variables.
func TestEqualityUndoSkipsUnmergedEntry(t *testing.T) {
	f := newSMTEqualityUndoFixture(t, "undo-unmerged", 2)
	s := f.s
	left, right := s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1])
	saved := smtEqualityUndoImage(s)
	astTestMust(t, s.PushScope())
	s.recordEqualityUndo(left, left, right)
	astTestMust(t, s.PopScopes(1))
	requireSMTEqualityImage(t, s, saved)
}

// One merge must record a fixed-size trail entry, without per-class allocation.
func TestEqualityUndoAllocationsDoNotScaleWithClassSize(t *testing.T) {
	measure := func(classSize int) float64 {
		f := newSMTEqualityUndoFixture(t, fmt.Sprintf("undo-allocs-%d", classSize), classSize+2)
		s := f.s
		for i := 1; i < classSize; i++ {
			astTestMust(t, s.addEquality(s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[i]), NewSMTEqualityAxiom()))
		}
		small := s.context.enodes.get(f.base[classSize])
		large := s.context.enodes.get(f.base[0])
		return testing.AllocsPerRun(50, func() {
			astTestMust(t, s.PushScope())
			astTestMust(t, s.addEquality(large, small, NewSMTEqualityAxiom()))
			astTestMust(t, s.PopScopes(1))
		})
	}
	smallClass, largeClass := measure(4), measure(400)
	if largeClass > smallClass+2 {
		t.Fatalf("allocations grew from %v to %v with class size", smallClass, largeClass)
	}
}

func BenchmarkSMTMergeAndUndo(b *testing.B) {
	for _, unrelated := range []int{0, 2000} {
		b.Run(fmt.Sprintf("unrelated=%d", unrelated), func(b *testing.B) {
			f := newSMTEqualityUndoFixture(b, "bench-undo", unrelated+2)
			s := f.s
			for _, value := range f.base {
				application, err := f.m.MkApp(f.unary, []ast.ASTHandle{value})
				astTestMust(b, err)
				astTestMust(b, s.Internalize(application, false))
			}
			left, right := s.context.enodes.get(f.base[0]), s.context.enodes.get(f.base[1])
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				astTestMust(b, s.PushScope())
				astTestMust(b, s.addEquality(left, right, NewSMTEqualityAxiom()))
				astTestMust(b, s.propagateCoreEqualities())
				astTestMust(b, s.PopScopes(1))
			}
		})
	}
}
