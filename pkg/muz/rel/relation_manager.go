package rel

// Source: src/muz/rel/dl_relation_manager.cpp finite relation/table signature
// and fact conversion paths.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

// RelationManager borrows an AST manager, owns stored finite predicate tables,
// and converts between finite-domain relation values and table elements.
type RelationManager struct {
	manager       *ast.ASTManager
	relations     map[ast.ASTHandle]*HashTable
	finite        map[ast.ASTHandle]FiniteTable
	explanations  map[ast.ASTHandle]*FiniteExplanationProduct
	relationLevel map[ast.ASTHandle]*RelationExplanationProduct
	saturated     map[ast.ASTHandle]bool
	// layout is m_relations' obj_map layout over every stored predicate;
	// reset and restrict walk it in table order.
	layout        *ast.ObjMap
	defaultFinite string
	// favouriteTable is the table plugin of a table_relation favourite
	// relation plugin other than the default table's (datalog.default_relation
	// tr_<table>), "" when there is none.
	favouriteTable          string
	lazyFinite              *LazySparseFiniteFactory // borrowed; context closes it after this manager
	checkedFinite           *FiniteCheckedFactory    // borrowed for one checked execution scope
	explanationUDoc         *UDocRelationFactory     // owned by the relation manager for explanation products
	explanationUDocOwned    bool
	explanationNumericKind  string
	explanationNumericCheck *smtcore.SMTSolverConfig
	// varSubst is the context var_subst used to ground interpreted
	// conditions (GroundCondition).
	varSubst *astrewriter.VarSubstitution
	// contextRewriter is context::get_rewriter() (borrowed), the
	// simplifier of default_table_filter_interpreted_fn.
	contextRewriter *astrewriter.TheoryRewriter
	closed          bool

	// requestedFinite is get_requested_predicate_kind for finite table
	// plugins (tr_sparse, tr_hashtable, tr_lazy_sparse).
	requestedFinite map[ast.ASTHandle]string
	// placeholders holds the predicates whose table the source has not
	// created yet (relation_manager_placeholder.go).
	placeholders map[ast.ASTHandle]bool

	// relationPlugins is m_relation_plugins past the table_relation
	// plugins; favouriteRelation is m_favourite_relation_plugin when it is
	// one of them; defaultRelation is context::default_relation().
	relationPlugins   []RelationPlugin
	favouriteRelation RelationPlugin
	defaultRelation   string
	// requestedRelation is get_requested_predicate_kind for relation
	// plugins.
	requestedRelation map[ast.ASTHandle]RelationPlugin
	// requestedTable is an explicitly requested table_relation kind.
	requestedTable map[ast.ASTHandle]string
	// arithElements interns the Int/Real relation elements the compiler
	// encodes (EncodeRelationElement); fnRoots holds the conditions the
	// relation plugins' filter functors keep (app_ref m_cond).
	arithElements []ast.ASTRef
	arithIndex    map[ast.ASTHandle]Element
	fnRoots       []ast.ASTRef
	// Relation kinds (family ids): nextRelationKind is
	// m_next_relation_fid, kindPlugins m_kind2plugin; the table_relation
	// plugins hold the first kinds. requestedKinds is m_pred_kinds.
	nextRelationKind     int
	kindPlugins          map[int]RelationPlugin
	pluginKinds          map[RelationPlugin]int
	tableRelationPlugins []*TableRelationPlugin
	requestedKinds       map[ast.ASTHandle]int
	// fnOwners holds the relations relation functors own (the full
	// relations of product_relation's join_fn).
	fnOwners []RelationBase
}

func NewRelationManager(manager *ast.ASTManager) (*RelationManager, error) {
	if manager == nil {
		return nil, fmt.Errorf("%w: nil AST manager", ErrTable)
	}
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	// context::m_var_subst (std_order = true) is a context member built
	// with the context; the relation manager owns it here.
	varSubst, err := astrewriter.NewVarSubstitution(manager, true)
	if err != nil {
		if varSubst != nil {
			err = errors.Join(err, varSubst.Close())
		}
		return nil, err
	}
	return &RelationManager{manager: manager, relations: make(map[ast.ASTHandle]*HashTable), finite: make(map[ast.ASTHandle]FiniteTable), explanations: make(map[ast.ASTHandle]*FiniteExplanationProduct), relationLevel: make(map[ast.ASTHandle]*RelationExplanationProduct), saturated: make(map[ast.ASTHandle]bool), layout: ast.NewObjMap(manager), defaultFinite: "hashtable", varSubst: varSubst}, nil
}

func (m *RelationManager) open() error {
	if m == nil || m.closed || m.manager == nil || m.relations == nil || m.finite == nil || m.explanations == nil || m.relationLevel == nil || m.saturated == nil {
		return ErrTable
	}
	return nil
}

func (m *RelationManager) MarkSaturated(predicate ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if m.explanations[predicate] == nil && m.relationLevel[predicate] == nil {
		if _, _, err := m.RelationPredicateSignature(predicate); err != nil {
			return err
		}
	}
	m.saturated[predicate] = true
	return nil
}

func (m *RelationManager) IsSaturated(predicate ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	return m.saturated[predicate], nil
}

// ResetSaturatedMarks mirrors relation_manager::reset_saturated_marks before
// a new relational query is compiled.
func (m *RelationManager) ResetSaturatedMarks() error {
	if err := m.open(); err != nil {
		return err
	}
	clear(m.saturated)
	return nil
}

// TryGetRelation returns the borrowed table stored for predicate.
func (m *RelationManager) TryGetRelation(predicate ast.ASTHandle) (*HashTable, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	if m.finite[predicate] != nil || m.explanations[predicate] != nil || m.relationLevel[predicate] != nil {
		return nil, false, ErrTable
	}
	table, found := m.relations[predicate]
	if table != nil {
		if err := table.open(); err != nil {
			return nil, false, err
		}
	}
	return table, found, nil
}

// GetRelation returns the borrowed table stored for predicate, creating and
// storing an empty finite table on first use.
func (m *RelationManager) GetRelation(predicate ast.ASTHandle) (*HashTable, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if m.finite[predicate] != nil || m.explanations[predicate] != nil || m.relationLevel[predicate] != nil {
		return nil, ErrTable
	}
	if table := m.relations[predicate]; table != nil {
		if err := table.open(); err != nil {
			return nil, err
		}
		return table, nil
	}
	_, signature, err := m.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	table := NewHashTable(signature)
	if err = m.StoreRelation(predicate, table); err != nil {
		return nil, err
	}
	return table, nil
}

// StoreRelation transfers ownership of table to the manager. The predicate is
// retained on first insertion and the previous table is replaced.
func (m *RelationManager) StoreRelation(predicate ast.ASTHandle, table *HashTable) error {
	return m.StoreFiniteRelation(predicate, table)
}

// noteRelation is m_relations.insert_if_not_there2(pred, 0) for the layout.
func (m *RelationManager) noteRelation(predicate ast.ASTHandle) error {
	return m.layout.Insert(predicate, [2]ast.ASTHandle{})
}

func (m *RelationManager) holdsRelation(predicate ast.ASTHandle) bool {
	_, a := m.explanations[predicate]
	_, b := m.relationLevel[predicate]
	_, c := m.finite[predicate]
	_, d := m.relations[predicate]
	return a || b || c || d
}

// retireRelation releases predicate's stored relations: deallocate, then
// dec_ref(pred), as restrict_predicates does. (reset_relations dec_refs
// first; g3's relation owners read the predicate while closing, so both
// paths close first.)
func (m *RelationManager) retireRelation(predicate ast.ASTHandle) (result error) {
	release := func(close func() error) bool {
		if err := close(); err != nil {
			result = errors.Join(result, err)
			return false
		}
		consumed, err := m.manager.DecRefConsumed(predicate)
		result = errors.Join(result, err)
		return consumed
	}
	if product, ok := m.explanations[predicate]; ok && release(product.Close) {
		delete(m.explanations, predicate)
	}
	if product, ok := m.relationLevel[predicate]; ok && release(product.Close) {
		delete(m.relationLevel, predicate)
	}
	if table, ok := m.finite[predicate]; ok && release(table.Close) {
		delete(m.finite, predicate)
	}
	if table, ok := m.relations[predicate]; ok && release(table.Close) {
		delete(m.relations, predicate)
	}
	return result
}

// ResetRelations is reset_relations: m_relations in table order.
func (m *RelationManager) ResetRelations() (result error) {
	if err := m.open(); err != nil {
		return err
	}
	for _, entry := range m.layout.Entries() {
		result = errors.Join(result, m.retireRelation(entry.Key))
		if !m.holdsRelation(entry.Key) {
			delete(m.saturated, entry.Key)
		}
	}
	if result == nil {
		m.layout.Reset()
	}
	result = errors.Join(result, m.retirePlaceholders(nil))
	return result
}

// DetachContextVarSubst hands context::m_var_subst to the caller, which
// destroys it with the context's members, after the engine (and this
// relation manager) that context::reset drops.
func (m *RelationManager) DetachContextVarSubst() *astrewriter.VarSubstitution {
	if m == nil {
		return nil
	}
	v := m.varSubst
	m.varSubst = nil
	return v
}

func (m *RelationManager) Close() (result error) {
	if m == nil {
		return ErrTable
	}
	if m.closed {
		return nil
	}
	result = m.ResetRelations()
	result = errors.Join(result, m.closeRelationPlugins())
	result = errors.Join(result, m.closeRelationElements())
	for i := len(m.fnRoots) - 1; i >= 0; i-- {
		result = errors.Join(result, m.manager.DelRef(&m.fnRoots[i]))
	}
	m.fnRoots = nil
	result = errors.Join(result, closeRelationBases(m.fnOwners))
	m.fnOwners = nil
	if len(m.relations) != 0 || len(m.finite) != 0 || len(m.explanations) != 0 || len(m.relationLevel) != 0 {
		return errors.Join(result, ErrTable)
	}
	if m.explanationUDoc != nil && m.explanationUDocOwned {
		m.explanationUDoc.Close()
	}
	m.explanationUDoc = nil
	m.explanationUDocOwned = false
	if m.varSubst != nil {
		result = errors.Join(result, m.varSubst.Close())
		m.varSubst = nil
	}
	m.closed = true
	m.manager = nil
	m.relations = nil
	m.finite = nil
	m.explanations = nil
	m.relationLevel = nil
	m.saturated = nil
	m.lazyFinite = nil
	m.checkedFinite = nil
	return result
}

func (m *RelationManager) sortDomain(sort ast.ASTHandle) (uint64, error) {
	size, err := m.manager.GetSortSize(sort)
	if err != nil {
		return 0, err
	}
	if !size.IsFinite() {
		return 0, fmt.Errorf("%w: relation sort is not a bounded finite domain", ErrTable)
	}
	domain, err := size.Size()
	if err != nil {
		return 0, err
	}
	if domain == 0 {
		return 0, fmt.Errorf("%w: relation sort has an empty domain", ErrTable)
	}
	return domain, nil
}

// PredicateSignature returns borrowed domain sorts plus their concrete table
// signature. The returned sort slice itself is caller-owned.
func (m *RelationManager) PredicateSignature(declaration ast.ASTHandle) ([]ast.ASTHandle, Signature, error) {
	if err := m.open(); err != nil {
		return nil, Signature{}, err
	}
	kind, err := m.manager.GetKind(declaration)
	if err != nil {
		return nil, Signature{}, err
	}
	if kind != ast.ASTFuncDecl {
		return nil, Signature{}, fmt.Errorf("%w: predicate is not a function declaration", ErrTable)
	}
	arity, err := m.manager.GetArity(declaration)
	if err != nil {
		return nil, Signature{}, err
	}
	sorts := make([]ast.ASTHandle, arity)
	domains := make([]uint64, arity)
	for i := uint32(0); i < arity; i++ {
		sorts[i], err = m.manager.GetDomain(declaration, i)
		if err != nil {
			return nil, Signature{}, err
		}
		domains[i], err = m.sortDomain(sorts[i])
		if err != nil {
			return nil, Signature{}, err
		}
	}
	signature, err := NewSignature(domains)
	return sorts, signature, err
}

func (m *RelationManager) bitVectorElement(value ast.ASTHandle) (number uint64, matched bool, resultErr error) {
	service := m.manager.NumeralService()
	numeral, err := service.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&numeral)) }()
	var width uint32
	matched, err = m.manager.BitVectorNumeralValue(value, &numeral, &width)
	if err != nil || !matched {
		return 0, matched, err
	}
	valid, err := service.IsUint64(&numeral)
	if err != nil {
		return 0, false, err
	}
	if !valid {
		return 0, false, fmt.Errorf("%w: bit-vector value is not a table element", ErrTable)
	}
	number, err = service.Uint64(&numeral)
	return number, true, err
}

// EncodeValue converts a concrete finite-domain value to its table element.
func (m *RelationManager) EncodeValue(sort, value ast.ASTHandle) (Element, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	domain, err := m.sortDomain(sort)
	if err != nil {
		return 0, err
	}
	actual, err := m.manager.GetSort(value)
	if err != nil {
		return 0, err
	}
	if actual != sort {
		return 0, fmt.Errorf("%w: relation value has the wrong sort", ErrTable)
	}
	number, matched, err := m.manager.DatalogNumeralValue(value)
	if err != nil {
		return 0, err
	}
	if !matched {
		number, matched, err = m.bitVectorElement(value)
		if err != nil {
			return 0, err
		}
		if !matched && m.manager.IsTrue(value) {
			number, matched = 1, true
		} else if !matched && m.manager.IsFalse(value) {
			number, matched = 0, true
		}
	}
	if !matched || number >= domain {
		return 0, fmt.Errorf("%w: expression is not a finite relation value", ErrTable)
	}
	return Element(number), nil
}

func (m *RelationManager) EncodeFact(sorts, values []ast.ASTHandle) (Fact, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if len(sorts) != len(values) {
		return nil, fmt.Errorf("%w: relation fact arity mismatch", ErrTable)
	}
	fact := make(Fact, len(values))
	for i := range values {
		value, err := m.EncodeValue(sorts[i], values[i])
		if err != nil {
			return nil, err
		}
		fact[i] = value
	}
	return fact, nil
}

// DecodeValue returns one owning AST reference for the table element.
func (m *RelationManager) DecodeValue(sort ast.ASTHandle, value Element) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	domain, err := m.sortDomain(sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if uint64(value) >= domain {
		return ast.ASTRef{}, fmt.Errorf("%w: table element outside relation sort", ErrTable)
	}
	// Reserve the owner before constructing the numeral. A following failed
	// reference allocation would otherwise leave a live unowned AST node.
	reference, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	expression, cause := m.manager.MkDatalogNumeral(uint64(value), sort)
	cause = errors.Join(cause, m.manager.SetRef(reference, expression))
	if cause != nil {
		return ast.ASTRef{}, finishDecodedFact(m.manager, []ast.ASTRef{reference}, cause)
	}
	return reference, nil
}

// DecodeFact returns one owning reference per element. On failure it releases
// every reference already acquired.
func (m *RelationManager) DecodeFact(sorts []ast.ASTHandle, fact Fact) (_ []ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if len(sorts) != len(fact) {
		return nil, fmt.Errorf("%w: table fact arity mismatch", ErrTable)
	}
	result := make([]ast.ASTRef, 0, len(fact))
	defer func() {
		if resultErr != nil {
			resultErr = finishDecodedFact(m.manager, result, resultErr)
		}
	}()
	for i, value := range fact {
		reference, err := m.DecodeValue(sorts[i], value)
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	return result, nil
}
