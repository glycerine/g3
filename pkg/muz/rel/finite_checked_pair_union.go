package rel

// unionFromConcrete mirrors check_table_plugin::union_fn: each selected
// concrete plugin performs its own union (including LazySparse forcing), then
// both the target and optional delta are checked extensionally.
func (p *FiniteCheckedPair) unionFromConcrete(source, delta *FiniteCheckedPair) error {
	if err := checkTableVerbose("operator()"); err != nil {
		return err
	}
	if err := p.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil {
		return ErrCheckTable
	}
	var primaryDelta, checkerDelta FiniteTable
	if delta != nil {
		if err := delta.open(); err != nil {
			return err
		}
		primaryDelta, checkerDelta = delta.primary, delta.checker
	}
	if err := finiteTableUnion(p.primary, source.primary, primaryDelta); err != nil {
		return err
	}
	if err := finiteTableUnion(p.checker, source.checker, checkerDelta); err != nil {
		return err
	}
	if err := p.wellFormed(); err != nil {
		return err
	}
	if delta != nil {
		return delta.wellFormed()
	}
	return nil
}
