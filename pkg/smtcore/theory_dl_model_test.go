package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type dlModelFixture struct {
	m    *ast.ASTManager
	s    *SMTInternalizerState
	dl   *SMTDatalogTheory
	bv   *SMTBitVectorTheory
	g    *SMTModelGenerator
	node *SMTEnode
	sort ast.ASTHandle
}

func newDLModelFixture(t *testing.T, size uint64) *dlModelFixture {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	family, err := m.RegisterDatalogPlugin()
	astTestMust(t, err)
	dl, err := NewSMTDatalogTheory(family)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(dl))
	bvFamily, err := m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	bv, err := NewSMTBitVectorTheory(bvFamily)
	astTestMust(t, err)
	astTestMust(t, s.RegisterTheory(bv))
	sort, err := m.MkDatalogFiniteSort(astTestSymbol(t, m, "native-model-domain"), size)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "native-model-element"), sort)
	astTestMust(t, err)
	node, err := s.MkEnode(x, false, false, true)
	astTestMust(t, err)
	_, err = dl.CreateVariable(node)
	astTestMust(t, err)
	g, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	astTestMust(t, g.SetContext(s.context))
	astTestMust(t, g.InitializeModel(cfg))
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	return &dlModelFixture{m, s, dl, bv, g, node, sort}
}
func (f *dlModelFixture) representative(t *testing.T) (ast.ASTHandle, SMTTheoryVar) {
	t.Helper()
	rep, err := f.dl.rep(f.node.owner)
	astTestMust(t, err)
	astTestMust(t, f.s.Internalize(rep, false))
	return rep, f.s.context.enodes.get(rep).TheoryVar(f.bv.FamilyID())
}
func (f *dlModelFixture) assign(t *testing.T, v SMTTheoryVar, value uint64) {
	t.Helper()
	for i, bit := range f.bv.bitLiterals[v] {
		actual := (value&(uint64(1)<<uint(i)) != 0) != bit.Sign()
		astTestMust(t, f.s.SetAssignment(bit.Var(), lbool.FromBool(actual)))
	}
}
func dlModelNumber(t *testing.T, f *dlModelFixture, p SMTModelValueProcedure, want uint64) ast.ASTHandle {
	t.Helper()
	value, err := p.MakeValue(f.g, nil)
	astTestMust(t, err)
	got, yes, err := f.m.DatalogNumeralValue(value)
	astTestMust(t, err)
	if !yes || got != want {
		t.Fatal("native finite-domain model value", got, want)
	}
	return value
}

func TestSMTDatalogNativeModelDeferredNoLateInternalization(t *testing.T) {
	f := newDLModelFixture(t, 8)
	before := f.m.Stats()
	f.m.Budget().FailAfter(0)
	p, err := f.dl.ModelValue(f.node, f.g)
	f.m.Budget().FailAfter(-1)
	astTestMust(t, err)
	if f.m.Stats() != before || len(f.dl.declarations) != 0 || len(f.bv.variables) != 0 || len(p.Dependencies()) != 0 || p.IsFresh() {
		t.Fatal("procedure creation evaluated the model")
	}
	dlModelNumber(t, f, p, 0)
	rep, err := f.dl.rep(f.node.owner)
	astTestMust(t, err)
	if f.s.context.enodes.get(rep) != nil || len(f.bv.variables) != 0 {
		t.Fatal("missing representative was internalized during MakeValue")
	}
	// The same procedure observes assignments created after its construction.
	_, v := f.representative(t)
	f.assign(t, v, 5)
	dlModelNumber(t, f, p, 5)
	f.assign(t, v, 2)
	dlModelNumber(t, f, p, 2)
}

func TestSMTDatalogNativeModelUnfixedSignedBitsAndRange(t *testing.T) {
	f := newDLModelFixture(t, 5)
	_, v := f.representative(t)
	p, err := f.dl.ModelValue(f.node, f.g)
	astTestMust(t, err)
	f.assign(t, v, 5)
	astTestMust(t, f.s.SetAssignment(f.bv.bitLiterals[v][1].Var(), lbool.Undef))
	dlModelNumber(t, f, p, 0) // high assigned bits must not form a partial value.
	f.assign(t, v, 3)
	// Change an actual native literal's polarity. The AST bit cache retains
	// its projection expression and cannot substitute for this authoritative word.
	f.bv.bitLiterals[v][0] = f.bv.bitLiterals[v][0].Negated()
	dlModelNumber(t, f, p, 2)
	f.assign(t, v, 7)
	if _, err = p.MakeValue(f.g, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("out-of-range source numeral was silently clamped", err)
	}
}

func TestSMTDatalogNativeModelMissingBVAssertionAndFallback(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, action := range []string{"T", "C"} {
			t.Run(fmt.Sprint(enabled, "/", action), func(t *testing.T) {
				d, _ := newLRASourceAssertionContext(t)
				report := bvDebugActionInput(t, d, strings.Repeat(action, 8))
				astTestMust(t, d.EnableAssertions(enabled))
				f := newDLModelFixture(t, 8)
				p, err := f.dl.ModelValue(f.node, f.g)
				astTestMust(t, err)
				delete(f.s.theories, f.bv.FamilyID())
				defer func() { f.s.theories[f.bv.FamilyID()] = f.bv }()
				value, err := p.MakeValue(f.g, nil)
				text, e := os.ReadFile(report)
				astTestMust(t, e)
				if diagnostic.DebugBuild && enabled && action == "T" {
					var ex z3exception.DefaultException
					if !errors.As(err, &ex) || !value.IsNull() || !strings.Contains(string(text), "File: src/smt/theory_dl.cpp\nLine: 90\nth_bv\n") {
						t.Fatal("source missing-BV assertion", err, string(text))
					}
				} else {
					astTestMust(t, err)
					n, yes, e := f.m.DatalogNumeralValue(value)
					astTestMust(t, e)
					if !yes || n != 0 {
						t.Fatal("missing BV source fallback", n)
					}
					if diagnostic.DebugBuild && enabled {
						if strings.Count(string(text), "ASSERTION VIOLATION") != 1 {
							t.Fatal("continue notification", string(text))
						}
					} else if len(text) != 0 {
						t.Fatal("erased/disabled assertion", string(text))
					}
				}
			})
		}
	}
}

func TestSMTDatalogNativeModelSignedConversionAndTrace(t *testing.T) {
	d, _ := newLRASourceAssertionContext(t)
	report := bvDebugActionInput(t, d, "CCCCCCCC")
	f := newDLModelFixture(t, uint64(1)<<63|3)
	_, v := f.representative(t)
	f.assign(t, v, uint64(1)<<63|1)
	p, err := f.dl.ModelValue(f.node, f.g)
	astTestMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("theory_dl")
	f.dl.trace = tags
	value := dlModelNumber(t, f, p, uint64(1)<<63|1)
	// A numeric result must remain live after its trace formatter finishes.
	root, err := f.m.NewRef(value)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&root)) }()
	want := "-------- [theory_dl] mk_value src/smt/theory_dl.cpp:99 ---------\n9223372036854775809\n------------------------------------------------\n"
	if tags.ShouldTrace("theory_dl") {
		if output.String() != want {
			t.Fatal("source deferred model trace", output.String(), want)
		}
	} else if output.Len() != 0 {
		t.Fatal("native model trace not erased")
	}
	text, err := os.ReadFile(report)
	astTestMust(t, err)
	if diagnostic.DebugBuild {
		if !strings.Contains(string(text), "src/util/mpq.h") || !strings.Contains(string(text), "is_int64(a)") {
			t.Fatal("source signed conversion assertions were bypassed", string(text))
		}
	} else if len(text) != 0 {
		t.Fatal("native signed conversion assertion was not erased", string(text))
	}
}

func TestSMTDatalogNativeModelGeneratorAndScopedProcedureLifetime(t *testing.T) {
	f := newDLModelFixture(t, 8)
	_, v := f.representative(t)
	f.assign(t, v, 6)
	astTestMust(t, f.s.context.SetRelevant(f.node.owner, true))
	astTestMust(t, f.g.BuildStateValues(f.s))
	value, err := f.g.Value(f.node)
	astTestMust(t, err)
	n, yes, err := f.m.DatalogNumeralValue(value)
	astTestMust(t, err)
	if !yes || n != 6 {
		t.Fatal("registered generator did not evaluate the native deferred procedure", n)
	}
	astTestMust(t, f.s.PushScope())
	x, err := f.m.MkConst(astTestSymbol(t, f.m, "scoped-model-element"), f.sort)
	astTestMust(t, err)
	node, err := f.s.MkEnode(x, false, false, true)
	astTestMust(t, err)
	_, err = f.dl.CreateVariable(node)
	astTestMust(t, err)
	p, err := f.dl.ModelValue(node, f.g)
	astTestMust(t, err)
	astTestMust(t, f.s.PopScopes(1))
	if _, err = p.MakeValue(f.g, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("retired borrowed enode was reused by a stale procedure", err)
	}
	// Generator pins its result independently of the later logical scope.
	n, yes, err = f.m.DatalogNumeralValue(value)
	astTestMust(t, err)
	if !yes || n != 6 {
		t.Fatal("generated model value lost its owner", n)
	}
}

func TestSMTDatalogNativeModelRefusalAndRetry(t *testing.T) {
	failures := 0
	for step := int64(0); step < 8; step++ {
		t.Run(fmt.Sprint(step), func(t *testing.T) {
			f := newDLModelFixture(t, 8)
			p, err := f.dl.ModelValue(f.node, f.g)
			astTestMust(t, err)
			// Fill existing reference storage without permitting a reserve,
			// stopping at the real lexical-root allocation frontier.
			var padding []ast.ASTRef
			t.Cleanup(func() {
				f.m.Budget().FailAfter(-1)
				for i := range padding {
					astTestMust(t, f.m.DelRef(&padding[i]))
				}
			})
			f.m.Budget().FailAfter(0)
			cold := false
			for i := 0; i < 8192; i++ {
				ref, e := f.m.NewRef(f.node.owner)
				if e != nil {
					cold = errors.Is(e, arena.ErrInjectedFailure)
					break
				}
				padding = append(padding, ref)
			}
			f.m.Budget().FailAfter(-1)
			if !cold {
				t.Fatal("did not reach a cold model lexical-root frontier")
			}
			beforeVariables := len(f.bv.variables)
			f.m.Budget().FailAfter(step)
			_, err = p.MakeValue(f.g, nil)
			f.m.Budget().FailAfter(-1)
			if err != nil {
				failures++
			}
			if len(f.bv.variables) != beforeVariables || len(f.bv.temporary) != 0 {
				t.Fatal("model allocation refusal internalized a late BV term")
			}
			dlModelNumber(t, f, p, 0)
		})
	}
	if failures == 0 {
		t.Fatal("model construction did not reach an injected refusal")
	}
}

func TestSMTDatalogNativeDeclarationWidthAndScopedMap(t *testing.T) {
	f := newDLModelFixture(t, 5)
	astTestMust(t, f.s.PushScope())
	first, err := f.dl.declarationsFor(f.sort)
	astTestMust(t, err)
	if first.bits != 64 {
		t.Fatal("source finite-domain representation width", first.bits)
	}
	// Independent owners keep both declarations inspectable after source pop;
	// shrinking the Go reference slice alone would hide a leaked AST retain.
	var held []ast.ASTRef
	var counts []uint32
	for _, declaration := range []ast.ASTHandle{first.rep, first.abs} {
		ref, err := f.m.NewRef(declaration)
		astTestMust(t, err)
		held = append(held, ref)
		count, err := f.m.GetRefCount(declaration)
		astTestMust(t, err)
		counts = append(counts, count)
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			astTestMust(t, f.m.DelRef(&held[i]))
		}
	}()
	before := len(f.dl.declarationRefs)
	astTestMust(t, f.s.PopScopes(1))
	for i, declaration := range []ast.ASTHandle{first.rep, first.abs} {
		count, err := f.m.GetRefCount(declaration)
		astTestMust(t, err)
		if count+1 != counts[i] {
			t.Fatal("scope pop did not release its declaration reference", i, count, counts[i])
		}
	}
	if len(f.dl.declarations) != 0 || len(f.dl.declarationTrail) != 0 || len(f.dl.declarationRefs) != 0 {
		t.Fatal("source declaration maps and their AST-reference trail did not retire")
	}
	second, err := f.dl.declarationsFor(f.sort)
	astTestMust(t, err)
	if second.bits != 64 || len(f.dl.declarationRefs) != before {
		t.Fatal("declaration recreation did not retain its new source trail references")
	}
	if _, err := f.m.GetKind(second.rep); err != nil {
		t.Fatal("recreated declaration", err)
	}
}

func TestSMTDatalogDeclarationMapsPrecedeTrailOwnership(t *testing.T) {
	reached := false
	// Factory scratch allocations differ between build profiles. Sweep until
	// the actual abstraction-trail reference allocation is refused.
	for stop := int64(0); stop < 32 && !reached; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newDLModelFixture(t, 5)
			bitVector, err := f.m.BitVectorSort(64)
			astTestMust(t, err)
			rep, err := f.m.MkDatalogRepDecl(f.sort, bitVector)
			astTestMust(t, err)
			abs, err := f.m.MkDatalogAbsDecl(bitVector, f.sort)
			astTestMust(t, err)
			astTestPin(t, f.m, rep)
			astTestPin(t, f.m, abs)
			astTestMust(t, f.s.PushScope())
			var occupied []ast.ASTRef
			t.Cleanup(func() {
				f.m.Budget().FailAfter(-1)
				for i := range occupied {
					astTestMust(t, f.m.DelRef(&occupied[i]))
				}
			})
			f.m.Budget().FailAfter(0)
			for len(occupied) < 8192 {
				ref, e := f.m.NewRef(ast.ASTHandle{})
				if errors.Is(e, arena.ErrInjectedFailure) {
					break
				}
				astTestMust(t, e)
				occupied = append(occupied, ref)
			}
			if len(occupied) == 8192 {
				t.Fatal("reference page frontier not reached")
			}
			i := len(occupied) - 1
			if i < 0 {
				t.Fatal("reference frontier had no occupied record")
			}
			astTestMust(t, f.m.DelRef(&occupied[i]))
			occupied = occupied[:i]
			f.m.Budget().FailAfter(stop)
			_, err = f.dl.declarationsFor(f.sort)
			f.m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatalf("missed refused trail frontier at %d: %v", stop, err)
			}
			if len(f.dl.declarationRefs) == 0 {
				return
			} // Earlier factory allocation refusal.
			if f.dl.representatives[f.sort] != rep || f.dl.abstractions[f.sort] != abs || len(f.dl.declarations) != 0 || len(f.dl.declarationTrail) != 0 || len(f.dl.declarationRefs) != 1 {
				t.Fatal("source declaration maps were made atomic with trail ownership")
			}
			reached = true
			// A retry must consult both published maps, even with no allocation budget.
			f.m.Budget().FailAfter(0)
			cached, err := f.dl.declarationsFor(f.sort)
			f.m.Budget().FailAfter(-1)
			astTestMust(t, err)
			if cached.rep != rep || cached.abs != abs || cached.bits != 64 || len(f.dl.declarationRefs) != 1 || len(f.dl.declarationTrail) != 0 {
				t.Fatal("cached partial publication changed source ownership")
			}
			astTestMust(t, f.s.PopScopes(1))
			if len(f.dl.declarationRefs) != 0 || len(f.dl.declarationTrail) != 0 || f.dl.representatives[f.sort] != rep || f.dl.abstractions[f.sort] != abs {
				t.Fatal("unpublished source map trails were incorrectly scoped")
			}
		})
	}
	if !reached {
		t.Fatal("abstraction trail refusal frontier not reached")
	}
}

func TestSMTDatalogNativeOrderBridgeTermContextAndRelevance(t *testing.T) {
	f := newDLModelFixture(t, 5)
	y, err := f.m.MkConst(astTestSymbol(t, f.m, "native-model-other-element"), f.sort)
	astTestMust(t, err)
	yn, err := f.s.MkEnode(y, false, false, true)
	astTestMust(t, err)
	_, err = f.dl.CreateVariable(yn)
	astTestMust(t, err)
	lt, err := f.m.MkDatalogLT(f.node.owner, y)
	astTestMust(t, err)
	astTestMust(t, f.s.Internalize(lt, true))
	left, err := f.dl.rep(f.node.owner)
	astTestMust(t, err)
	right, err := f.dl.rep(y)
	astTestMust(t, err)
	le, err := f.m.MkBitVectorULE(right, left)
	astTestMust(t, err)
	if f.s.context.enodes.get(lt) == nil || f.s.context.enodes.get(le) == nil || !f.s.relevancy.IsRelevant(lt) || !f.s.relevancy.IsRelevant(le) {
		t.Fatal("source order bridge lost term-context enodes or literal relevance")
	}
	// Relevant's inverse/range units must mark their Boolean atom before the
	// axiom; they are not guarded by an extra permanent once-only cache.
	astTestMust(t, f.dl.Relevant(f.node.owner))
	var units int
	for _, clause := range f.s.clauses {
		if !clause.Root && len(clause.Literals) == 1 {
			expression, err := f.s.BoolExpression(clause.Literals[0].Var())
			astTestMust(t, err)
			if !f.s.relevancy.IsRelevant(expression) {
				t.Fatal("constraint unit was not marked relevant")
			}
			units++
		}
	}
	// Unit clauses may be stored as assignments instead of clause objects.
	if units == 0 && len(f.s.assignedLiterals) == 0 {
		t.Fatal("inverse/range constraints were not published")
	}
}
