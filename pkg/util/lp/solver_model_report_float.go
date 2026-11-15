package lp

// Source: src/util/lp/lp_solver.h:259-264; retained floating primal caller
// src/shell/lp_frontend.cpp:92.

import "io"

// PrintModel reports the original objective and named external column values
// in m_names_to_columns' iteration order: the libstdc++
// std::unordered_map<std::string, unsigned> node order that
// StdUnorderedStringMap reproduces (user-approved, AUDIT.md util_lp_06-F1).
func (m *FloatPrimalSimplex) PrintModel(out io.Writer) error {
	if err := writeConstraintText(out, "objective = "); err != nil {
		return err
	}
	cost, err := m.GetCurrentCost()
	if err != nil {
		return err
	}
	if err = writeDriverLine(out, floatLPNumberText(cost)); err != nil {
		return err
	}
	if err = writeConstraintText(out, "column values\n"); err != nil {
		return err
	}
	for name, column := range m.NamesToColumns.All() {
		if err = writeConstraintText(out, name+" = "); err != nil {
			return err
		}
		value, e := m.GetColumnValue(column)
		if e != nil {
			return e
		}
		if err = writeDriverLine(out, floatLPNumberText(value)); err != nil {
			return err
		}
	}
	return nil
}
