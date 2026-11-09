package bit_blaster

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	parent "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestBitBlasterRewriterProofOutputAndConstantKeyLifetime(t *testing.T) {
	m := gateManager(t)
	paramsMust(t, m.RegisterDeclarationPlugins())
	paramsMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	r, err := NewBitBlasterRewriter(m, nil)
	paramsMust(t, err)
	t.Cleanup(func() { paramsMust(t, r.Close()) })
	sort, err := m.BitVectorSort(4)
	paramsMust(t, err)
	name, err := m.Symbols.Intern("proof-word")
	paramsMust(t, err)
	x, err := m.MkConst(name, sort)
	paramsMust(t, err)
	root, err := m.NewRef(x)
	paramsMust(t, err)
	t.Cleanup(func() {
		if root != (ast.ASTRef{}) {
			paramsMust(t, m.DelRef(&root))
		}
	})
	decl, err := m.GetDecl(x)
	paramsMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&proof)) }()
	paramsMust(t, r.RewriteWithProof(x, out, proof))
	pr, err := m.RefGet(proof)
	paramsMust(t, err)
	if pr.IsNull() {
		t.Fatal("missing proof")
	}
	fact, err := m.GetProofFact(pr)
	paramsMust(t, err)
	left, err := m.GetArg(fact, 0)
	paramsMust(t, err)
	right, err := m.GetArg(fact, 1)
	paramsMust(t, err)
	value, err := m.RefGet(out)
	paramsMust(t, err)
	if left != x || right != value {
		t.Fatal("rewrite proof relation", left, right, x, value)
	}
	paramsMust(t, m.ResetRef(proof))
	paramsMust(t, r.Cleanup())
	paramsMust(t, m.DelRef(&root))
	if _, err := m.GetKind(decl); err != nil {
		t.Fatal("constant cache lost original declaration", err)
	}
	got, _, err := r.FindConst2Bits(decl)
	paramsMust(t, err)
	if got != value {
		t.Fatal("cleanup lost constant mapping")
	}
}

func TestBitBlasterSourceProcessMemoryAndCooperation(t *testing.T) {
	m, r := rewriterFixture(t)
	owner, err := cooperate.NewCooperation(m.Budget())
	paramsMust(t, err)
	defer func() { paramsMust(t, owner.Close()) }()
	var trace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&trace)
	tags.Enable("cooperate_detail")
	paramsMust(t, owner.BindTrace(tags))
	paramsMust(t, owner.BeginSection())
	defer func() { paramsMust(t, owner.EndSection()) }()
	paramsMust(t, r.BindCooperation(owner))
	runtime := m.NumeralService().RuntimeOwner()
	used, err := runtime.AllocationSize()
	paramsMust(t, err)
	limit := uint32((used+(1<<20)-1)/(1<<20) + 1)
	p := DefaultRewriterParams()
	p.MaxMemoryMB = limit
	paramsMust(t, r.UpdateParams(p))
	// Arena bookkeeping is outside the source process allocation counter.
	charged := uint64(limit+1) << 20
	paramsMust(t, m.Budget().Reserve(charged))
	defer m.Budget().Release(charged)
	if exceeded, err := r.config.MaxStepsExceeded(0); err != nil || exceeded {
		t.Fatal("arena usage mistaken for process memory", exceeded, err)
	}
	if tags.ShouldTrace("cooperate_detail") && !strings.Contains(trace.String(), "bit blaster, tid: 0\n") {
		t.Fatal("rewriter cooperation checkpoint", trace.String())
	}
	block, err := runtime.Allocate(uint64(limit) << 20)
	paramsMust(t, err)
	blockHeld := true
	defer func() {
		if blockHeld {
			paramsMust(t, runtime.Deallocate(&block))
		}
	}()
	trace.Reset()
	_, err = r.config.MaxStepsExceeded(0)
	if err == nil || err.Error() != messages.MaxMemory || !parent.IsRewriterException(err) {
		t.Fatal("rewriter source memory exception", err)
	}
	if tags.ShouldTrace("cooperate_detail") && trace.Len() == 0 {
		t.Fatal("rewriter memory check preceded cooperation")
	}
	trace.Reset()
	err = r.blaster.checkpoint()
	if err == nil || err.Error() != messages.MaxMemory || !parent.IsRewriterException(err) {
		t.Fatal("template source memory exception", err)
	}
	if trace.Len() != 0 {
		t.Fatal("template must check memory before cooperation", trace.String())
	}
	paramsMust(t, runtime.Deallocate(&block))
	blockHeld = false
	paramsMust(t, r.blaster.checkpoint())
	if tags.ShouldTrace("cooperate_detail") && !strings.Contains(trace.String(), "bit-blaster, tid: 0\n") {
		t.Fatal("template cooperation checkpoint", trace.String())
	}
}
