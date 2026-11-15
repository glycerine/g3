package fp

import "testing"

func TestBMCSolverProfilesInstallSourceParameters(t *testing.T) {
	engine := newDirectFactFixture(t).engine
	for _, test := range []struct {
		name                 string
		linear, quantified   bool
		wantRelevancy        uint32
		wantMBQI, mbqiStored bool
	}{
		{name: "linear", linear: true, wantRelevancy: 0, mbqiStored: true},
		{name: "quantified-linear", linear: true, quantified: true, wantRelevancy: 2, wantMBQI: true, mbqiStored: true},
		{name: "nonlinear", wantRelevancy: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			local, err := engine.params.NewValue()
			fragmentMust(t, err)
			defer func() { fragmentMust(t, engine.params.Del(&local)) }()
			fragmentMust(t, engine.configureBMCSolverParams(&local, test.linear, test.quantified))
			compact := "compact"
			gotCompact, err := engine.params.GetBoolName(&local, &compact, false)
			if err != nil || !gotCompact {
				t.Fatal("source model_compact profile was not installed", gotCompact, err)
			}
			relevancy := "relevancy"
			gotRelevancy, err := engine.params.GetUIntName(&local, &relevancy, ^uint32(0))
			if err != nil || gotRelevancy != test.wantRelevancy {
				t.Fatal("source relevancy profile", gotRelevancy, test.wantRelevancy, err)
			}
			mbqi := "mbqi"
			stored, err := engine.params.ContainsName(&local, &mbqi)
			if err != nil || stored != test.mbqiStored {
				t.Fatal("source MBQI override presence", stored, test.mbqiStored, err)
			}
			if stored {
				gotMBQI, cause := engine.params.GetBoolName(&local, &mbqi, !test.wantMBQI)
				if cause != nil || gotMBQI != test.wantMBQI {
					t.Fatal("source MBQI profile", gotMBQI, test.wantMBQI, cause)
				}
			}
		})
	}
}

// muz_fp_01-F1: bmc::nonlinear::setup does not assign m_fparams.m_mbqi, so a
// nonlinear query inherits the value the previous linear setup() left on the
// same engine.
func TestBMCNonlinearInheritsLinearMBQI(t *testing.T) {
	engine := newDirectFactFixture(t).engine
	engine.bmcSlot = &bmcSolverSlot{owner: engine}
	defer func() { engine.bmcSlot = nil }()
	mbqi := "mbqi"
	run := func(linear bool) (bool, bool) {
		local, err := engine.params.NewValue()
		fragmentMust(t, err)
		defer func() { fragmentMust(t, engine.params.Del(&local)) }()
		fragmentMust(t, engine.configureBMCSolverParams(&local, linear, false))
		stored, err := engine.params.ContainsName(&local, &mbqi)
		fragmentMust(t, err)
		value, err := engine.params.GetBoolName(&local, &mbqi, true)
		fragmentMust(t, err)
		return stored, value
	}
	if stored, _ := run(false); stored {
		t.Fatal("nonlinear setup on a fresh engine must keep the smt_params default")
	}
	if stored, value := run(true); !stored || value {
		t.Fatal("linear setup must disable MBQI", stored, value)
	}
	if stored, value := run(false); !stored || value {
		t.Fatal("nonlinear setup must inherit the linear m_mbqi=false", stored, value)
	}
}
