package ast

// Source: src/ast/format.h:164-195. Concrete format-pointer specialization.

// FormatSeq5 builds the grouped parenthesized sequence used by SMT2 function
// signatures. Its source mk_seq tail skips nil documents but keeps the first.
func (m *ASTManager) FormatSeq5(children []ASTHandle) (document ASTHandle, resultErr error) {
	documents, err := newASTFormatDocuments(m)
	if err != nil {
		return ASTHandle{}, err
	}
	defer documents.finish(&document, &resultErr)
	// mk_seq4 with GCC's right-to-left argument evaluation: rp, the indented
	// tail, then lp and the first element.
	right, err := documents.own(m.FormatString(")"))
	if err != nil {
		return ASTHandle{}, err
	}
	if len(children) == 0 {
		left, err := documents.own(m.FormatString("("))
		if err != nil {
			return ASTHandle{}, err
		}
		return documents.own(m.FormatCompose([]ASTHandle{left, right}))
	}
	formatOwner, err := m.GetFormatManager()
	if err != nil {
		return ASTHandle{}, err
	}
	tail := make([]ASTHandle, 0, len(children)-1)
	for _, child := range children[1:] {
		kind, e := formatOwner.GetDeclKind(child)
		if e != nil {
			return ASTHandle{}, e
		}
		if kind == ASTFormatNil {
			continue
		}
		line, e := documents.own(m.FormatLineBreak())
		if e != nil {
			return ASTHandle{}, e
		}
		tail = append(tail, line, child)
	}
	seq, err := documents.own(m.FormatCompose(tail))
	if err != nil {
		return ASTHandle{}, err
	}
	last, err := documents.own(m.FormatCompose([]ASTHandle{seq, right}))
	if err != nil {
		return ASTHandle{}, err
	}
	last, err = documents.own(m.FormatIndent(1, last))
	if err != nil {
		return ASTHandle{}, err
	}
	left, err := documents.own(m.FormatString("("))
	if err != nil {
		return ASTHandle{}, err
	}
	first, err := documents.own(m.FormatCompose([]ASTHandle{left, children[0]}))
	if err != nil {
		return ASTHandle{}, err
	}
	first, err = documents.own(m.FormatIndent(1, first))
	if err != nil {
		return ASTHandle{}, err
	}
	whole, err := documents.own(m.FormatCompose([]ASTHandle{first, last}))
	if err != nil {
		return ASTHandle{}, err
	}
	return documents.own(m.FormatGroup(whole))
}
