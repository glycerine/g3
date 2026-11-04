.PHONY: fast tags slow

fast:
	time go test -timeout=0 -count=1 ./...

tags:
	time go test -timeout=0 -count=1 -short -tags g3checked,g3debug,g3trace $$( go list ./... | grep -v math/polynomial | grep -v util/lp |grep -v pkg/qe)

slow:
	time go test -timeout=0 -count=1 -short -tags g3checked,g3debug,g3trace github.com/glycerine/g3/pkg/qe  github.com/glycerine/g3/pkg/math/polynomial   github.com/glycerine/g3/pkg/util/lp
