package ast

// Source: src/ast/format.cpp:119-145; src/ast/recurse_expr.h:28-60;
// src/ast/recurse_expr_def.h:25-107.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
)

// Formatting recursion borrows node references. The cache is lookup-only;
// traversal order comes exclusively from todo and each application's arguments.
type astFormatFlatFrame struct {
	next  *astFormatFlatFrame
	owner *ASTManager
	cache map[astNodeRef]astNodeRef
	// small holds the cache while it has at most astFormatFlatSmall
	// entries; cache is built from it on the next insert.
	small              []astFormatFlatEntry
	todo               []astNodeRef
	args               []astNodeRef // scratch: one application's arguments
	results            []ASTHandle
	family             FamilyID
	header             uint64
	invocation         uint64
	bodyCause          error
	sourceDone, active bool
	// dense says the frame owns owner.formatFlatDense instead of small/cache.
	dense bool
}

type astFormatFlatDenseEntry struct {
	invocation uint64
	generation uint32
	value      astNodeRef
}

func (f *astFormatFlatFrame) bytes() uint64 { return f.header }
func (f *astFormatFlatFrame) find(key astNodeRef) (astNodeRef, bool, error) {
	if e := f.owner.validNode(key); e != nil {
		return astNodeRef{}, false, e
	}
	if f.dense {
		if d := f.owner.formatFlatDense; uint64(key.slot) < uint64(len(d)) {
			if x := &d[key.slot]; x.invocation == f.invocation && x.generation == key.generation {
				return x.value, true, nil
			}
		}
		return astNodeRef{}, false, nil
	}
	if f.cache == nil {
		for i := range f.small {
			if f.small[i].key == key {
				return f.small[i].value, true, nil
			}
		}
		return astNodeRef{}, false, nil
	}
	value, found := f.cache[key]
	return value, found, nil
}

type astFormatFlatEntry struct{ key, value astNodeRef }

const astFormatFlatSmall = 32

func (f *astFormatFlatFrame) insert(key, value astNodeRef) error {
	if e := f.owner.validNode(key); e != nil {
		return e
	}
	if f.dense {
		m := f.owner
		if uint64(key.slot) >= uint64(len(m.formatFlatDense)) {
			n := 2 * len(m.formatFlatDense)
			if n <= int(key.slot) {
				n = int(key.slot) + 256
			}
			grown := make([]astFormatFlatDenseEntry, n)
			copy(grown, m.formatFlatDense)
			m.formatFlatDense = grown
		}
		m.formatFlatDense[key.slot] = astFormatFlatDenseEntry{invocation: f.invocation, generation: key.generation, value: value}
		return nil
	}
	if f.cache == nil {
		for i := range f.small {
			if f.small[i].key == key {
				f.small[i].value = value
				return nil
			}
		}
		if len(f.small) < astFormatFlatSmall {
			f.small = append(f.small, astFormatFlatEntry{key, value})
			return nil
		}
		f.cache = make(map[astNodeRef]astNodeRef, 2*astFormatFlatSmall)
		for _, entry := range f.small {
			f.cache[entry.key] = entry.value
		}
		f.small = nil
	}
	f.cache[key] = value
	return nil
}
func (f *astFormatFlatFrame) sourceDestroy() {
	if f.sourceDone {
		return
	}
	// Borrowed pointer elements have no AST INC/DEC ownership.
	f.results = nil
	f.todo = nil
	f.args = nil
	if f.dense {
		f.dense = false
		if f.owner.formatFlatDenseOwner == f {
			f.owner.formatFlatDenseOwner = nil
		}
	}
	f.cache = nil
	f.small = nil
	f.sourceDone = true
	f.active = false
}
func (m *ASTManager) formatFramesBytes() uint64 {
	var n uint64
	for f := m.formatFrames; f != nil; f = f.next {
		n += f.bytes()
	}
	return n
}
func (m *ASTManager) releaseFormatFrames() {
	for f := m.formatFrames; f != nil; {
		next := f.next
		f.sourceDestroy()
		m.BudgetField.Release(f.header)
		f.header = 0
		f = next
	}
	m.formatFrames = nil
}
func (m *ASTManager) flatVisit(f *astFormatFlatFrame, n astNodeRef) (astNodeRef, error) {
	if m.nodeAt(n.slot).kind != ASTApp {
		return astNodeRef{}, ErrASTDomain
	} // selected source UNREACHABLE
	a := m.appAt(n.slot)
	id, kind, e := m.appIdentity(a)
	if e != nil {
		return astNodeRef{}, e
	}
	if id == f.family && kind == ASTFormatLineBreak {
		name, e := m.Symbols.Intern(" ")
		if e != nil {
			return astNodeRef{}, e
		}
		p := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
		return m.formatApp(ASTFormatString, &p, nil)
	}
	if id == f.family && kind == ASTFormatLineBreakExt {
		info, e := m.funcInfo(m.declAt(a.decl.slot).info)
		if e != nil {
			return astNodeRef{}, e
		}
		p, e := m.infoParameter(info, 0)
		if e != nil {
			return astNodeRef{}, e
		}
		value, e := m.parameter(p)
		if e != nil {
			return astNodeRef{}, e
		}
		if value.Kind != ASTParameterSymbol {
			return astNodeRef{}, ErrASTDomain
		}
		// symbol bare_str access is performed before actual mk_string symbol ctor.
		name, e := m.Symbols.InternBorrowedSymbolCString(value.symbol)
		if e != nil {
			return astNodeRef{}, e
		}
		parameter := astParameterRecord{Kind: ASTParameterSymbol, symbol: name}
		return m.formatApp(ASTFormatString, &parameter, nil)
	}
	if id == f.family && kind == ASTFormatChoice {
		return m.readInternal(a.args, 0)
	}
	if id == f.family && m.flatUnchanged(f, a) {
		return n, nil
	}
	return m.mkAppArguments(a.decl, SliceArguments(f.results))
}

// flatUnchanged reports that every flattened argument of format application
// a (f.args, read by formatFlat) is the argument itself. Source mk_app then
// finds the existing node through the hash-cons table: it creates no node,
// takes no id and writes no [mk-app] trace line, so the node itself is the
// result. The general path is kept when the app_ground debug trace, which
// mk_app writes for found nodes too, is enabled, and for declarations whose
// mk_app reshapes the arguments.
func (m *ASTManager) flatUnchanged(f *astFormatFlatFrame, a *astAppRecord) bool {
	if uint32(len(f.results)) != a.count || uint32(len(f.args)) != a.count {
		return false
	}
	for i, h := range f.results {
		if r := f.args[i]; h.origin != m.origin || h.slot != r.slot || h.generation != r.generation {
			return false
		}
	}
	flags, e := m.funcFlags(a.decl)
	if e != nil || flags.LeftAssociative || flags.RightAssociative || flags.Chainable || m.declAt(a.decl.slot).arity != a.count {
		return false
	}
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	return !tags.ShouldTrace("app_ground")
}
func (m *ASTManager) formatFlat(root astNodeRef, documents *astFormatDocuments) (result astNodeRef, err error) {
	family, e := m.formatFamilyID()
	if e != nil {
		return astNodeRef{}, e
	}
	if m.formatFrameSerial == math.MaxUint64 {
		return astNodeRef{}, arena.ErrBounds
	}
	header := uint64(reflect.TypeOf(astFormatFlatFrame{}).Size())
	if e = m.BudgetField.Reserve(header); e != nil {
		return astNodeRef{}, e
	}
	m.formatFrameSerial++
	f := &astFormatFlatFrame{owner: m, family: family, header: header, invocation: m.formatFrameSerial, active: true, next: m.formatFrames}
	m.formatFrames = f
	defer func() {
		if err != nil && f.bodyCause == nil {
			f.bodyCause = err
		}
		f.sourceDestroy()
		if err == nil {
			m.formatFrames = f.next
			m.BudgetField.Release(f.header)
			f.header = 0
		}
	}()
	if m.formatFlatDenseOwner == nil {
		m.formatFlatDenseOwner = f
		f.dense = true
	} else {
		f.small = make([]astFormatFlatEntry, 0, 8)
	}
	f.todo = append(f.todo, root)
	for len(f.todo) != 0 {
		n := f.todo[len(f.todo)-1]
		_, found, e := f.find(n)
		if e != nil {
			return astNodeRef{}, e
		}
		if found {
			f.todo = f.todo[:len(f.todo)-1]
			continue
		}
		if e = m.validNode(n); e != nil {
			return astNodeRef{}, e
		}
		if m.isGroupChoice(n) {
			// flat(choice(flat(x), x)) is flat(x), the first argument. Source
			// recurses into both arguments first, but flattening the already
			// flat first argument and x rebuilds only nodes inside flat(x),
			// which exist: their mk_app calls create, number and trace no node.
			f.todo = f.todo[:len(f.todo)-1]
			result, e = documents.ownNode(m.readInternal(m.appAt(n.slot).args, 0))
			if e != nil {
				return astNodeRef{}, e
			}
			if e = f.insert(n, result); e != nil {
				return astNodeRef{}, e
			}
			continue
		}
		visited := true
		switch m.nodeAt(n.slot).kind {
		case ASTApp:
			a := m.appAt(n.slot)
			f.args, e = m.readInternalAll(a.args, a.count, f.args[:0])
			if e != nil {
				return astNodeRef{}, e
			}
			for _, child := range f.args {
				_, found, e = f.find(child)
				if e != nil {
					return astNodeRef{}, e
				}
				if !found {
					f.todo = append(f.todo, child)
					visited = false
				}
			}
		case ASTQuantifier:
			child := m.quantifierAt(n.slot).body
			_, found, e = f.find(child)
			if e != nil {
				return astNodeRef{}, e
			}
			if !found {
				f.todo = append(f.todo, child)
				visited = false
			}
		}
		if !visited {
			continue
		}
		f.todo = f.todo[:len(f.todo)-1]
		if m.nodeAt(n.slot).kind != ASTApp {
			return astNodeRef{}, ErrASTDomain
		}
		f.results = f.results[:0]
		a := m.appAt(n.slot)
		f.args, e = m.readInternalAll(a.args, a.count, f.args[:0])
		if e != nil {
			return astNodeRef{}, e
		}
		for _, child := range f.args {
			value, found, e := f.find(child)
			if e != nil {
				return astNodeRef{}, e
			}
			if !found {
				return astNodeRef{}, ErrASTDomain
			}
			f.results = append(f.results, m.external(value))
		}
		result, e = documents.ownNode(m.flatVisit(f, n))
		if e != nil {
			return astNodeRef{}, e
		}
		if e = f.insert(n, result); e != nil {
			return astNodeRef{}, e
		}
	}
	result, found, e := f.find(root)
	if e != nil {
		return astNodeRef{}, e
	}
	if !found {
		return astNodeRef{}, ErrASTDomain
	}
	return result, nil
}
func (m *ASTManager) FormatFlat(root ASTHandle) (document ASTHandle, resultErr error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	documents := &astFormatDocuments{manager: f}
	defer documents.finish(&document, &resultErr)
	defer f.unlockFormat()
	r, e := f.requireExpr(root)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = documents.borrowRoot(r); e != nil {
		return ASTHandle{}, e
	}
	r, e = f.formatFlat(r, documents)
	return f.external(r), e
}
func (m *ASTManager) FormatGroup(root ASTHandle) (document ASTHandle, resultErr error) {
	f, e := m.lockFormat()
	if e != nil {
		return ASTHandle{}, e
	}
	documents := &astFormatDocuments{manager: f}
	defer documents.finish(&document, &resultErr)
	defer f.unlockFormat()
	r, e := f.requireExpr(root)
	if e != nil {
		return ASTHandle{}, e
	}
	if e = documents.borrowRoot(r); e != nil {
		return ASTHandle{}, e
	}
	flat, e := f.formatFlat(r, documents)
	if e != nil {
		return ASTHandle{}, e
	}
	args := [2]ASTHandle{f.external(flat), root}
	r, e = f.formatApp(ASTFormatChoice, nil, args[:])
	if e == nil && r.slot != 0 {
		f.markGroupChoice(r)
	}
	return f.external(r), e
}

func (m *ASTManager) markGroupChoice(r astNodeRef) {
	if uint64(r.slot) >= uint64(len(m.formatGroupChoice)) {
		n := 2 * len(m.formatGroupChoice)
		if n <= int(r.slot) {
			n = int(r.slot) + 256
		}
		grown := make([]uint32, n)
		copy(grown, m.formatGroupChoice)
		m.formatGroupChoice = grown
	}
	m.formatGroupChoice[r.slot] = r.generation
}

// isGroupChoice reports a live choice node marked by FormatGroup, with the
// app_ground debug trace (written by every mk_app) disabled.
func (m *ASTManager) isGroupChoice(r astNodeRef) bool {
	if uint64(r.slot) >= uint64(len(m.formatGroupChoice)) || m.formatGroupChoice[r.slot] != r.generation {
		return false
	}
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	return !tags.ShouldTrace("app_ground")
}
