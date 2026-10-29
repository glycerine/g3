package api

// Source: src/api/api_datatype.cpp:373-394.

type APIDatatypeConstructorList struct {
	owner        *smtContextState
	constructors []*APIDatatypeConstructorObject
	closed       bool
}

func (c *SMTContext) NewDatatypeConstructorList(constructors ...*APIDatatypeConstructorObject) (*APIDatatypeConstructorList, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	result := &APIDatatypeConstructorList{owner: c.smtContextState, constructors: append([]*APIDatatypeConstructorObject(nil), constructors...)}
	for _, constructor := range result.constructors {
		if constructor == nil || constructor.owner != c.smtContextState {
			return nil, ErrSMTContextOwner
		}
		if err := constructor.open(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (l *APIDatatypeConstructorList) open() error {
	if l == nil || l.owner == nil || l.closed || l.owner.closed || l.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

// Close releases only the pointer vector. Constructors remain independently
// owned, matching Z3_del_constructor_list.
func (l *APIDatatypeConstructorList) Close() error {
	if l == nil || l.closed {
		return nil
	}
	l.owner = nil
	l.constructors = nil
	l.closed = true
	return nil
}
