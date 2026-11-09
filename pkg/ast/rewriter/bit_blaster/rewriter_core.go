package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_rewriter.h:22-41;
// src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:69-210,625-684.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	parent "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type RewriterParams struct {
	MaxSteps    uint32
	MaxMemoryMB uint32
	BlastMul    bool
	BlastAdd    bool
	BlastQuant  bool
	BlastFull   bool
}

func DefaultRewriterParams() RewriterParams {
	return RewriterParams{MaxSteps: ^uint32(0), MaxMemoryMB: ^uint32(0), BlastMul: true, BlastAdd: true}
}

type BitBlasterRewriter struct {
	self      *BitBlasterRewriter
	manager   *ast.ASTManager
	family    ast.FamilyID
	blaster   *Engine
	params    RewriterParams
	config    bitBlasterTraversalConfiguration
	traversal *parent.Traversal
	// cache is m_const2bits (obj_map<func_decl, expr*>, holding no
	// references); keyRoots and values are m_keys and m_values.
	cache    *ast.ObjMap
	keys     []ast.ASTHandle
	keyRoots []ast.ASTRef
	values   []ast.ASTRef
	limits   []int
	bindings []ast.ASTRef
	// in1, in2 and outBits are blaster_rewriter_cfg's m_in1, m_in2 and
	// m_out: each operation resets them when it starts, so an operation's
	// bits stay alive until the next one (or the rewriter's destruction).
	in1, in2, outBits []ast.ASTRef
	closed            bool
	trace             *diagnostic.TraceTags
}

func NewBitBlasterRewriter(m *ast.ASTManager, params *RewriterParams) (*BitBlasterRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	name, err := m.Symbols.Intern("bv")
	if err != nil {
		return nil, err
	}
	family, err := m.FamilyIDForSymbol(name)
	if err != nil {
		return nil, err
	}
	config, err := newRewriterConfig(m, family)
	if err != nil {
		return nil, err
	}
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}
	actual := DefaultRewriterParams()
	if params != nil {
		actual = *params
	}
	if err = engine.SetMaxMemoryMB(actual.MaxMemoryMB); err != nil {
		return nil, err
	}
	r := &BitBlasterRewriter{
		manager: m,
		family:  family,
		blaster: engine,
		params:  actual,
		cache:   ast.NewObjMap(m),
	}
	r.self, r.config.owner = r, r
	proofs, err := m.ProofsEnabled()
	if err == nil && proofs {
		r.traversal, err = parent.NewProofTraversal(m, &r.config)
	} else if err == nil {
		r.traversal, err = parent.NewTraversal(m, &r.config)
	}
	if err != nil {
		if r.traversal != nil {
			err = errors.Join(err, r.traversal.Close())
		}
		return r, err
	}
	return r, nil
}

func (r *BitBlasterRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.blaster == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (r *BitBlasterRewriter) UpdateParams(params RewriterParams) error {
	if err := r.open(); err != nil {
		return err
	}
	r.params = params
	return r.blaster.SetMaxMemoryMB(params.MaxMemoryMB)
}

func (r *BitBlasterRewriter) BindCooperation(owner *cooperate.Cooperation) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.blaster.BindCooperation(owner)
}

func (r *BitBlasterRewriter) Manager() (*ast.ASTManager, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.manager, nil
}

func (r *BitBlasterRewriter) Push() error {
	if err := r.open(); err != nil {
		return err
	}
	r.limits = append(r.limits, len(r.keys))
	return nil
}

func (r *BitBlasterRewriter) NumScopes() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return uint32(len(r.limits)), nil
}

func (r *BitBlasterRewriter) Pop(scopes uint32) (result error) {
	if err := r.open(); err != nil {
		return err
	}
	if uint64(scopes) > uint64(len(r.limits)) {
		return ast.ErrASTDomain
	}
	if scopes == 0 {
		return nil
	}
	newScopeCount := len(r.limits) - int(scopes)
	limit := r.limits[newScopeCount]
	if err := r.releaseCache(limit); err != nil {
		return err
	}
	r.limits = r.limits[:newScopeCount]
	return result
}

// Const2Bits is const2bits(): the declarations and their bits, in the
// obj_map's iteration order.
func (r *BitBlasterRewriter) Const2Bits() ([]ast.ObjMapEntry, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return r.cache.Entries(), nil
}

// FindConst2Bits is const2bits().find(d).
func (r *BitBlasterRewriter) FindConst2Bits(d ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	value, found, err := r.cache.Find(d)
	return value[0], found, err
}

func (r *BitBlasterRewriter) getBits(term ast.ASTHandle) (bits []ast.ASTRef, result error) {
	assembled, err := r.manager.IsBitVectorOperation(term, 56)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(r.manager, bits))
			bits = nil
		}
	}()
	if assembled {
		count, err := r.manager.GetNumArgs(term)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < count; i++ {
			bit, err := r.manager.GetArg(term, i)
			if err != nil {
				return nil, err
			}
			ref, err := r.manager.NewRef(bit)
			if err != nil {
				return nil, err
			}
			bits = append(bits, ref)
		}
		return bits, nil
	}
	width, err := r.manager.BitVectorSize(term)
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < width; i++ {
		bit, err := r.manager.MkBitVectorBit2Bool(i, term)
		if err != nil {
			return nil, err
		}
		ref, err := r.manager.NewRef(bit)
		if err != nil {
			return nil, err
		}
		bits = append(bits, ref)
	}
	return bits, nil
}

func (r *BitBlasterRewriter) setBitVector(bits []ast.ASTRef, out ast.ASTRef) error {
	handles, err := r.blaster.handles(bits)
	if err != nil {
		return err
	}
	assembled, err := r.manager.MkBitVectorFromBits(handles)
	if err != nil {
		return err
	}
	return r.manager.SetRef(out, assembled)
}

func (r *BitBlasterRewriter) makeConstant(decl ast.ASTHandle, out ast.ASTRef) (result error) {
	if cached, ok, err := r.cache.Find(decl); err != nil {
		return err
	} else if ok {
		return r.manager.SetRef(out, cached[0])
	}
	rangeSort, err := r.manager.GetRange(decl)
	if err != nil {
		return err
	}
	width, err := r.manager.BitVectorSize(rangeSort)
	if err != nil {
		return err
	}
	boolSort, err := r.manager.MkBoolSort()
	if err != nil {
		return err
	}
	// m_out.reset(); push fresh constants; r = mk_mkbv(m_out)
	if err = r.resetBits(&r.outBits); err != nil {
		return err
	}
	for i := uint32(0); i < width; i++ {
		bit, err := r.manager.MkFreshConstNullable(nil, boolSort)
		if err != nil {
			return err
		}
		ref, err := r.manager.NewRef(bit)
		if err != nil {
			return err
		}
		r.outBits = append(r.outBits, ref)
	}
	handles, err := r.blaster.handles(r.outBits)
	if err != nil {
		return err
	}
	assembled, err := r.manager.MkBitVectorFromBits(handles)
	if err != nil {
		return err
	}
	owned, err := r.manager.NewRef(assembled)
	if err != nil {
		return err
	}
	if err = r.manager.SetRef(out, assembled); err != nil {
		return errors.Join(err, r.manager.DelRef(&owned))
	}
	keyRoot, err := r.manager.NewRef(decl)
	if err != nil {
		return errors.Join(err, r.manager.DelRef(&owned))
	}
	if err = r.cache.Insert(decl, [2]ast.ASTHandle{assembled}); err != nil {
		return errors.Join(err, r.manager.DelRef(&owned), r.manager.DelRef(&keyRoot))
	}
	r.keys = append(r.keys, decl)
	r.keyRoots = append(r.keyRoots, keyRoot)
	r.values = append(r.values, owned)
	return nil
}

// releaseCache is pop's body: the newest keys leave m_const2bits, then
// m_keys.resize(lim) and m_values.resize(lim) release forward.
func (r *BitBlasterRewriter) releaseCache(limit int) error {
	for i := len(r.keys); i > limit; {
		i--
		if err := r.cache.Erase(r.keys[i]); err != nil {
			return err
		}
	}
	r.keys = r.keys[:limit]
	if err := releaseRefsForward(r.manager, &r.keyRoots, limit); err != nil {
		return err
	}
	return releaseRefsForward(r.manager, &r.values, limit)
}

// releaseRefsForward is ref_vector shrink(limit): dec_ref from limit up.
func releaseRefsForward(m *ast.ASTManager, refs *[]ast.ASTRef, limit int) error {
	for i := limit; i < len(*refs); i++ {
		if (*refs)[i] != (ast.ASTRef{}) {
			if err := m.DelRef(&(*refs)[i]); err != nil {
				*refs = append((*refs)[:limit], (*refs)[i:]...)
				return err
			}
		}
	}
	*refs = (*refs)[:limit]
	return nil
}

func (r *BitBlasterRewriter) Close() (result error) {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	// ~imp: m_cfg's members in reverse declaration order (m_values,
	// m_keys, m_bindings, m_out, m_in2, m_in1), then the rewriter_tpl base.
	if err := releaseRefsForward(r.manager, &r.values, 0); err != nil {
		return err
	}
	if err := releaseRefsForward(r.manager, &r.keyRoots, 0); err != nil {
		return err
	}
	r.keys = nil
	if err := r.releaseBindings(0); err != nil {
		return err
	}
	if err := errors.Join(r.resetBits(&r.outBits), r.resetBits(&r.in2), r.resetBits(&r.in1)); err != nil {
		return err
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	r.cache = nil
	r.keys = nil
	r.limits = nil
	r.bindings = nil
	r.closed = true
	return result
}
