package cmd_context

// Source: src/cmd_context/pdecl.cpp:325,375-405,691,732-829,984-1045
// (pdecl_manager::save_info, m_sort2info, reset_sort_info, pp) and
// src/cmd_context/cmd_context.cpp:432-434,1906-1909 (pp_env::pp_sort).

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// sortInfoRecord is a pdecl_manager sort_info: it holds the sort and the
// app_sort_info argument sorts (m().inc_ref(s), inc_array_ref).
type sortInfoRecord struct {
	info ast.SMT2SortInfo
	refs []ast.ASTRef
}

// pendingSortInfo is a psort_app inside a define-sort body: parse_psort
// builds it without instantiating, and psort_app::instantiate saves its
// sort info each time the definition is instantiated (arguments first).
type pendingSortInfo struct {
	name string
	sort ast.ASTHandle
	args []ast.ASTHandle
	refs []ast.ASTRef
}

// SMT2SortInfo is the m_sort2info lookup used by pdecl_manager::pp.
func (c *Context) SMT2SortInfo(s ast.ASTHandle) (ast.SMT2SortInfo, bool) {
	record, found := c.sortInfos[s]
	if !found {
		return ast.SMT2SortInfo{}, false
	}
	return record.info, true
}

func (c *Context) holdSorts(sorts ...ast.ASTHandle) ([]ast.ASTRef, error) {
	refs := make([]ast.ASTRef, 0, len(sorts))
	for _, s := range sorts {
		ref, err := c.manager.NewRef(s)
		if err != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				err = errors.Join(err, c.manager.DelRef(&refs[i]))
			}
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// saveSortInfo is pdecl_manager::save_info(s, d, num_args, args): the first
// psort declaration a sort is instantiated through names it for printing.
// While a define-sort body is parsed (parse_psort), sort applications are
// recorded for instantiation time instead.
func (c *Context) saveSortInfo(sort ast.ASTHandle, name string, args []ast.ASTHandle) error {
	if sort.IsNull() || c.manager == nil {
		return nil
	}
	if c.psortRecording != nil {
		refs, err := c.holdSorts(append([]ast.ASTHandle{sort}, args...)...)
		if err != nil {
			return err
		}
		*c.psortRecording = append(*c.psortRecording, pendingSortInfo{name: name, sort: sort, args: append([]ast.ASTHandle(nil), args...), refs: refs})
		return nil
	}
	return c.storeSortInfo(sort, ast.SMT2SortInfo{Name: name, Args: append([]ast.ASTHandle(nil), args...)})
}

// saveIndexedSortInfo is save_info(s, d, num_indices, indices). parse_psort
// instantiates indexed sorts immediately (mk_psort_cnst), so these are never
// deferred.
func (c *Context) saveIndexedSortInfo(sort ast.ASTHandle, name string, indices []uint32) error {
	if sort.IsNull() || c.manager == nil {
		return nil
	}
	return c.storeSortInfo(sort, ast.SMT2SortInfo{Name: name, Indexed: true, Indices: append([]uint32(nil), indices...)})
}

func (c *Context) storeSortInfo(sort ast.ASTHandle, info ast.SMT2SortInfo) error {
	if _, found := c.sortInfos[sort]; found {
		return nil
	}
	refs, err := c.holdSorts(append([]ast.ASTHandle{sort}, info.Args...)...)
	if err != nil {
		return err
	}
	if c.sortInfos == nil {
		c.sortInfos = make(map[ast.ASTHandle]*sortInfoRecord)
	}
	c.sortInfos[sort] = &sortInfoRecord{info: info, refs: refs}
	c.sortInfoOrder = append(c.sortInfoOrder, sort)
	return nil
}

// SaveIndexedSortInfo records an indexed builtin sort instantiation
// (parse_indexed_sort).
func (c *Context) SaveIndexedSortInfo(sort ast.ASTHandle, name string, indices []uint32) error {
	return c.saveIndexedSortInfo(sort, name, indices)
}

// SaveSortInfo records a builtin sort application such as (Array I E),
// (Seq E) or (RegEx E) (pop_sort_app_frame).
func (c *Context) SaveSortInfo(sort ast.ASTHandle, name string, args []ast.ASTHandle) error {
	return c.saveSortInfo(sort, name, args)
}

// BeginPSort starts a define-sort body parse (parse_psort): sort
// applications are recorded rather than instantiated.
func (c *Context) BeginPSort() {
	recording := []pendingSortInfo{}
	c.psortRecording = &recording
}

// EndPSort stops recording and returns the body's pending applications.
func (c *Context) endPSort() []pendingSortInfo {
	if c.psortRecording == nil {
		return nil
	}
	recorded := *c.psortRecording
	c.psortRecording = nil
	return recorded
}

// AbortPSort releases a body recording after a parse failure.
func (c *Context) AbortPSort() error {
	return c.releasePending(c.endPSort())
}

func (c *Context) releasePending(pending []pendingSortInfo) (result error) {
	for i := len(pending) - 1; i >= 0; i-- {
		for j := len(pending[i].refs) - 1; j >= 0; j-- {
			result = errors.Join(result, c.manager.DelRef(&pending[i].refs[j]))
		}
	}
	return result
}

// releaseSortDefinitionInfos drops a popped definition's recorded body.
func (c *Context) releaseSortDefinitionInfos(name string) error {
	pending, found := c.sortDefinitionInfos[name]
	if !found {
		return nil
	}
	delete(c.sortDefinitionInfos, name)
	return c.releasePending(pending)
}

// instantiateSortDefinitionInfos is m_def->instantiate's psort_app saves.
// Inside another define-sort body they are recorded for that body instead,
// so the definition's own name never names a sort (psort_user_decl's
// save_info follows its body's).
func (c *Context) instantiateSortDefinitionInfos(name string, formals, arguments []ast.ASTHandle) error {
	for _, pending := range c.sortDefinitionInfos[name] {
		sort, args := pending.sort, pending.args
		if len(formals) != 0 {
			var err error
			if sort, err = c.manager.SubstituteSort(sort, formals, arguments); err != nil {
				return err
			}
			args = make([]ast.ASTHandle, len(pending.args))
			for i, arg := range pending.args {
				if args[i], err = c.manager.SubstituteSort(arg, formals, arguments); err != nil {
					return err
				}
			}
		}
		if err := c.saveSortInfo(sort, pending.name, args); err != nil {
			return err
		}
	}
	return nil
}

// resetSortInfo is pdecl_manager::reset_sort_info.
func (c *Context) resetSortInfo() (result error) {
	for _, sort := range c.sortInfoOrder {
		record := c.sortInfos[sort]
		for j := len(record.refs) - 1; j >= 0; j-- {
			result = errors.Join(result, c.manager.DelRef(&record.refs[j]))
		}
	}
	c.sortInfos = nil
	c.sortInfoOrder = nil
	names := make([]string, 0, len(c.sortDefinitionInfos))
	for name := range c.sortDefinitionInfos {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result = errors.Join(result, c.releaseSortDefinitionInfos(name))
	}
	return errors.Join(result, c.AbortPSort())
}

// withPPEnv formats through cmd_context's pp_env, whose pp_sort is
// pdecl_manager::pp.
func (c *Context) withPPEnv(format func() error) error {
	if c.manager == nil {
		return format()
	}
	previous := c.manager.SetSMT2SortInfoTable(c)
	defer c.manager.SetSMT2SortInfoTable(previous)
	return format()
}

// SMT2FuncDecls is pp_env's m_func_decls.find(f->get_name(), fs) &&
// fs.contains(f).
func (c *Context) SMT2FuncDecls(decl ast.ASTHandle) ([]ast.ASTHandle, bool) {
	symbol, err := c.manager.GetASTName(decl)
	if err != nil {
		return nil, false
	}
	name, err := c.manager.Symbols.String(symbol)
	if err != nil {
		return nil, false
	}
	decls := c.functions[name]
	for _, d := range decls {
		if d == decl {
			return decls, true
		}
	}
	return nil, false
}
