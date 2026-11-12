package ast

// Source: src/cmd_context/pdecl.cpp:732-829,1012-1045 (pdecl_manager::pp,
// app_sort_info::pp, indexed_sort_info::pp) and src/cmd_context/
// cmd_context.cpp:432-434 (cmd_context::pp_env::pp_sort).

// SMT2SortInfo is a pdecl_manager sort_info record: the psort declaration
// name a sort was instantiated through, with either its sort arguments
// (app_sort_info) or its integer indices (indexed_sort_info).
type SMT2SortInfo struct {
	Name    string
	Indexed bool
	Args    []ASTHandle
	Indices []uint32
}

// SMT2SortInfoTable is pdecl_manager::m_sort2info.
type SMT2SortInfoTable interface {
	SMT2SortInfo(sort ASTHandle) (SMT2SortInfo, bool)
}

// SetSMT2SortInfoTable installs the command context's pp_env sort printer
// (pdecl_manager::pp) for SMT2PPSort and returns the previous table. A nil
// table selects the base smt2_pp_environment printer.
func (m *ASTManager) SetSMT2SortInfoTable(table SMT2SortInfoTable) SMT2SortInfoTable {
	previous := m.smt2SortInfoTable
	m.smt2SortInfoTable = table
	return previous
}

// pdeclPPSort is pdecl_manager::pp(s).
func (m *ASTManager) pdeclPPSort(table SMT2SortInfoTable, sort ASTHandle) (result ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&result, &resultErr)
	if info, found := table.SMT2SortInfo(sort); found {
		if info.Indexed {
			if len(info.Indices) == 0 {
				return m.FormatString(info.Name)
			}
			children := make([]ASTHandle, 0, len(info.Indices)+1)
			name, e := documents.own(m.FormatString(info.Name))
			if e != nil {
				return ASTHandle{}, e
			}
			children = append(children, name)
			for _, index := range info.Indices {
				f, e := documents.own(m.FormatUnsigned(index))
				if e != nil {
					return ASTHandle{}, e
				}
				children = append(children, f)
			}
			return m.FormatSeq1("_", children)
		}
		if len(info.Args) == 0 {
			return m.FormatString(info.Name)
		}
		children := make([]ASTHandle, 0, len(info.Args))
		for _, argument := range info.Args {
			f, e := documents.own(m.pdeclPPSort(table, argument))
			if e != nil {
				return ASTHandle{}, e
			}
			children = append(children, f)
		}
		return m.FormatSeq1(info.Name, children)
	}
	v, err := m.smt2SortView(sort)
	if err != nil {
		return ASTHandle{}, err
	}
	name, err := m.Symbols.String(v.name)
	if err != nil {
		return ASTHandle{}, err
	}
	// "Small hack to display FP and BitVec sorts that were not explicitly
	// referenced by the user": all-integer parameters print indexed by the
	// sort's own name.
	if v.family != NullFamilyID && len(v.params) > 0 {
		allInt := true
		for _, p := range v.params {
			if p.kind != ASTParameterInt {
				allInt = false
				break
			}
		}
		if allInt {
			children := make([]ASTHandle, 0, len(v.params)+1)
			f, e := documents.own(m.FormatString(name))
			if e != nil {
				return ASTHandle{}, e
			}
			children = append(children, f)
			for _, p := range v.params {
				f, e := documents.own(m.FormatUnsigned(p.ival))
				if e != nil {
					return ASTHandle{}, e
				}
				children = append(children, f)
			}
			return m.FormatSeq1("_", children)
		}
	}
	return m.FormatString(name)
}

// SMT2FuncDeclTable is the m_func_decls lookup of cmd_context::pp_env
// (cmd_context.cpp:398-456). A table installed with SetSMT2SortInfoTable
// may also implement it.
type SMT2FuncDeclTable interface {
	// SMT2FuncDecls is m_func_decls.find(f->get_name(), fs) && fs.contains(f):
	// it returns fs's members.
	SMT2FuncDecls(decl ASTHandle) ([]ASTHandle, bool)
}

func (m *ASTManager) smt2FuncDecls(decl ASTHandle) ([]ASTHandle, bool) {
	table, ok := m.smt2SortInfoTable.(SMT2FuncDeclTable)
	if !ok {
		return nil, false
	}
	return table.SMT2FuncDecls(decl)
}

// smt2FuncDeclsClash is func_decls::clash(f): another member has f's
// domain.
func (m *ASTManager) smt2FuncDeclsClash(decls []ASTHandle, f ASTHandle) (bool, error) {
	arity, err := m.GetArity(f)
	if err != nil {
		return false, err
	}
	for _, g := range decls {
		if g == f {
			continue
		}
		garity, err := m.GetArity(g)
		if err != nil {
			return false, err
		}
		if garity != arity {
			continue
		}
		same := true
		for i := uint32(0); i < arity && same; i++ {
			a, err := m.GetDomain(g, i)
			if err != nil {
				return false, err
			}
			b, err := m.GetDomain(f, i)
			if err != nil {
				return false, err
			}
			same = a == b
		}
		if same {
			return true, nil
		}
	}
	return false, nil
}
