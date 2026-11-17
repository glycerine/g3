package rel

// Source: src/muz/rel/dl_lazy_table.cpp: the verbose_action (default level
// 11) of each lazy_table force, of lazy_table_plugin::union_fn and of
// lazy_table::clone. Each starts once the operation's sources are
// evaluated and ends when the operation's scope ends.

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

// lazyVerbose runs op inside verbose_action(msg).
func lazyVerbose(msg string, op func() error) error {
	action, err := base.StartVerboseAction(msg, base.VerboseActionDefaultLevel)
	if err != nil {
		return err
	}
	err = op()
	return errors.Join(err, action.End())
}
