package fp

import (
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (e *PropositionalEngine) snapshotSpacerStatistics() (spacer.StatisticsSnapshot, error) {
	var result spacer.StatisticsSnapshot
	result.Merge(e.spacerStatistics)
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "spacer" {
			if err := pending.captureStatistics(); err != nil {
				return result, err
			}
			result.Merge(pending.spacer)
		}
	}
	if e.spacer != nil {
		current, err := e.spacer.SnapshotStatistics()
		if err != nil {
			return result, err
		}
		result.Merge(current)
	}
	if e.queryClone != nil && e.queryClone.backend == "spacer" {
		current, err := e.queryClone.snapshotSpacerStatistics()
		if err != nil {
			return result, err
		}
		result.Merge(current)
	}
	return result, nil
}

func (e *PropositionalEngine) collectSpacerStatistics(destination *statistics.Statistics, previous spacer.StatisticsSnapshot) error {
	previous.Merge(e.spacerStatistics)
	for _, pending := range e.queryCleanup {
		if pending.clone.backend == "spacer" {
			if err := pending.captureStatistics(); err != nil {
				return err
			}
			previous.Merge(pending.spacer)
		}
	}
	if e.queryClone != nil && e.queryClone.backend == "spacer" {
		if e.spacer != nil {
			current, err := e.spacer.SnapshotStatistics()
			if err != nil {
				return err
			}
			previous.Merge(current)
		}
		return e.queryClone.collectSpacerStatistics(destination, previous)
	}
	if e.spacer != nil {
		return e.spacer.CollectStatisticsWithSnapshot(destination, previous)
	}
	return previous.Collect(destination)
}
