package params

import (
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestParamsNamedReadAfterSharedValidation(t *testing.T) {
	c, symbols, _ := paramsTestOwner(t)
	local, e := c.NewValue()
	paramsMust(t, e)
	defer c.Del(&local)
	alias, e := c.NewValue()
	paramsMust(t, e)
	defer c.Del(&alias)
	fallback, e := c.NewValue()
	paramsMust(t, e)
	defer c.Del(&fallback)
	d, e := NewParamDescrs(symbols, ParamDescrsConfig{})
	paramsMust(t, e)
	defer d.Close()
	module, description := "smt", "prefix test"
	name := "macro_finder"
	paramsMust(t, d.InsertName(name, ParamBool, &description, nil, &module))
	paramsMust(t, c.SetBool(&local, internSymbol(t, symbols, module+"."+name), true))
	paramsMust(t, c.SetUInt(&local, internSymbol(t, symbols, "unknown"), 3))
	paramsMust(t, c.SetUInt(&fallback, internSymbol(t, symbols, name), 47))
	paramsMust(t, c.Assign(&alias, &local))
	if e = c.Validate(&local, d); e == nil {
		t.Fatal("expected rejection of the second key")
	}
	query := name + "\x00ignored"
	if value, e := c.GetBoolName(&alias, &query, false); e != nil || !value {
		t.Fatal("validated shared name", value, e)
	}
	if value, e := c.GetUIntFallbackName(&local, &query, &fallback, 0); e != nil || value != 47 {
		t.Fatal("fallback past wrong kind", value, e)
	}
	qualified := module + "." + name
	if value, e := c.ContainsName(&alias, &qualified); e != nil || value {
		t.Fatal("obsolete name", value, e)
	}
	paramsMust(t, c.ResetKeyName(&local, &name))
	if value, e := c.GetBoolName(&local, &name, false); e != nil || value {
		t.Fatal("reset name", value, e)
	}
}

func TestParamsNamedReadOfNumericalKey(t *testing.T) {
	c, _, _ := paramsTestOwner(t)
	v, e := c.NewValue()
	paramsMust(t, e)
	defer c.Del(&v)
	paramsMust(t, c.SetUInt(&v, usymbol.Numerical(1), 7))
	name := "k!1"
	if got, e := c.GetUIntName(&v, &name, 0); e != nil || got != 7 {
		t.Fatal("numerical key spelling", got, e)
	}
}
