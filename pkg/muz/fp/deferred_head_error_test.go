package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDeferredHeadRejectsJoinedCleanupErrors(t *testing.T) {
	head := &base.InvalidRuleHeadError{SourceMessage: "head"}
	if !onlyInvalidRuleHead(fmt.Errorf("normalize: %w", head)) {
		t.Fatal("single wrapped head was not deferred")
	}
	if onlyInvalidRuleHead(errors.Join(head, errors.New("cleanup"))) || onlyInvalidRuleHead(errors.Join(head, head)) {
		t.Fatal("deferred an unrelated or repeated failure")
	}
}
