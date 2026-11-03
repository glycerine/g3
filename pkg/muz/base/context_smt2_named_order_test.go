package base_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type contextNamedPrefixRefusal struct {
	symbols          *symbol.Manager
	previous, atName uint32
	seen             bool
}

func (w *contextNamedPrefixRefusal) Write(body []byte) (int, error) {
	count := w.symbols.Stats().Symbols
	if string(body) == " :named " {
		w.seen, w.atName = true, count
		return 0, io.ErrClosedPipe
	}
	w.previous = count
	return len(body), nil
}

func TestContextSMT2NamedPrefixPrecedesFreshLabel(t *testing.T) {
	for _, extensions := range []bool{false, true} {
		f := newRuleSetFixture(t, "P")
		set, err := base.NewRuleSet(f.rules)
		ruleMust(t, err)
		rule, err := f.rules.NewRule(f.app["P"], nil, nil, "ruleset-P")
		ruleMust(t, err)
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
		config := base.DefaultContextSMT2Config()
		config.FixedpointExtensions = extensions
		before := f.manager.Stats()
		out := &contextNamedPrefixRefusal{symbols: f.manager.Symbols}
		err = set.WriteContextSMT2(out, config)
		if !errors.Is(err, io.ErrClosedPipe) || !out.seen {
			t.Fatal("named prefix did not reach the writer refusal", err, out.seen)
		}
		// The source writes the attribute prefix before creating the fresh
		// collision label. Symbol interning is observable here even though
		// its Go map storage has no injected native allocation frontier.
		if out.atName != out.previous {
			t.Fatal("fresh label was interned before the source attribute prefix", out.previous, out.atName)
		}
		after := f.manager.Stats()
		if before.LiveNodes != after.LiveNodes || before.LiveInfos != after.LiveInfos || before.LiveParameters != after.LiveParameters || before.LiveReferences != after.LiveReferences {
			t.Fatal("label prefix refusal leaked AST ownership", before, after)
		}
		var retry bytes.Buffer
		ruleMust(t, set.WriteContextSMT2(&retry, config))
		if !strings.Contains(retry.String(), ":named ruleset-P!") {
			t.Fatal("retry did not freshen the colliding source label", retry.String())
		}
		ruleMust(t, set.Close())
	}
}
