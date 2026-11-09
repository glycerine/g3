package rewriter

// Source: src/ast/fpa/fpa2bv_rewriter.cpp/.h.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fpa2BVConfiguration struct {
	DefaultConfiguration
	owner     *FPA2BVRewriter
	bindings  []ast.ASTHandle
	maxSteps  uint32
	maxMemory uint64
}

type FPA2BVRewriter struct {
	self      *FPA2BVRewriter
	manager   *ast.ASTManager
	converter *astfpa.FPA2BVConverter
	config    fpa2BVConfiguration
	traversal *Traversal
	header    uint64
	closed    bool
}

// NewFPA2BVRewriter borrows the manager and converter. The converter must
// outlive the rewriter. A nonnil partial result must close even on error.
func NewFPA2BVRewriter(m *ast.ASTManager, converter *astfpa.FPA2BVConverter) (owner *FPA2BVRewriter, result error) {
	if m == nil || converter == nil {
		return nil, ast.ErrASTHandle
	}
	bytes := uint64(reflect.TypeOf(FPA2BVRewriter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	owner = &FPA2BVRewriter{manager: m, converter: converter, header: bytes}
	owner.self = owner
	owner.config.owner = owner
	owner.config.defaults.manager = m
	owner.config.maxSteps = math.MaxUint32
	owner.config.maxMemory = math.MaxUint64
	traversal, err := NewTraversal(m, &owner.config)
	owner.traversal = traversal
	if err != nil {
		return owner, err
	}
	return owner, nil
}

func (r *FPA2BVRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.converter == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	_, err := r.manager.ProofsEnabled()
	return err
}

func (r *FPA2BVRewriter) SetLimits(maxSteps uint32, maxMemory uint64) error {
	if err := r.open(); err != nil {
		return err
	}
	r.config.maxSteps, r.config.maxMemory = maxSteps, maxMemory
	return nil
}

func (r *FPA2BVRewriter) SetHighUnspecified(value bool) error {
	if err := r.open(); err != nil {
		return err
	}
	r.converter.SetUnspecifiedFPHi(value)
	return nil
}

func (r *FPA2BVRewriter) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(root, out)
}

func (r *FPA2BVRewriter) RewriteWithProof(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.RewriteWithProof(root, out, proof)
}

func (r *FPA2BVRewriter) NumSteps() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.traversal.NumSteps()
}

func (r *FPA2BVRewriter) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	r.config.bindings = nil
	return r.traversal.Reset()
}

func (r *FPA2BVRewriter) Close() (result error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.config.bindings = nil
	r.traversal = nil
	r.closed = true
	return result
}

func (c *fpa2BVConfiguration) MaxStepsExceeded(steps uint32) (bool, error) {
	return steps > c.maxSteps || c.owner.manager.Budget().Used() > c.maxMemory, nil
}

func fpa2BVSlice(args ast.AppArguments) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, args.Count())
	for i := range result {
		value, err := args.At(uint32(i))
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

// ReduceApp is fpa2bv_rewriter_cfg::reduce_app, which dispatches to the
// converter.
func (c *fpa2BVConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, _ ast.ASTRef) (RewriteStatus, error) {
	values, err := fpa2BVSlice(args)
	if err != nil {
		return RewriteFailed, err
	}
	done, err := c.owner.converter.ReduceApp(decl, values, out)
	if err != nil || !done {
		return RewriteFailed, err
	}
	return RewriteDone, nil
}

func (c *fpa2BVConfiguration) PreVisit(term ast.ASTHandle) (bool, error) {
	kind, err := c.owner.manager.GetKind(term)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTQuantifier {
		return true, nil
	}
	count, err := c.owner.manager.GetNumDeclarations(term)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		sort, e := c.owner.manager.GetDeclarationSort(term, i)
		if e != nil {
			return false, e
		}
		c.bindings = append(c.bindings, sort)
	}
	return true, nil
}

func (c *fpa2BVConfiguration) ReduceVar(variable ast.ASTHandle, out, _ ast.ASTRef) (bool, error) {
	m := c.owner.manager
	index, err := m.GetVarIndex(variable)
	if err != nil {
		return false, err
	}
	if index >= uint32(len(c.bindings)) {
		return false, nil
	}
	sort, err := m.GetSort(variable)
	if err != nil {
		return false, err
	}
	ebits, sbits, floating, err := m.FloatingPointSortShape(sort)
	if err != nil {
		return false, err
	}
	if !floating {
		value, e := m.MkVar(index, sort)
		if e != nil {
			return false, e
		}
		return true, m.SetRef(out, value)
	}
	// expr_ref new_var; new_exp = mk_fp(extract(sgn), extract(exp),
	// extract(sig)): GCC builds the extracts right to left.
	width := ebits + sbits
	bvSort, err := m.BitVectorSort(width)
	if err != nil {
		return false, err
	}
	newVar, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { _ = m.DelRef(&newVar) }()
	packed, err := m.MkVar(index, bvSort)
	if err == nil {
		err = m.SetRef(newVar, packed)
	}
	if err != nil {
		return false, err
	}
	significand, err := m.MkBitVectorExtract(width-2, ebits, packed)
	if err != nil {
		return false, err
	}
	exponent, err := m.MkBitVectorExtract(ebits-1, 0, packed)
	if err != nil {
		return false, err
	}
	sign, err := m.MkBitVectorExtract(width-1, width-1, packed)
	if err != nil {
		return false, err
	}
	value, err := c.owner.converter.MkFP(sign, exponent, significand)
	if err != nil {
		return false, err
	}
	return true, m.SetRef(out, value)
}

func (c *fpa2BVConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, _ ast.ASTRef) (bool, error) {
	m := c.owner.manager
	count, err := m.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	if int(count) > len(c.bindings) {
		return false, ast.ErrASTDomain
	}
	oldSize := len(c.bindings) - int(count)
	defer func() { c.bindings = c.bindings[:oldSize] }()
	sorts := make([]ast.ASTHandle, count)
	names := make([]usymbol.Symbol, count)
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDeclarationSort(q, i)
		if e != nil {
			return false, e
		}
		name, e := m.GetDeclarationName(q, i)
		if e != nil {
			return false, e
		}
		ebits, sbits, floating, e := m.FloatingPointSortShape(sort)
		if e != nil {
			return false, e
		}
		if floating {
			sorts[i], e = m.BitVectorSort(ebits + sbits)
			if e != nil {
				return false, e
			}
			text, e := m.Symbols.Display(name)
			if e != nil {
				return false, e
			}
			names[i], e = m.Symbols.Intern(text + ".bv")
			if e != nil {
				return false, e
			}
		} else {
			sorts[i], names[i] = sort, name
		}
	}
	newPatterns, err := fpa2BVSlice(patterns)
	if err != nil {
		return false, err
	}
	newNoPatterns, err := fpa2BVSlice(noPatterns)
	if err != nil {
		return false, err
	}
	forall, err := m.IsForall(q)
	if err != nil {
		return false, err
	}
	weight, err := m.GetQuantifierWeight(q)
	if err != nil {
		return false, err
	}
	qid, skid, err := m.GetQuantifierIDs(q)
	if err != nil {
		return false, err
	}
	value, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: forall, Sorts: sorts, Names: names, Body: body,
		Weight: weight, QID: qid, SkolemID: skid, Patterns: newPatterns, NoPatterns: newNoPatterns,
	})
	if err != nil {
		return false, err
	}
	return true, m.SetRef(out, value)
}

func (c *fpa2BVConfiguration) Reset()   { c.bindings = nil }
func (c *fpa2BVConfiguration) Cleanup() { c.bindings = nil }

var _ Configuration = (*fpa2BVConfiguration)(nil)

// NewFPA2BVConverter constructs fpa2bv_converter(m) with its m_simp, a
// default-parameter bool_rewriter the converter owns.
func NewFPA2BVConverter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*astfpa.FPA2BVConverter, error) {
	simp, err := newBoolRewriter(m, c, g, nil)
	if err != nil {
		if simp != nil {
			err = errors.Join(err, simp.Close())
		}
		return nil, err
	}
	return astfpa.NewFPA2BVConverter(m, simp)
}
