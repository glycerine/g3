package interp

// Source checkpoint: src/interp/iz3base.cpp and src/interp/iz3base.h core state.

import (
	"errors"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type baseRanges struct {
	rangeValue    ScopeRange
	scope         ScopeRange
	scopeComputed bool
}

// Base is the retained shared interpolation state. It borrows Manager and owns
// references for constraints, theory formulas, and memoized simplifications.
type Base struct {
	self        *Base
	manager     *Manager
	scopes      *Scopes
	constraints []ast.ASTHandle
	theory      []ast.ASTHandle
	// The source members cnsts, theory, ast_ranges_hash, simplify_memo and
	// frame_map hold their ASTs (ast_r); ~iz3base destroys them in reverse
	// declaration order: frame_map, simplify_memo, ast_ranges_hash, then
	// theory and cnsts first to last. The tables keep the source's hash
	// layout so they release in its order.
	cnstRefs      []ast.ASTRef
	theoryRefs    iz3RefVector
	astRangesRefs iz3RefTable
	memoRefs      iz3RefTable
	frameRefs     iz3RefTable
	symbolRange   map[ast.ASTHandle]ScopeRange
	astRanges     map[ast.ASTHandle]*baseRanges
	simplified    map[ast.ASTHandle]ast.ASTHandle
	frames        map[ast.ASTHandle]int
	weak          bool
	solver        solver.Factory
	evaluator     solver.ConsequenceEvaluatorFactory
	closed        bool
}

func (b *Base) ConfigureSolver(factory solver.Factory, evaluator solver.ConsequenceEvaluatorFactory) error {
	if err := b.open(); err != nil {
		return err
	}
	if factory == nil {
		return ast.ErrASTHandle
	}
	b.solver, b.evaluator = factory, evaluator
	return nil
}

func NewBase(manager *Manager, constraints []ast.ASTHandle, parents []int, theory []ast.ASTHandle) (result *Base, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	scopes, err := NewScopes(parents)
	if err != nil {
		return nil, err
	}
	b := newBaseState(manager, scopes)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, b.Close())
		}
	}()
	if err = b.initialize(constraints, theory); err != nil {
		return nil, err
	}
	return b, nil
}

// NewBaseFromParts is the iz3base constructor over a vector of frame parts
// (initialize(const std::vector<std::vector<ast> >&, ...)): cnsts[i] is
// made the conjunction of the frame's parts once per part (it stays null
// for a frame without parts), the ranges are added for the parts, not the
// conjunctions, and frame_map maps each part to its frame.
func NewBaseFromParts(manager *Manager, parts [][]ast.ASTHandle, parents []int, theory []ast.ASTHandle) (result *Base, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	scopes, err := NewScopes(parents)
	if err != nil {
		return nil, err
	}
	b := newBaseState(manager, scopes)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, b.Close())
		}
	}()
	b.constraints = make([]ast.ASTHandle, len(parts))
	b.cnstRefs = make([]ast.ASTRef, len(parts))
	b.theory = append([]ast.ASTHandle(nil), theory...)
	for _, formula := range b.theory {
		if err = b.theoryRefs.push(manager.ast, formula); err != nil {
			return nil, err
		}
	}
	for i := range parts {
		for _, formula := range parts[i] {
			conjunction, e := manager.Make(OperatorAnd, parts[i]...)
			if e != nil {
				return nil, e
			}
			if e = b.setConstraint(i, conjunction); e != nil {
				return nil, e
			}
			if e = b.addFrameRange(i, formula); e != nil {
				return nil, e
			}
			if e = b.setFrame(formula, i); e != nil {
				return nil, e
			}
		}
	}
	for _, formula := range b.theory {
		if err = b.addFrameRange(ScopeBottom, formula); err != nil {
			return nil, err
		}
		if err = b.addFrameRange(ScopeTop, formula); err != nil {
			return nil, err
		}
		if err = b.setFrame(formula, math.MaxInt); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func newBaseState(manager *Manager, scopes *Scopes) *Base {
	b := &Base{manager: manager, scopes: scopes, symbolRange: make(map[ast.ASTHandle]ScopeRange), astRanges: make(map[ast.ASTHandle]*baseRanges), simplified: make(map[ast.ASTHandle]ast.ASTHandle), frames: make(map[ast.ASTHandle]int)}
	b.self = b
	return b
}

// setConstraint is cnsts[i] = formula (ast_r::operator=).
func (b *Base) setConstraint(i int, formula ast.ASTHandle) error {
	ref, err := b.manager.ast.NewRef(formula)
	if err != nil {
		return err
	}
	old := b.cnstRefs[i]
	b.cnstRefs[i] = ref
	b.constraints[i] = formula
	if old != (ast.ASTRef{}) {
		return b.manager.ast.DelRef(&old)
	}
	return nil
}

// setFrame is frame_map[formula] = frame.
func (b *Base) setFrame(formula ast.ASTHandle, frame int) error {
	if err := b.frameRefs.insert(b.manager.ast, formula); err != nil {
		return err
	}
	b.frames[formula] = frame
	return nil
}

func (b *Base) open() error {
	if b == nil || b.self != b || b.closed || b.manager == nil {
		return ast.ErrASTHandle
	}
	return b.manager.open()
}

func (b *Base) hold(refs *[]ast.ASTRef, handle ast.ASTHandle) error {
	ref, err := b.manager.ast.NewRef(handle)
	if err != nil {
		return err
	}
	*refs = append(*refs, ref)
	return nil
}

// initialize is iz3base::initialize(const std::vector<ast>&, ...): the
// ranges of the constraints and the theory, then frame_map.
func (b *Base) initialize(constraints, theory []ast.ASTHandle) error {
	b.constraints = append([]ast.ASTHandle(nil), constraints...)
	b.theory = append([]ast.ASTHandle(nil), theory...)
	for _, formula := range b.constraints {
		if err := b.hold(&b.cnstRefs, formula); err != nil {
			return err
		}
	}
	for _, formula := range b.theory {
		if err := b.theoryRefs.push(b.manager.ast, formula); err != nil {
			return err
		}
	}
	for i, formula := range b.constraints {
		if err := b.addFrameRange(i, formula); err != nil {
			return err
		}
	}
	for _, formula := range b.theory {
		if err := b.addFrameRange(ScopeBottom, formula); err != nil {
			return err
		}
		if err := b.addFrameRange(ScopeTop, formula); err != nil {
			return err
		}
	}
	for i, formula := range b.constraints {
		if err := b.setFrame(formula, i); err != nil {
			return err
		}
	}
	for _, formula := range b.theory {
		if err := b.setFrame(formula, math.MaxInt); err != nil {
			return err
		}
	}
	return nil
}

func (b *Base) Close() (result error) {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	manager := b.manager.ast
	result = errors.Join(result, b.frameRefs.clear(manager))
	result = errors.Join(result, b.memoRefs.clear(manager))
	result = errors.Join(result, b.astRangesRefs.clear(manager))
	result = errors.Join(result, b.theoryRefs.clear(manager))
	for i := range b.cnstRefs {
		if b.cnstRefs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&b.cnstRefs[i]))
		}
	}
	b.cnstRefs = nil
	b.closed = true
	return result
}

func (b *Base) Manager() (*Manager, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return b.manager, nil
}

func (b *Base) Scopes() (*Scopes, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return b.scopes, nil
}

func (b *Base) SetOption(name, value string) error {
	if err := b.open(); err != nil {
		return err
	}
	if name == "weak" && value == "1" {
		b.weak = true
	}
	return nil
}

func (b *Base) WeakMode() (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	return b.weak, nil
}

// astRangeEntry is ast_ranges_hash[handle]; the table entry holds the key.
func (b *Base) astRangeEntry(handle ast.ASTHandle) (*baseRanges, error) {
	if err := b.astRangesRefs.insert(b.manager.ast, handle); err != nil {
		return nil, err
	}
	entry := b.astRanges[handle]
	if entry == nil {
		entry = &baseRanges{rangeValue: EmptyScopeRange()}
		b.astRanges[handle] = entry
	}
	return entry, nil
}

func (b *Base) addFrameRange(frame int, term ast.ASTHandle) error {
	entry, err := b.astRangeEntry(term)
	if err != nil {
		return err
	}
	inside, err := b.scopes.InRange(frame, entry.rangeValue)
	if err != nil {
		return err
	}
	if inside {
		return nil
	}
	if err = b.scopes.RangeAdd(frame, &entry.rangeValue); err != nil {
		return err
	}
	count, err := b.manager.NumArgs(term)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		child, e := b.manager.Arg(term, i)
		if e != nil {
			return e
		}
		if e = b.addFrameRange(frame, child); e != nil {
			return e
		}
	}
	if op, err := b.manager.Operator(term); err != nil {
		return err
	} else if op == OperatorUninterpreted {
		declaration, e := b.manager.Symbol(term)
		if e != nil {
			return e
		}
		r, found := b.symbolRange[declaration]
		if !found {
			r = EmptyScopeRange()
		}
		if e = b.scopes.RangeAdd(frame, &r); e != nil {
			return e
		}
		b.symbolRange[declaration] = r
	}
	return nil
}

func (b *Base) ASTRange(term ast.ASTHandle) (ScopeRange, error) {
	if err := b.open(); err != nil {
		return ScopeRange{}, err
	}
	entry, err := b.astRangeEntry(term)
	if err != nil {
		return ScopeRange{}, err
	}
	return entry.rangeValue, nil
}

func (b *Base) SymbolRange(declaration ast.ASTHandle) (ScopeRange, error) {
	if err := b.open(); err != nil {
		return ScopeRange{}, err
	}
	r, found := b.symbolRange[declaration]
	if !found {
		return EmptyScopeRange(), nil
	}
	return r, nil
}

func (b *Base) ASTScope(term ast.ASTHandle) (ScopeRange, error) {
	if err := b.open(); err != nil {
		return ScopeRange{}, err
	}
	entry, err := b.astRangeEntry(term)
	if err != nil {
		return ScopeRange{}, err
	}
	if entry.scopeComputed {
		return entry.scope, nil
	}
	rangeValue := FullScopeRange()
	count, err := b.manager.NumArgs(term)
	if err != nil {
		return ScopeRange{}, err
	}
	for i := uint32(0); i < count; i++ {
		child, e := b.manager.Arg(term, i)
		if e != nil {
			return ScopeRange{}, e
		}
		childScope, e := b.ASTScope(child)
		if e != nil {
			return ScopeRange{}, e
		}
		rangeValue, e = b.scopes.RangeGLB(rangeValue, childScope)
		if e != nil {
			return ScopeRange{}, e
		}
	}
	op, err := b.manager.Operator(term)
	if err != nil {
		return ScopeRange{}, err
	}
	if op == OperatorUninterpreted && (!b.scopes.TreeMode() || count == 0) {
		declaration, e := b.manager.Symbol(term)
		if e != nil {
			return ScopeRange{}, e
		}
		symbolScope, e := b.SymbolRange(declaration)
		if e != nil {
			return ScopeRange{}, e
		}
		rangeValue, e = b.scopes.RangeGLB(rangeValue, symbolScope)
		if e != nil {
			return ScopeRange{}, e
		}
	}
	entry.scope, entry.scopeComputed = rangeValue, true
	return rangeValue, nil
}

func (b *Base) IsLocal(term ast.ASTHandle) (bool, error) {
	r, err := b.ASTScope(term)
	return !r.IsEmpty(), err
}

func (b *Base) FrameOfAssertion(assertion ast.ASTHandle) (int, error) {
	if err := b.open(); err != nil {
		return 0, err
	}
	frame, found := b.frames[assertion]
	if !found {
		return 0, iz3Exception("unknown assertion")
	}
	return frame, nil
}

func (b *Base) ProofCheckAssumption(scope ScopeRange) (ast.ASTHandle, error) {
	if err := b.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if scope.Hi < 0 || scope.Hi >= len(b.constraints) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	values := append([]ast.ASTHandle(nil), b.theory...)
	values = append(values, b.constraints[scope.Hi])
	return b.manager.Make(OperatorAnd, values...)
}

func (b *Base) Print(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	formula, err := b.manager.Make(OperatorAnd, b.constraints...)
	if err != nil {
		return err
	}
	return b.manager.PrintSATProblem(out, formula)
}
