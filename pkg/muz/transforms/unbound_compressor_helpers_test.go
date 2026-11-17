package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

// newTestUnboundCompressor is an UnboundCompressorPlugin with the rule
// manager services mk_decompression_rule's fix_unbound_vars uses (a qe_lite;
// xform.fix_unbound_vars off).
func newTestUnboundCompressor(t *testing.T, f *coneFixture, enabled bool) *transforms.UnboundCompressorPlugin {
	t.Helper()
	cooperation, err := cooperate.NewCooperation(f.manager.Budget())
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, cooperation.Close()) })
	lite, err := qe.NewLite(f.manager, nil, false, qe.Config{Context: f.globals.Context, Globals: f.globals, Cooperation: cooperation})
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, lite.Close()) })
	plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, enabled, 0)
	plugin.SetFixUnboundServices(func() (*qe.Lite, error) { return lite, nil }, false)
	return plugin
}
