package params

import "testing"

func TestParamDescriptionGroupEraseAndRecollect(t *testing.T) {
	_, symbols, _ := paramsTestOwner(t)
	d, e := NewParamDescrs(symbols, ParamDescrsConfig{})
	paramsMust(t, e)
	defer d.Close()
	rows := []NamedParamDescription{{Name: "first", Kind: ParamBool, Description: "original", Default: "true", Module: "m"}, {Name: "second", Kind: ParamUInt, Description: "unsigned", Default: "7"}}
	group := NewParamDescriptionGroup(rows...)
	rows[0].Name = "changed"
	paramsMust(t, d.InsertGroup(group))
	paramsMust(t, d.InsertGroup(group))
	if n, e := d.Size(); e != nil || n != 2 {
		t.Fatal("duplicate group", n, e)
	}
	if text, present, e := d.DescriptionName("first"); e != nil || !present || text != "original" {
		t.Fatal("owned description", text, present, e)
	}
	other := NewParamDescriptionGroup(NamedParamDescription{Name: "first", Kind: ParamBool, Description: "later"})
	paramsMust(t, d.InsertGroup(other))
	if text, _, e := d.DescriptionName("first"); e != nil || text != "original" {
		t.Fatal("first declaration wins", text, e)
	}
	paramsMust(t, d.EraseName("first"))
	paramsMust(t, d.InsertGroup(group))
	if k, e := d.KindName("first"); e != nil || k != ParamBool {
		t.Fatal("missing restored entry", k, e)
	}
	if n, e := d.ParamName(2); e != nil || n != internSymbol(t, symbols, "first") {
		t.Fatal("restored declaration order", n, e)
	}
}
