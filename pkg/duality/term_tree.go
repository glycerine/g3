package duality

// Source: duality_wrapper.h:1274-1328.

// TermTree owns its term copies and children. Child ownership transfers to
// the tree on successful construction or SetChildren.
type TermTree struct {
	term     *Expr
	terms    []*Expr
	children []*TermTree
	number   int
	closing  bool
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
	if t == nil || t.closing || t.closed {
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

// SetChildren transfers child ownership while the tree is open. Once cleanup
// starts, it leaves the existing children and the caller's inputs unchanged.
func (t *TermTree) SetChildren(children []*TermTree) {
	if t == nil || t.closing || t.closed {
		return
	}
	t.children = append([]*TermTree(nil), children...)
}
func (t *TermTree) Close() error {
	if t == nil || t.closed {
		return nil
	}
	t.closing = true
	// ~TermTree deletes children[0..n-1]; then the members go: terms (a
	// std::vector, destroyed front to back) and term (AUDIT.md
	// duality_03-F7). A failed close is retried from the same index.
	for len(t.children) > 0 {
		if err := t.children[0].Close(); err != nil {
			return err
		}
		t.children[0] = nil
		t.children = t.children[1:]
	}
	for len(t.terms) > 0 {
		err := t.terms[0].Close()
		if t.terms[0].Closed() {
			t.terms[0] = nil
			t.terms = t.terms[1:]
		}
		if err != nil {
			return err
		}
	}
	if t.term != nil {
		err := t.term.Close()
		if t.term.Closed() {
			t.term = nil
		}
		if err != nil {
			return err
		}
	}
	t.closed = true
	return nil
}
