package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:158-164 (operator<<).

import (
	"io"
	"strconv"
)

func (v *ModularInterval) Display(w io.Writer) error {
	if v == nil || w == nil {
		return ErrExtendedDomain
	}
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	if _, err := io.WriteString(w, strconv.FormatUint(v.lower, 10)); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ", "); err != nil {
		return err
	}
	if _, err := io.WriteString(w, strconv.FormatUint(v.upper, 10)); err != nil {
		return err
	}
	_, err := io.WriteString(w, "]")
	return err
}
