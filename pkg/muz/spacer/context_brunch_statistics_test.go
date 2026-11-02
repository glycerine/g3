package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/statistics"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerContextCollectStatisticsWritesSourceBrunchRecords(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	c.stats = contextStatistics{
		queries: 2, reachQueries: 3, reuseReach: 5,
		maxQueryLevel: 7, maxDepth: 11, queryLevelSet: true,
	}
	c.inductive = 13
	c.cexDepth, c.cexDepthSet = 17, true
	previous := StatisticsSnapshot{
		counters: contextStatistics{
			queries: 19, reachQueries: 23, reuseReach: 29,
			maxQueryLevel: 31, maxDepth: 37, queryLevelSet: true,
		},
		cexDepth: 41, cexDepthSet: true,
	}

	original, err := diagnostic.BorrowProcessVerboseStream()
	depthMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	depthMust(t, err)
	defer func() {
		depthMust(t, diagnostic.SetProcessVerboseStream(original))
		depthMust(t, stream.Close())
	}()
	depthMust(t, diagnostic.SetProcessVerboseStream(stream))

	collected := statistics.New()
	depthMust(t, c.CollectStatisticsWithSnapshot(collected, previous))
	got, err := stream.String()
	depthMust(t, err)
	want := "BRUNCH_STAT max_query_lvl 7\n" +
		"BRUNCH_STAT num_queries 21\n" +
		"BRUNCH_STAT num_reach_queries 26\n" +
		"BRUNCH_STAT num_reach_reuse 34\n" +
		"BRUNCH_STAT inductive_lvl 13\n" +
		"BRUNCH_STAT max_depth 37\n" +
		"BRUNCH_STAT cex_depth 17\n"
	if got != want {
		t.Fatalf("brunch statistics = %q, want %q", got, want)
	}
}
