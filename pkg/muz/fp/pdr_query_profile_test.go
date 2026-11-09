package fp

// Source: pdr_dl_interface.cpp display_rules calls and dl_rule_set.cpp:511.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRQueryRuleProfileParameters(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		for _, tc := range []struct {
			name            string
			globalProfile   bool
			globalThreshold uint32
			localProfile    string
			localThreshold  int
			profile         bool
			threshold       uint32
		}{
			{"module-profile", true, 0, "", -1, true, 0},
			{"local-profile-off", true, 0, "false", -1, false, 0},
			{"local-profile-on", false, 0, "true", -1, true, 0},
			{"module-threshold", true, 1, "", -1, true, 1},
			{"local-threshold-zero", true, 1, "", 0, true, 0},
			{"local-threshold-one", true, 0, "", 1, true, 1},
		} {
			t.Run(backend+"/"+tc.name, func(t *testing.T) {
				f := newPDRLifecycleFixture(t)
				e := f.engine
				fragmentMust(t, e.SelectEngine(backend))
				fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.output_profile", fmt.Sprint(tc.globalProfile)))
				fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.profile_timeout_milliseconds", fmt.Sprint(tc.globalThreshold)))
				parameters, err := e.params.NewValue()
				fragmentMust(t, err)
				defer func() { fragmentMust(t, e.params.Del(&parameters)) }()
				for _, name := range []string{"xform.inline_eager", "xform.inline_linear", "xform.subsumption_checker"} {
					fragmentMust(t, e.params.SetBoolName(&parameters, &name, false))
				}
				if tc.localProfile != "" {
					name := "datalog.output_profile"
					fragmentMust(t, e.params.SetBoolName(&parameters, &name, tc.localProfile == "true"))
				}
				if tc.localThreshold >= 0 {
					name := "datalog.profile_timeout_milliseconds"
					fragmentMust(t, e.params.SetUIntName(&parameters, &name, uint32(tc.localThreshold)))
				}
				fragmentMust(t, e.SetQueryParameters(&parameters))
				oldLevel, err := diagnostic.ProcessVerbosityLevel()
				fragmentMust(t, err)
				oldStream, err := diagnostic.BorrowProcessVerboseStream()
				fragmentMust(t, err)
				stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget()})
				fragmentMust(t, err)
				defer func() {
					fragmentMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
					fragmentMust(t, diagnostic.SetProcessVerboseStream(oldStream))
					fragmentMust(t, stream.Close())
				}()
				fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(2))
				status, err := e.QueryPredicate(f.declarations["q"])
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("profile options changed query result", status)
				}
				var ordinary, expected bytes.Buffer
				fragmentMust(t, e.pdrRules.WriteDisplay(&ordinary))
				if !strings.Contains(ordinary.String(), " :- ") {
					t.Fatal("fixture must preserve a non-fact rule for source profiling", ordinary.String())
				}
				fragmentMust(t, e.pdrRules.WriteDisplayWithProfile(&expected, tc.profile, tc.threshold))
				got, err := stream.String()
				fragmentMust(t, err)
				if !strings.HasPrefix(got, expected.String()) {
					t.Fatalf("diagnostic did not honor parameter precedence: got %q; want prefix %q", got, expected.String())
				}
				if suffix := strings.TrimPrefix(got, expected.String()); !strings.HasPrefix(suffix, "\n2 query!0 closed\n  true 1\n 1 pdr-lifecycle-q closed\n") {
					t.Fatalf("unexpected rules between listing and reachable-search diagnostic: %q", suffix)
				}
				listing := expected.String()
				if tc.threshold > 0 {
					if strings.Contains(listing, " :- ") || strings.Contains(listing, "pdr-lifecycle-fact") {
						t.Fatal("zero-cost rules passed positive threshold", listing)
					}
				} else if strings.Contains(listing, "{instr: 0  time: 0ms}") != tc.profile {
					t.Fatal("profile annotation disagrees with selected option", listing)
				}
				// Fresh-rule diagnostics have no rule-set threshold filter.
				rule, err := e.set.Rule(1)
				fragmentMust(t, err)
				var fresh bytes.Buffer
				fragmentMust(t, e.writeRuleDisplay(&fresh, rule))
				fragmentMust(t, rule.Close())
				if !strings.Contains(fresh.String(), " :- ") || strings.Contains(fresh.String(), "{instr: 0  time: 0ms}") != tc.profile {
					t.Fatal("fresh-rule profile/threshold semantics", fresh.String())
				}
			})
		}
	}
}
