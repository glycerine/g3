package rel

// Source: src/muz/rel/dl_check_table.cpp: check_table_plugin::mk_empty,
// join_fn/union_fn/rename_fn/filter_by_negation_fn::operator() and
// check_table::add_fact/remove_fact/clone begin with
// IF_VERBOSE(1, verbose_stream() << __FUNCTION__ << "\n";).

import "github.com/glycerine/g3/pkg/util/diagnostic"

func checkTableVerbose(function string) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString(function + "\n")
}
