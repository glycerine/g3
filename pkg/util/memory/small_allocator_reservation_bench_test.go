package memory

import "testing"

// Model a large live location ledger with a reusable slot near its start.
func BenchmarkSmallAllocatorReserveExistingLocation(b *testing.B) {
	a := &SmallAllocator{locations: make([]smallObjectLocation, 8192)}
	for i := 1; i < len(a.locations); i++ {
		a.locations[i].live = true
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := a.reserveLocations(1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSmallAllocatorInstallAfterLiveLocations(b *testing.B) {
	a := &SmallAllocator{locations: make([]smallObjectLocation, 8192, 8193)}
	for i := range a.locations {
		a.locations[i].live = true
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.installLocation(smallObjectLocation{live: true})
		a.locations = a.locations[:8192]
	}
}
