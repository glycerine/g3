package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
)

type datalogParserFixture struct {
	context *base.DatalogContext
}

func newDatalogParserFixture(t *testing.T) *datalogParserFixture {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	fpMust(t, err)
	manager, err := command.Manager()
	fpMust(t, err)
	context, err := base.NewDatalogContext(manager)
	fpMust(t, err)
	t.Cleanup(func() {
		fpMust(t, context.Close())
		fpMust(t, command.Close())
	})
	return &datalogParserFixture{context: context}
}

func (f *datalogParserFixture) ruleCount(t *testing.T) int {
	t.Helper()
	count, err := f.context.RuleCount()
	fpMust(t, err)
	return count
}
