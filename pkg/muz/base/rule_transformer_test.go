package base_test

// Source: src/muz/base/dl_rule_transformer.cpp priority ordering, cancellation,
// no-op application, reset, and owned plugin destruction.

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

type recordingTransformPlugin struct {
	priority uint32
	id       string
	order    *[]string
	canceled bool
	closed   bool
}

type replacementTransformPlugin struct {
	next        *base.RuleSet
	destratify  bool
	closed      bool
	transferred bool
}

func (p *replacementTransformPlugin) Priority() uint32 { return 1 }
func (p *replacementTransformPlugin) CanDestratifyNegation() bool {
	return p.destratify
}
func (p *replacementTransformPlugin) Transform(*base.RuleSet) (*base.RuleSet, error) {
	result := p.next
	p.next = nil
	p.transferred = true
	return result, nil
}
func (p *replacementTransformPlugin) Cancel() {}
func (p *replacementTransformPlugin) Close() error {
	p.closed = true
	if p.next != nil {
		err := p.next.Close()
		p.next = nil
		return err
	}
	return nil
}

func (p *recordingTransformPlugin) Priority() uint32 { return p.priority }
func (p *recordingTransformPlugin) CanDestratifyNegation() bool {
	return false
}
func (p *recordingTransformPlugin) Transform(*base.RuleSet) (*base.RuleSet, error) {
	*p.order = append(*p.order, p.id)
	return nil, nil
}
func (p *recordingTransformPlugin) Cancel() { p.canceled = true }
func (p *recordingTransformPlugin) Close() error {
	p.closed = true
	return nil
}

func TestRuleTransformerOrdersOwnsAndCancelsPlugins(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	rule := f.rule("P", nil, nil)
	ruleMust(t, set.Add(rule))
	ruleMust(t, rule.Close())
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("transformer fixture rejected")
	}
	var order []string
	low := &recordingTransformPlugin{priority: 10, id: "low", order: &order}
	high := &recordingTransformPlugin{priority: 30, id: "high", order: &order}
	middle := &recordingTransformPlugin{priority: 20, id: "middle", order: &order}
	transformer := base.NewRuleTransformer()
	for _, plugin := range []base.RuleTransformPlugin{low, high, middle} {
		ruleMust(t, transformer.Register(plugin))
	}
	modified, err := transformer.Apply(set)
	ruleMust(t, err)
	if modified {
		t.Fatal("all-no-op transformer reported a change")
	}
	if len(order) != 3 || order[0] != "high" || order[1] != "middle" || order[2] != "low" {
		t.Fatal("plugin priority order", order)
	}
	transformer.Cancel()
	if !low.canceled || !middle.canceled || !high.canceled {
		t.Fatal("cancel was not broadcast")
	}
	ruleMust(t, transformer.Close())
	if !low.closed || !middle.closed || !high.closed {
		t.Fatal("plugin ownership was not released")
	}
	ruleMust(t, transformer.Close())
}

func TestRuleTransformerSkipsPluginWhenManagerIsCanceled(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	rule := f.rule("P", nil, nil)
	ruleMust(t, set.Add(rule))
	ruleMust(t, rule.Close())
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("transformer fixture rejected")
	}
	var order []string
	plugin := &recordingTransformPlugin{priority: 1, id: "must-not-run", order: &order}
	transformer := base.NewRuleTransformer()
	ruleMust(t, transformer.Register(plugin))
	t.Cleanup(func() { ruleMust(t, transformer.Close()) })
	f.manager.Limit.Cancel()
	modified, err := transformer.Apply(set)
	ruleMust(t, err)
	if modified || len(order) != 0 {
		t.Fatalf("canceled transformer ran a plugin or modified rules: modified=%v order=%v", modified, order)
	}
}

func TestRuleTransformerSkipsAllowedDestratification(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	fact := f.rule("P", nil, nil)
	ruleMust(t, set.Add(fact))
	ruleMust(t, fact.Close())
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("transform source rejected")
	}

	bad, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	for _, rule := range []*base.Rule{
		f.rule("P", []string{"Q"}, []bool{true}),
		f.rule("Q", []string{"P"}, nil),
	} {
		ruleMust(t, bad.Add(rule))
		ruleMust(t, rule.Close())
	}
	plugin := &replacementTransformPlugin{next: bad, destratify: true}
	transformer := base.NewRuleTransformer()
	ruleMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(set)
	ruleMust(t, err)
	if modified {
		t.Fatal("destratifying result replaced the source")
	}
	if !plugin.transferred {
		t.Fatal("destratifying plugin was not invoked")
	}
	if _, err = bad.IsFinalized(); err == nil {
		t.Fatal("discarded destratifying result remained open")
	}
	if count, err := set.Len(); err != nil || count != 1 {
		t.Fatal("source changed after skipped transform", count, err)
	}
	ruleMust(t, transformer.Close())
	if !plugin.closed {
		t.Fatal("replacement plugin was not released")
	}
}
