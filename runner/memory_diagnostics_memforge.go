//go:build memforge_debug

package runner

import "memforge"

func memoryDiagnosticsCollect() MemoryDiagnosticResult {
	snapshot := memforge.MemforgeMemorySnapshotGet()
	if !snapshot.Available {
		return MemoryDiagnosticResult{
			Provider: "memforge",
		}
	}

	return MemoryDiagnosticResult{
		Available:                true,
		Provider:                 "memforge",
		AllocatorTypeCount:       snapshot.AllocatorTypeCount,
		AllocatorCount:           snapshot.AllocatorCount,
		ActiveAllocatorCount:     snapshot.ActiveAllocatorCount,
		DestroyedAllocatorCount:  snapshot.DestroyedAllocatorCount,
		AllocatorsWithLiveAllocs: snapshot.AllocatorsWithLiveAllocs,
		TotalAllocationCount:     snapshot.TotalAllocationCount,
		LiveAllocationCount:      snapshot.LiveAllocationCount,
		TotalBytes:               snapshot.TotalBytes,
		LiveBytes:                snapshot.LiveBytes,
		LeakDetected:             snapshot.LeakDetected,
		Allocators:               memoryDiagnosticAllocatorsFromMemforge(snapshot.Allocators),
	}
}

func memoryDiagnosticAllocatorsFromMemforge(allocators []memforge.MemforgeAllocatorSnapshot) []MemoryDiagnosticAllocatorResult {
	out := make([]MemoryDiagnosticAllocatorResult, len(allocators))
	for i, allocator := range allocators {
		out[i] = MemoryDiagnosticAllocatorResult{
			Name:                  allocator.Name,
			Address:               allocator.Address,
			Destroyed:             allocator.Destroyed,
			CreatedAt:             allocator.CreatedAt,
			Creator:               allocator.Creator,
			TotalAllocations:      allocator.TotalAllocations,
			LiveAllocations:       allocator.LiveAllocations,
			TotalBytes:            allocator.TotalBytes,
			LiveBytes:             allocator.LiveBytes,
			Status:                allocator.Status,
			LiveAllocationDetails: memoryDiagnosticAllocationsFromMemforge(allocator.LiveAllocationDetails),
		}
	}
	return out
}

func memoryDiagnosticAllocationsFromMemforge(allocations []memforge.MemforgeAllocationSnapshot) []MemoryDiagnosticAllocationResult {
	out := make([]MemoryDiagnosticAllocationResult, len(allocations))
	for i, allocation := range allocations {
		out[i] = MemoryDiagnosticAllocationResult{
			Address:   allocation.Address,
			SizeBytes: allocation.SizeBytes,
			CreatedAt: allocation.CreatedAt,
			Creator:   allocation.Creator,
		}
	}
	return out
}
