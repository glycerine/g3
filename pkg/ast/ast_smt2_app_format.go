package ast

// Source: src/ast/ast_smt2_pp.cpp:738-801; src/ast/format.h:67-79.

// smt2FormatTail is the concrete format-pointer mk_seq loop: a line break
// precedes each non-nil document in the supplied suffix.
func (m *ASTManager) smt2FormatTail(args []ASTHandle) (document ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&document, &resultErr)
	formatOwner, err := m.GetFormatManager()
	if err != nil {
		return ASTHandle{}, err
	}
	parts := make([]ASTHandle, 0, 2*len(args))
	for _, arg := range args {
		kind, e := formatOwner.GetDeclKind(arg)
		if e != nil {
			return ASTHandle{}, e
		}
		if kind == ASTFormatNil {
			continue
		}
		breakDoc, e := documents.own(m.FormatLineBreak())
		if e != nil {
			return ASTHandle{}, e
		}
		parts = append(parts, breakDoc, arg)
	}
	return documents.own(m.FormatCompose(parts))
}

// SMT2PPAppFormat builds process_app's source document after the expression
// traversal has produced ordered child documents. Constant and label/pattern
// dispatch remain the surrounding traversal's responsibility.
func (m *ASTManager) SMT2PPAppFormat(decl ASTHandle, args []ASTHandle) (document ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&document, &resultErr)
	if len(args) == 0 {
		return ASTHandle{}, ErrASTDomain
	}
	fname, length, err := m.SMT2Fdecl(decl)
	fname, err = documents.own(fname, err)
	if err != nil {
		return ASTHandle{}, err
	}
	// GCC evaluates the nested call arguments right to left: the indented body
	// (closing paren first) is built before the "(" fname header.
	right, err := documents.own(m.FormatString(")"))
	if err != nil {
		return ASTHandle{}, err
	}
	var body ASTHandle
	var indent uint32
	if length > 16 {
		tail, e := documents.own(m.smt2FormatTail(args))
		if e != nil {
			return ASTHandle{}, e
		}
		if body, e = documents.own(m.FormatCompose([]ASTHandle{tail, right})); e != nil {
			return ASTHandle{}, e
		}
		indent = 2
	} else {
		tail, e := documents.own(m.smt2FormatTail(args[1:]))
		if e != nil {
			return ASTHandle{}, e
		}
		space, e := documents.own(m.FormatString(" "))
		if e != nil {
			return ASTHandle{}, e
		}
		if body, e = documents.own(m.FormatCompose([]ASTHandle{space, args[0], tail, right})); e != nil {
			return ASTHandle{}, e
		}
		indent = length + 2
	}
	body, err = documents.own(m.FormatIndent(indent, body))
	if err != nil {
		return ASTHandle{}, err
	}
	left, err := documents.own(m.FormatString("("))
	if err != nil {
		return ASTHandle{}, err
	}
	header, err := documents.own(m.FormatCompose([]ASTHandle{left, fname}))
	if err != nil {
		return ASTHandle{}, err
	}
	header, err = documents.own(m.FormatIndent(1, header))
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := documents.own(m.FormatCompose([]ASTHandle{header, body}))
	if err != nil {
		return ASTHandle{}, err
	}
	return documents.own(m.FormatGroup(whole))
}
