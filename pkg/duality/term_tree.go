package duality

// Source: duality_wrapper.h:1274-1328.

// TermTree owns its term copies and children. Child ownership transfers to
// the tree on successful construction or SetChildren.
type TermTree struct {
	term     *Expr
	terms    []*Expr
	children []*TermTree
	number   int
	closed   bool
}

func NewTermTree(term *Expr, children []*TermTree) (*TermTree, error) {
	copy, err := term.Clone()
	if err != nil {
		return nil, err
	}
	return &TermTree{term: copy, children: append([]*TermTree(nil), children...)}, nil
}
func (t *TermTree) open() error {
	if t == nil || t.closed {
		return ErrWrapperOwner
	}
	return t.term.open()
}
func (t *TermTree) Term() (*Expr, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return t.term.Clone()
}
func (t *TermTree) SetTerm(term *Expr) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := term.open(); err != nil {
		return err
	}
	return t.term.value.Assign(term.value)
}
func (t *TermTree) AddTerm(term *Expr) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := term.open(); err != nil {
		return err
	}
	if term.value.object.context != t.term.value.object.context {
		return ErrContextMismatch
	}
	copy, err := term.Clone()
	if err != nil {
		return err
	}
	t.terms = append(t.terms, copy)
	return nil
}
func (t *TermTree) Children() []*TermTree { return append([]*TermTree(nil), t.children...) }
func (t *TermTree) Number(from int) int {
	for _, child := range t.children {
		from = child.Number(from)
	}
	t.number = from
	return from + 1
}
func (t *TermTree) GetNumber() int       { return t.number }
func (t *TermTree) SetNumber(number int) { t.number = number }
func (t *TermTree) SetChildren(children []*TermTree) {
	t.children = append([]*TermTree(nil), children...)
}
func (t *TermTree) Close() error {
	if t == nil || t.closed {
		return nil
	}
	for len(t.children) > 0 {
		n := len(t.children) - 1
		if err := t.children[n].Close(); err != nil {
			return err
		}
		t.children = t.children[:n]
	}
	for len(t.terms) > 0 {
		n := len(t.terms) - 1
		if err := t.terms[n].Close(); err != nil {
			return err
		}
		t.terms = t.terms[:n]
	}
	if t.term != nil {
		if err := t.term.Close(); err != nil {
			return err
		}
		t.term = nil
	}
	t.closed = true
	return nil
}
