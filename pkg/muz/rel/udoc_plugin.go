package rel

// Source: src/muz/rel/udoc_relation.h/.cpp udoc_relation and udoc_plugin,
// ported statement by statement on the doc.h/doc.cpp/tbv.cpp port
// (DocManager, DocUnion, TBVManager). The union_bvec merge members used by
// the plugin are udocMerge*.

import (
	"errors"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/backtrack"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// UDocPlugin is udoc_plugin.
type UDocPlugin struct {
	RelationPluginDefaults
	rm *RelationManager
	m  *ast.ASTManager
	// dms is m_dms: one doc_manager per signature bit width.
	dms    map[uint32]*DocManager
	closed bool
}

var _ RelationPlugin = (*UDocPlugin)(nil)

// NewUDocPlugin is udoc_plugin::udoc_plugin(rm).
func NewUDocPlugin(rm *RelationManager) (*UDocPlugin, error) {
	if err := rm.open(); err != nil {
		return nil, err
	}
	return &UDocPlugin{rm: rm, m: rm.manager, dms: make(map[uint32]*DocManager)}, nil
}

func (*UDocPlugin) Name() string                { return "doc" }
func (p *UDocPlugin) Manager() *RelationManager { return p.rm }

// Close is ~udoc_plugin: the doc managers are deallocated.
func (p *UDocPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	for _, dm := range p.dms {
		dm.Close()
	}
	p.dms = nil
	p.closed = true
	return nil
}

// dm is udoc_plugin::dm(unsigned n).
func (p *UDocPlugin) dm(n uint32) (*DocManager, error) {
	if p.closed {
		return nil, arena.ErrClosed
	}
	if r := p.dms[n]; r != nil {
		return r, nil
	}
	r, err := NewDocManager(n, arena.VectorConfig{})
	if err != nil {
		return nil, err
	}
	p.dms[n] = r
	return r, nil
}

// dmFor is udoc_plugin::dm(relation_signature const&).
func (p *UDocPlugin) dmFor(sorts []ast.ASTHandle) (*DocManager, error) {
	n, err := p.numSignatureBits(sorts)
	if err != nil {
		return nil, err
	}
	return p.dm(n)
}

// numSortBits is udoc_plugin::num_sort_bits(sort*).
func (p *UDocPlugin) numSortBits(sort ast.ASTHandle) (uint32, error) {
	isBV, err := p.m.IsBitVectorSort(sort)
	if err != nil {
		return 0, err
	}
	if isBV {
		return p.m.BitVectorSize(sort)
	}
	boolean, err := p.m.MkBoolSort()
	if err != nil {
		return 0, err
	}
	if sort == boolean {
		return 1, nil
	}
	sz, ok, err := p.m.DatalogFiniteSortSize(sort)
	if err != nil {
		return 0, err
	}
	if ok {
		var numBits uint32
		for sz > 0 {
			numBits++
			sz /= 2
		}
		return numBits, nil
	}
	// UNREACHABLE()
	return 0, ErrDOC
}

// numExprBits is num_sort_bits(expr*): the bits of get_sort(e).
func (p *UDocPlugin) numExprBits(e ast.ASTHandle) (uint32, error) {
	sort, err := p.m.GetSort(e)
	if err != nil {
		return 0, err
	}
	return p.numSortBits(sort)
}

// numSignatureBits is udoc_plugin::num_signature_bits.
func (p *UDocPlugin) numSignatureBits(sorts []ast.ASTHandle) (uint32, error) {
	var result uint32
	for _, sort := range sorts {
		n, err := p.numSortBits(sort)
		if err != nil {
			return 0, err
		}
		result += n
	}
	return result, nil
}

// isFiniteSort is udoc_plugin::is_finite_sort: bv.is_bv_sort(s) ||
// dl.is_finite_sort(s).
func (p *UDocPlugin) isFiniteSort(sort ast.ASTHandle) (bool, error) {
	isBV, err := p.m.IsBitVectorSort(sort)
	if err != nil || isBV {
		return isBV, err
	}
	return p.m.IsDatalogFiniteSort(sort)
}

// CanHandleSignature is udoc_plugin::can_handle_signature.
func (p *UDocPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	for _, sort := range sorts {
		ok, err := p.isFiniteSort(sort)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// isBV is bv.is_bv(e).
func (p *UDocPlugin) isBV(e ast.ASTHandle) (bool, error) {
	sort, err := p.m.GetSort(e)
	if err != nil {
		return false, err
	}
	return p.m.IsBitVectorSort(sort)
}

// isNumeral is udoc_plugin::is_numeral.
func (p *UDocPlugin) isNumeral(e ast.ASTHandle) (*big.Int, uint32, bool, error) {
	if value, width, ok, err := p.m.BitVectorNumeralBigInt(e); err != nil || ok {
		return value, width, ok, err
	}
	if p.m.IsTrue(e) {
		return big.NewInt(1), 1, true, nil
	}
	if p.m.IsFalse(e) {
		return big.NewInt(0), 1, true, nil
	}
	n, ok, err := p.m.DatalogNumeralValue(e)
	if err != nil || !ok {
		return nil, 0, false, err
	}
	sort, err := p.m.GetSort(e)
	if err != nil {
		return nil, 0, false, err
	}
	sz, ok, err := p.m.DatalogFiniteSortSize(sort)
	if err != nil || !ok {
		return nil, 0, false, err
	}
	var numBits uint32
	for sz > 0 {
		numBits++
		sz /= 2
	}
	return new(big.Int).SetUint64(n), numBits, true, nil
}

// mkNumeral is udoc_plugin::mk_numeral.
func (p *UDocPlugin) mkNumeral(r *big.Int, sort ast.ASTHandle) (ast.ASTHandle, error) {
	isBV, err := p.m.IsBitVectorSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if isBV {
		width, err := p.m.BitVectorSize(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return p.m.BitVectorNumeralRat(new(big.Rat).SetInt(r), width)
	}
	boolean, err := p.m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort == boolean {
		if r.Sign() == 0 {
			return p.m.MkFalse()
		}
		return p.m.MkTrue()
	}
	return p.m.MkDatalogNumeral(r.Uint64(), sort)
}

// MkEmpty is udoc_plugin::mk_empty.
func (p *UDocPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	return p.newRelation(sorts)
}

// MkFull is udoc_plugin::mk_full.
func (p *UDocPlugin) MkFull(_ ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	r, err := p.newRelation(sorts)
	if err != nil {
		return nil, err
	}
	d, err := r.dm.AllocateX()
	if err == nil {
		err = r.elems.PushOwned(d)
	}
	if err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

// UDocPluginRelation is udoc_relation.
type UDocPluginRelation struct {
	plugin *UDocPlugin
	sorts  []ast.ASTHandle
	dm     *DocManager
	elems  *DocUnion
	// columnInfo is m_column_info: column i occupies bits
	// [columnInfo[i], columnInfo[i+1]).
	columnInfo []uint32
	closed     bool
}

var _ RelationBase = (*UDocPluginRelation)(nil)

// newRelation is udoc_relation::udoc_relation(p, sig).
func (p *UDocPlugin) newRelation(sorts []ast.ASTHandle) (*UDocPluginRelation, error) {
	dm, err := p.dmFor(sorts)
	if err != nil {
		return nil, err
	}
	elems, err := NewDocUnion(dm)
	if err != nil {
		return nil, err
	}
	r := &UDocPluginRelation{plugin: p, sorts: append([]ast.ASTHandle(nil), sorts...), dm: dm, elems: elems}
	var column uint32
	for _, sort := range sorts {
		r.columnInfo = append(r.columnInfo, column)
		n, err := p.numSortBits(sort)
		if err != nil {
			return nil, err
		}
		column += n
	}
	r.columnInfo = append(r.columnInfo, column)
	return r, nil
}

func udocGet(r RelationBase) (*UDocPluginRelation, bool) {
	u, ok := r.(*UDocPluginRelation)
	return u, ok
}

// checkKind is relation_plugin::check_kind.
func (p *UDocPlugin) checkKind(r RelationBase) bool {
	u, ok := udocGet(r)
	return ok && u.plugin == p
}

func (r *UDocPluginRelation) open() error {
	if r == nil || r.closed || r.plugin == nil || r.plugin.closed {
		return arena.ErrClosed
	}
	return nil
}

func (r *UDocPluginRelation) numCols() uint32             { return uint32(len(r.sorts)) }
func (r *UDocPluginRelation) numBits() uint32             { return r.columnInfo[len(r.columnInfo)-1] }
func (r *UDocPluginRelation) columnIdx(col uint32) uint32 { return r.columnInfo[col] }
func (r *UDocPluginRelation) columnNumBits(col uint32) uint32 {
	return r.columnInfo[col+1] - r.columnInfo[col]
}

func (r *UDocPluginRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *UDocPluginRelation) RelationPlugin() RelationPlugin     { return r.plugin }

// Signature is the table signature of the column sorts (FiniteTable).
func (r *UDocPluginRelation) Signature() (Signature, error) {
	if err := r.open(); err != nil {
		return Signature{}, err
	}
	signature, ok := r.plugin.rm.signatureIsTable(r.sorts)
	if !ok {
		return Signature{}, ErrRelationNotTable
	}
	return signature, nil
}

func (r *UDocPluginRelation) Facts() ([]Fact, error) { return nil, ErrRelationNotTable }

// factToDoc is udoc_relation::fact2doc for the relation fact whose
// numerals are the table elements.
func (r *UDocPluginRelation) factToDoc(f Fact) (*Doc, error) {
	if len(f) != len(r.sorts) {
		return nil, ErrDOC
	}
	d, err := r.dm.Allocate0()
	if err != nil {
		return nil, err
	}
	for i, value := range f {
		lo := r.columnIdx(uint32(i))
		hi := r.columnIdx(uint32(i) + 1)
		if err = r.dm.vectors.SetBigInt(d.positive, new(big.Int).SetUint64(uint64(value)), hi-1, lo); err != nil {
			return nil, errors.Join(err, r.dm.Deallocate(d))
		}
	}
	return d, nil
}

// AddFactAST is udoc_relation::add_fact of a relation_fact: fact2doc
// reads each value as a numeral (VERIFY(is_numeral)) of the column's
// width, which may exceed a table element (bit-vectors of any width).
func (r *UDocPluginRelation) AddFactAST(values []ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if len(values) != len(r.sorts) {
		return ErrDOC
	}
	d, err := r.dm.Allocate0()
	if err != nil {
		return err
	}
	for i, value := range values {
		number, _, ok, err := r.plugin.isNumeral(value)
		if err == nil && !ok {
			err = ErrDOC
		}
		if err == nil {
			lo := r.columnIdx(uint32(i))
			hi := r.columnIdx(uint32(i) + 1)
			err = r.dm.vectors.SetBigInt(d.positive, number, hi-1, lo)
		}
		if err != nil {
			return errors.Join(err, r.dm.Deallocate(d))
		}
	}
	_, err = r.elems.InsertOwned(d)
	return err
}

// Add is udoc_relation::add_fact.
func (r *UDocPluginRelation) Add(f Fact) (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	d, err := r.factToDoc(f)
	if err != nil {
		return false, err
	}
	return r.elems.InsertOwned(d)
}

// Contains is udoc_relation::contains_fact.
func (r *UDocPluginRelation) Contains(f Fact) (_ bool, result error) {
	if err := r.open(); err != nil {
		return false, err
	}
	d, err := r.factToDoc(f)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, r.dm.Deallocate(d)) }()
	return r.elems.Contains(d)
}

// Len is get_size_estimate_rows: m_elems.size().
func (r *UDocPluginRelation) Len() (int, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return int(r.elems.Len()), nil
}

func (r *UDocPluginRelation) SizeEstimateRows() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.elems.Len(), nil
}

// udocRelationObjectSize is sizeof(udoc_relation) in the reference build
// (relation_base with its signature, the doc_manager reference, m_elems and
// m_column_info), 128 with the fork's release build (g++ -O3, x86-64).
const udocRelationObjectSize = 128

// SizeEstimateBytes is udoc_relation::get_size_estimate_bytes.
func (r *UDocPluginRelation) SizeEstimateBytes() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	bytes, err := r.elems.SizeEstimateBytes()
	return udocRelationObjectSize + bytes, err
}

// Reset is udoc_relation::reset.
func (r *UDocPluginRelation) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.elems.Reset()
}

// Close is deallocate (~udoc_relation).
func (r *UDocPluginRelation) Close() error {
	if r == nil || r.closed {
		return nil
	}
	var err error
	if r.plugin != nil && !r.plugin.closed {
		err = r.elems.Reset()
	}
	r.closed = true
	return err
}

// Empty is udoc_relation::empty: m_elems.is_empty_complete(m, dm).
func (r *UDocPluginRelation) Empty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.elems.EmptyComplete(r.plugin.m)
}

func (r *UDocPluginRelation) FastEmpty() bool { return r == nil || r.closed || r.elems.Empty() }
func (r *UDocPluginRelation) IsPrecise() bool { return true }

// CloneRelation is udoc_relation::clone.
func (r *UDocPluginRelation) CloneRelation() (RelationBase, error) {
	return r.clone()
}

func (r *UDocPluginRelation) clone() (*UDocPluginRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	result, err := r.plugin.newRelation(r.sorts)
	if err != nil {
		return nil, err
	}
	for _, d := range r.elems.values {
		c, err := r.dm.AllocateCopy(d)
		if err != nil {
			return nil, errors.Join(err, result.Close())
		}
		result.elems.values = append(result.elems.values, c)
	}
	return result, nil
}

func (r *UDocPluginRelation) CloneFinite() (FiniteTable, error) { return r.clone() }

// Display is udoc_relation::display.
func (r *UDocPluginRelation) Display(out io.Writer) error {
	text, err := r.displayText()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}

func (r *UDocPluginRelation) displayText() (string, error) {
	if err := r.open(); err != nil {
		return "", err
	}
	text, err := r.elems.String()
	return text + "\n", err
}

// verbose3 is IF_VERBOSE(3, r.display(verbose_stream() << prefix)).
func udocVerbose3(prefix string, r *UDocPluginRelation) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(3)
	if err != nil || !enabled {
		return err
	}
	text, err := r.displayText()
	if err != nil {
		return err
	}
	return diagnostic.ProgressVerbose(3, prefix+text)
}

// ToFormula is udoc_relation::to_formula(fml).
func (r *UDocPluginRelation) ToFormula() (ast.ASTRef, error) {
	if err := r.open(); err != nil {
		return ast.ASTRef{}, err
	}
	m := r.plugin.m
	disj := make([]ast.ASTHandle, 0, len(r.elems.values))
	for _, d := range r.elems.values {
		f, err := r.docFormula(d)
		if err != nil {
			return ast.ASTRef{}, err
		}
		disj = append(disj, f)
	}
	fml, err := udocMkOr(m, disj)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(fml)
}

// docFormula is udoc_relation::to_formula(doc const&).
func (r *UDocPluginRelation) docFormula(d *Doc) (ast.ASTHandle, error) {
	m := r.plugin.m
	conjs := make([]ast.ASTHandle, 0, 1+len(d.negative.values))
	pos, err := r.tbvFormula(d.positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjs = append(conjs, pos)
	for _, neg := range d.negative.values {
		f, err := r.tbvFormula(neg)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		n, err := m.MkNot(f)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		conjs = append(conjs, n)
	}
	return udocMakeAnd(m, conjs)
}

// tbvFormula is udoc_relation::to_formula(tbv const&).
func (r *UDocPluginRelation) tbvFormula(t *TBV) (ast.ASTHandle, error) {
	p := r.plugin
	m := p.m
	conjs := make([]ast.ASTHandle, 0)
	for i := uint32(0); i < r.numCols(); i++ {
		v, err := m.MkVar(i, r.sorts[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		lo := r.columnIdx(i)
		hi := r.columnIdx(i + 1)
		value := new(big.Int)
		lo1 := lo
		isX := true
		for j := lo; j < hi; j++ {
			bit, err := r.dm.vectors.Get(t, j)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			switch bit {
			case TBit0:
				if isX {
					isX, lo1 = false, j
					value.SetInt64(0)
				}
			case TBit1:
				if isX {
					isX, lo1 = false, j
					value.SetInt64(0)
				}
				value.SetBit(value, int(j-lo1), 1)
			case TBitX:
				if !isX {
					ext, err := m.MkBitVectorExtract(j-1-lo, lo1-lo, v)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					num, err := m.BitVectorNumeralRat(new(big.Rat).SetInt(value), j-lo1)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					eq, err := m.MkEq(ext, num)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					conjs = append(conjs, eq)
				}
				isX = true
			default:
				return ast.ASTHandle{}, ErrDOC
			}
		}
		if !isX {
			var eq ast.ASTHandle
			if lo1 == lo {
				num, err := p.mkNumeral(value, r.sorts[i])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if eq, err = m.MkEq(v, num); err != nil {
					return ast.ASTHandle{}, err
				}
			} else {
				num, err := m.BitVectorNumeralRat(new(big.Rat).SetInt(value), hi-lo1)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				ext, err := m.MkBitVectorExtract(hi-1-lo, lo1-lo, v)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if eq, err = m.MkEq(ext, num); err != nil {
					return ast.ASTHandle{}, err
				}
			}
			conjs = append(conjs, eq)
		}
	}
	return udocMakeAnd(m, conjs)
}

// udocMkOr is ast_util mk_or(m, n, args).
func udocMkOr(m *ast.ASTManager, terms []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(terms) {
	case 0:
		return m.MkFalse()
	case 1:
		return terms[0], nil
	default:
		return m.MkOrArgs(terms)
	}
}

// expandColumnVector is udoc_relation::expand_column_vector(v, other).
func (r *UDocPluginRelation) expandColumnVector(v []int, other *UDocPluginRelation) []uint32 {
	result := make([]uint32, 0, len(v))
	for _, c := range v {
		col := uint32(c)
		var bit, limit uint32
		if col < r.numCols() {
			bit = r.columnIdx(col)
			limit = bit + r.columnNumBits(col)
		} else {
			idx := col - r.numCols()
			bit = r.numBits() + other.columnIdx(idx)
			limit = bit + other.columnNumBits(idx)
		}
		for ; bit < limit; bit++ {
			result = append(result, bit)
		}
	}
	return result
}

// ---------------------------------------------------------------------
// union_bvec members not in DocUnion.

// udocMerge is union_bvec::merge(m, lo, length, equalities, discard_cols).
func udocMerge(u *DocUnion, lo, length uint32, equalities *backtrack.UnionFind, discard []bool) error {
	j := 0
	for _, d := range u.values {
		ok, err := u.owner.MergeRange(d, lo, length, equalities, discard)
		if err != nil {
			return err
		}
		if !ok {
			if err = u.owner.Deallocate(d); err != nil {
				return err
			}
			continue
		}
		u.values[j] = d
		j++
	}
	u.values = u.values[:j]
	return nil
}

// udocMergePair is union_bvec::merge(m, lo1, lo2, length, discard_cols).
func udocMergePair(u *DocUnion, lo1, lo2, length uint32, discard []bool) error {
	equalities := backtrack.NewDefaultUnionFind()
	for range discard {
		equalities.MkVar()
	}
	for j := uint32(0); j < length; j++ {
		equalities.Merge(lo1+j, lo2+j)
	}
	return udocMerge(u, lo1, length, equalities, discard)
}

// udocMergeRoots is union_bvec::merge(m, roots, equalities, discard_cols).
func udocMergeRoots(u *DocUnion, roots []uint32, equalities *backtrack.UnionFind, discard []bool) error {
	for _, root := range roots {
		if err := udocMerge(u, root, 1, equalities, discard); err != nil {
			return err
		}
	}
	return nil
}

// newSubsetInts is subset_ints equalities(union_ctx) with n variables.
func newSubsetInts(n uint32) *backtrack.UnionFind {
	equalities := backtrack.NewDefaultUnionFind()
	for i := uint32(0); i < n; i++ {
		equalities.MkVar()
	}
	return equalities
}

// ---------------------------------------------------------------------
// Operations.

// MkJoinFn is udoc_plugin::mk_join_fn / join_fn.
func (p *UDocPlugin) MkJoinFn(t1, t2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	if !p.checkKind(t1) || !p.checkKind(t2) {
		return nil, nil
	}
	u1, _ := udocGet(t1)
	u2, _ := udocGet(t2)
	sig := joinSorts(u1.sorts, u2.sorts)
	dm, err := p.dmFor(sig)
	if err != nil {
		return nil, err
	}
	mCols1 := u1.expandColumnVector(cols1, nil)
	mCols2 := u2.expandColumnVector(cols2, nil)
	return func(r1b, r2b RelationBase) (RelationBase, error) {
		r1, _ := udocGet(r1b)
		r2, _ := udocGet(r2b)
		result, err := p.newRelation(sig)
		if err != nil {
			return nil, err
		}
		// r.join(d1, d2, dm, dm1, m_cols1, m_cols2)
		for _, a := range r1.elems.values {
			for _, b := range r2.elems.values {
				d, err := dm.Join(a, b, r1.dm, mCols1, mCols2)
				if err != nil {
					return nil, errors.Join(err, result.Close())
				}
				if d != nil {
					if _, err = result.elems.InsertOwned(d); err != nil {
						return nil, errors.Join(err, result.Close())
					}
				}
			}
		}
		if err = udocVerbose3("join result:\n", result); err != nil {
			return nil, errors.Join(err, result.Close())
		}
		return result, nil
	}, nil
}

// toDelete builds project_fn's m_to_delete bit vector.
func toDelete(n uint32, removed []uint32) []bool {
	result := make([]bool, n)
	for _, bit := range removed {
		result[bit] = true
	}
	return result
}

// MkProjectFn is udoc_plugin::mk_project_fn / project_fn.
func (p *UDocPlugin) MkProjectFn(t RelationBase, removed []int) (RelationTransformerFn, error) {
	if !p.checkKind(t) {
		return nil, nil
	}
	u, _ := udocGet(t)
	resultSig := projectSorts(u.sorts, removed)
	mToDelete := toDelete(u.dm.NumTBits(), u.expandColumnVector(removed, nil))
	return func(tb RelationBase) (RelationBase, error) {
		t, _ := udocGet(tb)
		r, err := p.newRelation(resultSig)
		if err != nil {
			return nil, err
		}
		for _, d := range t.elems.values {
			d2, err := t.dm.Project(r.dm, mToDelete, d)
			if err != nil {
				return nil, errors.Join(err, r.Close())
			}
			r.elems.values = append(r.elems.values, d2)
		}
		return r, nil
	}, nil
}

// MkRenameFn is udoc_plugin::mk_rename_fn / rename_fn.
func (p *UDocPlugin) MkRenameFn(t RelationBase, cycle []int) (RelationTransformerFn, error) {
	if !p.checkKind(t) {
		return nil, nil
	}
	u, _ := udocGet(t)
	sig1 := u.sorts
	sig2 := renameSorts(u.sorts, cycle)
	permutation := make([]uint32, u.numBits())
	for i := range permutation {
		permutation[i] = uint32(i)
	}
	permutation0 := make([]uint32, len(sig1))
	for i := range permutation0 {
		permutation0[i] = uint32(i)
	}
	for i := range cycle {
		j := (i + 1) % len(cycle)
		col1 := cycle[i]
		col2 := cycle[j]
		permutation0[col2] = uint32(col1)
	}
	columnInfo := make([]uint32, 0, len(sig2)+1)
	var column uint32
	for _, sort := range sig2 {
		columnInfo = append(columnInfo, column)
		n, err := p.numSortBits(sort)
		if err != nil {
			return nil, err
		}
		column += n
	}
	columnInfo = append(columnInfo, column)
	for i := range sig1 {
		length := u.columnNumBits(uint32(i))
		lo1 := u.columnIdx(uint32(i))
		col2 := permutation0[i]
		lo2 := columnInfo[col2]
		for k := uint32(0); k < length; k++ {
			permutation[k+lo1] = k + lo2
		}
	}
	return func(rb RelationBase) (RelationBase, error) {
		r, _ := udocGet(rb)
		result, err := p.newRelation(sig2)
		if err != nil {
			return nil, err
		}
		for _, d := range r.elems.values {
			c, err := r.dm.AllocatePermutation(d, permutation)
			if err != nil {
				return nil, errors.Join(err, result.Close())
			}
			result.elems.values = append(result.elems.values, c)
		}
		return result, nil
	}, nil
}

// MkUnionFn is udoc_plugin::mk_union_fn / union_fn.
func (p *UDocPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	if !p.checkKind(target) || !p.checkKind(source) || delta != nil && !p.checkKind(delta) {
		return nil, nil
	}
	return func(rb, srcb, deltab RelationBase) error {
		r, _ := udocGet(rb)
		src, _ := udocGet(srcb)
		var d *UDocPluginRelation
		var d1 *DocUnion
		if deltab != nil {
			d, _ = udocGet(deltab)
			d1 = d.elems
		}
		if err := udocVerbose3("orig:  ", r); err != nil {
			return err
		}
		if err := udocMkUnion(r.dm, r.elems, src.elems, d1); err != nil {
			return err
		}
		if err := udocVerbose3("union: ", r); err != nil {
			return err
		}
		if d != nil {
			return udocVerbose3("delta: ", d)
		}
		return nil
	}, nil
}

// MkWidenFn is udoc_plugin::mk_widen_fn: the union.
func (p *UDocPlugin) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.MkUnionFn(target, source, delta)
}

// udocMkUnion is udoc_plugin::mk_union.
func udocMkUnion(dm *DocManager, dst, src, delta *DocUnion) error {
	deltaEmpty := false
	if delta != nil {
		deltaEmpty = delta.Empty()
	}
	addDelta := func(d *Doc) error {
		c, err := dm.AllocateCopy(d)
		if err != nil {
			return err
		}
		if deltaEmpty {
			delta.values = append(delta.values, c)
			return nil
		}
		_, err = delta.InsertOwned(c)
		return err
	}
	if dst.Empty() {
		for _, d := range src.values {
			c, err := dm.AllocateCopy(d)
			if err != nil {
				return err
			}
			dst.values = append(dst.values, c)
			if delta != nil {
				if err = addDelta(d); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, d := range src.values {
		c, err := dm.AllocateCopy(d)
		if err != nil {
			return err
		}
		inserted, err := dst.InsertOwned(c)
		if err != nil {
			return err
		}
		if inserted && delta != nil {
			if err = addDelta(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// MkFilterIdenticalFn is udoc_plugin::mk_filter_identical_fn.
func (p *UDocPlugin) MkFilterIdenticalFn(t RelationBase, identical []int) (RelationMutatorFn, error) {
	if !p.checkKind(t) {
		return nil, nil
	}
	u, _ := udocGet(t)
	size := u.columnNumBits(uint32(identical[0]))
	emptyBV := make([]bool, u.numBits())
	cols := make([]uint32, len(identical))
	for i, c := range identical {
		cols[i] = u.columnIdx(uint32(c))
	}
	equalities := newSubsetInts(uint32(len(emptyBV)))
	for i := 1; i < len(cols); i++ {
		for j := uint32(0); j < size; j++ {
			equalities.Merge(cols[0]+j, cols[i]+j)
		}
	}
	return func(rb RelationBase) error {
		r, _ := udocGet(rb)
		return udocMerge(r.elems, cols[0], size, equalities, emptyBV)
	}, nil
}

// MkFilterEqualFn is udoc_plugin::mk_filter_equal_fn / filter_equal_fn.
func (p *UDocPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	if !p.checkKind(t) {
		return nil, nil
	}
	u, _ := udocGet(t)
	dm, err := p.dmFor(u.sorts)
	if err != nil {
		return nil, err
	}
	r, _, ok, err := p.isNumeral(value)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrDOC
	}
	filter, err := dm.AllocateX()
	if err != nil {
		return nil, err
	}
	lo := u.columnIdx(uint32(col))
	hi := u.columnIdx(uint32(col) + 1)
	if err = dm.vectors.SetBigInt(filter.positive, r, hi-1, lo); err != nil {
		return nil, errors.Join(err, dm.Deallocate(filter))
	}
	// ~filter_equal_fn deallocates m_filter: the functor is destroyed with
	// its owner (the instruction that cached it, else the relation
	// manager), after any number of applications. A doc manager already
	// destroyed (with the plugin) has released it.
	p.rm.fnClosers = append(p.rm.fnClosers, func() error {
		if err := dm.Deallocate(filter); err != nil && !errors.Is(err, arena.ErrClosed) {
			return err
		}
		return nil
	})
	return func(tb RelationBase) error {
		t, _ := udocGet(tb)
		return t.elems.IntersectDoc(filter)
	}, nil
}

// MkFilterInterpretedFn is udoc_plugin::mk_filter_interpreted_fn /
// filter_interpreted_fn.
func (p *UDocPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	if !p.checkKind(t) {
		return nil, nil
	}
	u, _ := udocGet(t)
	dm := u.dm
	numBits := u.numBits()
	emptyBV := make([]bool, numBits)
	equalities := newSubsetInts(numBits)
	guard, reduced, err := u.extractGuard(condition)
	if err != nil {
		return nil, err
	}
	mUDoc, err := NewDocUnion(dm)
	if err != nil {
		return nil, err
	}
	x, err := dm.AllocateX()
	if err == nil {
		err = mUDoc.PushOwned(x)
	}
	if err == nil {
		err = u.applyGuard(guard, mUDoc, equalities, emptyBV)
	}
	if err != nil {
		return nil, errors.Join(err, mUDoc.Reset())
	}
	// ~filter_interpreted_fn resets m_udoc when the functor is destroyed (with the
	// instruction that cached it, else the relation manager), after any
	// number of applications.
	p.rm.fnClosers = append(p.rm.fnClosers, func() error {
		if err := mUDoc.Reset(); err != nil && !errors.Is(err, arena.ErrClosed) {
			return err
		}
		return nil
	})
	return func(tb RelationBase) (result error) {
		t, _ := udocGet(tb)
		uu := t.elems
		if err := uu.Intersect(mUDoc); err != nil {
			return err
		}
		if err := t.applyGuard(reduced, uu, equalities, emptyBV); err != nil {
			return err
		}
		if err := uu.Simplify(); err != nil {
			return err
		}
		return udocVerbose3("", t)
	}, nil
}

// MkJoinProjectFn is udoc_plugin::mk_join_project_fn.
func (p *UDocPlugin) MkJoinProjectFn(t1, t2 RelationBase, cols1, cols2, removed []int) (RelationJoinFn, error) {
	if !p.checkKind(t1) || !p.checkKind(t2) {
		return nil, nil
	}
	// special case where we have h(X) :- f(X), g(X).
	if len(cols1) == len(removed) && len(t1.RelationSignature()) == len(cols1) && len(t2.RelationSignature()) == len(cols1) {
		special := true
		for i := range removed {
			if removed[i] != i || cols1[i] != cols2[i] {
				special = false
				break
			}
		}
		if special {
			// join_project_and_fn
			return func(r1b, r2b RelationBase) (RelationBase, error) {
				r1, _ := udocGet(r1b)
				r2, _ := udocGet(r2b)
				result, err := r1.clone()
				if err != nil {
					return nil, err
				}
				if err = result.elems.Intersect(r2.elems); err != nil {
					return nil, errors.Join(err, result.Close())
				}
				return result, nil
			}, nil
		}
	}
	u1, _ := udocGet(t1)
	u2, _ := udocGet(t2)
	return p.joinProjectFn(u1, u2, cols1, cols2, removed)
}

// joinProjectFn is udoc_plugin::join_project_fn.
func (p *UDocPlugin) joinProjectFn(t1, t2 *UDocPluginRelation, cols1, cols2, removed []int) (RelationJoinFn, error) {
	resultSig := projectSorts(joinSorts(t1.sorts, t2.sorts), removed)
	numBits := t1.numBits() + t2.numBits()
	removedBits := t1.expandColumnVector(removed, t2)
	mCols1 := t1.expandColumnVector(cols1, nil)
	mCols2 := t2.expandColumnVector(cols2, nil)
	mToDelete := toDelete(numBits, removedBits)
	prodSig := joinSorts(t1.sorts, t2.sorts)
	return func(r1b, r2b RelationBase) (RelationBase, error) {
		t1, _ := udocGet(r1b)
		t2, _ := udocGet(r2b)
		dmProd, err := p.dmFor(prodSig)
		if err != nil {
			return nil, err
		}
		result, err := p.newRelation(resultSig)
		if err != nil {
			return nil, err
		}
		res := result.elems
		dmRes := result.dm
		for i, a := range t1.elems.values {
			for j, b := range t2.elems.values {
				d, err := dmProd.Join(a, b, t1.dm, mCols1, mCols2)
				if err != nil {
					return nil, errors.Join(err, result.Close())
				}
				if d == nil {
					continue
				}
				projected, err := dmProd.Project(dmRes, mToDelete, d)
				err = errors.Join(err, dmProd.Deallocate(d))
				if err == nil {
					_, err = res.InsertOwned(projected)
				}
				if err != nil {
					return nil, errors.Join(err, result.Close())
				}
				if diagnostic.VerboseProgress && res.Len() > 0 && res.Len()%10000 == 0 {
					if err = diagnostic.ProgressVerbose(2, udocResultSizeText(res.Len(), i, j, len(t1.elems.values))); err != nil {
						return nil, errors.Join(err, result.Close())
					}
				}
			}
		}
		return result, nil
	}, nil
}

func udocResultSizeText(size uint32, i, j, n int) string {
	return "result size: " + utoa(uint64(size)) + " i:" + utoa(uint64(i)) + " j:" + utoa(uint64(j)) + " " + utoa(uint64(100*i/n)) + "% complete\n"
}

func utoa(v uint64) string {
	return new(big.Int).SetUint64(v).String()
}

// MkFilterByNegationFn is udoc_plugin::mk_filter_by_negation_fn /
// negation_filter_fn.
func (p *UDocPlugin) MkFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	if !p.checkKind(t) || !p.checkKind(neg) {
		return nil, nil
	}
	ut, _ := udocGet(t)
	un, _ := udocGet(neg)
	sz1, sz2 := len(ut.sorts), len(un.sorts)
	removeCols := make([]int, 0, sz2)
	for i := 0; i < sz2; i++ {
		removeCols = append(removeCols, sz1+i)
	}
	joinProject, err := p.joinProjectFn(ut, un, tCols, negCols, removeCols)
	if err != nil {
		return nil, err
	}
	joined := len(tCols)
	isSubtract := joined == sz1
	isSubtract = isSubtract && joined == sz2
	found := make([]bool, joined)
	for i := 0; isSubtract && i < joined; i++ {
		isSubtract = !found[tCols[i]] && tCols[i] == negCols[i]
		found[tCols[i]] = true
	}
	return func(tb, negb RelationBase) error {
		t, _ := udocGet(tb)
		n, _ := udocGet(negb)
		if err := udocVerbose3("dst:", t); err != nil {
			return err
		}
		if err := udocVerbose3("neg:", n); err != nil {
			return err
		}
		if t.FastEmpty() || n.FastEmpty() {
			return nil
		}
		if len(n.sorts) == 0 {
			return t.elems.Reset()
		}
		if isSubtract {
			return t.elems.Subtract(n.elems)
		}
		// slow_pass
		jpb, err := joinProject(t, n)
		if err != nil {
			return err
		}
		jp, _ := udocGet(jpb)
		if !jp.FastEmpty() {
			err = t.elems.Subtract(jp.elems)
		}
		return errors.Join(err, jp.Close())
	}, nil
}

// MkFilterInterpretedAndProjectFn is
// udoc_plugin::mk_filter_interpreted_and_project_fn / filter_proj_fn.
func (p *UDocPlugin) MkFilterInterpretedAndProjectFn(t RelationBase, condition ast.ASTHandle, removed []int) (RelationTransformerFn, error) {
	if !p.checkKind(t) {
		return nil, nil
	}
	u, _ := udocGet(t)
	resultSig := projectSorts(u.sorts, removed)
	numBits := u.numBits()
	mToDelete := toDelete(numBits, u.expandColumnVector(removed, nil))
	equalities := newSubsetInts(numBits)
	nonEqCond, roots, err := u.extractEqualitiesCond(condition, equalities)
	if err != nil {
		return nil, err
	}
	guard, reduced, err := u.extractGuard(nonEqCond)
	if err != nil {
		return nil, err
	}
	mUDoc, err := NewDocUnion(u.dm)
	if err != nil {
		return nil, err
	}
	if err = u.compileGuard(guard, mUDoc, mToDelete); err != nil {
		return nil, errors.Join(err, mUDoc.Reset())
	}
	// ~filter_proj_fn resets m_udoc when the functor is destroyed (with the
	// instruction that cached it, else the relation manager), after any
	// number of applications.
	p.rm.fnClosers = append(p.rm.fnClosers, func() error {
		if err := mUDoc.Reset(); err != nil && !errors.Is(err, arena.ErrClosed) {
			return err
		}
		return nil
	})
	return func(tb RelationBase) (_ RelationBase, result error) {
		t, _ := udocGet(tb)
		dm := t.dm
		mUDoc2, err := NewDocUnion(dm)
		if err != nil {
			return nil, err
		}
		defer func() { result = errors.Join(result, mUDoc2.Reset()) }()
		if err = mUDoc2.Copy(t.elems); err != nil {
			return nil, err
		}
		if err = mUDoc2.Intersect(mUDoc); err != nil {
			return nil, err
		}
		if err = t.applyGuard(reduced, mUDoc2, equalities, mToDelete); err != nil {
			return nil, err
		}
		if err = udocMergeRoots(mUDoc2, roots, equalities, mToDelete); err != nil {
			return nil, err
		}
		r, err := p.newRelation(resultSig)
		if err != nil {
			return nil, err
		}
		for _, d := range mUDoc2.values {
			projected, err := dm.Project(r.dm, mToDelete, d)
			if err == nil {
				_, err = r.elems.InsertOwned(projected)
			}
			if err != nil {
				return nil, errors.Join(err, r.Close())
			}
		}
		if err = udocVerbose3("filter project result:\n", r); err != nil {
			return nil, errors.Join(err, r.Close())
		}
		return r, nil
	}, nil
}

// ---------------------------------------------------------------------
// Guards.

// basicKind returns the decl kind of a basic-family application.
func basicKind(m *ast.ASTManager, e ast.ASTHandle) (int32, bool, error) {
	kind, err := m.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := m.GetFamilyID(e)
	if err != nil || family != m.BasicFamilyID() {
		return 0, false, err
	}
	k, err := m.GetDeclKind(e)
	return k, err == nil, err
}

func isVar(m *ast.ASTManager, e ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(e)
	return kind == ast.ASTVar, err
}

func appArgs(m *ast.ASTManager, e ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := m.GetNumArgs(e)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		if args[i], err = m.GetArg(e, uint32(i)); err != nil {
			return nil, err
		}
	}
	return args, nil
}

// isVarRange is udoc_relation::is_var_range.
func (r *UDocPluginRelation) isVarRange(e ast.ASTHandle) (hi, lo, v uint32, ok bool, err error) {
	m := r.plugin.m
	if yes, err := isVar(m, e); err != nil {
		return 0, 0, 0, false, err
	} else if yes {
		if v, err = m.GetVarIndex(e); err != nil {
			return 0, 0, 0, false, err
		}
		n, err := r.plugin.numExprBits(e)
		if err != nil {
			return 0, 0, 0, false, err
		}
		return n - 1, 0, v, true, nil
	}
	var e2 ast.ASTHandle
	found, err := m.BitVectorExtractValue(e, &lo, &hi, &e2)
	if err != nil || !found {
		return 0, 0, 0, false, err
	}
	if yes, err := isVar(m, e2); err != nil || !yes {
		return 0, 0, 0, false, err
	}
	v, err = m.GetVarIndex(e2)
	return hi, lo, v, err == nil, err
}

// isGuard is udoc_relation::is_guard(expr*).
func (r *UDocPluginRelation) isGuard(g ast.ASTHandle) (bool, error) {
	m := r.plugin.m
	k, basic, err := basicKind(m, g)
	if err != nil {
		return false, err
	}
	if basic && (k == ast.ASTOpAnd || k == ast.ASTOpOr || k == ast.ASTOpNot || k == ast.ASTOpTrue || k == ast.ASTOpFalse) {
		args, err := appArgs(m, g)
		if err != nil {
			return false, err
		}
		for _, a := range args {
			ok, err := r.isGuard(a)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	}
	if basic && k == ast.ASTOpEq {
		args, err := appArgs(m, g)
		if err != nil {
			return false, err
		}
		e1, e2 := args[0], args[1]
		bv, err := r.plugin.isBV(e1)
		if err != nil {
			return false, err
		}
		if bv {
			if _, _, _, ok, err := r.isVarRange(e1); err != nil {
				return false, err
			} else if ok {
				ground, err := m.IsGround(e2)
				if err != nil || ground {
					return ground, err
				}
			}
			if _, _, _, ok, err := r.isVarRange(e2); err != nil {
				return false, err
			} else if ok {
				ground, err := m.IsGround(e1)
				if err != nil || ground {
					return ground, err
				}
			}
		}
	}
	return isVar(m, g)
}

// extractGuard is udoc_relation::extract_guard.
func (r *UDocPluginRelation) extractGuard(cond ast.ASTHandle) (guard, rest ast.ASTHandle, err error) {
	m := r.plugin.m
	conds, err := m.FlattenAnd([]ast.ASTHandle{cond})
	if err != nil {
		return
	}
	guards := make([]ast.ASTHandle, 0, len(conds))
	rests := make([]ast.ASTHandle, 0, len(conds))
	for _, g := range conds {
		ok, e := r.isGuard(g)
		if e != nil {
			err = e
			return
		}
		if ok {
			guards = append(guards, g)
		} else {
			rests = append(rests, g)
		}
	}
	if guard, err = udocMakeAnd(m, guards); err != nil {
		return
	}
	rest, err = udocMakeAnd(m, rests)
	return
}

// extractEqualitiesCond is udoc_relation::extract_equalities(g, rest,
// equalities, roots).
func (r *UDocPluginRelation) extractEqualitiesCond(g ast.ASTHandle, equalities *backtrack.UnionFind) (ast.ASTHandle, []uint32, error) {
	m := r.plugin.m
	conds, err := m.FlattenAnd([]ast.ASTHandle{g})
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	var roots []uint32
	for i := 0; i < len(conds); i++ {
		gi := conds[i]
		k, basic, err := basicKind(m, gi)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if basic && k == ast.ASTOpEq {
			args, err := appArgs(m, gi)
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
			if conds, roots, err = r.extractEqualities(args[0], args[1], conds, equalities, roots); err != nil {
				return ast.ASTHandle{}, nil, err
			}
			conds[i] = conds[len(conds)-1]
			conds = conds[:len(conds)-1]
		}
	}
	rest, err := udocMakeAnd(m, conds)
	return rest, roots, err
}

// newThRewriter is th_rewriter rw(m) with the default parameters.
func (p *UDocPlugin) newThRewriter() (*astrewriter.TheoryRewriter, error) {
	context, globals, err := p.m.RewriterParameterServices()
	if err != nil {
		return nil, err
	}
	return astrewriter.NewTheoryRewriter(p.m, context, globals, nil)
}

// rewriteExtract is e3 = bv.mk_extract(hi, lo, e2); rw(e3). The result is
// held in out, which the caller releases.
func (p *UDocPlugin) rewriteExtract(rw *astrewriter.TheoryRewriter, hi, lo uint32, e2 ast.ASTHandle) (ast.ASTRef, ast.ASTHandle, error) {
	e3, err := p.m.MkBitVectorExtract(hi, lo, e2)
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, err
	}
	in, err := p.m.NewRef(e3)
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, err
	}
	out, err := p.m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, errors.Join(err, p.m.DelRef(&in))
	}
	err = rw.Rewrite(e3, out)
	err = errors.Join(err, p.m.DelRef(&in))
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, errors.Join(err, p.m.DelRef(&out))
	}
	h, err := p.m.RefGet(out)
	return out, h, err
}

// extractEqualities is udoc_relation::extract_equalities(e1, e2, conds,
// equalities, roots).
func (r *UDocPluginRelation) extractEqualities(e1, e2 ast.ASTHandle, conds []ast.ASTHandle, equalities *backtrack.UnionFind, roots []uint32) (_ []ast.ASTHandle, _ []uint32, result error) {
	p := r.plugin
	m := p.m
	if _, ok, err := m.BitVectorConcatArguments(e2); err != nil {
		return nil, nil, err
	} else if ok {
		e1, e2 = e2, e1
	}
	if args, ok, err := m.BitVectorConcatArguments(e1); err != nil {
		return nil, nil, err
	} else if ok {
		rw, err := p.newThRewriter()
		if err != nil {
			return nil, nil, err
		}
		defer func() { result = errors.Join(result, rw.Close()) }()
		n, err := p.numExprBits(e1)
		if err != nil {
			return nil, nil, err
		}
		hi := n - 1
		for _, e := range args {
			sz, err := p.numExprBits(e)
			if err != nil {
				return nil, nil, err
			}
			ref, e3, err := p.rewriteExtract(rw, hi, hi-sz+1, e2)
			if err != nil {
				return nil, nil, err
			}
			// e3 stays referenced: it may be kept in conds.
			_ = ref
			if conds, roots, err = r.extractEqualities(e, e3, conds, equalities, roots); err != nil {
				return nil, nil, err
			}
			hi -= sz
		}
		return conds, roots, nil
	}
	hi1, lo1, v1, ok1, err := r.isVarRange(e1)
	if err != nil {
		return nil, nil, err
	}
	if ok1 {
		_, lo2, v2, ok2, err := r.isVarRange(e2)
		if err != nil {
			return nil, nil, err
		}
		if ok2 {
			col1 := r.columnIdx(v1)
			lo1 += col1
			hi1 += col1
			col2 := r.columnIdx(v2)
			lo2 += col2
			for j := uint32(0); j <= hi1-lo1; j++ {
				roots = append(roots, lo1+j)
				equalities.Merge(lo1+j, lo2+j)
			}
			return conds, roots, nil
		}
	}
	eq, err := m.MkEq(e1, e2)
	if err != nil {
		return nil, nil, err
	}
	return append(conds, eq), roots, nil
}

// compileGuard is udoc_relation::compile_guard.
func (r *UDocPluginRelation) compileGuard(g ast.ASTHandle, result *DocUnion, discardCols []bool) error {
	x, err := r.dm.AllocateX()
	if err != nil {
		return err
	}
	if err = result.PushOwned(x); err != nil {
		return err
	}
	equalities := newSubsetInts(uint32(len(discardCols)))
	return r.applyGuard(g, result, equalities, discardCols)
}

// applyGroundEq is udoc_relation::apply_ground_eq: the cube fixing bits
// [lo, hi] of column v to c, or nil when c is not a numeral.
func (r *UDocPluginRelation) applyGroundEq(v, hi, lo uint32, c ast.ASTHandle) (*Doc, error) {
	col := r.columnIdx(v)
	lo += col
	hi += col
	value, _, ok, err := r.plugin.isNumeral(c)
	if err != nil || !ok {
		return nil, err
	}
	d, err := r.dm.AllocateX()
	if err != nil {
		return nil, err
	}
	if err = r.dm.vectors.SetBigInt(d.positive, value, hi, lo); err != nil {
		return nil, errors.Join(err, r.dm.Deallocate(d))
	}
	return d, nil
}

// applyBVEq is udoc_relation::apply_bv_eq.
func (r *UDocPluginRelation) applyBVEq(e1, e2 ast.ASTHandle, discardCols []bool, result *DocUnion) (_ bool, resultErr error) {
	p := r.plugin
	m := p.m
	if _, ok, err := m.BitVectorConcatArguments(e2); err != nil {
		return false, err
	} else if ok {
		e1, e2 = e2, e1
	}
	if args, ok, err := m.BitVectorConcatArguments(e1); err != nil {
		return false, err
	} else if ok {
		rw, err := p.newThRewriter()
		if err != nil {
			return false, err
		}
		defer func() { resultErr = errors.Join(resultErr, rw.Close()) }()
		n, err := p.numExprBits(e1)
		if err != nil {
			return false, err
		}
		hi := n - 1
		for _, e := range args {
			sz, err := p.numExprBits(e)
			if err != nil {
				return false, err
			}
			ref, e3, err := p.rewriteExtract(rw, hi, hi-sz+1, e2)
			if err != nil {
				return false, err
			}
			ok, err := r.applyBVEq(e, e3, discardCols, result)
			err = errors.Join(err, m.DelRef(&ref))
			if err != nil || !ok {
				return false, err
			}
			hi -= sz
		}
		return true, nil
	}
	if ground, err := m.IsGround(e1); err != nil {
		return false, err
	} else if ground {
		e1, e2 = e2, e1
	}
	hi, lo, v, ok, err := r.isVarRange(e1)
	if err != nil {
		return false, err
	}
	if ok {
		ground, err := m.IsGround(e2)
		if err != nil {
			return false, err
		}
		if ground {
			d, err := r.applyGroundEq(v, hi, lo, e2)
			if err != nil {
				return false, err
			}
			if d != nil {
				err = result.IntersectDoc(d)
				return err == nil, errors.Join(err, r.dm.Deallocate(d))
			}
		}
	}
	hi1, lo1, v1, ok1, err := r.isVarRange(e1)
	if err != nil {
		return false, err
	}
	if ok1 {
		_, lo2, v2, ok2, err := r.isVarRange(e2)
		if err != nil {
			return false, err
		}
		if ok2 {
			idx1 := lo1 + r.columnIdx(v1)
			idx2 := lo2 + r.columnIdx(v2)
			length := hi1 - lo1 + 1
			return true, udocMergePair(result, idx1, idx2, length, discardCols)
		}
	}
	return false, nil
}

// notGroundEq matches m.is_not(g, e0) && m.is_eq(e0, a, b) &&
// bv.is_bv(a) && is_var_range(a, hi, lo, v) && is_ground(b) &&
// apply_ground_eq(d, v, hi, lo, b), with (a, b) = (e1, e2) for swap=false
// and (e2, e1) for swap=true.
func (r *UDocPluginRelation) notGroundEq(g ast.ASTHandle, swap bool) (*Doc, error) {
	m := r.plugin.m
	k, basic, err := basicKind(m, g)
	if err != nil || !basic || k != ast.ASTOpNot {
		return nil, err
	}
	e0, err := m.GetArg(g, 0)
	if err != nil {
		return nil, err
	}
	k, basic, err = basicKind(m, e0)
	if err != nil || !basic || k != ast.ASTOpEq {
		return nil, err
	}
	args, err := appArgs(m, e0)
	if err != nil {
		return nil, err
	}
	a, b := args[0], args[1]
	if swap {
		a, b = b, a
	}
	if bv, err := r.plugin.isBV(a); err != nil || !bv {
		return nil, err
	}
	hi, lo, v, ok, err := r.isVarRange(a)
	if err != nil || !ok {
		return nil, err
	}
	if ground, err := m.IsGround(b); err != nil || !ground {
		return nil, err
	}
	return r.applyGroundEq(v, hi, lo, b)
}

// newUniverse is "udoc sub; sub.push_back(dm.allocateX())".
func (r *UDocPluginRelation) newUniverse() (*DocUnion, error) {
	sub, err := NewDocUnion(r.dm)
	if err != nil {
		return nil, err
	}
	x, err := r.dm.AllocateX()
	if err != nil {
		return nil, err
	}
	return sub, sub.PushOwned(x)
}

// applyGuard is udoc_relation::apply_guard.
func (r *UDocPluginRelation) applyGuard(g ast.ASTHandle, result *DocUnion, equalities *backtrack.UnionFind, discardCols []bool) error {
	m := r.plugin.m
	dm := r.dm
	if result.Empty() {
		return nil
	}
	if m.IsTrue(g) {
		return nil
	}
	if m.IsFalse(g) {
		return result.Reset()
	}
	k, basic, err := basicKind(m, g)
	if err != nil {
		return err
	}
	if basic && k == ast.ASTOpAnd {
		args, err := appArgs(m, g)
		if err != nil {
			return err
		}
		for i := 0; !result.Empty() && i < len(args); i++ {
			if err = r.applyGuard(args[i], result, equalities, discardCols); err != nil {
				return err
			}
		}
		return nil
	}
	for _, swap := range []bool{false, true} {
		d, err := r.notGroundEq(g, swap)
		if err != nil {
			return err
		}
		if d != nil {
			err = result.SubtractDoc(d)
			return errors.Join(err, dm.Deallocate(d))
		}
	}
	if basic && k == ast.ASTOpNot {
		e1, err := m.GetArg(g, 0)
		if err != nil {
			return err
		}
		sub, err := r.newUniverse()
		if err != nil {
			return err
		}
		empty := make([]bool, len(discardCols))
		if err = r.applyGuard(e1, sub, equalities, empty); err == nil {
			if err = result.Subtract(sub); err == nil {
				err = result.Simplify()
			}
		}
		return errors.Join(err, sub.Reset())
	}
	if basic && k == ast.ASTOpOr {
		args, err := appArgs(m, g)
		if err != nil {
			return err
		}
		sub, err := r.newUniverse()
		if err != nil {
			return err
		}
		for i := 0; err == nil && !sub.Empty() && i < len(args); i++ {
			// expr_ref arg = mk_not(m, g_i) dies at the end of the iteration.
			var arg ast.ASTHandle
			if arg, err = m.MkNotSimplified(args[i]); err == nil {
				var held ast.ASTRef
				if held, err = m.NewRef(arg); err == nil {
					err = r.applyGuard(arg, sub, equalities, discardCols)
					err = errors.Join(err, m.DelRef(&held))
				}
			}
		}
		if err == nil {
			err = result.Subtract(sub)
		}
		return errors.Join(err, sub.Reset())
	}
	if yes, err := isVar(m, g); err != nil {
		return err
	} else if yes {
		v, err := m.GetVarIndex(g)
		if err != nil {
			return err
		}
		idx := r.columnIdx(v)
		d, err := dm.AllocateX()
		if err != nil {
			return err
		}
		if err = dm.Set(d, idx, TBit1); err == nil {
			err = result.IntersectDoc(d)
		}
		return errors.Join(err, dm.Deallocate(d))
	}
	if basic && (k == ast.ASTOpEq || k == ast.ASTOpIff) {
		args, err := appArgs(m, g)
		if err != nil {
			return err
		}
		e1, e2 := args[0], args[1]
		sort, err := m.GetSort(e1)
		if err != nil {
			return err
		}
		boolean, err := m.MkBoolSort()
		if err != nil {
			return err
		}
		if sort == boolean {
			f1, err := m.MkNotSimplified(e1)
			if err != nil {
				return err
			}
			f2, err := m.MkNotSimplified(e2)
			if err != nil {
				return err
			}
			diff1, err := r.newUniverse()
			if err != nil {
				return err
			}
			diff2, err := r.newUniverse()
			if err != nil {
				return errors.Join(err, diff1.Reset())
			}
			err = r.applyGuard(e1, diff1, equalities, discardCols)
			if err == nil {
				err = r.applyGuard(f2, diff1, equalities, discardCols)
			}
			if err == nil {
				err = result.Subtract(diff1)
			}
			err = errors.Join(err, diff1.Reset())
			if err == nil {
				err = r.applyGuard(f1, diff2, equalities, discardCols)
			}
			if err == nil {
				err = r.applyGuard(e2, diff2, equalities, discardCols)
			}
			if err == nil {
				err = result.Subtract(diff2)
			}
			return errors.Join(err, diff2.Reset())
		}
		if k == ast.ASTOpEq {
			bv, err := r.plugin.isBV(e1)
			if err != nil {
				return err
			}
			if bv {
				ok, err := r.applyBVEq(e1, e2, discardCols, result)
				if err != nil || ok {
					return err
				}
			}
		}
	}
	// failure_case
	return udocGuardNotHandled(m, g)
}
