package cmd_context

// Source: src/util/dictionary.h dictionary<T> (map<symbol, T,
// symbol_hash_proc, symbol_eq_proc>) over src/util/hashtable.h
// core_hashtable; src/cmd_context/cmd_context.cpp func_decls,
// m_func_decls and m_psort_decls. cmd_context::complete_model iterates both
// dictionaries in table order, which depends on the symbol hashes and on the
// history of insertions and removals; this type reproduces that layout.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

const (
	symbolDictionaryInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	symbolDictionarySmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

type symbolDictionaryState uint8

const (
	symbolDictionaryFree symbolDictionaryState = iota
	symbolDictionaryDeleted
	symbolDictionaryUsed
)

// funcDecls is cmd_context.cpp func_decls: one declaration, or from the
// second insertion on an obj_hashtable<func_decl> (kept until it empties).
type funcDecls struct {
	// Rows use one-based indices into functionTrail. A scope erases its
	// dictionary entries before truncating the trail. Overloads carry their
	// row in ObjMapEntry.Num and keep the native table iteration order.
	singleRow int
	set       *ast.ObjMap
}

func (f *funcDecls) empty() bool { return f.set == nil && f.singleRow == 0 }

func (f *funcDecls) insert(m *ast.ASTManager, trail []declaration) error {
	row := len(trail)
	d := trail[row-1].handle
	switch {
	case f.empty():
		f.singleRow = row
	case f.set == nil:
		set := ast.NewObjMap(m)
		if err := set.InsertNum(trail[f.singleRow-1].handle, uint64(f.singleRow)); err != nil {
			return err
		}
		if err := set.InsertNum(d, uint64(row)); err != nil {
			return err
		}
		f.singleRow = 0
		f.set = set
	default:
		return f.set.InsertNum(d, uint64(row))
	}
	return nil
}

func (f *funcDecls) erase(trail []declaration, d ast.ASTHandle) error {
	if f.set == nil {
		if f.singleRow != 0 && trail[f.singleRow-1].handle == d {
			f.singleRow = 0
		}
		return nil
	}
	if err := f.set.Erase(d); err != nil {
		return err
	}
	if f.set.Len() == 0 {
		f.set = nil
	}
	return nil
}

// entries follows the native overload set iteration order.
func (f *funcDecls) entries(trail []declaration) []ast.ASTHandle {
	if f.set == nil {
		if f.singleRow == 0 {
			return nil
		}
		return []ast.ASTHandle{trail[f.singleRow-1].handle}
	}
	return f.set.FirstKeys(f.set.Len())
}

// psortEntry is the psort_decl payload complete_model reads.
type psortEntry struct {
	userDecl bool
}

type symbolDictionaryEntry struct {
	name  string
	hash  uint32
	state symbolDictionaryState
	funcs *funcDecls
	psort *psortEntry
}

type symbolDictionary struct {
	table      []symbolDictionaryEntry
	size       int
	numDeleted int
}

func newSymbolDictionary() *symbolDictionary {
	return &symbolDictionary{table: make([]symbolDictionaryEntry, symbolDictionaryInitialCapacity)}
}

// symbolHash is symbol::hash of a string symbol: the string_hash(s, 17) the
// internal symbol table stores with its spelling.
func symbolHash(name string) uint32 { return uhash.String(name, 17) }

func symbolDictionaryMove(source, target []symbolDictionaryEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != symbolDictionaryUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == symbolDictionaryFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == symbolDictionaryFree {
				target[i], placed = entry, true
			}
		}
	}
}

// insertIfNotThere is map::insert_if_not_there2(k, T()).
func (d *symbolDictionary) insertIfNotThere(name string) *symbolDictionaryEntry {
	if (d.size+d.numDeleted)<<2 > len(d.table)*3 {
		target := make([]symbolDictionaryEntry, len(d.table)<<1)
		symbolDictionaryMove(d.table, target)
		d.table, d.numDeleted = target, 0
	}
	hash := symbolHash(name)
	mask := uint32(len(d.table) - 1)
	begin := int(hash & mask)
	deleted := -1
	visit := func(i int) *symbolDictionaryEntry {
		curr := &d.table[i]
		switch curr.state {
		case symbolDictionaryUsed:
			if curr.hash == hash && curr.name == name {
				return curr
			}
		case symbolDictionaryFree:
			n := i
			if deleted >= 0 {
				n = deleted
				d.numDeleted--
			}
			d.table[n] = symbolDictionaryEntry{name: name, hash: hash, state: symbolDictionaryUsed}
			d.size++
			return &d.table[n]
		default:
			deleted = i
		}
		return nil
	}
	for i := begin; i < len(d.table); i++ {
		if e := visit(i); e != nil {
			return e
		}
	}
	for i := 0; i < begin; i++ {
		if e := visit(i); e != nil {
			return e
		}
	}
	return nil // UNREACHABLE: the table always has a free slot
}

func (d *symbolDictionary) index(name string) int {
	hash := symbolHash(name)
	mask := uint32(len(d.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(d.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &d.table[i]
			if curr.state == symbolDictionaryUsed {
				if curr.hash == hash && curr.name == name {
					return i
				}
			} else if curr.state == symbolDictionaryFree {
				return -1
			}
		}
	}
	return -1
}

func (d *symbolDictionary) find(name string) *symbolDictionaryEntry {
	if i := d.index(name); i >= 0 {
		return &d.table[i]
	}
	return nil
}

// erase is map::erase (core_hashtable::remove).
func (d *symbolDictionary) erase(name string) {
	i := d.index(name)
	if i < 0 {
		return
	}
	next := i + 1
	if next == len(d.table) {
		next = 0
	}
	d.size--
	if d.table[next].state == symbolDictionaryFree {
		d.table[i] = symbolDictionaryEntry{}
		return
	}
	d.table[i] = symbolDictionaryEntry{state: symbolDictionaryDeleted}
	d.numDeleted++
	if d.numDeleted > d.size && d.numDeleted > symbolDictionarySmallCapacity {
		target := make([]symbolDictionaryEntry, len(d.table))
		symbolDictionaryMove(d.table, target)
		d.table, d.numDeleted = target, 0
	}
}

// reset is core_hashtable::reset, which halves a mostly free large table.
func (d *symbolDictionary) reset() {
	if d.size == 0 && d.numDeleted == 0 {
		return
	}
	overhead := 0
	for i := range d.table {
		if d.table[i].state != symbolDictionaryFree {
			d.table[i] = symbolDictionaryEntry{}
		} else {
			overhead++
		}
	}
	if len(d.table) > 16 && overhead<<2 > len(d.table)*3 {
		d.table = make([]symbolDictionaryEntry, len(d.table)>>1)
	}
	d.size, d.numDeleted = 0, 0
}

// entries lists the used entries in iteration (table) order.
func (d *symbolDictionary) entries() []*symbolDictionaryEntry {
	out := make([]*symbolDictionaryEntry, 0, d.size)
	for i := range d.table {
		if d.table[i].state == symbolDictionaryUsed {
			out = append(out, &d.table[i])
		}
	}
	return out
}

// Context hooks: m_func_decls follows c.functions, and m_psort_decls with
// its single m_psort_decls_stack follows the sort and sort-constructor
// declarations.

func (c *Context) funcDictionary() *symbolDictionary {
	if c.funcDecls == nil {
		c.funcDecls = newSymbolDictionary()
	}
	return c.funcDecls
}

func (c *Context) psortDictionary() *symbolDictionary {
	if c.psortDecls == nil {
		c.psortDecls = newSymbolDictionary()
	}
	return c.psortDecls
}

// recordFuncDecl is the m_func_decls part of cmd_context::insert(s, f).
func (c *Context) recordFuncDecl(row declaration) error {
	c.functionTrail = append(c.functionTrail, row)
	e := c.funcDictionary().insertIfNotThere(row.name)
	if e.funcs == nil {
		e.funcs = &funcDecls{}
	}
	return e.funcs.insert(c.manager, c.functionTrail)
}

// eraseFuncDecl is erase_func_decl_core(s, f).
func (c *Context) eraseFuncDecl(name string, d ast.ASTHandle) error {
	dict := c.funcDictionary()
	e := dict.find(name)
	if e == nil || e.funcs == nil {
		return nil
	}
	if err := e.funcs.erase(c.functionTrail, d); err != nil {
		return err
	}
	if e.funcs.empty() {
		dict.erase(name)
	}
	return nil
}

// recordPSortDecl is the m_psort_decls part of cmd_context::insert(s, p).
func (c *Context) recordPSortDecl(name string, userDecl bool) {
	e := c.psortDictionary().insertIfNotThere(name)
	e.psort = &psortEntry{userDecl: userDecl}
	if !c.GlobalDeclarations {
		c.psortStack = append(c.psortStack, name)
	}
}

// restorePSortDecls is cmd_context::restore_psort_decls: forward erasure.
func (c *Context) restorePSortDecls(size int) {
	if size > len(c.psortStack) {
		return
	}
	dict := c.psortDictionary()
	for _, name := range c.psortStack[size:] {
		dict.erase(name)
	}
	c.psortStack = c.psortStack[:size]
}

// dropPSortDecls removes declarations a failed command published.
func (c *Context) dropPSortDecls(names []string) {
	dict := c.psortDictionary()
	for _, name := range names {
		dict.erase(name)
		for i := len(c.psortStack) - 1; i >= 0; i-- {
			if c.psortStack[i] == name {
				c.psortStack = append(c.psortStack[:i], c.psortStack[i+1:]...)
				break
			}
		}
	}
}

// resetDeclarationDictionaries is reset_func_decls and reset_psort_decls.
func (c *Context) resetDeclarationDictionaries() {
	c.funcDictionary().reset()
	c.psortDictionary().reset()
	c.psortStack = nil
}

// recordBuiltinPSortDecls is the m_psort_decls part of init_manager_core for
// a manager the command context created: register_builtin_sorts of basic
// and of each installed plugin (get_sort_names), then the List declaration
// when no logic is set. The parametric List sort itself is not ported; its
// entry still shapes the dictionary layout.
func (c *Context) recordBuiltinPSortDecls() {
	logic := c.logic
	var names []string
	if logic == "" {
		names = append(names, "bool")
	}
	names = append(names, "Bool")
	if logic == "" || solver.LogicHasArithmetic(logic) {
		if logic == "NRA" || logic == "QF_NRA" || logic == "QF_UFNRA" {
			names = append(names, "Real")
		} else {
			names = append(names, "Int", "Real")
		}
	}
	if logic == "" || solver.LogicHasBitVector(logic) {
		if logic == "" || logic == "ALL" {
			names = append(names, "bv")
		}
		names = append(names, "BitVec")
	}
	if logic == "" || solver.LogicHasArray(logic) {
		names = append(names, "Array")
	}
	if logic == "" || solver.LogicHasSequence(logic) {
		names = append(names, "Seq", "RegEx", "String", "StringSequence")
	}
	if logic == "" || solver.LogicHasFPA(logic) {
		names = append(names, "FloatingPoint", "RoundingMode", "Float16", "Float32", "Float64", "Float128")
	}
	if logic == "" {
		names = append(names, "List")
	}
	for _, name := range names {
		c.recordPSortDecl(name, false)
	}
}

// resetFuncDecls is cmd_context::reset_func_decls: each m_func_decls entry,
// in table order, finalizes its declarations (func_decls::finalize). The
// trail rows keep their (released) slots until restoreDeclarations.
func (c *Context) resetFuncDecls() error {
	if c.funcDecls == nil {
		return nil
	}
	for _, e := range c.funcDictionary().entries() {
		if e.funcs == nil || e.funcs.empty() {
			continue
		}
		if e.funcs.set == nil {
			if err := c.releaseFunctionRow(e.funcs.singleRow); err != nil {
				return err
			}
			continue
		}
		for _, entry := range e.funcs.set.Entries() {
			if err := c.releaseFunctionRow(int(entry.Num)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Context) releaseFunctionRow(row int) error {
	ref := &c.functionTrail[row-1].ref
	if *ref == (ast.ASTRef{}) {
		return nil
	}
	return c.manager.DelRef(ref)
}

// resetPSortDecls is cmd_context::reset_psort_decls: each m_psort_decls
// entry, in table order, drops its psort_decl, whose instance cache holds
// the declared sorts. The sort infos stay until reset_sort_info.
func (c *Context) resetPSortDecls() error {
	if c.psortDecls == nil {
		return nil
	}
	for _, e := range c.psortDictionary().entries() {
		if e.psort == nil {
			continue
		}
		if declaration, found := c.sortConstructors[e.name]; found {
			for j := len(declaration.refs) - 1; j >= 0; j-- {
				if declaration.refs[j] == (ast.ASTRef{}) {
					continue
				}
				if err := c.manager.DelRef(&declaration.refs[j]); err != nil {
					return err
				}
			}
		}
		for i := len(c.sortTrail) - 1; i >= 0; i-- {
			row := &c.sortTrail[i]
			if row.name == e.name && row.ref != (ast.ASTRef{}) {
				if err := c.manager.DelRef(&row.ref); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
