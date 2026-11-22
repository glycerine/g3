package params

import utext "github.com/glycerine/g3/pkg/util/text"

type scalarParam struct {
	name  string
	kind  ParamKind
	value uint32
}

// ScalarParams captures the Boolean and unsigned options read during one rewriter
// update. Local values precede module defaults. The snapshot owns no parameter references.
type ScalarParams struct {
	entries [12]scalarParam
	count   int
}

// ReadScalarParams returns false when either reference cannot be represented.
// Callers then use the regular getters, which determine the reached error.
func (c *ParamsContext) ReadScalarParams(out *ScalarParams, local, global *ParamsRef) bool {
	if out == nil {
		return false
	}
	out.count = 0
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, ref := range [2]*ParamsRef{local, global} {
		if e := c.check(ref); e != nil {
			return false
		}
		if ref.Slot == 0 {
			continue
		}
		b := &c.Blocks[ref.Slot-1]
		if b.Size > uint32(len(out.entries)-count) || b.resetActive {
			return false
		}
		for i := uint32(0); i < b.Size; i++ {
			src := c.Entry(b, i)
			if src.key.Tag != 0 || src.key.IsNull() {
				return false
			}
			name, ok, err := c.SymbolsField.BareString(src.key)
			if err != nil || !ok {
				return false
			}
			if src.Kind != ParamBool && src.Kind != ParamUInt {
				continue
			}
			out.entries[count] = scalarParam{name, src.Kind, uint32(src.bits)}
			count++
		}
	}
	out.count = count
	return true
}

func (s *ScalarParams) Bool(key string, def bool) bool {
	for _, e := range s.entries[:s.count] {
		if e.kind == ParamBool && utext.EqualCString(e.name, key) {
			return e.value != 0
		}
	}
	return def
}

func (s *ScalarParams) UInt(key string, def uint32) uint32 {
	for _, e := range s.entries[:s.count] {
		if e.kind == ParamUInt && utext.EqualCString(e.name, key) {
			return e.value
		}
	}
	return def
}
